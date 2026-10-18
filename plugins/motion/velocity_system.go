package motion

import (
	"time"

	cgoke "github.com/kjkrol/ceremony/goke"

	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*velocitySystem)(nil)

type velocitySystem struct {
	host   *host.Rules[Moving]
	cols   *cgoke.Columns
	tick   host.TickSource
	query  *goke.Query
	base   goke.Comp[world.Base]
	pace   goke.OptComp[steering.Pace]
	course goke.OptComp[steering.Course]
	// the bands' pace, not the steering's share of speed
	rhythm cgoke.Pace
	blocks cgoke.Blocks

	ids     []uid.UID64
	bases   []world.Base
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
	s.blocks = cgoke.Over(s.rhythm, qb)
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
		dt, ok := s.blocks.Step(cursor, d)
		if !ok {
			continue
		}
		if hosted && dt != d {
			tick = s.tick.Of(dt)
		}
		s.ids, s.bases, s.courses = cursor.IDs, s.base.Slice(cursor), s.course.Slice(cursor)
		if paces := s.pace.Slice(cursor); paces != nil {
			for i := range s.bases {
				s.bases[i].Vel.Value *= paces[i].Share
			}
		}
		if hosted {
			s.host.Run(tick, s.cols.Row(cursor), len(cursor.IDs), s.about)
			if dt != d {
				tick = s.tick.Of(d)
			}
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
