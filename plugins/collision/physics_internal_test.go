package collision

import (
	"math"
	"testing"
)

// The response is given one over the weight, nothing for a wall, and Restitution held in 0 to 1.
func TestPhysics_AsTheResponseTakesIt(t *testing.T) {
	cases := map[string]struct {
		p       Physics
		inv, by float64
	}{
		"a weight":                 {Physics{Mass: 4, Restitution: 0.5}, 0.25, 0.5},
		"unnamed mass is default":  {Physics{Restitution: 1}, 1 / DefaultMass, 1},
		"a wall":                   {Physics{Mass: math.Inf(1), Restitution: 1}, 0, 1},
		"restitution past its end": {Physics{Mass: 1, Restitution: 5}, 1, 1},
		"restitution below zero":   {Physics{Mass: 1, Restitution: -1}, 1, 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := c.p.inverseMass(); got != c.inv {
				t.Errorf("inverseMass = %v, want %v", got, c.inv)
			}
			if got := c.p.bounce(); got != c.by {
				t.Errorf("bounce = %v, want %v", got, c.by)
			}
		})
	}
}
