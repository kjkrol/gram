package motion

import (
	"time"

	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

var _ goke.Module = (*module)(nil)

type module struct {
	w       *world.Plugin
	movers  *host.Rules[Moving]
	leavers *host.Rules[Leaving]
	steer   *steering.System

	step clock.Block // bound once, so a tick allocates no method value

	steeringRunnable goke.Runnable
	velocityRunnable goke.Runnable
	moveRunnable     goke.Runnable
	exitRunnable     goke.Runnable

	// CompID 0 is an id like any other
	outside goke.CompID
	set     bool
}

func newModule(w *world.Plugin, movers *host.Rules[Moving], leavers *host.Rules[Leaving]) *module {
	m := &module{w: w, movers: movers, leavers: leavers, steer: steering.NewSystem()}
	m.step = m.simulate
	return m
}

func (m *module) RegSystems(ecs *goke.ECS) {
	if m.steeringRunnable != nil {
		return
	}
	pace := m.w.Pace(sector.Paced)
	m.steeringRunnable = ecs.RegSys(m.steer.Paced(pace))
	velocity := newVelocitySystem(m.movers)
	velocity.tick, velocity.rhythm = m.w.Read(0).Tick, pace
	m.velocityRunnable = ecs.RegSys(velocity)
	move := newMoveSystem(m.w.Write().Space())
	move.pace = pace
	m.moveRunnable = ecs.RegSys(move)
	exit := newExitSystem(m)
	exit.pace = pace
	m.exitRunnable = ecs.RegSys(exit)
}

func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
	clock.Simulate(m.w.Write().Clock(), ctx, d, m.step)
}

func (m *module) simulate(ctx goke.RunCtx, step time.Duration) {
	ctx.Run(m.steeringRunnable, step)
	ctx.Run(m.velocityRunnable, step)
	ctx.Run(m.moveRunnable, step)
	// lands the Outside marks, so a leaver is dealt with the step it left
	ctx.Sync()
	ctx.Run(m.exitRunnable, step)
	ctx.Sync()
}

func (m *module) SetupSystems() []goke.System { return nil }

func (m *module) LoadComps() []goke.CompToken {
	return []goke.CompToken{
		goke.LoadComp[steering.Steering](),
		goke.LoadComp[steering.Course](),
		goke.LoadComp[steering.Pace](),
		goke.LoadComp[steering.Driven](),
		goke.LoadComp[Outside](),
		goke.LoadComp[Drift](),
	}
}

func (m *module) leave(cb *goke.CmdBuf, id uid.UID64) {
	if !m.set {
		panic("motion: an entity left before motion was set up")
	}
	cb.AddOne(id, m.outside, Outside{})
}
