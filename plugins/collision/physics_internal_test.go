package collision

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
)

func TestPhysics_AsTheResponseTakesIt(t *testing.T) {
	box := geom.NewVec(2, 3)
	cases := map[string]struct {
		p       Physics
		inv, by float32
	}{
		"a weight":                 {Physics{Mass: 4, Restitution: 0.5}, 0.25, 0.5},
		"unnamed mass is the area": {Physics{Restitution: 1}, 1.0 / 6, 1},
		"a wall":                   {Physics{Mass: f32.Inf(1), Restitution: 1}, 0, 1},
		"restitution past its end": {Physics{Mass: 1, Restitution: 5}, 1, 1},
		"restitution below zero":   {Physics{Mass: 1, Restitution: -1}, 1, 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := c.p.inverseMass(box); got != c.inv {
				t.Errorf("inverseMass = %v, want %v", got, c.inv)
			}
			if got := c.p.bounce(); got != c.by {
				t.Errorf("bounce = %v, want %v", got, c.by)
			}
		})
	}
	if got := (Physics{}).inverseMass(geom.Vec{}); got != 0 {
		t.Errorf("a box of no area weighs 1/%v, want nothing to move", got)
	}
}
