package board

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/effects"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

var _ goke.Module = (*module)(nil)

// module runs, every tick, the cells — shaping, and what effects changed — then the terrain bodies
// when the plugin was built WithCollision, then the altitudes and the standing report.
type module struct {
	cells    *cellSystem
	altitude *altitudeSystem // Quasi3D only
	standing *standingSystem
	bodies   *terrainBodySystem

	cellsRunnable    goke.Runnable
	altitudeRunnable goke.Runnable
	standingRunnable goke.Runnable
	bodiesRunnable   goke.Runnable
}

// =================================================================
// goke.Module contract
// =================================================================

func (m *module) RegSystems(ecs *goke.ECS) {
	m.cellsRunnable = ecs.RegSys(m.cells)
	if m.bodies != nil {
		m.bodiesRunnable = ecs.RegSys(m.bodies)
	}
	if m.altitude != nil {
		m.altitudeRunnable = ecs.RegSys(m.altitude)
	}
	m.standingRunnable = ecs.RegSys(m.standing)
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.cellsRunnable, d)
	if m.bodies != nil {
		ctx.Run(m.bodiesRunnable, d)
	}
	if m.altitude != nil {
		ctx.Run(m.altitudeRunnable, d)
	}
	ctx.Run(m.standingRunnable, d)
	ctx.Sync()
}

// SetupSystems is empty — the cells and the bodies build themselves in their own Init.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types board writes or reads, so a save loads without the vision
// and effects plugins — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	tokens := []goke.CompToken{
		goke.LoadComp[Cell](), goke.LoadComp[Mover](),
		goke.LoadComp[Plot](), goke.LoadComp[Ground](),
		goke.LoadComp[effects.Active](), goke.LoadComp[effects.Idle](),
	}
	if m.bodies != nil {
		tokens = append(tokens, goke.LoadComp[vision.Transparency](), goke.LoadComp[world.Layers]())
	}
	return tokens
}
