package bullet

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p   *Plugin
	ecs *goke.ECS

	shoot, flight, burst goke.Runnable
	step                 clock.Block // bound once, so a tick allocates no method value
	built                bool
}

func newModule(p *Plugin, ecs *goke.ECS) *module {
	m := &module{p: p, ecs: ecs}
	m.step = m.simulate
	return m
}

func (m *module) RegSystems(ecs *goke.ECS) {
	if m.built {
		return
	}
	p := m.p
	w := p.worldPlugin
	m.shoot = ecs.RegSys(newShootSystem(w, &p.shoots, p.selected, p.ground))
	flight := newFlightSystem(w, &p.landings, &p.restings, p.ground)
	flight.leave = p.motion.Write().Leave
	m.flight = ecs.RegSys(flight)
	m.burst = ecs.RegSys(newBurstSystem(w, &p.bursts, &p.blasts))
	m.built = true
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.shoot, d)
	ctx.Sync()
	clock.Simulate(m.p.worldPlugin.Write().Clock(), ctx, d, m.step)
}

func (m *module) simulate(ctx goke.RunCtx, step time.Duration) {
	ctx.Run(m.flight, step)
	ctx.Sync()
	ctx.Run(m.burst, step)
	ctx.Sync()
}

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[Body](),
		goke.LoadComp[Flight](),
	}
}
