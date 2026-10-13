package gravity

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/gravity/internal/planar"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*planeSystem)(nil)

type planeSystem struct {
	p        *Plugin
	tickOf   host.TickSource
	landings *host.Rules[Landing]

	pulls *goke.Query
	pull  goke.Comp[Pull]

	bodies *goke.Query
	base   goke.Comp[world.Base]
	body   goke.Comp[Body]
	drift  goke.Comp[world.Drift]
	marks  goke.Comp[tag.Tags[unit.States]]
	coll   goke.OptComp[collision.Collider]

	whence

	normals []geom.Vec
	walking struct {
		ids    []uid.UID64
		speeds []float64
	}
	landingAt func(i int) Landing
	landsAt   func(i int) bool
}

func newPlaneSystem(p *Plugin) *planeSystem {
	s := &planeSystem{p: p, tickOf: p.worldPlugin.Tick, landings: &p.landings}
	s.landingAt = func(i int) Landing {
		return Landing{ID: s.walking.ids[i], Speed: s.walking.speeds[i]}
	}
	s.landsAt = func(i int) bool { return s.walking.speeds[i] >= 0 }
	return s
}

func (s *planeSystem) Init(si *goke.SysInit) {
	s.pulls = si.NewQueryBuilder(&s.pull).Build()
	qb := si.NewQueryBuilder(&s.base, &s.body, &s.drift, &s.marks).Optional(&s.coll)
	s.landings.Bind(qb)
	s.bodies = qb.Build()
	s.whence.init(si)
}

func (s *planeSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	pull := pullOf(s.pulls, &s.pull)
	g := geom.NewVec(pull.X, pull.Y)
	s.throw(g)
	hosted := !s.landings.Empty()
	var t host.Tick
	if hosted {
		t = s.tickOf.Of(cb, d)
	}
	dt := d.Seconds()
	for s.bodies.All(); s.bodies.Next(); {
		cur := s.bodies.Cursor()
		bases, drifts, marks, colls := s.base.Slice(cur), s.drift.Slice(cur), s.marks.Slice(cur),
			s.coll.Slice(cur)
		s.walking.ids, s.walking.speeds = cur.IDs, s.walking.speeds[:0]
		for i := range cur.IDs {
			s.normals = s.normals[:0]
			if colls != nil {
				for _, c := range colls[i].Contacts() {
					if !c.Sensed {
						s.normals = append(s.normals, c.Normal)
					}
				}
			}
			b := planeOf(&drifts[i], marks[i])
			speed, landed := planar.Settle(&b, g, s.normals)
			// the pull presses a borne body into what bears it, so collision finds the touch again
			planar.Fall(&b, g, dt, bases[i].Pos.MaxStep()/dt)
			planePut(&b, &drifts[i], &marks[i])
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

func (s *planeSystem) throw(g geom.Vec) {
	s.p.leaps.Drain(func(i command.Issued[Leap]) {
		if !i.ByEntity || !s.bodies.Seek(i.Entity) {
			return
		}
		cur := s.bodies.Cursor()
		b := planeOf(s.drift.At(cur), *s.marks.At(cur))
		planar.Leap(&b, g, s.body.At(cur).Leap)
		planePut(&b, s.drift.At(cur), s.marks.At(cur))
	})
	s.p.aways.Drain(func(i command.Issued[Away]) {
		if !i.ByEntity {
			return
		}
		way, ok := s.away(i.Entity, i.Command)
		if !ok || !s.bodies.Seek(i.Entity) {
			return
		}
		cur := s.bodies.Cursor()
		b := planeOf(s.drift.At(cur), *s.marks.At(cur))
		a := i.Command
		planar.Throw(&b, g, geom.NewVec(way.X*a.Speed, way.Y*a.Speed), a.Rise)
		planePut(&b, s.drift.At(cur), s.marks.At(cur))
	})
}

func planeOf(d *world.Drift, marks tag.Tags[unit.States]) planar.Body {
	return planar.Body{Drift: geom.NewVec(d.X, d.Y), Airborne: marks.Has(unit.Airborne)}
}

func planePut(b *planar.Body, d *world.Drift, marks *tag.Tags[unit.States]) {
	*d = world.Drift{X: b.Drift.X, Y: b.Drift.Y}
	if b.Airborne {
		*marks = marks.With(unit.Airborne)
	} else {
		*marks = marks.Without(unit.Airborne)
	}
}
