package cameras

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/motion/steering"
)

var _ goke.Module = (*module)(nil)

type module struct {
	p        *Plugin
	views    goke.Runnable // nil for plain cameras
	runnable goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) {
	if v := m.p.views; v != nil {
		m.views = ecs.RegSys(&riding{keeper: v.Keeper(&m.p.orders)})
	}
	m.runnable = ecs.RegSys(&cameraSystem{p: m.p})
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	if m.views != nil {
		ctx.Run(m.views, d)
	}
	ctx.Run(m.runnable, d)
	ctx.Sync()
}

func (m *module) SetupSystems() []goke.System { return nil }

// the views write the eye's part of what a hand drives, in a world without motion too
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[steering.Driven]()}
}
