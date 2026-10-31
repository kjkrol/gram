package navigation

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/navigation/internal/traffic"
)

type module struct {
	navigationSystem  *traffic.System
	moveCommandSystem *routing.System
	clock             *clock.Clock
	step              clock.Block // bound once, so a tick allocates no method value
	classify          func()      // the units' classes read off their kinds, at Setup

	navSysRunnable     goke.Runnable
	moveCmdSysRunnable goke.Runnable
}

var _ goke.Module = (*module)(nil)

func (m *module) RegSystems(ecs *goke.ECS) {
	m.navSysRunnable = ecs.RegSys(m.navigationSystem)
	m.moveCmdSysRunnable = ecs.RegSys(m.moveCommandSystem)
}

// the orders are taken at once so a player orders in the tactical pause too
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.moveCmdSysRunnable, d)
	ctx.Sync()
	clock.Simulate(m.clock, ctx, d, m.step)
}

func (m *module) simulate(ctx goke.RunCtx, step time.Duration) {
	ctx.Run(m.navSysRunnable, step)
	ctx.Sync()
}

// the kinds are all defined by Setup, and nothing plans a route before it
func (m *module) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(*goke.SysInit) { m.classify() }}}
}

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[unit.At](),
		goke.LoadComp[MoveOrder](),
		goke.LoadComp[LastOrder](),
		goke.LoadComp[Blocked](),
		goke.LoadComp[Arrived](),
		goke.LoadComp[Round](),
	}
}
