package vision_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

type looked struct{}

func (looked) Command() {}

type lookeds struct{ command.Queue[looked] }

func (l *lookeds) Queues() []command.CommandQueue     { return []command.CommandQueue{&l.Queue} }
func (l *lookeds) DefaultBindings() []control.Binding { return nil }

// scanned 15 times a second at 60 steps a second, a quarter of the observers look each step
func TestFrequency_ObserversTakeTurnsAndTheRulesGoOnEveryStep(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 2000, Height: 2000},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 1, MaxSize: 400},
	})
	var heard lookeds
	if err := w.Carry(&heard); err != nil {
		t.Fatal(err)
	}
	v := vision.NewPlugin(w).WithFrequency(15)
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := v.Install(ctx); err != nil {
		t.Fatal(err)
	}
	look := act.Branch[vision.Sighting]{Name: "look", When: cond.Self.If(cond.Always), Then: act.Self.Do(looked{})}
	if err := ctx.Deliver(look); err != nil {
		t.Fatal(err)
	}
	const observers = 8
	kind.Define[spawn](w.Kinds(), "observer", kind.Spec{
		comp.Load(at),
		comp.Const(world.Velocity{}),
		comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: 600}),
		comp.Const(world.Eye{Angle: math.Pi / 2}),
		comp.Const(vision.Sighted{}),
	})
	kind.Define[spawn](w.Kinds(), "target", kind.Spec{comp.Load(at), comp.Const(world.Velocity{})})
	for i := range observers {
		row := spawn{x: 100, y: 100 + 50*float64(i)}
		w.Write().Seed(kind.Named[spawn](w.Kinds(), "observer").Entry(row))
	}
	w.Write().Seed(kind.Named[spawn](w.Kinds(), "target").Entry(spawn{x: 400, y: 100, size: 400}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var sighted goke.Comp[vision.Sighted]
	var query *goke.Query
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&sighted).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		v.RunPlan(rc, d)
		w.Write().Clock().Replay(rc, d)
	})
	seeing := func() int {
		n := 0
		for query.All(); query.Next(); {
			for _, s := range sighted.Slice(query.Cursor()) {
				if s.Count > 0 {
					n++
				}
			}
		}
		return n
	}
	for step := 1; step <= 4; step++ {
		ctx.ecs.Tick(time.Second / 60)
		if n, want := seeing(), 2*step; n != want {
			t.Errorf("after step %d %d observers see the target, want %d", step, n, want)
		}
		if n := len(drain(&heard)); n != observers {
			t.Errorf("step %d: the Sighting rules went on %d times, want once for each of %d",
				step, n, observers)
		}
	}
}

func drain(l *lookeds) []command.Issued[looked] {
	var out []command.Issued[looked]
	l.Drain(func(i command.Issued[looked]) { out = append(out, i) })
	return out
}
