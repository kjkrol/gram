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

// System carries out the Course asked of each entity between the plans and movement, as
// its Steering lets it: heading by at most TurnRate a tick, and base speed rewritten each tick for
// an entity with a motion profile; one Halted stands. The commands entities gave themselves —
// Away, Toward, Turn — it asks of their Helms first. An entity without a Course gets one, steered
// from its next step. The world runs it in every step of its simulation, before movement.
type System struct {
	query    *goke.Query
	steer    goke.Comp[Steering]
	course   goke.OptComp[Course]
	base     goke.Comp[entity.Base]
	courseID goke.CompID

	// told are the commands given, carried out first in a step, through lookup
	told       queues
	obeyed     map[uid.UID64]bool
	lookup     *goke.Query
	lookBase   goke.Comp[entity.Base]
	lookSteer  goke.OptComp[Steering]
	lookCourse goke.OptComp[Course]
}

// NewSystem is the steering system; the world registers it.
func NewSystem() *System { return &System{obeyed: map[uid.UID64]bool{}} }

func (s *System) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.steer, &s.base).Optional(&s.course).Build()
	s.lookup = si.NewQueryBuilder(&s.lookBase).Optional(&s.lookSteer, &s.lookCourse).Build()
	s.courseID = si.RegComp[Course]()
}

func (s *System) Update(cb *goke.CmdBuf, d time.Duration) {
	s.obey()
	dt := d.Seconds()
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		if !s.course.Present(cursor) {
			for _, id := range cursor.IDs {
				cb.AddOne(id, s.courseID, Course{})
			}
			continue
		}
		steers, courses := s.steer.Slice(cursor), s.course.Slice(cursor)
		bases := s.base.Slice(cursor)

		for i := range cursor.IDs {
			h := Helm{Steering: &steers[i], Course: &courses[i]}
			if h.Halted {
				h.Speed = 0
				bases[i].Vel.Value = 0
				continue
			}
			if h.Want.X != 0 || h.Want.Y != 0 {
				bases[i].Vel.Dir = turnTowards(bases[i].Vel.Dir, h.Want, h.TurnRate)
			}
			if h.Delay > 0 {
				h.Delay--
				if h.Delay == 0 {
					h.Want = h.Pending
				}
			}
			if h.MaxSpeed > 0 {
				h.advance(dt)
				bases[i].Vel.Value = h.Speed
			}
		}
	}
}

// turnTowards rotates from towards to by at most rate radians; a zero rate or heading snaps to it.
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
