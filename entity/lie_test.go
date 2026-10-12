package entity_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity"
)

// A Lie lies Length along the way its entity is headed and Width across, on its middle: headed
// south, a sprite authored facing east has its top-left in the north-east.
func TestLie_CornersRunAlongTheHeading(t *testing.T) {
	lie := entity.Lie{Length: 40, Width: 10}
	m := geom.NewVec(100, 100)
	for _, c := range []struct {
		name string
		dir  geom.Vec
		want [4]geom.Vec
	}{
		{"east", geom.NewVec(1, 0), [4]geom.Vec{
			geom.NewVec(80, 95), geom.NewVec(120, 95), geom.NewVec(80, 105), geom.NewVec(120, 105),
		}},
		{"south", geom.NewVec(0, 1), [4]geom.Vec{
			geom.NewVec(105, 80), geom.NewVec(105, 120), geom.NewVec(95, 80), geom.NewVec(95, 120),
		}},
		{"standing", geom.Vec{}, [4]geom.Vec{
			geom.NewVec(80, 95), geom.NewVec(120, 95), geom.NewVec(80, 105), geom.NewVec(120, 105),
		}},
	} {
		if got := lie.Corners(m, c.dir); got != c.want {
			t.Errorf("headed %s: %v, want %v", c.name, got, c.want)
		}
	}
}

// A Lie Ahead of its box lies that far along the way its entity is headed: a bridge's head, half
// a link long from the middle of its tile.
func TestLie_AheadShiftsTheRectangleAlongTheHeading(t *testing.T) {
	lie := entity.Lie{Length: 20, Width: 10, Ahead: 10}
	got := lie.Corners(geom.NewVec(100, 100), geom.NewVec(0, 1))
	want := [4]geom.Vec{
		geom.NewVec(105, 100), geom.NewVec(105, 120), geom.NewVec(95, 100), geom.NewVec(95, 120),
	}
	if got != want {
		t.Errorf("ahead by 10, headed south: %v, want %v", got, want)
	}
}
