package underground

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p     *Plugin
	hands goke.Runnable
	done  goke.Runnable
	step  clock.Block // bound once, so a tick allocates no method value
	built bool
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
	m.done = ecs.RegSys(digSystem{m.p})
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

type digSystem struct{ p *Plugin }

func (digSystem) Init(*goke.SysInit) {}

func (s digSystem) Update(*goke.CmdBuf, time.Duration) {
	res := &s.p.Res
	for _, o := range res.Orders {
		s.p.dig(o)
	}
	res.Orders = res.Orders[:0]
}
