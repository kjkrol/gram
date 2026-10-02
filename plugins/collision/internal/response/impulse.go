package response

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

// Normal is pen as a unit vector, false when there is no penetration to point along.
func Normal(pen geom.Vec) (geom.Vec, bool) {
	mag := math.Hypot(pen.X, pen.Y)
	if mag == 0 {
		return geom.Vec{}, false
	}
	return geom.NewVec(pen.X/mag, pen.Y/mag), true
}

// Side is one side of a contact as the impulse sees it: how much of an impulse it takes (InvMass;
// 0, nothing moves it), how much of its approach it gives back (Bounce, 0 to 1) and its velocity,
// changed in place.
type Side struct {
	InvMass float64
	Bounce  float64
	Vel     *world.Velocity
}

// Exchange trades the impulse of two sides along n — none unless they are closing, the lower
// Bounce of the two given back — changing their velocities, and returns it.
func Exchange(a, b Side, n geom.Vec) float64 {
	deltaA, deltaB := a.Vel.Delta(), b.Vel.Delta()
	impact := impulse(a, b, deltaA, deltaB, n)
	if impact == 0 {
		return 0
	}
	if a.InvMass != 0 {
		a.Vel.SetDelta(geom.NewVec(deltaA.X+impact*a.InvMass*n.X, deltaA.Y+impact*a.InvMass*n.Y))
	}
	if b.InvMass != 0 {
		b.Vel.SetDelta(geom.NewVec(deltaB.X-impact*b.InvMass*n.X, deltaB.Y-impact*b.InvMass*n.Y))
	}
	return impact
}

// impulse is what two sides moving by deltaA and deltaB exchange along n, zero unless closing.
func impulse(a, b Side, deltaA, deltaB, n geom.Vec) float64 {
	if a.InvMass+b.InvMass == 0 {
		return 0
	}
	approach := (deltaA.X-deltaB.X)*n.X + (deltaA.Y-deltaB.Y)*n.Y
	if approach > 0 {
		return 0
	}
	return -(1 + min(a.Bounce, b.Bounce)) * approach / (a.InvMass + b.InvMass)
}
