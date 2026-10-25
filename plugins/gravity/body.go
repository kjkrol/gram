package gravity

import "github.com/kjkrol/ceremony/effect"

// Pull is the gravity of a world, a knob on the plugin's own entity in world units a second
// squared: along Z, or any way across X and Y, never turned from the one to the other. The plugin
// only reads it, so an effect's Alter turns it.
type Pull struct{ X, Y, Z float32 }

// Knob: an effect may turn it (effect.Knob).
func (Pull) Knob() {}

var _ effect.Knob = Pull{}

// Body makes an entity fall under the Pull, stand on what bears it and leap; Leap is how high it
// leaps, in world units (0, not at all). Its drift and, with heights, its altitude are written by
// gravity alone.
type Body struct{ Leap float32 }
