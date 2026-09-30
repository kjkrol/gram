package vision

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*ScanSystem)(nil)

// ScanSystem fills in what every Sight-carrying entity sees, and the outline of those that also
// carry SightOutline: observers enough at a time on several goroutines at once (Workers), each
// with a scanner of its own; then it runs the pair triggers over what each saw.
type ScanSystem struct {
	scanner // the system's own, for one observer at a time

	commands *control.Carrier // the world's, for the triggers

	// scanners are the system's own and one more a goroutine sharing a chunk's observers, at most
	// count of them: 0 as many as there are CPUs, 1 none
	scanners []*scanner
	count    int

	// In a world with heights the cone has heights: groundOf resolves the world's Ground at first
	// use, grounded once it has.
	heights  bool
	groundOf func() board.Heights
	step     float64
	grounded bool
	bend     float64

	// coverOf resolves the world's Cover at first use, covered once it has.
	coverOf func() board.Cover
	covered bool

	query   *goke.Query
	sight   goke.Comp[Sight]
	eye     goke.Comp[world.Eye]
	base    goke.Comp[world.Base]
	steer   goke.OptComp[steering.Steering]
	outline goke.OptComp[SightOutline]
	z       goke.OptComp[world.Z]

	// host runs the pair triggers registered with the plugin, inside this pass.
	host *host.PairHost[Sighting]

	jobs []job // the frame's chunks of observers

	// What the host is being run over: the observer in hand, everyone it sees, and their tags.
	observer   Sighting
	seen       []Seen
	seenTags   []plugin.Marks
	matched    []Seen
	sightingOf func(matched []int) Sighting
}

// scanner is what scanning one observer's cone works with: a view of its own, the cone's callbacks
// resolving what it meets through a lookup of its own, and the cover walked for the observer.
type scanner struct {
	space *aabbworld.Space
	view  aabbworld.View

	shadows []aabbworld.Shadow // the view's shadows, read into an outline

	// tau answers the cone how see-through an entity is to the observer in hand.
	tau func(uid.UID64) float64

	// In a world with heights the cone has heights: elev answers an entity's band, groundAt the
	// ground, step how far apart the ground is sampled.
	heights  bool
	elev     func(uid.UID64) (float64, float64)
	groundAt func(geom.Vec) float64
	step     float64
	// On a world with a Scale the ground and what stands on it sink under the observer's level
	// bend·d², d how far off: sunk is the ground so, for the observer at ox, oy.
	bend   float64
	sunk   func(geom.Vec) float64
	ox, oy float64

	// covering walks the world's Cover for the observer in hand and holds its Blockers.
	covering covering

	// lookup resolves a sighted id back to the entity and what it carries.
	lookup     *goke.Query
	lookupBase goke.Comp[world.Base]
	lookupTau  goke.OptComp[Transparency]
	lookupLay  goke.OptComp[world.Layers]
	lookupZ    goke.OptComp[world.Z]
	lookupHot  bool
}

// observersPerWorker is the fewest observers worth a goroutine of their own.
const observersPerWorker = 8

// The two queries the scan offers its hosted triggers, by index.
const (
	walked = iota // the observer, a chunk at a time
	sought        // what it sees, one entity at a time
)

func NewScanSystem(space *aabbworld.Space) *ScanSystem {
	return newScanSystem(space, &host.PairHost[Sighting]{})
}

func newScanSystem(space *aabbworld.Space, host *host.PairHost[Sighting]) *ScanSystem {
	s := &ScanSystem{host: host}
	s.scanner.bind(space)
	s.sightingOf = s.sighting
	return s
}

// bind makes the scanner one of space's, answering the cone itself.
func (c *scanner) bind(space *aabbworld.Space) {
	c.space = space
	c.tau = c.transparency
	c.elev = c.elevation
}

// Workers sets how many goroutines at most share a chunk's observers: 0 as many as there are CPUs,
// 1 none. Call it before Init.
func (s *ScanSystem) Workers(n int) { s.count = max(n, 0) }

func (s *ScanSystem) Init(si *goke.SysInit) {
	walk := si.NewQueryBuilder(&s.sight, &s.eye, &s.base).Optional(&s.outline, &s.steer, &s.z)
	seek := si.NewQueryBuilder(&s.lookupBase).Optional(&s.lookupTau, &s.lookupLay, &s.lookupZ)
	s.host.Bind(walk, seek)
	s.query, s.lookup = walk.Build(), seek.Build()
	s.scanners = append(s.scanners[:0], &s.scanner)
	for range parallel.Workers(1<<30, 1, s.count) - 1 {
		w := &scanner{heights: s.heights, step: s.step, bend: s.bend}
		w.bind(s.space)
		w.covering.bend = s.covering.bend
		w.lookup = si.NewQueryBuilder(&w.lookupBase).Optional(&w.lookupTau, &w.lookupLay, &w.lookupZ).Build()
		s.scanners = append(s.scanners, w)
	}
}

// transparency is how see-through id is to the observer in hand: as empty on none of its
// Blockers, else its Transparency, 0 without one.
func (s *scanner) transparency(id uid.UID64) float64 {
	if !s.lookup.Seek(id) {
		return 0
	}
	s.lookupHot = false
	cur := s.lookup.Cursor()
	if !world.LayersOf(s.lookupLay.At(cur)).Meets(s.covering.blockers) {
		return 1
	}
	if t := s.lookupTau.At(cur); t != nil {
		return t.Value
	}
	return 0
}

// elevation is the band id spans in height: its Z, or the ground level at no height without one.
func (s *scanner) elevation(id uid.UID64) (bottom, top float64) {
	if !s.lookup.Seek(id) {
		return 0, 0
	}
	s.lookupHot = false
	cur := s.lookup.Cursor()
	sink := 0.0
	if s.bend > 0 { // sunk under the observer's level as far off as it stands
		c := s.lookupBase.At(cur).Pos.AABB
		dx, dy := (c.TopLeft.X+c.BottomRight.X)/2-s.ox, (c.TopLeft.Y+c.BottomRight.Y)/2-s.oy
		sink = s.bend * (dx*dx + dy*dy)
	}
	if z := s.lookupZ.At(cur); z != nil {
		return z.Altitude - sink, z.Top() - sink
	}
	return -sink, -sink
}

// ground binds the world's Ground once, when the board has had its say, to every scanner.
func (s *ScanSystem) ground() {
	s.grounded = true
	if s.groundOf == nil {
		return
	}
	g := s.groundOf()
	for _, c := range s.scanners {
		if g != nil {
			c.groundAt = g.At
			if c.step == 0 {
				c.step = g.Step()
			}
		}
		if c.bend > 0 {
			c.sunk = c.sunkGround
		}
	}
}

// cover binds the world's Cover once to every scanner.
func (s *ScanSystem) cover() {
	s.covered = true
	if s.coverOf == nil {
		return
	}
	cover := s.coverOf()
	for _, c := range s.scanners {
		c.covering.cover = cover
	}
}

// sunkGround is the ground at p as the observer in hand sees it: sunk under its level as far off
// as p lies.
func (s *scanner) sunkGround(p geom.Vec) float64 {
	g := 0.0
	if s.groundAt != nil {
		g = s.groundAt(p)
	}
	dx, dy := p.X-s.ox, p.Y-s.oy
	return g - s.bend*(dx*dx+dy*dy)
}

func (s *ScanSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	t := plugin.Tick{CmdBuf: cb, Now: time.Now(), Dt: d, Commands: s.commands}
	hosting := !s.host.Empty()
	s.lookupHot = false
	if s.heights && !s.grounded {
		s.ground()
	}
	if !s.covered {
		s.cover()
	}

	// every chunk's observers, counted: shared out among the scanners when they are many — a chunk
	// of observers with outlines holds a few, so the runs cut across chunks
	s.jobs = s.jobs[:0]
	n := 0
	s.query.All()
	for s.query.Next() {
		j := s.job(s.query.Cursor())
		j.first = n
		n += len(j.ids)
		s.jobs = append(s.jobs, j)
	}
	k := parallel.Workers(n, observersPerWorker, len(s.scanners))
	if k > 1 {
		s.settle(s.jobs[0].bases[0].Pos.AABB.AABB)
		parallel.Run(k, n, func(w, from, to int) {
			for _, j := range s.jobs {
				for i := max(from, j.first); i < min(to, j.first+len(j.ids)); i++ {
					j.scan(s.scanners[w], i-j.first)
				}
			}
		})
	}

	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		j := s.job(cursor)
		steers := s.steer.Slice(cursor)
		for i, id := range cursor.IDs {
			if k <= 1 {
				j.scan(&s.scanner, i)
			}
			if hosting {
				sight := &j.sights[i]
				s.observer = Sighting{Self: id, Base: &j.bases[i], Sight: sight}
				if i < len(steers) {
					s.observer.Steering = &steers[i]
				}
				s.gather(&sight.Seen)
				s.host.DispatchGrouped(t, s.host.InChunk(walked, cursor, i), s.seenTags, s.sightingOf)
			}
		}
	}
}

// job is one chunk of observers as the scanners take them: what each carries, by index, and where
// the chunk's first stands among all the frame's observers.
type job struct {
	ids      []uid.UID64
	sights   []Sight
	eyes     []world.Eye
	bases    []world.Base
	zs       []world.Z
	outlines []SightOutline
	first    int
}

// job is the chunk under cursor as a job.
func (s *ScanSystem) job(cursor *goke.Cursor) job {
	j := job{ids: cursor.IDs, sights: s.sight.Slice(cursor), eyes: s.eye.Slice(cursor), bases: s.base.Slice(cursor), zs: s.z.Slice(cursor)}
	if s.outline.Present(cursor) {
		j.outlines = s.outline.Slice(cursor)
	}
	return j
}

// scan has c scan the job's i-th observer.
func (j *job) scan(c *scanner, i int) {
	var z world.Z
	if j.zs != nil {
		z = j.zs[i]
	}
	var outline *SightOutline
	if j.outlines != nil {
		outline = &j.outlines[i]
	}
	c.scan(j.ids[i], &j.sights[i], &j.bases[i], j.eyes[i], z, outline)
}

// settle reads once, here, what the scanners are about to read together: the space's index and
// the cover, which read the world as they are first asked and would else do so all at once.
func (s *ScanSystem) settle(box geom.AABB) {
	s.space.Query(box, aabbworld.AnyCapability, func(uid.UID64) {})
	if r, ok := s.covering.cover.(board.Readied); ok {
		r.Ready()
	}
}

// scan fills in what the observer id sees — its Seen, and its outline where it has one — from its
// base, the eye it sees with and its height.
func (c *scanner) scan(id uid.UID64, sight *Sight, base *world.Base, eye world.Eye, z world.Z, outline *SightOutline) {
	c.covering.blockers = sight.Blockers
	box := base.Pos.AABB
	c.ox, c.oy = (box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2
	c.covering.ox, c.covering.oy = c.ox, c.oy
	if c.space.Scan(id, c.cone(sight, eye, z), &c.view) {
		record(&sight.Seen, &c.view)
		if outline != nil {
			c.trace(outline, sight, eye.Angle/2)
		}
	} else {
		sight.Seen.Count = 0
	}
}

// gather looks up everyone in found, keeping who is still there and the tags each carries.
func (s *ScanSystem) gather(found *Sighted) {
	s.seen, s.seenTags = s.seen[:0], s.seenTags[:0]
	for k := range int(found.Count) {
		id := found.IDs[k]
		ok := s.lookupHot && s.lookup.SeekH(id)
		if !ok {
			ok = s.lookup.Seek(id)
			s.lookupHot = ok
		}
		if !ok {
			continue
		}
		cursor := s.lookup.Cursor()
		tags := s.host.At(sought, cursor)
		s.seen = append(s.seen, Seen{ID: id, Base: s.lookupBase.At(cursor), Dist: found.Dists[k], Marks: tags})
		s.seenTags = append(s.seenTags, tags)
	}
}

// sighting is the observer in hand, seeing just the entities a trigger asked for.
func (s *ScanSystem) sighting(matched []int) Sighting {
	s.matched = s.matched[:0]
	for _, k := range matched {
		s.matched = append(s.matched, s.seen[k])
	}
	out := s.observer
	out.Seen = s.matched
	return out
}

// cone is the query for one Sight, as wide as its eye sees, see-through as the entities are to
// it and, in a world with heights, from the eye's level over the ground: its Height above the
// entity's bottom, its top for none.
func (s *scanner) cone(sight *Sight, eye world.Eye, z world.Z) aabbworld.Cone {
	c := aabbworld.Cone{Direction: sight.Facing, HalfAngle: eye.Angle / 2, Radius: sight.Radius, Transparency: s.tau}
	if s.covering.cover != nil {
		c.Cover = &s.covering
	}
	if !s.heights {
		if eye.Height != 0 {
			panic("vision: Eye.Height in a flat world; set world.Config.Heights")
		}
		return c
	}
	if sight.Blockers != 0 {
		panic("vision: Sight.Blockers in a world with heights; layers cut sight only in a flat one")
	}
	c.Eye, c.Elevation, c.Ground, c.GroundStep = eye.Level(z), s.elev, s.groundAt, s.step
	if s.sunk != nil {
		c.Ground = s.sunk
	}
	return c
}

// covering is the world's Cover as the cone asks for it: walked for one observer's Blockers.
type covering struct {
	cover    board.Cover
	blockers world.Layers
	// bend sinks the cover under the observer's level at ox, oy as far off as it stands: visit is
	// the walk's own, sunk the step handed to the cover in its place
	bend   float64
	ox, oy float64
	visit  func(near, far, bottom, top, tau float64) bool
	sunk   func(near, far, bottom, top, tau float64) bool
}

func (c *covering) Walk(origin, dir geom.Vec, length float64, visit func(near, far, bottom, top, tau float64) bool) {
	if c.bend <= 0 {
		c.cover.Walk(origin, dir, length, c.blockers, visit)
		return
	}
	if c.sunk == nil {
		c.sunk = c.sink
	}
	c.visit = visit
	c.cover.Walk(origin, dir, length, c.blockers, c.sunk)
}

// sink hands the walk's visit a stretch of cover sunk as far off as its middle lies.
func (c *covering) sink(near, far, bottom, top, tau float64) bool {
	m := (near + far) / 2
	d := c.bend * m * m
	return c.visit(near, far, bottom-d, top-d, tau)
}

// record keeps the nearest MaxSeen entities of view.
func record(dst *Sighted, view *aabbworld.View) {
	dst.Count = 0
	view.Entities(func(id uid.UID64, dist float64) {
		if dst.Count == MaxSeen {
			return
		}
		dst.IDs[dst.Count] = id
		dst.Dists[dst.Count] = float32(dist)
		dst.Count++
	})
}

// trace samples the cone at the resolution its reach and width call for, within the buffer. In a
// world with heights the view reaches its full Radius and the ground out of sight is kept as
// shadows; only the samples read (Count) are written, the buffer beyond them is left as it was.
func (s *scanner) trace(dst *SightOutline, sight *Sight, half float64) {
	k := samplesFor(sight, half)
	clear(dst.Shadows[:k])
	if !s.heights {
		dst.Count = uint8(len(s.view.Depths(k, dst.Depths[:0])))
		return
	}
	dst.Count = uint8(k)
	for i := range k {
		dst.Depths[i] = float32(sight.Radius)
	}
	s.shadows = s.view.Shadows(k, s.shadows[:0])
	for _, sh := range s.shadows {
		bands := &dst.Shadows[sh.Sample]
		for j := range bands {
			if bands[j] == (Band{}) {
				bands[j] = Band{From: sh.From, To: sh.To}
				break
			}
		}
	}
}

// samplesFor is how many samples keep the reach within EdgeTolerance at full range.
func samplesFor(s *Sight, half float64) int {
	k := int(math.Ceil(2*half*s.Radius/EdgeTolerance)) + 1
	return min(max(k, 2), MaxSamples)
}
