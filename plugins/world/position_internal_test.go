package world

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
)

func TestPosition_MaxSpeed_FollowsTheShorterSideAndTheTickRate(t *testing.T) {
	p := Position{AABB: plane.NewAABB(geom.NewVec(0, 0), 40, 10)}

	if got := p.MaxStep(); got != 5 {
		t.Errorf("MaxStep of a 40x10 entity = %v, want 5 — half the shorter side", got)
	}
	if got := p.MaxSpeed(60); got != 300 {
		t.Errorf("MaxSpeed at 60 ticks a second = %v, want 300", got)
	}
}
