package world

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*VelocitySystem)(nil)

// VelocitySystem runs the rules of a Moving over every entity, after Steering wrote the
// base speed and before movement, so each may scale Velocity.Value.
type VelocitySystem struct {
	host     *host.EachHost[Moving]
	commands *control.Carrier
	query    *goke.Query
	base     goke.Comp[Base]

	ids   []uid.UID64
	bases []Base
	about func(i int) Moving // at, bound once so a tick allocates no method value
}

func NewVelocitySystem(host *host.EachHost[Moving]) *VelocitySystem {
	s := &VelocitySystem{host: host}
	s.about = s.at
	return s
}

func (s *VelocitySystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base)
	s.host.Bind(qb)
	s.query = qb.Build()
}

func (s *VelocitySystem) Update(cb *goke.CmdBuf, d time.Duration) {
	if s.host.Empty() {
		return
	}
	tick := plugin.Tick{CmdBuf: cb, Now: time.Now(), Dt: d, Commands: s.commands}
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		s.ids, s.bases = cursor.IDs, s.base.Slice(cursor)
		s.host.Run(tick, cursor, s.about)
	}
}

// at describes the i-th entity of the chunk being walked.
func (s *VelocitySystem) at(i int) Moving { return Moving{ID: s.ids[i], Base: &s.bases[i]} }
