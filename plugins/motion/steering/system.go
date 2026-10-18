package steering

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*System)(nil)

// System carries out the Course asked of each entity as its Steering lets it, run by the world
// in every step before movement. An entity without a Course gets one, steered from its next step.
type System struct {
	query    *goke.Query
	steer    goke.Comp[Steering]
	course   goke.OptComp[Course]
	base     goke.Comp[entity.Base]
	courseID goke.CompID
	pace     goke.Opt

	told       queues
	obeyed     map[uid.UID64]bool
	lookup     *goke.Query
	lookBase   goke.Comp[entity.Base]
	lookSteer  goke.OptComp[Steering]
	lookCourse goke.OptComp[Course]
}

// NewSystem is the steering system.
func NewSystem() *System { return &System{obeyed: map[uid.UID64]bool{}} }

// Paced has the system steer each block of entities as pace, an option of its query, says,
// turning and counting the reflex down by as many steps as it is given; set before Init.
func (s *System) Paced(pace goke.Opt) *System {
	s.pace = pace
	return s
}

func (s *System) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.steer, &s.base).Optional(&s.course)
	if s.pace != nil {
		qb.With(s.pace)
	}
	s.query = qb.Build()
	s.lookup = si.NewQueryBuilder(&s.lookBase).Optional(&s.lookSteer, &s.lookCourse).Build()
	s.courseID = si.RegComp[Course]()
}

func (s *System) Update(cb *goke.CmdBuf, d time.Duration) {
	s.obey()
	for s.query.Paced(d); s.query.Next(); {
		cursor := s.query.Cursor()
		if !s.course.Present(cursor) {
			for _, id := range cursor.IDs {
				cb.AddOne(id, s.courseID, Course{})
			}
			continue
		}
		step := s.query.Dt()
		dt, n := step.Seconds(), steps(step, d)
		steers, courses := s.steer.Slice(cursor), s.course.Slice(cursor)
		bases := s.base.Slice(cursor)

		for i := range cursor.IDs {
			h := Helm{Steering: &steers[i], Course: &courses[i]}
			if h.Halted {
				h.Speed, h.Side = 0, 0
				bases[i].Vel.Value, bases[i].Vel.Side = 0, 0
				continue
			}
			if h.Want.X != 0 || h.Want.Y != 0 {
				bases[i].Vel.Dir = turnTowards(bases[i].Vel.Dir, h.Want, h.TurnRate*float64(n))
			}
			if h.Delay > 0 {
				h.Delay -= uint8(min(int(h.Delay), n))
				if h.Delay == 0 {
					h.Want = h.Pending
				}
			}
			if h.MaxSpeed > 0 {
				h.advance(dt)
				bases[i].Vel.Value, bases[i].Vel.Side = h.Speed, h.Side
			}
		}
	}
}

// how many steps of d a step given lasts, at least one
func steps(given, d time.Duration) int {
	if d <= 0 {
		return 1
	}
	return max(int(given/d), 1)
}

// a zero rate or heading snaps to the target
func turnTowards(from, to geom.Vec, rate float64) geom.Vec {
	if rate <= 0 || (from.X == 0 && from.Y == 0) {
		return to
	}
	at := math.Atan2(from.Y, from.X)
	delta := math.Mod(math.Atan2(to.Y, to.X)-at+3*math.Pi, 2*math.Pi) - math.Pi
	if math.Abs(delta) <= rate {
		return to
	}
	at += math.Copysign(rate, delta)
	return geom.NewVec(math.Cos(at), math.Sin(at))
}
