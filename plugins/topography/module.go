package topography

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	irelief "github.com/kjkrol/gram/plugins/topography/internal/relief"
)

var _ goke.Module = (*module)(nil)

type module struct {
	heights  goke.System
	cameras  goke.System
	shaping  goke.System
	altitude goke.System
	clock    *clock.Clock // nil: at once

	heightsRun, camerasRun, shapingRun, altitudeRun goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) {
	// first: a loaded game's heights before anything reads them
	m.heightsRun = ecs.RegSys(m.heights)
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

// The heights find or make their entity in their own Init.
func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[irelief.Heights](),
		// the relief reads them in a world without motion too
		goke.LoadComp[steering.Course](),
		goke.LoadComp[steering.Driven](),
		goke.LoadComp[motion.Drift](),
	}
}
