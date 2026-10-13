package entity_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity"
)

func TestVelocity_SideRunsAcrossTheHeadingToTheRight(t *testing.T) {
	v := entity.Velocity{Dir: geom.NewVec(1, 0), Value: 2, Side: 3}
	if d := v.Delta(); d != geom.NewVec(2, 3) {
		t.Errorf("east at 2, strafing right at 3: the rate is %v, want (2, 3), south being right", d)
	}
	v.SetDelta(geom.NewVec(0, 5))
	if v.Side != 0 || v.Dir != geom.NewVec(0, 1) {
		t.Errorf("a rate set: Side %v, Dir %v; want the strafe gone, facing south", v.Side, v.Dir)
	}
	v = entity.Velocity{Dir: geom.NewVec(1, 0), Value: 2, Side: 3}
	v.Slow(geom.NewVec(1, 1))
	if v.Side != 0 || v.Value != 1 {
		t.Errorf("slowed: Side %v, Value %v; want the strafe gone and 1 along", v.Side, v.Value)
	}
}

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
