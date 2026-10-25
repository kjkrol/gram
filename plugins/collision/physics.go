package collision

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/internal/f32"
)

// Physics makes an entity take part in the physical side of a contact: pushed apart, bouncing.
// The zero value weighs as much as its box is wide by deep and does not bounce.
type Physics struct {
	// Mass is how hard the entity is to shift; non-positive weighs its box's area in square world
	// units, +Inf is a wall.
	Mass float32
	// Restitution is the share of approach speed given back, 0 to 1; a pair uses the lower one.
	Restitution float32
}

// Knob: an effect may turn it (effect.Knob).
func (Physics) Knob() {}

var _ effect.Knob = Physics{}

func (p Physics) weight(size geom.Vec) float32 {
	if p.Mass <= 0 {
		return size.X * size.Y
	}
	return p.Mass
}

func (p Physics) bounce() float32 { return min(max(p.Restitution, 0), 1) }

func (p Physics) immovable() bool { return f32.IsInf(p.Mass, 1) }

// inverseMass is 1 over the weight of an entity whose box is size; 0 for a wall, and for a box of
// no area, which overlaps nothing
func (p Physics) inverseMass(size geom.Vec) float32 {
	w := p.weight(size)
	if p.immovable() || w <= 0 {
		return 0
	}
	return 1 / w
}
