package response

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/world"
)

// Normal is pen as a unit vector, false when there is no penetration to point along.
func Normal(pen geom.Vec) (geom.Vec, bool) {
	mag := f32.Hypot(pen.X, pen.Y)
	if mag == 0 {
		return geom.Vec{}, false
	}
	return geom.NewVec(pen.X/mag, pen.Y/mag), true
}

// Side is one side of a contact: how much of an impulse it takes (InvMass, 0 for nothing moves
// it), how much of its approach it gives back (Bounce, 0 to 1) and its velocity, changed in
// place. A Steered side changes its speed alone, its heading being its steering's.
type Side struct {
	InvMass float32
	Bounce  float32
	Vel     *world.Velocity
	Steered bool
	// the speed its steering holds, slowed in the same share, so the next tick keeps what the
	// impulse took; nil, none
	Speed *float32
}

// Share is the part of a push apart that moves the side whose inverse mass is a, the other's
// being b: the lighter moves more, and two that nothing moves share it evenly.
func Share(a, b float32) float32 {
	if a+b == 0 {
		return 0.5
	}
	return a / (a + b)
}

// Exchange trades the impulse of two sides along n, changing their velocities, and returns it;
// none unless they are closing, the lower Bounce of the two given back.
func Exchange(a, b Side, n geom.Vec) float32 {
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
	before := s.Vel.Value
	if s.Steered {
		s.Vel.Slow(d)
	} else {
		s.Vel.SetDelta(d)
	}
	// a share, not the value: the ground's pace has already scaled the velocity down
	if s.Speed != nil && before != 0 && f32.Abs(s.Vel.Value) < f32.Abs(before) {
		*s.Speed *= s.Vel.Value / before
	}
}

func impulse(a, b Side, deltaA, deltaB, n geom.Vec) float32 {
	if a.InvMass+b.InvMass == 0 {
		return 0
	}
	approach := (deltaA.X-deltaB.X)*n.X + (deltaA.Y-deltaB.Y)*n.Y
	if approach > 0 {
		return 0
	}
	return -(1 + min(a.Bounce, b.Bounce)) * approach / (a.InvMass + b.InvMass)
}
