package entity_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity"
)

// SetDelta faces the way of the rate; an entity backing away goes on backing, facing against it.
func TestVelocity_SetDeltaKeepsABackingEntityBacking(t *testing.T) {
	on := entity.Velocity{Dir: geom.NewVec(1, 0), Value: 2}
	on.SetDelta(geom.NewVec(0, 3))
	if on.Dir != geom.NewVec(0, 1) || on.Value != 3 {
		t.Errorf("walking on, set to (0, 3): faces %v at %v; want south at 3", on.Dir, on.Value)
	}
	back := entity.Velocity{Dir: geom.NewVec(1, 0), Value: -2}
	back.SetDelta(geom.NewVec(-3, 0))
	if back.Dir != geom.NewVec(1, 0) || back.Value != -3 {
		t.Errorf("backing west, set to (-3, 0): faces %v at %v; want still east, backing at 3", back.Dir, back.Value)
	}
	if d := back.Delta(); d != geom.NewVec(-3, 0) {
		t.Errorf("backing velocity's rate is %v, want (-3, 0)", d)
	}
}
