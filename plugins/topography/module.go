package topography

import (
	"time"

	"github.com/kjkrol/goke/v3"
	irelief "github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/world/clock"
)

var _ goke.Module = (*module)(nil)

// module runs the heights' entity, the cameras and the shaping at once, and the altitudes in the
// simulation.
type module struct {
	heights  goke.System
	cameras  goke.System
	shaping  goke.System
	altitude goke.System
	clock    *clock.Clock // the world's; nil, run at once

	heightsRun, camerasRun, shapingRun, altitudeRun goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) {
	m.heightsRun = ecs.RegSys(m.heights) // first: a loaded game's heights before anything reads them
	m.camerasRun = ecs.RegSys(m.cameras)
	m.shapingRun = ecs.RegSys(m.shaping)
	m.altitudeRun = ecs.RegSys(m.altitude)
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.heightsRun, d)
	ctx.Run(m.camerasRun, d)
	ctx.Run(m.shapingRun, d)
	ctx.Sync()
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.altitudeRun, step)
		ctx.Sync()
	})
}

// SetupSystems is empty: the heights find or make their entity in their own Init.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the heights' component — see goke.CompProvider.
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[irelief.Heights]()}
}
