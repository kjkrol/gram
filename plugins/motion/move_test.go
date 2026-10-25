package motion_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/world"
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
