package climate

import (
	"time"

	"github.com/kjkrol/goke/v3"
)

var _ goke.Module = (*module)(nil)

// module runs the weather's one system every tick.
type module struct {
	sys      *weatherSystem
	runnable goke.Runnable
}

func (m *module) RegSystems(ecs *goke.ECS) { m.runnable = ecs.RegSys(m.sys) }

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.runnable, d)
	ctx.Sync()
}

// SetupSystems is empty: the weather finds or makes its entity in its own Init.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the weather's one component — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken { return []goke.CompToken{goke.LoadComp[Weather]()} }
