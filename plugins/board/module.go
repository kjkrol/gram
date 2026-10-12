package board

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

var _ goke.Module = (*module)(nil)

// module runs, in the simulation, the cells — what effects changed, what is built over them — the
// occupancy's upkeep and the rules of where units stand and of the cells.
type module struct {
	cells     goke.System
	built     goke.System
	standing  goke.System
	keep      goke.System
	cellRules goke.System
	clock     *clock.Clock   // the world's; nil, run at once
	template  *kind.Template // the world's roster's for cells, whose components the saves carry

	cellsRunnable     goke.Runnable
	builtRunnable     goke.Runnable
	standingRunnable  goke.Runnable
	keepRunnable      goke.Runnable
	cellRulesRunnable goke.Runnable
}

// =================================================================
// goke.Module contract
// =================================================================

func (m *module) RegSystems(ecs *goke.ECS) {
	m.cellsRunnable = ecs.RegSys(m.cells) // first: it makes or finds the cells
	m.builtRunnable = ecs.RegSys(m.built)
	m.standingRunnable = ecs.RegSys(m.standing)
	m.keepRunnable = ecs.RegSys(m.keep) // after standing, which takes unit.Entered off
	m.cellRulesRunnable = ecs.RegSys(m.cellRules)
}

// RunPlan hands the board's work to the simulation.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.cellsRunnable, step)
		ctx.Run(m.builtRunnable, step)
		ctx.Run(m.standingRunnable, step)
		ctx.Run(m.keepRunnable, step)
		ctx.Run(m.cellRulesRunnable, step)
		ctx.Sync()
	})
}

// SetupSystems is empty — the cells build themselves in their own Init, the occupancy's upkeep
// seeds it in its own.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types board writes or reads, so a save loads without the vision
// and effects plugins — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	var templated []goke.CompToken // what the world's plugins give every cell
	if m.template != nil {
		for _, c := range m.template.Spec() {
			templated = append(templated, c.LoadToken())
		}
	}
	return append(templated,
		goke.LoadComp[unit.At](),
		goke.LoadComp[unit.Mover](),
		goke.LoadComp[tag.Tags[unit.States]](),
		goke.LoadComp[cell.Plot](),
		goke.LoadComp[cell.Ground](),
		goke.LoadComp[cell.Way](),
		goke.LoadComp[cell.Built](),
		goke.LoadComp[cell.On](),
		goke.LoadComp[cell.Floor](),
		goke.LoadComp[cell.Wall](),
		goke.LoadComp[cell.Rail](),
		goke.LoadComp[effect.Active](),
		goke.LoadComp[tag.Tags[effect.States]](),
		goke.LoadComp[tag.Tags[rule.Roles]](),
	)
}
