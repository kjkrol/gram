package forest

import (
	"image/color"

	"github.com/kjkrol/gram/render"
)

// crown is a crown seen from above in c, lit from the upper left
func crown(c Colours) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		half := float32(size) / 2
		dst.FillCircle(half, half, half, c.Crown)
		dst.FillCircle(half*0.75, half*0.75, half*0.45, c.Leaves)
	}
}

// standing is a tree seen from the side in c, its trunk the share trunk of its height under a
// crown filling the rest, lit from the upper left; the sprite spans the box's side across and its
// height down
func standing(c Colours, trunk float32) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		s := float32(size)
		high := s * (1 - trunk)
		capsule(dst, 0, 0, s, high, c.Crown)
		r := min(s, high) / 2
		dst.FillCircle(s*0.35, min(high*0.35, r), r*0.45, c.Leaves)
		if trunk > 0 {
			wide := max(s/8, 1)
			dst.FillRect((s-wide)/2, high-1, wide, s-high+1, c.Trunk)
		}
	}
}

// capsule fills the rectangle w by h from (x, y) with the discs of its shorter side laid along its
// longer one, its ends round
func capsule(dst *render.Canvas, x, y, w, h float32, col color.RGBA) {
	r := min(w, h) / 2
	if r <= 0 {
		return
	}
	along, cx, cy := max(w, h)-2*r, x+r, y+r
	for d := float32(0); ; d += r / 4 {
		d = min(d, along)
		if w > h {
			dst.FillCircle(cx+d, cy, r, col)
		} else {
			dst.FillCircle(cx, cy+d, r, col)
		}
		if d >= along {
			return
		}
	}
}
