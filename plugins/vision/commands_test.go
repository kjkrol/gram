package vision_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/vision"
)

func TestCones_ToggleTheViewsDrawn(t *testing.T) {
	w := testWorldPlugin()
	v := vision.NewPlugin(w)
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatalf("world Install: %v", err)
	}
	if err := v.Install(ctx); err != nil {
		t.Fatalf("vision Install: %v", err)
	}
	r := rendererOf(t, ctx, v)
	if err := w.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		v.RunPlan(rc, d)
		w.Clock().Replay(rc, d)
	})

	queues := v.Queues()
	if len(queues) != 1 || queues[0].Accepts() != reflect.TypeFor[vision.Cones]() {
		t.Fatalf("Queues = %v, want the one for Cones", queues)
	}
	if v.ConesShown() || !r.Hidden() {
		t.Fatal("the views start shown, want hidden until asked for")
	}
	queues[0].Put(command.Nobody, vision.Cones{Shown: true})
	ctx.ecs.Tick(time.Second / 60)
	if !v.ConesShown() || r.Hidden() {
		t.Errorf("after Cones the plugin shows %v and the renderer hides %v, want both shown",
			v.ConesShown(), r.Hidden())
	}
	queues[0].Put(command.Nobody, vision.Cones{Shown: false})
	ctx.ecs.Tick(time.Second / 60)
	if v.ConesShown() || !r.Hidden() {
		t.Errorf("after a second Cones the plugin shows %v and the renderer hides %v, "+
			"want both hidden", v.ConesShown(), r.Hidden())
	}
}

func TestCones_ShowConesBeforeInstallReachesTheRenderer(t *testing.T) {
	v := vision.NewPlugin(testWorldPlugin())
	v.ShowCones()
	if rendererOf(t, nil, v).Hidden() {
		t.Error("a renderer built after ShowCones is hidden")
	}
}

func TestCones_DefaultBindingIsShiftC(t *testing.T) {
	b := vision.NewPlugin(testWorldPlugin()).DefaultBindings()
	if len(b) != 1 {
		t.Fatalf("%d bindings, want one", len(b))
	}
	if _, ok := b[0].Build(control.Context{}); !ok || b[0].Type() != reflect.TypeFor[vision.Cones]() {
		t.Errorf("the binding issues %v, want Cones", b[0].Type())
	}
	if k, ok := b[0].Trigger.(control.KeyPress); !ok || !k.Mods.Shift || k.Key.String() != "C" {
		t.Errorf("the binding is on %v, want Shift+C", b[0].Trigger)
	}
}
