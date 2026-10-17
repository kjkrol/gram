package collision

import (
	"log"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/uid"
)

var _ goke.Module = (*module)(nil)

type module struct {
	space *aabbworld.Space
	ecs   *goke.ECS

	pairs    *host.PairRules[Meeting]
	entities *host.Rules[Struck]

	system  goke.Runnable
	fieldOf func() Field
	heights bool
	stats   *ContactStats
	log     *log.Logger
	clock   *clock.Clock
	tick    host.TickSource
	leave   func(cb *goke.CmdBuf, id uid.UID64)
	built   bool
}

func newModule(space *aabbworld.Space, ecs *goke.ECS, pairs *host.PairRules[Meeting], entities *host.Rules[Struck]) *module {
	return &module{space: space, ecs: ecs, pairs: pairs, entities: entities}
}

func (m *module) RegSystems(ecs *goke.ECS) {
	if !m.built {
		m.build()
	}
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.system, step)
		ctx.Sync()
	})
}

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[Collider](),
		goke.LoadComp[Physics](),
		goke.LoadComp[Sweep](),
	}
}

func (m *module) build() {
	s := newCollisionSystem(m.space, m.pairs, m.entities, m.fieldOf)
	s.stats, s.log, s.heights = m.stats, m.log, m.heights
	s.tickOf = m.tick
	s.leave = m.leave
	m.system = m.ecs.RegSys(s)
	m.built = true
}
