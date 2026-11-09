package motion_test

import (
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

func TestMoveSystem_CarriesSubUnitSpeedEveryTick(t *testing.T) {
	r := newRig(t, 0, box(100, 100, 10), world.Velocity{Dir: geom.NewVec(1, 0), Value: 30}, nil)
	const step = 30 * (16666666.0 / 1e9)

	start := r.thing(t).Pos.TopLeft.X
	for tick := 1; tick <= 4; tick++ {
		r.ticks(1)
		want := start + step*float32(tick)
		if got := r.thing(t).Pos.TopLeft.X; f32.Abs(got-want) > 1e-6 {
			t.Fatalf("after %d ticks the entity is at %v, want %v — half a unit per tick, every "+
				"tick", tick, got, want)
		}
	}
}

func TestMoveSystem_WhatStandsForGoodTakesNoStepNorLeaves(t *testing.T) {
	r := newRig(t, aabbworld.OpenX, box(985, 100, 10),
		world.Velocity{Dir: geom.NewVec(1, 0), Value: 600}, []comp.Comp{comp.Const(world.Fixed{})})

	r.ticks(10)
	if got := r.thing(t).Pos.TopLeft; got != geom.NewVec(985, 100) {
		t.Errorf("standing for good with a speed, it went to %v; want it where it stands", got)
	}
	found := 0
	r.w.Read(0).Space().Query(geom.NewAABBAt(geom.NewVec(985, 100), 10, 10),
		aabbworld.AnyCapability, func(uid.UID64) { found++ })
	if found != 1 {
		t.Errorf("the space finds %d where it stands, want it", found)
	}
}
