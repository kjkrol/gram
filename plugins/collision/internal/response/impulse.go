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

// Side is one side of a contact: how much of an impulse it takes (InvMass, 0 for nothing moves
// it), how much of its approach it gives back (Bounce, 0 to 1) and its velocity, changed in
// place. A Steered side changes its speed alone, its heading being its steering's.
type Side struct {
	InvMass float64
	Bounce  float64
	Vel     *world.Velocity
	Steered bool
}

// Exchange trades the impulse of two sides along n, changing their velocities, and returns it;
// none unless they are closing, the lower Bounce of the two given back.
func Exchange(a, b Side, n geom.Vec) float64 {
	deltaA, deltaB := a.Vel.Delta(), b.Vel.Delta()
	impact := impulse(a, b, deltaA, deltaB, n)
	if impact == 0 {
		return 0
	}
	if a.InvMass != 0 {
		a.set(geom.NewVec(deltaA.X+impact*a.InvMass*n.X, deltaA.Y+impact*a.InvMass*n.Y))
	}
	if b.InvMass != 0 {
		b.set(geom.NewVec(deltaB.X-impact*b.InvMass*n.X, deltaB.Y-impact*b.InvMass*n.Y))
	}
	return impact
}

func (s Side) set(d geom.Vec) {
	if s.Steered {
		s.Vel.Slow(d)
		return
	}
	s.Vel.SetDelta(d)
}

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
