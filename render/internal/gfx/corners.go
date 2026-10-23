package gfx

import "math"

// Corners are a quad's screen corners: top-left, top-right, bottom-left, bottom-right.
type Corners [4][2]float32

// Fade is how many pixels each side of a soft quad takes to fade out; 0 leaves a side hard.
type Fade struct{ Left, Right, Top, Bottom float32 }

// a fade stays under softCap, a blended sprite's mark over it
const softCap = 10000

// FadeCustoms are the customs of the corners dst fading as fade says: per edge (left, right,
// top, bottom), 1 plus the corner's distance to that edge in units of the edge's fade; 0 leaves
// the edge alone.
func FadeCustoms(dst Corners, fade Fade) [4][4]float32 {
	across := func(a, b [2]float32) float32 {
		return float32(math.Hypot(float64(b[0]-a[0]), float64(b[1]-a[1])))
	}
	width := [4]float32{across(dst[0], dst[1]), across(dst[0], dst[1]), across(dst[2], dst[3]), across(dst[2], dst[3])}
	height := [4]float32{across(dst[0], dst[2]), across(dst[1], dst[3]), across(dst[0], dst[2]), across(dst[1], dst[3])}
	side := func(w, dist float32, near bool) float32 {
		switch {
		case w <= 0:
			return 0
		case near:
			return 1
		}
		return min(1+dist/w, softCap)
	}
	var out [4][4]float32
	for i := range dst {
		left, top := i%2 == 0, i < 2
		out[i] = [4]float32{side(fade.Left, width[i], left), side(fade.Right, width[i], !left), side(fade.Top, height[i], top), side(fade.Bottom, height[i], !top)}
	}
	return out
}
