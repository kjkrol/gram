package gravity

import (
	"math"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/gravity/internal/vertical"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*fallSystem)(nil)

// fallSystem is the gravity of a world with heights, a step: the Leaps and Aways given are thrown,
// every body is carried along Z between the surface under its feet — read with its feet as they
// were before the step, so a fall fast as it may be lands on a deck it came over — and the slab
// over its head, and the Landing rules told of those landing.
type fallSystem struct {
	p        *Plugin
	tickOf   plugin.TickSource
	landings *plugin.Rules[Landing]

	pulls *goke.Query
	pull  goke.Comp[Pull]

	bodies *goke.Query
	base   goke.Comp[world.Base]
	body   goke.Comp[Body]
	drift  goke.Comp[world.Drift]
	marks  goke.Comp[tag.Tags[unit.States]]
	z      goke.OptComp[world.Z]
	mover  goke.OptComp[unit.Mover]

	whence

	// the chunk being walked, for the rules: who of it lands this step, how fast
	walking struct {
		ids    []uid.UID64
		speeds []float64
	}
	landingAt func(i int) Landing
	landsAt   func(i int) bool
}

func newFallSystem(p *Plugin) *fallSystem {
	s := &fallSystem{p: p, tickOf: p.worldPlugin.Tick, landings: &p.landings}
	s.landingAt = func(i int) Landing {
		return Landing{ID: s.walking.ids[i], Speed: s.walking.speeds[i]}
	}
	s.landsAt = func(i int) bool { return s.walking.speeds[i] >= 0 }
	return s
}

func (s *fallSystem) Init(si *goke.SysInit) {
	s.pulls = si.NewQueryBuilder(&s.pull).Build()
	qb := si.NewQueryBuilder(&s.base, &s.body, &s.drift, &s.marks).Optional(&s.z).Optional(&s.mover)
	s.landings.Bind(qb)
	s.bodies = qb.Build()
	s.whence.init(si)
}

func (s *fallSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	g := max(-pullOf(s.pulls, &s.pull).Z, 0)
	s.throw(g)
	var surfaces ground.Surfaces
	if s.p.surfaces != nil {
		surfaces = s.p.surfaces()
	}
	hosted := !s.landings.Empty()
	var t plugin.Tick
	if hosted {
		t = s.tickOf.Of(cb, d)
	}
	dt := d.Seconds()
	for s.bodies.All(); s.bodies.Next(); {
		cur := s.bodies.Cursor()
		zs := s.z.Slice(cur)
		if zs == nil {
			continue
		}
		bases, drifts, marks, movers := s.base.Slice(cur), s.drift.Slice(cur), s.marks.Slice(cur),
			s.mover.Slice(cur)
		s.walking.ids, s.walking.speeds = cur.IDs, s.walking.speeds[:0]
		for i := range cur.IDs {
			z := &zs[i]
			floor, ceiling := 0.0, math.Inf(1)
			if surfaces != nil {
				floor, ceiling = surfaces.Surface(bases[i].Pos.Center(), unit.DomainAt(movers, i),
					z.Altitude, z.Reach())
			}
			if drifts[i] == (world.Drift{}) && !marks[i].Has(unit.Airborne) &&
				z.Altitude-floor <= z.Reach() {
				z.Altitude = floor // standing, as most do: nothing else changes
				s.walking.speeds = append(s.walking.speeds, -1)
				continue
			}
			b := bodyOf(z, &drifts[i], marks[i])
			speed, landed := vertical.Step(&b, g, floor, ceiling, dt)
			put(&b, &zs[i], &drifts[i], &marks[i])
			if !landed {
				speed = -1
			}
			s.walking.speeds = append(s.walking.speeds, speed)
		}
		if hosted {
			s.landings.RunWhere(t, cur, s.landsAt, s.landingAt)
		}
	}
}

// throw carries out the Leaps and Aways given, each by the entity for itself: a pull of g throws
// it up.
func (s *fallSystem) throw(g float64) {
	s.p.leaps.Drain(func(i control.Issued[Leap]) {
		if !i.ByEntity || !s.bodies.Seek(i.Entity) {
			return
		}
		cur := s.bodies.Cursor()
		z := s.z.At(cur)
		if z == nil {
			return
		}
		b := bodyOf(z, s.drift.At(cur), *s.marks.At(cur))
		vertical.Leap(&b, g, s.body.At(cur).Leap)
		put(&b, z, s.drift.At(cur), s.marks.At(cur))
	})
	s.p.aways.Drain(func(i control.Issued[Away]) {
		if !i.ByEntity {
			return
		}
		way, ok := s.away(i.Entity, i.Command)
		if !ok || !s.bodies.Seek(i.Entity) {
			return
		}
		cur := s.bodies.Cursor()
		z := s.z.At(cur)
		if z == nil {
			return
		}
		b := bodyOf(z, s.drift.At(cur), *s.marks.At(cur))
		a := i.Command
		vertical.Throw(&b, g, way.X*a.Speed, way.Y*a.Speed, a.Rise)
		put(&b, z, s.drift.At(cur), s.marks.At(cur))
	})
}

// bodyOf is the body z, d and marks say, as a step takes it.
func bodyOf(z *world.Z, d *world.Drift, marks tag.Tags[unit.States]) vertical.Body {
	return vertical.Body{Feet: z.Altitude, Height: z.Height, Reach: z.Reach(),
		X: d.X, Y: d.Y, Rise: d.Z, Airborne: marks.Has(unit.Airborne)}
}

// put writes b back into z, d and marks.
func put(b *vertical.Body, z *world.Z, d *world.Drift, marks *tag.Tags[unit.States]) {
	z.Altitude = b.Feet
	*d = world.Drift{X: b.X, Y: b.Y, Z: b.Rise}
	if b.Airborne {
		*marks = marks.With(unit.Airborne)
	} else {
		*marks = marks.Without(unit.Airborne)
	}
}
