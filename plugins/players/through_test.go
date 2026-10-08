package players_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/render"
)

// A picture of the world tells its player where it lies on the screen: the mouse there is that
// player's, its commands through the picture's camera.
func TestThrough_TellsThePlayerWhereItsPictureLies(t *testing.T) {
	r := newRig(t)
	half := geom.NewAABB(geom.NewVec(400, 0), geom.NewVec(800, 600))
	r.wire.Over(half, render.NewFeed(r.cam, nil))
	if a := r.local.Area(); a != half {
		t.Errorf("the player's area is %v, want its picture's %v", a, half)
	}
}

// A player acts through one picture at a time: two in one pass of input panic.
func TestThrough_TwoPicturesOfOnePlayerInOnePassPanic(t *testing.T) {
	r := newRig(t)
	defer func() {
		if recover() == nil {
			t.Error("a second picture of the player in the pass was taken")
		}
	}()
	r.p.Through(r.local).Over(geom.AABB{}, render.NewFeed(r.cam, nil))
}
