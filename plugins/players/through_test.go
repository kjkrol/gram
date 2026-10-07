package players_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
)

// A picture of the world tells its player where it lies on the screen: the mouse there is that
// player's.
func TestThrough_TellsThePlayerWhereItsPictureLies(t *testing.T) {
	r := newRig(t)
	half := geom.NewAABB(geom.NewVec(400, 0), geom.NewVec(800, 600))
	r.p.Through(r.local).Over(half)
	if a := r.local.Area(); a != half {
		t.Errorf("the player's area is %v, want its picture's %v", a, half)
	}
}
