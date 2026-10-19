package players_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

func TestThrough_TellsThePlayerWhereItsPictureLies(t *testing.T) {
	r := newRig(t)
	half := geom.NewAABB(geom.NewVec(400, 0), geom.NewVec(800, 600))
	r.wire.Over(half, render.NewFeed(r.cam, nil))
	if a := r.local.Area(); a != half {
		t.Errorf("the player's area is %v, want its picture's %v", a, half)
	}
}

func TestThrough_TwoPicturesOfOnePlayerInOnePassPanic(t *testing.T) {
	r := newRig(t)
	defer func() {
		if recover() == nil {
			t.Error("a second picture of the player in the pass was taken")
		}
	}()
	r.p.Through(r.local).Over(geom.AABB{}, render.NewFeed(r.cam, nil))
}

func TestThrough_ACoveredPictureTakesNothingOfTheCursorsPlace(t *testing.T) {
	r := newRig(t)
	r.cam.SetViewport(800, 600)
	r.bind(control.Build(control.CursorAtEdge{}, "scroll", orderOf(1)))
	atEdge := func() int {
		r.handle(&control.InputEvents{MousePos: geom.NewVec(790, 300), WindowFillsScreen: true})
		return len(r.drained())
	}
	hover := r.wire.(ui.Hovered)
	for _, c := range []struct {
		on   bool
		want int
	}{{true, 1}, {false, 0}, {true, 1}} {
		hover.Hover(c.on)
		if got := atEdge(); got != c.want {
			t.Errorf("the cursor on the picture %v: %d orders at the edge, want %d", c.on, got,
				c.want)
		}
	}
}
