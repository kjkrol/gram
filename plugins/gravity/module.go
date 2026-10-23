package gravity

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/motion"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p          *Plugin
	give, fall goke.Runnable
	step       clock.Block // bound once, so a tick allocates no method value
}

func (m *module) RegSystems(ecs *goke.ECS) {
	m.give = ecs.RegSys(&giveSystem{})
	if m.p.across {
		m.fall = ecs.RegSys(newPlaneSystem(m.p))
	} else {
		m.fall = ecs.RegSys(newFallSystem(m.p))
	}
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.give, d)
	ctx.Sync()
	clock.Simulate(m.p.worldPlugin.Write().Clock(), ctx, d, m.step)
}

func (m *module) simulate(ctx goke.RunCtx, step time.Duration) {
	ctx.Run(m.fall, step)
	ctx.Sync()
}

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[Body](), goke.LoadComp[motion.Drift]()}
}
