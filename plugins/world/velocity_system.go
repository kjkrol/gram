package world

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*velocitySystem)(nil)

// velocitySystem scales every entity's speed, after Steering wrote the base speed and before
// movement: by its steering.Pace, the ground's, then as the rules of a Moving say.
type velocitySystem struct {
	host  *rule.EachHost[Moving]
	tick  rule.TickSource
	query *goke.Query
	base  goke.Comp[Base]
	pace  goke.OptComp[steering.Pace]

	ids   []uid.UID64
	bases []Base
	about func(i int) Moving // at, bound once so a tick allocates no method value
}

func newVelocitySystem(host *rule.EachHost[Moving]) *velocitySystem {
	s := &velocitySystem{host: host}
	s.about = s.at
	return s
}

func (s *velocitySystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base).Optional(&s.pace)
	s.host.Bind(qb)
	s.query = qb.Build()
}

func (s *velocitySystem) Update(cb *goke.CmdBuf, d time.Duration) {
	hosted := !s.host.Empty()
	var tick rule.Tick
	if hosted {
		tick = s.tick.Of(cb, d)
	}
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		s.ids, s.bases = cursor.IDs, s.base.Slice(cursor)
		if paces := s.pace.Slice(cursor); paces != nil {
			for i := range s.bases {
				s.bases[i].Vel.Value *= paces[i].Share
			}
		}
		if hosted {
			s.host.Run(tick, cursor, s.about)
		}
	}
}

// at describes the i-th entity of the chunk being walked.
func (s *velocitySystem) at(i int) Moving { return Moving{ID: s.ids[i], Base: &s.bases[i]} }

// Moving is what a rule hosted by the world's velocity pass gets, every tick, for every entity:
// scale Base.Vel.Value to slow or stop it, after Steering has written the base speed.
type Moving struct {
	ID   uid.UID64
	Base *Base
}

// Who is the entity moving: whose moment it is, for a rule.
func (m Moving) Who() uid.UID64 { return m.ID }
