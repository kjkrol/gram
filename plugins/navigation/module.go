package navigation

import (
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/board/unit"
)

type module struct {
	navigationSystem  *navigationSystem
	moveCommandSystem *routing.System
	clock             *clock.Clock
	step              clock.Block // bound once, so a tick allocates no method value

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

func (m *module) SetupSystems() []goke.System { return nil }

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
