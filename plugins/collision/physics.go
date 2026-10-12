package collision

import "math"

// Physics makes an entity take part in the physical side of a contact: pushed apart, bouncing.
// The zero value weighs DefaultMass and does not bounce.
type Physics struct {
	// Mass is how hard the entity is to shift; non-positive weighs DefaultMass, +Inf is a wall.
	Mass float64
	// Restitution is the share of approach speed given back, 0 to 1; a pair uses the lower one.
	Restitution float64
}

// DefaultMass is what an entity whose Physics names no Mass weighs.
const DefaultMass = 1.0

func (p Physics) weight() float64 {
	if p.Mass <= 0 {
		return DefaultMass
	}
	return p.Mass
}

func (p Physics) bounce() float64 { return min(max(p.Restitution, 0), 1) }

func (p Physics) immovable() bool { return math.IsInf(p.Mass, 1) }

func (p Physics) inverseMass() float64 {
	if p.immovable() {
		return 0
	}
	return 1 / p.weight()
}
