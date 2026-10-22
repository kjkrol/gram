package bridges

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p           *Plugin
	hands, done goke.Runnable
	step        clock.Block // bound once, so a tick allocates no method value
	built       bool
}

func newModule(p *Plugin) *module {
	m := &module{p: p}
	m.step = m.simulate
	return m
}

func (m *module) RegSystems(ecs *goke.ECS) {
	if m.built {
		return
	}
	m.hands = ecs.RegSys(handSystem{m.p})
	m.done = ecs.RegSys(buildSystem{m.p})
	m.built = true
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.hands, d)
	clock.Simulate(m.p.worldPlugin.Write().Clock(), ctx, d, m.step)
}

func (m *module) simulate(ctx goke.RunCtx, step time.Duration) { ctx.Run(m.done, step) }

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken { return nil }

type handSystem struct{ p *Plugin }

func (handSystem) Init(*goke.SysInit) {}

func (s handSystem) Update(*goke.CmdBuf, time.Duration) { s.p.carryOut() }

type buildSystem struct{ p *Plugin }

func (buildSystem) Init(*goke.SysInit) {}

func (s buildSystem) Update(*goke.CmdBuf, time.Duration) {
	for _, b := range s.p.pending {
		s.p.build(b)
	}
	s.p.pending = s.p.pending[:0]
}
