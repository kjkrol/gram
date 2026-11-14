package forest

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p       *Plugin
	planted goke.Runnable
	step    clock.Block
	built   bool
}

func (m *module) RegSystems(ecs *goke.ECS) {
	if m.built {
		return
	}
	m.planted = ecs.RegSys(plantSystem{m.p})
	m.step = m.simulate
	m.built = true
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.p.worldPlugin.Write().Clock(), ctx, d, m.step)
}

func (m *module) simulate(ctx goke.RunCtx, step time.Duration) { ctx.Run(m.planted, step) }

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken { return nil }

type plantSystem struct{ p *Plugin }

func (plantSystem) Init(*goke.SysInit) {}

func (s plantSystem) Update(*goke.CmdBuf, time.Duration) { s.p.plant() }
