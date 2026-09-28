package vision_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/vision"
)

// A Cones command issued to the plugin's queue hides the views drawn on the next RunPlan, and the
// next one shows them again; the renderer follows.
func TestCones_ToggleTheViewsDrawn(t *testing.T) {
	w := testWorldPlugin()
	v := vision.NewPlugin(w)
	v.WithRenderer(nil)
	r := v.Renderer().(*vision.Renderer)

	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatalf("world Install: %v", err)
	}
	if err := v.Install(ctx); err != nil {
		t.Fatalf("vision Install: %v", err)
	}
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
	if v.Hidden() || r.Hidden() {
		t.Fatal("the views start hidden")
	}
	queues[0].Put(control.Nobody, vision.Cones{})
	ctx.ecs.Tick(time.Second / 60)
	if !v.Hidden() || !r.Hidden() {
		t.Errorf("after Cones the plugin hides %v and the renderer %v, want both hidden", v.Hidden(), r.Hidden())
	}
	queues[0].Put(control.Nobody, vision.Cones{})
	ctx.ecs.Tick(time.Second / 60)
	if v.Hidden() || r.Hidden() {
		t.Errorf("after a second Cones the plugin hides %v and the renderer %v, want both shown", v.Hidden(), r.Hidden())
	}
}

// Hide before WithRenderer reaches the renderer built later.
func TestCones_HideReachesARendererBuiltLater(t *testing.T) {
	v := vision.NewPlugin(testWorldPlugin())
	v.Hide(true)
	v.WithRenderer(nil)
	if !v.Renderer().(*vision.Renderer).Hidden() {
		t.Error("a renderer built after Hide(true) is shown")
	}
}

func TestCones_DefaultBindingIsShiftC(t *testing.T) {
	b := vision.NewPlugin(testWorldPlugin()).DefaultBindings()
	if len(b) != 1 {
		t.Fatalf("%d bindings, want one", len(b))
	}
	if _, ok := b[0].Build(control.Context{}); !ok || b[0].Command() != reflect.TypeFor[vision.Cones]() {
		t.Errorf("the binding issues %v, want Cones", b[0].Command())
	}
	if k, ok := b[0].Trigger.(control.KeyPress); !ok || !k.Mods.Shift || k.Key.String() != "C" {
		t.Errorf("the binding is on %v, want Shift+C", b[0].Trigger)
	}
}
