package light

import (
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/render"
)

// Source is what an entity shines with, its kind's to declare and an effect's to turn: a knob. A
// lamp shines from where the entity stands as far as Reach, in Color, Strength strong, fading
// with the distance; a distant source, Reach 0, is a Distant light (the sun), Color, Strength
// and Ambient scaling what Shining says of it beside. A zero Color is white; a zero Strength
// shines not at all.
type Source struct {
	Color    render.Light
	Strength float32
	Ambient  float32
	Reach    float32
}

// Knob: an effect may turn it (effect.Knob).
func (Source) Knob() {}

var _ effect.Knob = Source{}

// Shining is the light of a distant source as the sky writes it beside the Source each tick, or a
// scene once: the sun of the hour, a scene's lamp in the sky. Of a zero one the source shines not
// at all.
type Shining struct{ Light Distant }

// Lamp is a lamp in the world as the plugin sees it this tick (render.Lamp).
type Lamp = render.Lamp
