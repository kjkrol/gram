package vision

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/ground"
)

var _ goke.Module = (*module)(nil)

type module struct {
	sys      *ScanSystem
	runnable goke.Runnable
	clock    *clock.Clock
}

func newModule(space *aabbworld.Space, host *plugin.PairRules[Sighting], heights *heights, coverOf func() ground.Cover, workers int) *module {
	m := &module{sys: newScanSystem(space, host)}
	m.sys.coverOf = coverOf
	m.sys.Workers(workers)
	if heights != nil {
		m.sys.heights, m.sys.groundOf, m.sys.step = true, heights.groundOf, heights.step
		m.sys.scanner.heights, m.sys.scanner.step = true, heights.step
		m.sys.bend, m.sys.scanner.bend, m.sys.covering.bend = heights.bend, heights.bend, heights.bend
		m.sys.scanner.profiling.bend = heights.bend
	}
	return m
}

type heights struct {
	groundOf func() ground.Heights
	step     float64
	bend     float64 // per d², world.Scale.Bend
}

func (m *module) RegSystems(ecs *goke.ECS) {
	if m.runnable != nil {
		return
	}
	m.runnable = ecs.RegSys(m.sys)
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.runnable, step)
		ctx.Sync()
	})
}

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[Sight](),
		goke.LoadComp[Sighted](),
		goke.LoadComp[SightOutline](),
		goke.LoadComp[Transparency](),
	}
}
