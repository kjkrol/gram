package navigation

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world/clock"
)

// module registers and runs the move commands at once, and navigation and the driving in the
// simulation, as one goke.Module.
type module struct {
	navigationSystem  *navigationSystem
	moveCommandSystem *moveCommandSystem
	driveSystem       *driveSystem
	clock             *clock.Clock // the world's; nil, run at once

	navSysRunnable     goke.Runnable
	moveCmdSysRunnable goke.Runnable
	driveSysRunnable   goke.Runnable
}

var _ goke.Module = (*module)(nil)

// =================================================================
// goke.Module contract
// =================================================================

// RegSystems registers nav (and cmd, if enabled) as the per-tick systems — see [goke.Module].
func (m *module) RegSystems(ecs *goke.ECS) {
	m.navSysRunnable = ecs.RegSys(m.navigationSystem)
	m.moveCmdSysRunnable = ecs.RegSys(m.moveCommandSystem)
	m.driveSysRunnable = ecs.RegSys(m.driveSystem)

}

// RunPlan takes the orders at once — a player orders in the tactical pause too, and the routes are
// planned and shown — and hands the driving to the simulation: every step, each unit under orders
// is steered along its route, then a hand on an entity overrides whatever its order asked.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.moveCmdSysRunnable, d)
	ctx.Sync()
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.navSysRunnable, step)
		ctx.Sync()
		ctx.Run(m.driveSysRunnable, step)
		ctx.Sync()
	})
}

// SetupSystems seeds board.Occupancy from every entity's Cell, Mover and in-progress Leg — after
// a Populate as after a Load, so the board never depends on a spawn effect to know who stands where.
func (m *module) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var cell goke.Comp[board.Cell]
		var order goke.OptComp[MoveOrder]
		var mover goke.OptComp[board.Mover]
		query := si.NewQueryBuilder(&cell).Optional(&order, &mover).Build()
		occupancy := m.navigationSystem.occupancy
		query.All()
		for query.Next() {
			cursor := query.Cursor()
			cells := cell.Slice(cursor)
			orders := order.Slice(cursor)
			movers := mover.Slice(cursor)
			for i, id := range cursor.IDs {
				domain := board.DomainAt(movers, i)
				occupancy.Enter(cells[i].ID, id, domain)
				if orders != nil && orders[i].Leg.Active {
					for _, c := range orders[i].Leg.cells() {
						occupancy.Enter(c, id, domain)
					}
				}
			}
		}
	}}}
}

// LoadComps lists the component types navigation owns — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[board.Cell](),
		goke.LoadComp[MoveOrder](),
		goke.LoadComp[CellEntered](),
	}
}
