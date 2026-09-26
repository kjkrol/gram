package isometry

import (
	"time"

	"github.com/kjkrol/goke/v3"
)

var _ goke.Module = (*module)(nil)

// module runs the view's one system every tick.
type module struct {
	sys      *cameraSystem
	runnable goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) { m.runnable = ecs.RegSys(m.sys) }

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.runnable, d)
	ctx.Sync()
}

// SetupSystems is empty: the view spawns nothing.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps is empty: the view keeps no component of its own.
func (m *module) LoadComps() []goke.CompToken { return nil }
