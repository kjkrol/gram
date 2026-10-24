package light

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// Lamp is a lamp in the world: where it stands and how far it reaches, in Color at full strength.
type Lamp struct {
	X, Y, Z, Reach float32
	Color          Light
}

// Light is the lamp's light at (x, y, z), fading by the square of the share of its reach left;
// false out of its reach.
func (l Lamp) Light(x, y, z float32) (Light, bool) {
	dx, dy, dz := x-l.X, y-l.Y, z-l.Z
	d2 := dx*dx + dy*dy + dz*dz
	if l.Reach <= 0 || d2 >= l.Reach*l.Reach {
		return Light{}, false
	}
	left := 1 - sqrt32(d2)/l.Reach
	k := left * left
	return Light{l.Color[0] * k, l.Color[1] * k, l.Color[2] * k}, true
}

// ShadowOf is the shadow the lamp casts of what stands in box from alt up height on level ground:
// away from the lamp, the longer the nearer the lamp and the higher it stands against it; a
// patch, or its own shape where shape says. It is false where the lamp's light does not reach it.
func (l Lamp) ShadowOf(box geom.AABB, alt, height float64, shape bool) (Umbra, bool) {
	cx, cy := float32(box.TopLeft.X+box.BottomRight.X)/2, float32(box.TopLeft.Y+box.BottomRight.Y)/2
	low, top := float32(alt), float32(alt+height)
	c, ok := l.Light(cx, cy, low)
	veil := float32(uint8(shadowVeil*min(max(c[0], c[1], c[2])/shadowFull, 1))) / 255
	if !ok || veil <= 0 || low >= l.Z {
		return Umbra{}, false
	}
	dx, dy := cx-l.X, cy-l.Y
	d := float32(math.Hypot(float64(dx), float64(dy)))
	ux, uy := float32(1), float32(0)
	if d > 0 {
		ux, uy = dx/d, dy/d
	}
	// a point h high at d from the lamp's foot shadows the ground at d·L/(L−h) from it
	start := d * low / (l.Z - low)
	most := float32(height) * maxShadowReach
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
