package gravity

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/world"
)

var _ goke.Module = (*module)(nil)

// module gives the bodies their drift once a tick and runs the fall every step of the simulation.
type module struct {
	p          *Plugin
	give, fall goke.Runnable
}

// =================================================================
// goke.Module contract
// =================================================================

func (m *module) RegSystems(ecs *goke.ECS) {
	m.give = ecs.RegSys(&giveSystem{})
	if m.p.worldPlugin.HasHeights() {
		m.fall = ecs.RegSys(newFallSystem(m.p))
	} else {
		m.fall = ecs.RegSys(newPlaneSystem(m.p))
	}
}

// RunPlan gives the bodies new to it their drift at once, and carries, stands and lands them every
// step of the simulation.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.give, d)
	ctx.Sync()
	clock.Simulate(m.p.worldPlugin.Clock(), ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.fall, step)
		ctx.Sync()
	})
}

// SetupSystems is empty — the plugin seeds nothing.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types the plugin owns — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[Body](), goke.LoadComp[world.Drift]()}
}
