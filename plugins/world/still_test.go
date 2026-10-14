package world_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
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
