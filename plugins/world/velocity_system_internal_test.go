package world

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

type moved struct{}

func (moved) Command() {}

func velocityTick(t *testing.T, share float64, rules ...rule.Rule) (paced, unpaced float64, gave []uid.UID64) {
	t.Helper()
	var commands control.Carrier
	var given control.Queue[moved]
	if err := commands.Carry(&given); err != nil {
		t.Fatal(err)
	}
	host := &plugin.Rules[Moving]{}
	for _, r := range rules {
		if err := host.Add(r); err != nil {
			t.Fatal(err)
		}
	}
	sys := newVelocitySystem(host)
	sys.tick = func(cb *goke.CmdBuf, dt time.Duration) plugin.Tick {
		return plugin.Tick{CmdBuf: cb, Dt: dt, Commands: &commands}
	}

	ecs := goke.New()
	var base goke.Comp[Base]
	var pace goke.Comp[steering.Pace]
	var q *goke.Query
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &pace)
		f.Create(1)
		f.Next()
		base.Slice(&f.Cursor)[0].Vel = Velocity{Dir: geom.NewVec(1, 0), Value: 100}
		pace.Slice(&f.Cursor)[0] = steering.Pace{Share: share}
		g := si.NewFactory(&base)
		g.Create(1)
		g.Next()
		base.Slice(&g.Cursor)[0].Vel = Velocity{Dir: geom.NewVec(1, 0), Value: 100}
		q = si.NewQueryBuilder(&base).Build()
	}})
	handle := ecs.RegSys(sys)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(handle, d)
		ctx.Sync()
	})
	ecs.Tick(time.Second)

	for q.All(); q.Next(); {
		for _, b := range base.Slice(q.Cursor()) {
			if b.Vel.Value != 100 {
				paced = b.Vel.Value
			} else {
				unpaced = b.Vel.Value
			}
		}
	}
	given.Drain(func(i control.Issued[moved]) { gave = append(gave, i.Entity) })
	return paced, unpaced, gave
}

func TestVelocitySystem_ScalesByThePace(t *testing.T) {
	paced, unpaced, _ := velocityTick(t, 0.25)
	if paced != 25 || unpaced != 100 {
		t.Errorf("speeds %v and %v, want 25 at a pace of a quarter and 100 without one", paced, unpaced)
	}
}

func TestVelocitySystem_RunsTheMovingRules(t *testing.T) {
	_, _, gave := velocityTick(t, 0.5, rule.Branch[Moving]{Name: "moving", Do: rule.Order(moved{})})
	if len(gave) != 2 || gave[0] == gave[1] {
		t.Errorf("moved given by %v, want once by each of the two", gave)
	}
}
