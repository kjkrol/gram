package response

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

func TestExchange(t *testing.T) {
	elastic := func(mass float64) Side { return Side{InvMass: 1 / mass, Bounce: 1} }
	wall := Side{Bounce: 1}

	cases := map[string]struct {
		a, b           Side
		deltaA, deltaB geom.Vec
		n              geom.Vec
		want           float64
	}{
		"equal masses, head-on":    {elastic(1), elastic(1), geom.NewVec(-3, 0), geom.NewVec(4, 0), geom.NewVec(1, 0), 7},
		"already separating":       {elastic(1), elastic(1), geom.NewVec(3, 0), geom.NewVec(-4, 0), geom.NewVec(1, 0), 0},
		"across the other axis":    {elastic(1), elastic(1), geom.NewVec(0, -3), geom.NewVec(0, 4), geom.NewVec(0, 1), 7},
		"heavy into light":         {elastic(9), elastic(1), geom.NewVec(2, 0), geom.NewVec(-2, 0), geom.NewVec(-1, 0), 7.2},
		"into an immovable side":   {elastic(1), wall, geom.NewVec(3, 4), geom.Vec{}, geom.NewVec(-1, 0), 6},
		"immovable into immovable": {wall, wall, geom.Vec{}, geom.Vec{}, geom.NewVec(1, 0), 0},
		"half the bounce":          {Side{InvMass: 1, Bounce: 0.5}, elastic(1), geom.NewVec(-3, 0), geom.NewVec(4, 0), geom.NewVec(1, 0), 5.25},
		"no bounce at all":         {Side{InvMass: 1}, elastic(1), geom.NewVec(-3, 0), geom.NewVec(4, 0), geom.NewVec(1, 0), 3.5},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var velA, velB world.Velocity
			velA.SetDelta(c.deltaA)
			velB.SetDelta(c.deltaB)
			c.a.Vel, c.b.Vel = &velA, &velB
			if got := Exchange(c.a, c.b, c.n); math.Abs(got-c.want) > 1e-9 {
				t.Errorf("Exchange = %v, want %v", got, c.want)
			}
		})
	}
}

// The impulse turns both velocities along the normal, each by its share, and leaves an immovable
// side's alone.
func TestExchange_TurnsTheVelocities(t *testing.T) {
	var velA, velB, still world.Velocity
	velA.SetDelta(geom.NewVec(-3, 0))
	velB.SetDelta(geom.NewVec(4, 0))
	Exchange(Side{InvMass: 1, Bounce: 1, Vel: &velA}, Side{InvMass: 1, Bounce: 1, Vel: &velB}, geom.NewVec(1, 0))
	if velA.Delta() != geom.NewVec(4, 0) || velB.Delta() != geom.NewVec(-3, 0) {
		t.Errorf("equal masses: %v and %v, want them swapped", velA.Delta(), velB.Delta())
	}
	velA.SetDelta(geom.NewVec(3, 0))
	Exchange(Side{InvMass: 1, Bounce: 1, Vel: &velA}, Side{Bounce: 1, Vel: &still}, geom.NewVec(-1, 0))
	if velA.Delta() != geom.NewVec(-3, 0) || still.Delta() != (geom.Vec{}) {
		t.Errorf("into a wall: %v and %v, want {-3,0} and the wall still", velA.Delta(), still.Delta())
	}
}

func TestNormal_ZeroPenetrationHasNoDirection(t *testing.T) {
	if _, ok := Normal(geom.Vec{}); ok {
		t.Error("Normal(zero) reported a direction")
	}
	if n, ok := Normal(geom.NewVec(-5, 0)); !ok || n != geom.NewVec(-1, 0) {
		t.Errorf("Normal({-5,0}) = %v (ok %v), want {-1,0}", n, ok)
	}
}
