package world_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

func TestStill_HoldsForAnEntityNeitherWalkingNorStrafing(t *testing.T) {
	still := world.Still()
	for name, vel := range map[string]world.Velocity{
		"standing":   {Dir: geom.NewVec(1, 0)},
		"walking":    {Dir: geom.NewVec(1, 0), Value: 3},
		"backing":    {Dir: geom.NewVec(1, 0), Value: -3},
		"strafing":   {Dir: geom.NewVec(1, 0), Side: 2},
		"not at all": {},
	} {
		got := still.Holds(world.Moving{Base: &world.Base{Vel: vel}})
		want := vel.Value == 0 && vel.Side == 0
		if got != want {
			t.Errorf("%s: still %v, want %v", name, got, want)
		}
	}
}

func TestBraking_HoldsWhileTheCourseAsksForLessSpeedThanItHas(t *testing.T) {
	braking := world.Braking()
	base := &world.Base{Vel: world.Velocity{Dir: geom.NewVec(1, 0), Value: 3}}
	for name, c := range map[string]struct {
		course *steering.Course
		want   bool
	}{
		"no course":       {nil, false},
		"walking on":      {&steering.Course{Speed: 3, WantSpeed: 3}, false},
		"speeding up":     {&steering.Course{Speed: 2, WantSpeed: 3}, false},
		"braking":         {&steering.Course{Speed: 3, WantSpeed: 0}, true},
		"braking backing": {&steering.Course{Speed: -3, WantSpeed: 0}, true},
		"stopped":         {&steering.Course{Speed: 0, WantSpeed: 0}, false},
	} {
		if got := braking.Holds(world.Moving{Base: base, Course: c.course}); got != c.want {
			t.Errorf("%s: braking %v, want %v", name, got, c.want)
		}
	}
}
