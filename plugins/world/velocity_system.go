package world

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*velocitySystem)(nil)

// velocitySystem runs the rules of a Moving over every entity, after Steering wrote the
// base speed and before movement, so each may scale Velocity.Value.
type velocitySystem struct {
	host  *host.EachHost[Moving]
	tick  plugin.TickSource
	query *goke.Query
	base  goke.Comp[Base]

	ids   []uid.UID64
	bases []Base
	about func(i int) Moving // at, bound once so a tick allocates no method value
}

func newVelocitySystem(host *host.EachHost[Moving]) *velocitySystem {
	s := &velocitySystem{host: host}
	s.about = s.at
	return s
}

func (s *velocitySystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base)
	s.host.Bind(qb)
	s.query = qb.Build()
}

func (s *velocitySystem) Update(cb *goke.CmdBuf, d time.Duration) {
	if s.host.Empty() {
		return
	}
	tick := s.tick.Of(cb, d)
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		s.ids, s.bases = cursor.IDs, s.base.Slice(cursor)
		s.host.Run(tick, cursor, s.about)
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
