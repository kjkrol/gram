package board

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/clock"
	"github.com/kjkrol/gram/plugins/world/effects"
)

var _ goke.Module = (*module)(nil)

// module runs, in the simulation, the cells — what effects changed — and the standing report.
type module struct {
	cells    *cellSystem
	standing *standingSystem
	clock    *clock.Clock // the world's; nil, run at once

	cellsRunnable    goke.Runnable
	standingRunnable goke.Runnable
}

// =================================================================
// goke.Module contract
// =================================================================

func (m *module) RegSystems(ecs *goke.ECS) {
	m.cellsRunnable = ecs.RegSys(m.cells) // first: it makes or finds the cells
	m.standingRunnable = ecs.RegSys(m.standing)
}

// RunPlan hands the board's work to the simulation.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.cellsRunnable, step)
		ctx.Run(m.standingRunnable, step)
		ctx.Sync()
	})
}

// SetupSystems is empty — the cells build themselves in their own Init.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types board writes or reads, so a save loads without the vision
// and effects plugins — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[Cell](), goke.LoadComp[Mover](),
		goke.LoadComp[Plot](), goke.LoadComp[Ground](), goke.LoadComp[Way](), goke.LoadComp[Crossing](),
		goke.LoadComp[effects.Active](), goke.LoadComp[effects.Idle](),
	}
}
