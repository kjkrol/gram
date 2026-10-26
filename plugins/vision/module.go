package vision

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/board/ground"
)

var _ goke.Module = (*module)(nil)

type module struct {
	sys      *ScanSystem
	runnable goke.Runnable
	clock    *clock.Clock
	step     clock.Block // bound once, so a tick allocates no method value
}

func newModule(space *aabbworld.SpaceRead, host *host.PairRules[Sighting], heights *heights,
	coverOf func(w int) ground.Cover, workers int) *module {
	m := &module{sys: newScanSystem(space, host)}
	m.step = m.simulate
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
	step     float32
	bend     float32 // per d², world.Scale.Bend
}

func (m *module) RegSystems(ecs *goke.ECS) {
	if m.runnable != nil {
		return
	}
	m.runnable = ecs.RegSys(m.sys)
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.clock, ctx, d, m.step)
}

func (m *module) simulate(ctx goke.RunCtx, step time.Duration) {
	ctx.Run(m.runnable, step)
	ctx.Sync()
}

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[Sight](),
		goke.LoadComp[Sighted](),
		goke.LoadComp[SightOutline](),
		goke.LoadComp[Transparency](),
		goke.LoadComp[Watched](),
	}
}
