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

// DefaultMass is what an entity whose Physics names no Mass weighs — the value
// that makes an equal pair exchange velocities.
const DefaultMass = 1.0

// weight is what this entity weighs in a collision — DefaultMass unless Mass is a real weight.
func (p Physics) weight() float64 {
	if p.Mass <= 0 {
		return DefaultMass
	}
	return p.Mass
}

// bounce is Restitution held within its range.
func (p Physics) bounce() float64 { return min(max(p.Restitution, 0), 1) }

// immovable reports whether nothing can shift this entity.
func (p Physics) immovable() bool { return math.IsInf(p.Mass, 1) }

// inverseMass is how much of an impulse an entity takes — none for one nothing can move.
func (p Physics) inverseMass() float64 {
	if p.immovable() {
		return 0
	}
	return 1 / p.weight()
}
