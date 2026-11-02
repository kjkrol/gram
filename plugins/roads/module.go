package roads

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

// RunPlan carries out the players' commands at once, in the tactical pause too, and has the
// roads confirmed built in the steps of the simulation.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.hands, d)
	clock.Simulate(m.p.worldPlugin.Write().Clock(), ctx, d, m.step)
}

func (m *module) simulate(ctx goke.RunCtx, step time.Duration) { ctx.Run(m.done, step) }

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken { return nil }

type handSystem struct{ p *Plugin }

func (s handSystem) Init(si *goke.SysInit) {
	s.p.estimated.Bind()
	g := &s.p.ground
	g.query = si.NewQueryBuilder(&g.on).Build()
}

func (s handSystem) Update(_ *goke.CmdBuf, d time.Duration) { s.p.carryOut(d) }

type buildSystem struct{ p *Plugin }

func (s buildSystem) Init(*goke.SysInit) { s.p.built.Bind() }

func (s buildSystem) Update(_ *goke.CmdBuf, step time.Duration) {
	res := &s.p.Res
	for _, o := range res.Orders {
		s.p.lay(o, step)
	}
	clear(res.Orders)
	res.Orders = res.Orders[:0]
}
