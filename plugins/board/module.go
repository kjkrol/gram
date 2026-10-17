package board

import (
	cgoke "github.com/kjkrol/ceremony/goke"
	"time"

	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
)

var _ goke.Module = (*module)(nil)

type module struct {
	cells     goke.System
	built     goke.System
	standing  goke.System
	keep      goke.System
	cellRules goke.System
	clock     *clock.Clock // nil: run at once
	roster    *kind.Roster
	template  *kind.Template

	cellsRunnable     goke.Runnable
	builtRunnable     goke.Runnable
	standingRunnable  goke.Runnable
	keepRunnable      goke.Runnable
	cellRulesRunnable goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) {
	m.cellsRunnable = ecs.RegSys(m.cells) // first: it makes or finds the cells
	m.builtRunnable = ecs.RegSys(m.built)
	m.standingRunnable = ecs.RegSys(m.standing)
	m.keepRunnable = ecs.RegSys(m.keep) // after standing, which takes unit.Entered off
	m.cellRulesRunnable = ecs.RegSys(m.cellRules)
}

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

func (m *module) SetupSystems() []goke.System { return nil }

// the component types a save loads without the vision and effects plugins
func (m *module) LoadComps() []goke.CompToken {
	var templated []goke.CompToken
	if m.template != nil {
		for _, c := range m.roster.Spec(m.template) {
			templated = append(templated, cgoke.LoadComp(c))
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
		goke.LoadComp[cgoke.Active](),
		goke.LoadComp[tag.Tags[effect.States]](),
		goke.LoadComp[tag.Tags[cond.Roles]](),
	)
}
