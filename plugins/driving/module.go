package driving

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/world/steering"
)

var _ goke.Module = (*module)(nil)

// module puts the hands on the units at once and drives them in the simulation.
type module struct {
	hand  *handSystem
	drive *driveSystem
	clock *clock.Clock // the world's; nil, run at once

	handRunnable, driveRunnable goke.Runnable
}

// =================================================================
// goke.Module contract
// =================================================================

func (m *module) RegSystems(ecs *goke.ECS) {
	m.handRunnable = ecs.RegSys(m.hand)
	m.driveRunnable = ecs.RegSys(m.drive)
}

// RunPlan writes the tick's hands on the units at once and drives them every step of the
// simulation.
func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(m.handRunnable, d)
	ctx.Sync()
	clock.Simulate(m.clock, ctx, d, func(ctx goke.RunCtx, step time.Duration) {
		ctx.Run(m.driveRunnable, step)
		ctx.Sync()
	})
}

// SetupSystems is empty — the driving seeds nothing.
func (m *module) SetupSystems() []goke.System { return nil }

// LoadComps lists the component types the driving owns — see [goke.CompProvider].
func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[tag.Tags[States]](), goke.LoadComp[steering.Driven]()}
}
