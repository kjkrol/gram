package driving

import (
	"time"

	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/world/steering"
)

var _ goke.Module = (*module)(nil)

type module struct {
	hand  *handSystem
	drive *driveSystem
	clock *clock.Clock

	handRunnable, driveRunnable goke.Runnable
}

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

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[tag.Tags[States]](), goke.LoadComp[steering.Driven]()}
}
