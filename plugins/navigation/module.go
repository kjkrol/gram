package navigation

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/board/unit"
)

// module registers and runs the move commands at once, and navigation in the simulation, as one
// goke.Module.
type module struct {
	navigationSystem  *navigationSystem
	moveCommandSystem *moveCommandSystem
	clock             *clock.Clock // the world's; nil, run at once

	navSysRunnable     goke.Runnable
	moveCmdSysRunnable goke.Runnable
}

var _ goke.Module = (*module)(nil)

// =================================================================
// goke.Module contract
// =================================================================

// RegSystems registers nav (and cmd, if enabled) as the per-tick systems — see [goke.Module].
func (m *module) RegSystems(ecs *goke.ECS) {
	m.navSysRunnable = ecs.RegSys(m.navigationSystem)
	m.moveCmdSysRunnable = ecs.RegSys(m.moveCommandSystem)
}

// RunPlan takes the orders at once — a player orders in the tactical pause too, and the routes are
// planned and shown — and steers every unit under orders along its route every step of the
// simulation; a unit a hand is on gives its order up (the driving's, which runs after).
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.moveCmdSysRunnable, d)
	ctx.Sync()
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.navSysRunnable, step)
		ctx.Sync()
	})
}

// SetupSystems is empty — navigationSystem seeds the occupancy with the steps in progress in its
// own Init.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types navigation owns — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[unit.At](),
		goke.LoadComp[MoveOrder](),
		goke.LoadComp[LastOrder](),
		goke.LoadComp[Blocked](),
		goke.LoadComp[Arrived](),
	}
}
