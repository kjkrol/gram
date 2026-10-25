package vision

import (
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*ScanSystem)(nil)

// ScanSystem fills in what every observer sees, and the outline of those carrying SightOutline,
// then runs the rules of a Sighting over what each saw.
type ScanSystem struct {
	scanner

	pace goke.Opt

	tick host.TickSource

	log  *log.Logger
	told map[[2]uid.UID64]bool

	scanners []*scanner
	count    int
	pool     *parallel.Pool
	scanPart func(w, i int) // scanParted, bound once
	n, parts int

	every float64
	steps uint64
	due   func(id uid.UID64) bool // isDue, bound once
	round uint64

	heights  bool
	groundOf func() ground.Heights
	step     float32
	grounded bool
	bend     float32

	coverOf func(w int) ground.Cover // worker w's
	covered bool

	query     *goke.Query
	sight     goke.Comp[Sight]
	sighted   goke.OptComp[Sighted]
	sightedID goke.CompID
	eye       goke.Comp[world.Eye]
	base      goke.Comp[world.Base]
	outline   goke.OptComp[SightOutline]
	z         goke.OptComp[world.Z]

	host *host.PairRules[Sighting]

	walkCols, seekCols *cgoke.Columns

	jobs []job

	observer   Sighting
	seen       []Seen
	seenTags   []host.Marks
	matched    []Seen
	sightingOf func(matched []int) Sighting
}

type scanner struct {
	space *aabbworld.SpaceRead
	view  aabbworld.View

	shadows []aabbworld.Shadow

	tau func(uid.UID64) float32

	heights   bool
	elev      func(uid.UID64) (float32, float32)
	groundAt  func(geom.Vec) float32
	step      float32
	profiling profiling
	bend      float32 // the ground sinks bend·d² under the observer's level, d how far off
	sunk      func(geom.Vec) float32
	ox, oy    float32

	covering covering

	lookup     *goke.Query
	lookupBase goke.Comp[world.Base]
	lookupTau  goke.OptComp[Transparency]
	lookupLay  goke.OptComp[world.Layers]
	lookupZ    goke.OptComp[world.Z]
	lookupHot  bool
}

// the fewest observers worth a goroutine; with heights every ray reads the ground, so fewer are
const (
	observersPerWorker = 8
	elevatedPerWorker  = 2
)

// the two queries the scan offers its hosted rules, by index
const (
	walked = iota
	sought
)

func NewScanSystem(space *aabbworld.SpaceRead) *ScanSystem {
	return newScanSystem(space, &host.PairRules[Sighting]{})
}

// Paced has the system scan each block of observers as pace, an option of its query, says; set
// before Init.
func (s *ScanSystem) Paced(pace goke.Opt) *ScanSystem {
	s.pace = pace
	return s
}

func newScanSystem(space *aabbworld.SpaceRead, host *host.PairRules[Sighting]) *ScanSystem {
	s := &ScanSystem{host: host}
	s.scanner.bind(space)
	s.sightingOf = s.sighting
	s.due = s.isDue
	s.scanPart = s.scanParted
	return s
}

func (c *scanner) bind(space *aabbworld.SpaceRead) {
	c.space = space
	c.tau = c.transparency
	c.elev = c.elevation
}

// Workers sets how many goroutines of the pool at most share a scan: 0 all, 1 none. Call it
// before Init.
func (s *ScanSystem) Workers(n int) { s.count = max(n, 0) }

// Pool sets the goroutines a scan is shared among; nil, one. Call it before Init.
func (s *ScanSystem) Pool(p *parallel.Pool) { s.pool = p }

// Every has each observer scanned perSecond times a second, 0 every step. Call it before Init.
func (s *ScanSystem) Every(perSecond float64) { s.every = max(perSecond, 0) }

func (s *ScanSystem) isDue(id uid.UID64) bool {
	return s.round <= 1 || uint64(id.Index())%s.round == s.steps%s.round
}

func (s *ScanSystem) Init(si *goke.SysInit) {
	walk := si.NewQueryBuilder(&s.sight, &s.eye, &s.base).Optional(&s.sighted, &s.outline, &s.z)
	seek := si.NewQueryBuilder(&s.lookupBase).Optional(&s.lookupTau, &s.lookupLay, &s.lookupZ)
	s.walkCols, s.seekCols = cgoke.NewColumns(walk), cgoke.NewColumns(seek)
	s.host.Bind(s.walkCols, s.seekCols)
	if s.pace != nil {
		walk.With(s.pace)
	}
	s.query, s.lookup = walk.Build(), seek.Build()
	s.sightedID = si.RegComp[Sighted]()
	s.scanners = append(s.scanners[:0], &s.scanner)
	if s.count == 1 {
		return
	}
	for range s.pool.Size() - 1 {
		w := &scanner{heights: s.heights, step: s.step, bend: s.bend}
		w.profiling.bend = s.bend
		w.bind(s.space)
		w.covering.bend = s.covering.bend
		w.lookup = si.NewQueryBuilder(&w.lookupBase).Optional(&w.lookupTau, &w.lookupLay, &w.lookupZ).Build()
		s.scanners = append(s.scanners, w)
	}
}

func (s *scanner) transparency(id uid.UID64) float32 {
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

func (s *scanner) elevation(id uid.UID64) (bottom, top float32) {
	if !s.lookup.Seek(id) {
		return 0, 0
	}
	s.lookupHot = false
	cur := s.lookup.Cursor()
	sink := float32(0)
	if s.bend > 0 {
		c := s.lookupBase.At(cur).Pos.AABB
		dx, dy := (c.TopLeft.X+c.BottomRight.X)/2-s.ox, (c.TopLeft.Y+c.BottomRight.Y)/2-s.oy
		sink = s.bend * (dx*dx + dy*dy)
	}
	if z := s.lookupZ.At(cur); z != nil {
		return z.Altitude - sink, z.Top() - sink
	}
	return -sink, -sink
}

func (s *ScanSystem) ground() {
	s.grounded = true
	if s.groundOf == nil {
		return
	}
	g := s.groundOf()
	profile, _ := g.(ground.Profile)
	for _, c := range s.scanners {
		if g != nil {
			c.groundAt = g.At
			if c.step == 0 {
				c.step = g.Step()
			}
		}
		c.profiling.profile = profile
		if c.bend > 0 {
			c.sunk = c.sunkGround
		}
	}
}

func (s *ScanSystem) cover() {
	s.covered = true
	if s.coverOf == nil {
		return
	}
	for w, c := range s.scanners {
		c.covering.cover = s.coverOf(w)
	}
}

func (s *scanner) sunkGround(p geom.Vec) float32 {
	g := float32(0)
	if s.groundAt != nil {
		g = s.groundAt(p)
	}
	dx, dy := p.X-s.ox, p.Y-s.oy
	return g - s.bend*(dx*dx+dy*dy)
}

func (s *ScanSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	s.round = 1
	if s.every > 0 && d > 0 {
		s.round = uint64(max(math.Round(1/(s.every*d.Seconds())), 1))
	}
	defer func() { s.steps++ }()
	t := s.tick.Of(d)
	hosting := !s.host.Empty()
	s.lookupHot = false
	if s.heights && !s.grounded {
		s.ground()
	}
	if !s.covered {
		s.cover()
	}

	// the runs cut across chunks: a chunk of observers with outlines holds few
	s.jobs = s.jobs[:0]
	n, due := 0, 0
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		j := s.job(cursor)
		if j.seens == nil {
			for _, id := range j.ids {
				cb.AddOne(id, s.sightedID, Sighted{})
			}
			continue
		}
		if !s.query.Due() {
			forget(j.seens)
			continue
		}
		j.first = n
		n += len(j.ids)
		for _, id := range j.ids {
			if s.due(id) {
				due++
			}
		}
		s.jobs = append(s.jobs, j)
	}
	least := observersPerWorker
	if s.heights {
		least = elevatedPerWorker
	}
	k := s.pool.Parts(due, least, s.count)
	if k > 1 {
		s.space.Ready()
		s.n, s.parts = n, k
		b := s.pool.Begin(s.scanPart, nil)
		b.Add(k)
		b.Wait()
	}

	for s.query.Paced(d); s.query.Next(); {
		cursor := s.query.Cursor()
		j := s.job(cursor)
		if j.seens == nil {
			continue
		}
		for i, id := range cursor.IDs {
			if k <= 1 && s.due(id) {
				j.scan(&s.scanner, i)
			}
			if s.log != nil {
				s.logSightings(id, &j.seens[i])
			}
			if hosting {
				sight := &j.sights[i]
				s.observer = Sighting{Self: id, Base: &j.bases[i], Sight: sight}
				s.gather(&j.seens[i])
				s.host.DispatchGrouped(t, s.host.Read(walked, s.walkCols.Row(cursor), i), s.seenTags, s.sightingOf)
			}
		}
	}
}

// scanParted scans the due observers of a part on worker w's scanner
func (s *ScanSystem) scanParted(w, part int) {
	from, to := parallel.Part(s.n, s.parts, part)
	for _, j := range s.jobs {
		for i := max(from, j.first); i < min(to, j.first+len(j.ids)); i++ {
			if s.due(j.ids[i-j.first]) {
				j.scan(s.scanners[w], i-j.first)
			}
		}
	}
}

// an observer not looking sees nothing, so no rule acts on what it saw last
func forget(seens []Sighted) {
	for i := range seens {
		if seens[i].Count != 0 {
			seens[i] = Sighted{}
		}
	}
}

type job struct {
	ids      []uid.UID64
	sights   []Sight
	seens    []Sighted
	eyes     []world.Eye
	bases    []world.Base
	zs       []world.Z
	outlines []SightOutline
	first    int
}

func (s *ScanSystem) job(cursor *goke.Cursor) job {
	j := job{ids: cursor.IDs, sights: s.sight.Slice(cursor), seens: s.sighted.Slice(cursor), eyes: s.eye.Slice(cursor), bases: s.base.Slice(cursor), zs: s.z.Slice(cursor)}
	if s.outline.Present(cursor) {
		j.outlines = s.outline.Slice(cursor)
	}
	return j
}

func (j *job) scan(c *scanner, i int) {
	var z world.Z
	if j.zs != nil {
		z = j.zs[i]
	}
	var outline *SightOutline
	if j.outlines != nil {
		outline = &j.outlines[i]
	}
	c.scan(j.ids[i], &j.sights[i], &j.seens[i], &j.bases[i], j.eyes[i], z, outline)
}

func (c *scanner) scan(id uid.UID64, s *Sight, seen *Sighted, base *world.Base, eye world.Eye, z world.Z, outline *SightOutline) {
	look := *s
	look.Facing = s.Looking(base.Vel.Dir)
	sight := &look
	c.covering.blockers = sight.Blockers
	box := base.Pos.AABB
	c.ox, c.oy = (box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2
	c.covering.ox, c.covering.oy = c.ox, c.oy
	if c.space.Scan(id, c.cone(sight, eye, z), &c.view) {
		record(seen, &c.view, c.vertical(eye, z))
		if outline != nil {
			c.trace(outline, sight, eye.Angle/2)
		}
	} else {
		seen.Count = 0
	}
}

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
		tags := s.host.Read(sought, s.seekCols.Row(cursor), int(cursor.Slot))
		s.seen = append(s.seen, Seen{ID: id, Base: s.lookupBase.At(cursor), Dist: found.Dists[k]})
		s.seenTags = append(s.seenTags, tags)
	}
}

func (s *ScanSystem) sighting(matched []int) Sighting {
	s.matched = s.matched[:0]
	for _, k := range matched {
		s.matched = append(s.matched, s.seen[k])
	}
	out := s.observer
	out.Seen = s.matched
	return out
}

func (s *scanner) cone(sight *Sight, eye world.Eye, z world.Z) aabbworld.Cone {
	c := aabbworld.Cone{Direction: sight.Facing, HalfAngle: eye.Angle / 2, Radius: sight.Radius, Transparency: s.tau}
	if s.covering.cover != nil {
		c.Cover = &s.covering
	}
	if !s.heights {
		if eye.Height != 0 {
			panic("vision: Eye.Height in a flat world; set world.Config.Heights")
		}
		if eye.Vertical != 0 {
			panic("vision: Eye.Vertical in a flat world; set world.Config.Heights")
		}
		return c
	}
	c.Eye, c.Elevation, c.Ground, c.GroundStep = eye.Level(z), s.elev, s.groundAt, s.step
	if s.sunk != nil {
		c.Ground = s.sunk
	}
	if s.profiling.profile != nil {
		c.Profile = &s.profiling
	}
	return c
}

type profiling struct {
	profile ground.Profile
	bend    float32
	visit   func(d, alt float32) bool
	sunk    func(d, alt float32) bool
}

func (p *profiling) Along(origin, dir geom.Vec, length float32, visit func(d, alt float32) bool) {
	if p.bend <= 0 {
		p.profile.Along(origin, dir, length, visit)
		return
	}
	if p.sunk == nil {
		p.sunk = p.sink
	}
	p.visit = visit
	p.profile.Along(origin, dir, length, p.sunk)
}

func (p *profiling) sink(d, alt float32) bool { return p.visit(d, alt-p.bend*d*d) }

type covering struct {
	cover    ground.Cover
	blockers world.Layers
	bend     float32
	ox, oy   float32
	visit    func(near, far, bottom, top, tau float32) bool
	sunk     func(near, far, bottom, top, tau float32) bool
}

func (c *covering) Walk(origin, dir geom.Vec, length float32, visit func(near, far, bottom, top, tau float32) bool) {
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

func (c *covering) sink(near, far, bottom, top, tau float32) bool {
	m := (near + far) / 2
	d := c.bend * m * m
	return c.visit(near, far, bottom-d, top-d, tau)
}

// the vertical field of an eye: whether what is seen at a distance lies up or down within it;
// nil for an eye seeing every height
func (c *scanner) vertical(eye world.Eye, z world.Z) func(uid.UID64, float32) bool {
	if eye.Vertical <= 0 {
		return nil
	}
	level := eye.Level(z)
	return func(id uid.UID64, dist float32) bool {
		bottom, top := c.elev(id)
		return eye.Within(f32.Atan2(bottom-level, dist), f32.Atan2(top-level, dist))
	}
}

func record(dst *Sighted, view *aabbworld.View, within func(uid.UID64, float32) bool) {
	dst.Count = 0
	view.Entities(func(id uid.UID64, dist float32) {
		if dst.Count == MaxSeen || (within != nil && !within(id, dist)) {
			return
		}
		dst.IDs[dst.Count] = id
		dst.Dists[dst.Count] = dist
		dst.Count++
	})
}

func (s *scanner) trace(dst *SightOutline, sight *Sight, half float32) {
	k := samplesFor(sight, half)
	clear(dst.Shadows[:k])
	if !s.heights {
		dst.Count = uint8(len(s.view.Depths(k, dst.Depths[:0])))
		return
	}
	dst.Count = uint8(k)
	for i := range k {
		dst.Depths[i] = sight.Radius
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

func samplesFor(s *Sight, half float32) int {
	k := int(f32.Ceil(2*half*s.Radius/EdgeTolerance)) + 1
	return min(max(k, 2), MaxSamples)
}

func (s *ScanSystem) logSightings(id uid.UID64, seen *Sighted) {
	if s.told == nil {
		s.told = map[[2]uid.UID64]bool{}
	}
	for k := range int(seen.Count) {
		if pair := [2]uid.UID64{id, seen.IDs[k]}; !s.told[pair] {
			s.told[pair] = true
			s.log.Printf("entity %d sees entity %d at %.0f", id, seen.IDs[k], seen.Dists[k])
		}
	}
}
