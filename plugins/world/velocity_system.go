package world

import (
	"math"
	"time"

	"github.com/kjkrol/ceremony/cond"
	cgoke "github.com/kjkrol/ceremony/goke"

	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*velocitySystem)(nil)

type velocitySystem struct {
	host   *host.Rules[Moving]
	cols   *cgoke.Columns
	tick   host.TickSource
	query  *goke.Query
	base   goke.Comp[Base]
	pace   goke.OptComp[steering.Pace]
	course goke.OptComp[steering.Course]

	ids     []uid.UID64
	bases   []Base
	courses []steering.Course
	about   func(i int) Moving // bound once, so a tick allocates no method value
}

func newVelocitySystem(host *host.Rules[Moving]) *velocitySystem {
	s := &velocitySystem{host: host}
	s.about = s.at
	return s
}

func (s *velocitySystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base).Optional(&s.pace)
	s.cols = cgoke.NewColumns(qb)
	s.cols.Own(&s.course)
	s.host.Bind(s.cols)
	s.query = qb.Build()
}

func (s *velocitySystem) Update(cb *goke.CmdBuf, d time.Duration) {
	hosted := !s.host.Empty()
	var tick host.Tick
	if hosted {
		tick = s.tick.Of(d)
	}
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		s.ids, s.bases, s.courses = cursor.IDs, s.base.Slice(cursor), s.course.Slice(cursor)
		if paces := s.pace.Slice(cursor); paces != nil {
			for i := range s.bases {
				s.bases[i].Vel.Value *= paces[i].Share
			}
		}
		if hosted {
			s.host.Run(tick, s.cols.Row(cursor), len(cursor.IDs), s.about)
		}
	}
}

func (s *velocitySystem) at(i int) Moving {
	m := Moving{ID: s.ids[i], Base: &s.bases[i]}
	if s.courses != nil {
		m.Course = &s.courses[i]
	}
	return m
}

// Moving is the moment of every entity in the velocity pass, every step: a rule of it scales
// Base.Vel.Value to slow or stop the entity. Course is the steering's, nil for an entity with
// none.
type Moving struct {
	ID     uid.UID64
	Base   *Base
	Course *steering.Course
}

// Who is the entity moving.
func (m Moving) Who() uid.UID64 { return m.ID }

// Still is the word of an entity that neither walks nor strafes this step.
func Still() cond.Word[Moving] {
	return cond.NewWord("still", func(m Moving) bool {
		return m.Base.Vel.Value == 0 && m.Base.Vel.Side == 0
	})
}

// Braking is the word of an entity slowing down: still going, its course asking for less speed
// than it has.
func Braking() cond.Word[Moving] {
	return cond.NewWord("braking", func(m Moving) bool {
		return m.Course != nil && m.Course.Speed != 0 &&
			math.Abs(m.Course.WantSpeed) < math.Abs(m.Course.Speed)
	})
}

var (
	_ host.About = Moving{}
)
