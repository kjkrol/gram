package buildings

import (
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p      *Plugin
	raised goke.Runnable
	step   clock.Block
	built  bool
}

func (m *module) RegSystems(ecs *goke.ECS) {
	if m.built {
		return
	}
	m.raised = ecs.RegSys(raiseSystem{m.p})
	m.step = m.simulate
	m.built = true
}

// RunPlan carries out the players' settings at once and raises the buildings in the simulation.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p := m.p
	p.opened.Drain(func(i command.Issued[OpenRooms]) { p.write.OpenRooms(i.Command.On) })
	p.openedBehind.Drain(func(i command.Issued[OpenRoomsBehind]) {
		p.write.OpenRoomsBehind(i.Command.On)
	})
	clock.Simulate(p.worldPlugin.Write().Clock(), ctx, d, m.step)
}

func (m *module) simulate(ctx goke.RunCtx, step time.Duration) { ctx.Run(m.raised, step) }

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken { return nil }

type raiseSystem struct{ p *Plugin }

func (raiseSystem) Init(*goke.SysInit) {}

func (s raiseSystem) Update(*goke.CmdBuf, time.Duration) { s.p.raise() }
