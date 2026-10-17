package entity

import "github.com/kjkrol/ceremony/effect"

// Casting is how an entity casts its shadow on the ground.
type Casting uint8

const (
	// Patch casts a soft patch every frame, away from the sun: what moves.
	Patch Casting = iota
	// Silhouette casts the entity's own drawn shape every frame, away from the sun: a figure
	// worth the cost.
	Silhouette
	// Baked has the ground bake the shadow of the entity's solid with its own: what stands still,
	// one lying flat over the rectangle its [Lie] says, any other over its box.
	Baked
	// NoShadow casts none.
	NoShadow
)

// Shadow is how an entity casts its shadow, its kind's to declare and an effect's to turn: a knob.
// An entity carrying none casts a Patch.
type Shadow struct{ Cast Casting }

// Knob: an effect may turn it (effect.Knob).
func (Shadow) Knob() {}

var _ effect.Knob = Shadow{}
