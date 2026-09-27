package board

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/effects"
)

var _ goke.Module = (*module)(nil)

// module runs, every tick, the cells — shaping, and what effects changed — then the altitudes and
// the standing report.
type module struct {
	cells    *cellSystem
	altitude *altitudeSystem // Quasi3D only
	standing *standingSystem

	cellsRunnable    goke.Runnable
	altitudeRunnable goke.Runnable
	standingRunnable goke.Runnable
}

// =================================================================
// goke.Module contract
// =================================================================

func (m *module) RegSystems(ecs *goke.ECS) {
	m.cellsRunnable = ecs.RegSys(m.cells)
	if m.altitude != nil {
		m.altitudeRunnable = ecs.RegSys(m.altitude)
	}
	m.standingRunnable = ecs.RegSys(m.standing)
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.cellsRunnable, d)
	if m.altitude != nil {
		ctx.Run(m.altitudeRunnable, d)
	}
	ctx.Run(m.standingRunnable, d)
	ctx.Sync()
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
