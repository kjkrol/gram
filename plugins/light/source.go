package light

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/plugins/world"
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

// Lamp is a lamp in the world as the plugin sees it this tick: where it stands and how far it
// reaches, in Color at full strength.
type Lamp struct {
	X, Y, Z, Reach float32
	Color          render.Light
}

// light is the lamp's light at (x, y, z), fading by the square of the share of its reach left
func (l Lamp) light(x, y, z float32) (render.Light, bool) {
	dx, dy, dz := x-l.X, y-l.Y, z-l.Z
	d2 := dx*dx + dy*dy + dz*dz
	if l.Reach <= 0 || d2 >= l.Reach*l.Reach {
		return render.Light{}, false
	}
	left := 1 - sqrt32(d2)/l.Reach
	k := left * left
	return render.Light{l.Color[0] * k, l.Color[1] * k, l.Color[2] * k}, true
}

// ShadowOf is the shadow the lamp casts of an entity standing in box as z says on level ground:
// away from the lamp, the longer the nearer the lamp and the higher the entity stands against
// it; a patch, or the entity's own shape where shape says. It is false where the lamp's light
// does not reach the entity.
func (l Lamp) ShadowOf(box geom.AABB, z world.Z, shape bool) (Umbra, bool) {
	cx, cy := float32(box.TopLeft.X+box.BottomRight.X)/2, float32(box.TopLeft.Y+box.BottomRight.Y)/2
	alt, top := float32(z.Altitude), float32(z.Altitude+z.Height)
	c, ok := l.light(cx, cy, alt)
	veil := float32(uint8(shadowVeil*min(max(c[0], c[1], c[2])/shadowFull, 1))) / 255
	if !ok || veil <= 0 || alt >= l.Z {
		return Umbra{}, false
	}
	dx, dy := cx-l.X, cy-l.Y
	d := float32(math.Hypot(float64(dx), float64(dy)))
	ux, uy := float32(1), float32(0)
	if d > 0 {
		ux, uy = dx/d, dy/d
	}
	// a point h high at d from the lamp's foot shadows the ground at d·L/(L−h) from it
	start := d * alt / (l.Z - alt)
	most := float32(z.Height) * maxShadowReach
	length := most
	if top < l.Z {
		length = min(d*top/(l.Z-top)-start, most)
	}
	half := float32(max(box.BottomRight.X-box.TopLeft.X, box.BottomRight.Y-box.TopLeft.Y)) / 2
	u := Umbra{UX: ux, UY: uy, Along: length/2 + half, Wide: half, Fade: half / 2, Veil: veil}
	if shape {
		length = max(length, 1e-3)
		u.Along, u.Wide, u.Fade = length/2, float32(box.BottomRight.X-box.TopLeft.X)/2, 0
	}
	mid := start + length/2
	u.X, u.Y = cx+ux*mid, cy+uy*mid
	return u, true
}
