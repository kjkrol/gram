package selection

import (
	"time"

	"github.com/kjkrol/goke/v3"
)

// module registers SelectionSystem and, after it, FollowSystem as selection's per-tick systems.
type module struct {
	sys            *SelectionSystem
	follow         *FollowSystem
	runnable       goke.Runnable
	followRunnable goke.Runnable
}

var _ goke.Module = (*module)(nil)

// =================================================================
// goke.Module contract
// =================================================================

// RegSystems registers sys as the per-tick system — see [goke.Module].
func (m *module) RegSystems(ecs *goke.ECS) {
	if m.runnable != nil {
		return
	}
	m.runnable = ecs.RegSys(m.sys)
	if m.follow != nil {
		m.followRunnable = ecs.RegSys(m.follow)
	}
}

// RunPlan runs the selection and then the camera following for this tick — call from your own
// Game.Loop closure.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.runnable, d)
	if m.followRunnable != nil {
		ctx.Run(m.followRunnable, d)
	}
	ctx.Sync()
}

// SetupSystems is empty — selection has no one-time seeding of its own.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types selection owns — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return nil // the family's Tags are declared by the world's Kinds through DefineTag
}
