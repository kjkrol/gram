package bullet

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
)

var _ goke.Module = (*module)(nil)

// module runs the plugin's systems: the shots fired once a tick, the flights and the bursts
// every step of the simulation.
type module struct {
	p   *Plugin
	ecs *goke.ECS

	shoot, flight, burst goke.Runnable
	built                bool
}

func newModule(p *Plugin, ecs *goke.ECS) *module { return &module{p: p, ecs: ecs} }

// =================================================================
// goke.Module contract
// =================================================================

// RegSystems builds and registers the plugin's systems — see [goke.Module].
func (m *module) RegSystems(ecs *goke.ECS) {
	if m.built {
		return
	}
	p := m.p
	w := p.worldPlugin
	m.shoot = ecs.RegSys(newShootSystem(w, &p.shoots, p.selected, p.ground))
	m.flight = ecs.RegSys(newFlightSystem(w, &p.landings, &p.restings, p.ground))
	m.burst = ecs.RegSys(newBurstSystem(w, &p.bursts, &p.blasts))
	m.built = true
}

// RunPlan fires the Shoots given since the last tick, at once, and hands the flights to the
// simulation: every step the shots fly, land and rest, then the Bursts given burst.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.shoot, d)
	ctx.Sync()
	clock.Simulate(m.p.worldPlugin.Clock(), ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.flight, step)
		ctx.Sync()
		ctx.Run(m.burst, step)
		ctx.Sync()
	})
}

// SetupSystems is empty — the plugin seeds nothing of its own.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types the plugin owns — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[Body](),
		goke.LoadComp[Flight](),
	}
}
