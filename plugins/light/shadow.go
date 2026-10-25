package light

import (
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/render"
)

// Casting is how an entity casts its shadow on the ground (render.Casting): Patch every frame,
// away from the light, what moves; Silhouette its own drawn shape, a figure worth the cost;
// Baked into the ground with the ground's own, what stands still, one lying flat over the
// rectangle its Lie says, any other over its box; NoShadow none.
type Casting = render.Casting

// The ways an entity casts its shadow (render.Patch…).
const (
	Patch      = render.Patch
	Silhouette = render.Silhouette
	Baked      = render.Baked
	NoShadow   = render.NoShadow
)

// Shadow is how an entity casts its shadow and which shade it takes (render.Taking), its kind's
// to declare and an effect's to turn: a knob. A kind declaring none casts a Patch and takes all.
type Shadow struct {
	Cast Casting
	Take render.Taking
}

// Knob: an effect may turn it (effect.Knob).
func (Shadow) Knob() {}

var _ effect.Knob = Shadow{}
