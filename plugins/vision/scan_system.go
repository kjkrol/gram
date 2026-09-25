package vision

import (
	"github.com/kjkrol/gram/plugin/host"
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*ScanSystem)(nil)

// ScanSystem fills in what every Sight-carrying entity sees,
// and the outline of those that also carry SightOutline.
type ScanSystem struct {
	space *aabbworld.Space
	view  aabbworld.View // one for the whole system — see Update

	shadows []aabbworld.Shadow // the view's shadows, read into an outline

	// tau answers the cone how see-through an entity is to the observer in hand, blockers its Blockers.
	tau      func(uid.UID64) float64
	blockers world.Layers

	// In a Quasi3D world the cone has heights: elev answers an entity's band, groundAt the ground,
	// step how far apart the ground is sampled; groundOf resolves the world's Ground at first use.
	quasi3D  bool
	elev     func(uid.UID64) (float64, float64)
	groundOf func() world.Ground
	groundAt func(geom.Vec) float64
	step     float64
	grounded bool

	query   *goke.Query
	sight   goke.Comp[Sight]
	base    goke.Comp[world.Base]
	steer   goke.OptComp[world.Steering]
	outline goke.OptComp[SightOutline]
	z       goke.OptComp[world.Z]

	// lookup resolves a sighted id back to the entity and what it carries.
	lookup     *goke.Query
	lookupBase goke.Comp[world.Base]
	lookupTau  goke.OptComp[Transparency]
	lookupLay  goke.OptComp[world.Layers]
	lookupZ    goke.OptComp[world.Z]
	lookupHot  bool

	// host runs the Between behaviors registered with the plugin, inside this pass.
	host *host.PairHost[Sighting]

	// What the host is being run over: the observer in hand, everyone it sees, and their tags.
	observer   Sighting
	seen       []Seen
	seenTags   []plugin.Marks
	matched    []Seen
	sightingOf func(matched []int) Sighting
}

// The two queries the scan offers its hosted behaviors, by index.
const (
	walked = iota // the observer, a chunk at a time
	sought        // what it sees, one entity at a time
)

func NewScanSystem(space *aabbworld.Space) *ScanSystem {
	return newScanSystem(space, &host.PairHost[Sighting]{})
}

func newScanSystem(space *aabbworld.Space, host *host.PairHost[Sighting]) *ScanSystem {
	s := &ScanSystem{space: space, host: host}
	s.sightingOf = s.sighting
	s.tau = s.transparency
	s.elev = s.elevation
	return s
}

func (s *ScanSystem) Init(si *goke.SysInit) {
	walk := si.NewQueryBuilder(&s.sight, &s.base).Optional(&s.outline, &s.steer, &s.z)
	seek := si.NewQueryBuilder(&s.lookupBase).Optional(&s.lookupTau, &s.lookupLay, &s.lookupZ)
	s.host.Bind(walk, seek)
	s.query, s.lookup = walk.Build(), seek.Build()
}

// transparency is how see-through id is to the observer in hand: as empty on none of its
// Blockers, else its Transparency, 0 without one.
func (s *ScanSystem) transparency(id uid.UID64) float64 {
	if !s.lookup.Seek(id) {
		return 0
	}
	s.lookupHot = false
	cur := s.lookup.Cursor()
	if !world.LayersOf(s.lookupLay.At(cur)).Meets(s.blockers) {
		return 1
	}
	if t := s.lookupTau.At(cur); t != nil {
		return t.Value
	}
	return 0
}

// elevation is the band id spans in height: its Z, or the ground level at no height without one.
func (s *ScanSystem) elevation(id uid.UID64) (bottom, top float64) {
	if !s.lookup.Seek(id) {
		return 0, 0
	}
	s.lookupHot = false
	if z := s.lookupZ.At(s.lookup.Cursor()); z != nil {
		return z.Altitude, z.Top()
	}
	return 0, 0
}

// ground binds the world's Ground once, when the board has had its say.
func (s *ScanSystem) ground() {
	s.grounded = true
	if s.groundOf == nil {
		return
	}
	if g := s.groundOf(); g != nil {
		s.groundAt = g.At
		if s.step == 0 {
			s.step = g.Step()
		}
	}
}

func (s *ScanSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	t := plugin.Tick{CmdBuf: cb, Now: time.Now(), Dt: d}
	hosting := !s.host.Empty()
	s.lookupHot = false
	if s.quasi3D && !s.grounded {
		s.ground()
	}

	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		sights := s.sight.Slice(cursor)
		bases := s.base.Slice(cursor)
		steers := s.steer.Slice(cursor)
		zs := s.z.Slice(cursor)

		var outlines []SightOutline
		if s.outline.Present(cursor) {
			outlines = s.outline.Slice(cursor)
		}

		for i, id := range cursor.IDs {
			sight := &sights[i]
			s.blockers = sight.Blockers
			altitude := 0.0
			if zs != nil {
				altitude = zs[i].Altitude
			}
			if s.space.Scan(id, s.cone(sight, altitude), &s.view) {
				record(&sight.Seen, &s.view)
				if outlines != nil {
					s.trace(&outlines[i], sight)
				}
			} else {
				sight.Seen.Count = 0
			}
			if hosting {
				s.observer = Sighting{Self: id, Base: &bases[i], Sight: sight}
				if i < len(steers) {
					s.observer.Steering = &steers[i]
				}
				s.gather(&sight.Seen)
				s.host.DispatchGrouped(t, s.host.InChunk(walked, cursor, i), s.seenTags, s.sightingOf)
			}
		}
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

// sighting is the observer in hand, seeing just the entities a behavior asked for.
func (s *ScanSystem) sighting(matched []int) Sighting {
	s.matched = s.matched[:0]
	for _, k := range matched {
		s.matched = append(s.matched, s.seen[k])
	}
	out := s.observer
	out.Seen = s.matched
	return out
}

// cone is the query for one Sight, see-through as the entities are to it and, in a Quasi3D world,
// from an eye at altitude + Sight.Eye over the ground.
func (s *ScanSystem) cone(sight *Sight, altitude float64) aabbworld.Cone {
	c := aabbworld.Cone{Direction: sight.Facing, HalfAngle: sight.HalfAngle, Radius: sight.Radius, Transparency: s.tau}
	if !s.quasi3D {
		if sight.Eye != 0 {
			panic("vision: Sight.Eye in a flat world; set world.Config.Quasi3D")
		}
		return c
	}
	if sight.Blockers != 0 {
		panic("vision: Sight.Blockers in a Quasi3D world; layers cut sight only in a flat one")
	}
	c.Eye, c.Elevation, c.Ground, c.GroundStep = altitude+sight.Eye, s.elev, s.groundAt, s.step
	return c
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
// Quasi3D world the view reaches its full Radius and the ground out of sight is kept as shadows.
func (s *ScanSystem) trace(dst *SightOutline, sight *Sight) {
	k := samplesFor(sight)
	dst.Shadows = [MaxSamples][MaxShadowsPerSample]Band{}
	if !s.quasi3D {
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
func samplesFor(s *Sight) int {
	k := int(math.Ceil(2*s.HalfAngle*s.Radius/EdgeTolerance)) + 1
	return min(max(k, 2), MaxSamples)
}
