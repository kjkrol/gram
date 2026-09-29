package render

import (
	"image/color"
	"math"

	"github.com/kjkrol/gram/render/gpu"
)

// FillRect lays c over the rectangle x, y, w by h of dst, in its pixels.
func FillRect(dst *Image, x, y, w, h float32, c color.Color) {
	fillQuad(dst, [4][2]float32{{x, y}, {x + w, y}, {x, y + h}, {x + w, y + h}}, c)
}

// StrokeRect lays c over the outline of the rectangle x, y, w by h, width wide, centred on it.
func StrokeRect(dst *Image, x, y, w, h, width float32, c color.Color) {
	o := width / 2
	FillRect(dst, x-o, y-o, w+width, width, c)
	FillRect(dst, x-o, y+h-o, w+width, width, c)
	FillRect(dst, x-o, y+o, width, h-width, c)
	FillRect(dst, x+w-o, y+o, width, h-width, c)
}

// StrokeLine lays c over the line from (x0, y0) to (x1, y1), width wide.
func StrokeLine(dst *Image, x0, y0, x1, y1, width float32, c color.Color) {
	dx, dy := x1-x0, y1-y0
	n := float32(math.Hypot(float64(dx), float64(dy)))
	if n == 0 {
		return
	}
	nx, ny := -dy/n*width/2, dx/n*width/2
	fillQuad(dst, [4][2]float32{{x0 + nx, y0 + ny}, {x1 + nx, y1 + ny}, {x0 - nx, y0 - ny}, {x1 - nx, y1 - ny}}, c)
}

// FillCircle lays c over the disc round (cx, cy), radius r.
func FillCircle(dst *Image, cx, cy, r float32, c color.Color) {
	cr, cg, cb, ca := rgbaOf(c)
	v := func(x, y float32) gpu.Vertex {
		return gpu.Vertex{DstX: x, DstY: y, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca}
	}
	const sides = 48
	verts := []gpu.Vertex{v(cx, cy)}
	var indices []uint16
	for k := 0; k <= sides; k++ {
		s, co := math.Sincos(2 * math.Pi * float64(k) / sides)
		verts = append(verts, v(cx+r*float32(co), cy+r*float32(s)))
		if k > 0 {
			indices = append(indices, 0, uint16(k), uint16(k+1))
		}
	}
	gpu.Colored(dst.gpu(), verts, indices, gpu.SourceOver)
}

func fillQuad(dst *Image, q [4][2]float32, c color.Color) {
	cr, cg, cb, ca := rgbaOf(c)
	var verts [4]gpu.Vertex
	for k, p := range q {
		verts[k] = gpu.Vertex{DstX: p[0], DstY: p[1], ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca}
	}
	gpu.Colored(dst.gpu(), verts[:], []uint16{0, 1, 2, 1, 2, 3}, gpu.SourceOver)
}

// rates are the frames and ticks a second the engine last measured.
var rates struct{ fps, tps float64 }

// SetRates is the engine's: the frames and the ticks it ran in the last second.
func SetRates(fps, tps float64) { rates.fps, rates.tps = fps, tps }

// ActualFPS is how many frames the engine drew in the last second.
func ActualFPS() float64 { return rates.fps }

// ActualTPS is how many times a second the engine updated in the last second.
func ActualTPS() float64 { return rates.tps }
