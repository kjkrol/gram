package air

import (
	"math"

	"github.com/kjkrol/gram/plugins/world"
)

// GustWide is how wide a gust's band is along the wind, in metres of life: it lies across the
// wind four to six times as long, the next one some three widths behind it.
const GustWide = 16

// GustBand is how wide a gust's band is in a world of scale, in world units: GustWide at the
// size its figures are drawn at, 64 in a world without a scale.
func GustBand(scale world.Scale) float32 {
	if scale.Metres <= 0 {
		return unscaledGust
	}
	return scale.Figure(GustWide)
}

// A band in its widths: how long it lies across the wind at the least and at the most, the side
// of the square two bands are thrown in, how far off its place in it a band strays, how far its
// ends lag behind its middle, how far its front and its back reach from its middle, from what
// share of its length its ends fade and how faint the faintest is. The bands are carried gustPace
// times as far as the clouds, the drift wrapped every blownSpan world units.
const (
	unscaledGust = 64
	gustLeast    = 3.75
	gustMost     = 6.25
	gustCell     = 7.5
	gustStray    = 0.3
	gustBow      = 0.6
	gustFront    = 0.6
	gustBack     = 1.25
	gustTaper    = 0.55
	gustFaint    = 0.65
	gustPace     = 1.6
	blownSpan    = 262144
)

// gustSide is the side of the squares bands wide are thrown in: gustCell widths, fitted so the
// squares come round whole as the drift wraps; 0 for none.
func gustSide(wide float32) float32 {
	if wide <= 0 {
		return 0
	}
	n := float32(math.RoundToEven(gustPace * blownSpan / float64(gustCell*wide)))
	return gustPace * blownSpan / max(n, 1)
}

// Gust is how hard the gusts blow over the world point (x, y) now, 0 in the steady wind between
// them to 1 under a band's middle; 0 in the calm and where w has no Gusts.
func (w Weather) Gust(x, y float32) float32 { return w.gustOver(x, y, 0) }

// gustOver is Gust with every band as much wider as span, the world units the point stands for
// (shaders/gust.wgsl's gustAt, the same).
func (w Weather) gustOver(x, y, span float32) float32 {
	blow := float32(math.Hypot(float64(w.Wind[0]), float64(w.Wind[1])))
	side := gustSide(w.Gusts)
	if blow == 0 || side == 0 {
		return 0
	}
	wx, wy := w.Wind[0]/blow, w.Wind[1]/blow
	wide := side / gustCell
	cells := float32(math.RoundToEven(float64(gustPace * blownSpan / side)))
	qx := x - gustPace*float32(wrap(float64(w.Drift[0]), blownSpan))
	qy := y - gustPace*float32(wrap(float64(w.Drift[1]), blownSpan))
	fx := float32(math.Floor(float64(qx/side - 0.5)))
	fy := float32(math.Floor(float64(qy/side - 0.5)))
	var best float32
	for j := range 4 {
		cx, cy := fx+float32(j%2), fy+float32(j/2)
		ix := uint32(cx - cells*float32(math.Floor(float64(cx/cells))))
		iy := uint32(cy - cells*float32(math.Floor(float64(cy/cells))))
		for k := range 2 {
			h := gustThrow(ix, iy, uint32(k))
			sx := float32(h&1023)/1023 - 0.5
			sy := float32(h>>10&1023)/1023 - 0.5
			long := float32(h>>20&1023) / 1023
			strong := float32(h>>30) / 3
			// half a square apart along the wind, each strayed its own way
			off := (float32(k) - 0.5) * 0.5 * side
			dx := qx - ((cx+0.5)*side + wx*off + sx*gustStray*side)
			dy := qy - ((cy+0.5)*side + wy*off + sy*gustStray*side)
			half := 0.5 * wide * (gustLeast + (gustMost-gustLeast)*long)
			u := (dy*wx - dx*wy) / half
			if u <= -1 || u >= 1 {
				continue
			}
			a := dx*wx + dy*wy + gustBow*wide*u*u
			reach := gustBack*wide + span/2
			if a > 0 {
				reach = gustFront*wide + span/2
			}
			v := a / reach
			if v <= -1 || v >= 1 {
				continue
			}
			ends := smoothstep(gustTaper, 1, float32(math.Abs(float64(u))))
			gust := (1 - v*v) * (1 - v*v) * (1 - ends) * (gustFaint + (1-gustFaint)*strong)
			best = max(best, gust)
		}
	}
	return best
}

// gustThrow is the throw of the k-th band of the square (x, y): the same on every machine.
func gustThrow(x, y, k uint32) uint32 {
	h := x*0x9E3779B1 ^ y*0x85EBCA77 ^ k*0xC2B2AE3D
	h ^= h >> 15
	h *= 0x2C1B3C6D
	h ^= h >> 12
	h *= 0x297A2D39
	h ^= h >> 15
	return h
}
