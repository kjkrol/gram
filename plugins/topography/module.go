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

// heightsSystem keeps the ground's heights on the topography's own entities, a run of them each
// (Heights): found after a load — the relief takes them over — or made at Setup; written anew
// whenever the relief has changed.
type heightsSystem struct {
	relief  *Relief
	query   *goke.Query
	comp    goke.Comp[Heights]
	spawn   goke.Comp[Heights]
	written uint64 // one more than the relief's version the runs hold; 0 none
}

func (s *heightsSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.comp).Build()
	var loaded []Heights
	for s.query.All(); s.query.Next(); {
		loaded = append(loaded, s.comp.Slice(s.query.Cursor())...)
	}
	if len(loaded) > 0 {
		if s.relief.adopt(loaded) {
			s.written = s.relief.Version() + 1
		}
		return
	}
	f := si.NewFactory(&s.spawn)
	f.Create(s.relief.Runs())
	run := 0
	for f.Next() {
		for i := range f.Cursor.IDs {
			h := &s.spawn.Slice(&f.Cursor)[i]
			h.First = uint32(run * HeightsRun)
			s.relief.fill(h)
			run++
		}
	}
	s.written = s.relief.Version() + 1
}

func (s *heightsSystem) Update(*goke.CmdBuf, time.Duration) {
	if s.written == s.relief.Version()+1 {
		return
	}
	for s.query.All(); s.query.Next(); {
		runs := s.comp.Slice(s.query.Cursor())
		for i := range runs {
			s.relief.fill(&runs[i])
		}
	}
	s.written = s.relief.Version() + 1
}
