package cameras

import (
	"time"

	"github.com/kjkrol/goke/v3"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p        *Plugin
	runnable goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) {
	m.runnable = ecs.RegSys(&cameraSystem{p: m.p})
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.runnable, d)
	ctx.Sync()
}

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken { return nil }
