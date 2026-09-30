package vision

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world/clock"
)

var _ goke.Module = (*module)(nil)

// module registers ScanSystem as vision's single per-tick system.
type module struct {
	sys      *ScanSystem
	runnable goke.Runnable
	clock    *clock.Clock // the world's; nil, run at once
}

func newModule(space *aabbworld.Space, host *host.PairHost[Sighting], heights *heights, coverOf func() board.Cover, workers int) *module {
	m := &module{sys: newScanSystem(space, host)}
	m.sys.coverOf = coverOf
	m.sys.Workers(workers)
	if heights != nil {
		m.sys.heights, m.sys.groundOf, m.sys.step = true, heights.groundOf, heights.step
		m.sys.scanner.heights, m.sys.scanner.step = true, heights.step
		m.sys.bend, m.sys.scanner.bend, m.sys.covering.bend = heights.bend, heights.bend, heights.bend
	}
	return m
}

// heights is what the scan needs of a world with heights: where to find its Ground, and the step the
// game asked for (0: the Ground's own).
type heights struct {
	groundOf func() board.Heights
	step     float64
	bend     float64 // how far the ground d off sinks under an eye's level, per d² (world.Scale.Bend)
}

// =================================================================
// goke.Module contract
// =================================================================

// RegSystems registers the scan as the per-tick system — see [goke.Module].
func (m *module) RegSystems(ecs *goke.ECS) {
	if m.runnable != nil {
		return
	}
	m.runnable = ecs.RegSys(m.sys)
}

// RunPlan hands the scan to the simulation: every step, every observer looks.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.runnable, step)
		ctx.Sync()
	})
}

// SetupSystems is empty — vision has no one-time seeding of its own.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types vision owns — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[Sight](),
		goke.LoadComp[SightOutline](),
		goke.LoadComp[Transparency](),
	}
}
