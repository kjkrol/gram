package control_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
)

type grab struct{}

func (grab) Command() {}

func TestBinding_AYieldingBindingOverlapsOnlyAnotherYielding(t *testing.T) {
	held := control.ButtonHeld{Button: control.MouseButtonRight}
	build := func(control.Context) (grab, bool) { return grab{}, true }
	plain := control.Build(held, "turn", build)
	yielding := control.Build(held, "grab", build).Yielding()
	if plain.Yields() || !yielding.Yields() {
		t.Errorf("Yields: the plain binding %v, the yielding %v", plain.Yields(), yielding.Yields())
	}
	if plain.Overlaps(yielding) || yielding.Overlaps(plain) {
		t.Error("a yielding binding overlaps a plain one on its trigger, want them apart")
	}
	if !plain.Overlaps(plain) || !yielding.Overlaps(yielding) {
		t.Error("two plain or two yielding bindings on one trigger do not overlap, want them to")
	}
}

func TestContext_DraggedIsPastTheSlopOfAClick(t *testing.T) {
	ctx := control.Context{Start: geom.NewVec(10, 10), Cursor: geom.NewVec(14, 10)}
	if ctx.Dragged() {
		t.Error("4 pixels from the press is a drag, want a click's slop")
	}
	ctx.Cursor = geom.NewVec(15, 10)
	if !ctx.Dragged() {
		t.Error("5 pixels from the press is no drag")
	}
	// the slop is in shown pixels: twice as many of the camera's at a resolution of 2
	ctx.Resolution = 2
	if ctx.Dragged() {
		t.Error("5 camera pixels at a resolution of 2 are a drag, want within a slop of 8")
	}
	ctx.Cursor = geom.NewVec(19, 10)
	if !ctx.Dragged() {
		t.Error("9 camera pixels at a resolution of 2 are no drag")
	}
}
