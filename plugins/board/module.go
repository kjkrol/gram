package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/clock"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/rule/effect"
)

var _ goke.Module = (*module)(nil)

// module runs, in the simulation, the cells — what effects changed — the standing report and the
// rules of the cells.
type module struct {
	cells     *cellSystem
	standing  *standingSystem
	cellRules *cellRuleSystem
	clock     *clock.Clock // the world's; nil, run at once

	cellsRunnable     goke.Runnable
	standingRunnable  goke.Runnable
	cellRulesRunnable goke.Runnable
}

// =================================================================
// goke.Module contract
// =================================================================

func (m *module) RegSystems(ecs *goke.ECS) {
	m.cellsRunnable = ecs.RegSys(m.cells) // first: it makes or finds the cells
	m.standingRunnable = ecs.RegSys(m.standing)
	m.cellRulesRunnable = ecs.RegSys(m.cellRules)
}

// RunPlan hands the board's work to the simulation.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.cellsRunnable, step)
		ctx.Run(m.standingRunnable, step)
		ctx.Run(m.cellRulesRunnable, step)
		ctx.Sync()
	})
}

// SetupSystems is empty — the cells build themselves in their own Init.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types board writes or reads, so a save loads without the vision
// and effects plugins — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[At](), goke.LoadComp[Mover](),
		goke.LoadComp[cell.Plot](), goke.LoadComp[cell.Ground](), goke.LoadComp[cell.Way](), goke.LoadComp[cell.Crossing](),
		goke.LoadComp[effect.Active](), goke.LoadComp[tag.Tags[effect.States]](), goke.LoadComp[tag.Tags[cell.Family]](),
	}
}
