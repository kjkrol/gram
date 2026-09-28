package topography

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/clock"
)

var _ goke.Module = (*module)(nil)

// module runs the heights' entity, the cameras and the shaping at once, and the altitudes in the
// simulation.
type module struct {
	heights  *heightsSystem
	cameras  *cameraSystem
	shaping  *shapingSystem
	altitude *altitudeSystem
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
func (m *module) LoadComps() []goke.CompToken { return []goke.CompToken{goke.LoadComp[Heights]()} }

var _ goke.System = (*heightsSystem)(nil)

// heightsSystem keeps the ground's heights on the topography's own entity: found after a load —
// the relief takes them over — or made at Setup; every tick the entity holds the relief's values.
type heightsSystem struct {
	relief *Relief
	query  *goke.Query
	comp   goke.Comp[Heights]
	spawn  goke.Comp[Heights]
}

func (s *heightsSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.comp).Build()
	for s.query.All(); s.query.Next(); {
		s.relief.adopt(s.comp.Slice(s.query.Cursor())[0])
		return
	}
	f := si.NewFactory(&s.spawn)
	f.Create(1)
	for f.Next() {
		s.spawn.Slice(&f.Cursor)[0] = s.relief.Heights()
	}
}

func (s *heightsSystem) Update(*goke.CmdBuf, time.Duration) {
	for s.query.All(); s.query.Next(); {
		s.comp.Slice(s.query.Cursor())[0] = s.relief.Heights()
		return
	}
}
