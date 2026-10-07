package cameras

import (
	"time"

	"github.com/kjkrol/goke/v3"
)

var _ goke.Module = (*module)(nil)

// module runs the cameras' system.
type module struct {
	p        *Plugin
	runnable goke.Runnable
}

// =================================================================
// goke.Module contract
// =================================================================

func (m *module) RegSystems(ecs *goke.ECS) {
	m.runnable = ecs.RegSys(&cameraSystem{p: m.p})
}

// RunPlan moves the cameras as told and keeps the fastened ones over their entities.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.runnable, d)
	ctx.Sync()
}

// SetupSystems is empty — the cameras seed nothing.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps is empty — the cameras own no components.
func (m *module) LoadComps() []goke.CompToken { return nil }
