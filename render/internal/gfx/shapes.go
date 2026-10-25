package gfx

import (
	"image/color"
	"math"

	"github.com/kjkrol/gram/render/internal/gpu"
)

// FillRect lays c over the rectangle x, y, w by h of the image, in its pixels.
func (i *Image) FillRect(x, y, w, h float32, c color.Color) {
	i.fillQuad([4][2]float32{{x, y}, {x + w, y}, {x, y + h}, {x + w, y + h}}, c)
}

// StrokeRect lays c over the outline of the rectangle x, y, w by h, width wide, centred on it.
func (i *Image) StrokeRect(x, y, w, h, width float32, c color.Color) {
	o := width / 2
	i.FillRect(x-o, y-o, w+width, width, c)
	i.FillRect(x-o, y+h-o, w+width, width, c)
	i.FillRect(x-o, y+o, width, h-width, c)
	i.FillRect(x+w-o, y+o, width, h-width, c)
}

// StrokeLine lays c over the line from (x0, y0) to (x1, y1), width wide.
func (i *Image) StrokeLine(x0, y0, x1, y1, width float32, c color.Color) {
	dx, dy := x1-x0, y1-y0
	n := float32(math.Hypot(float64(dx), float64(dy)))
	if n == 0 {
		return
	}
	nx, ny := -dy/n*width/2, dx/n*width/2
	i.fillQuad([4][2]float32{{x0 + nx, y0 + ny}, {x1 + nx, y1 + ny}, {x0 - nx, y0 - ny}, {x1 - nx, y1 - ny}}, c)
}

// FillCircle lays c over the disc round (cx, cy), radius r.
func (i *Image) FillCircle(cx, cy, r float32, c color.Color) {
	cr, cg, cb, ca := Premultiply(c)
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
	gpu.Colored(i.gpu(), verts, indices, gpu.SourceOver)
}

func (i *Image) fillQuad(q [4][2]float32, c color.Color) {
	cr, cg, cb, ca := Premultiply(c)
	var verts [4]gpu.Vertex
	for k, p := range q {
		verts[k] = gpu.Vertex{DstX: p[0], DstY: p[1], ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca}
	}
	gpu.Colored(i.gpu(), verts[:], []uint16{0, 1, 2, 1, 2, 3}, gpu.SourceOver)
}

// FillQuad fills the image's corners in a colour each, blended across, top-left, top-right,
// bottom-left, bottom-right.
func (i *Image) FillQuad(corners Corners, c [4]color.RGBA) {
	verts := make([]gpu.Vertex, 4)
	for k, p := range corners {
		cr, cg, cb, ca := Premultiply(c[k])
		verts[k] = gpu.Vertex{DstX: p[0], DstY: p[1], ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca}
	}
	gpu.Colored(i.gpu(), verts, []uint16{0, 1, 2, 1, 2, 3}, gpu.SourceOver)
}

// FillPolygon lays c over the convex polygon pts, in the image's pixels.
func (i *Image) FillPolygon(pts [][2]float32, c color.Color) {
	if len(pts) < 3 {
		return
	}
	cr, cg, cb, ca := Premultiply(c)
	verts := make([]gpu.Vertex, len(pts))
	for k, p := range pts {
		verts[k] = gpu.Vertex{DstX: p[0], DstY: p[1], ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca}
	}
	indices := make([]uint16, 0, 3*(len(pts)-2))
	for k := 1; k+1 < len(pts); k++ {
		indices = append(indices, 0, uint16(k), uint16(k+1))
	}
	gpu.Colored(i.gpu(), verts, indices, gpu.SourceOver)
}

// StrokePolygon lays c over the outline of the closed polygon pts, width wide, at one draw.
func (i *Image) StrokePolygon(pts [][2]float32, width float32, c color.Color) {
	cr, cg, cb, ca := Premultiply(c)
	verts := make([]gpu.Vertex, 0, 4*len(pts))
	indices := make([]uint16, 0, 6*len(pts))
	for k, p := range pts {
		q := pts[(k+1)%len(pts)]
		dx, dy := q[0]-p[0], q[1]-p[1]
		n := float32(math.Hypot(float64(dx), float64(dy)))
		if n == 0 {
			continue
		}
		nx, ny := -dy/n*width/2, dx/n*width/2
		at := uint16(len(verts))
		for _, v := range [4][2]float32{{p[0] + nx, p[1] + ny}, {q[0] + nx, q[1] + ny},
			{p[0] - nx, p[1] - ny}, {q[0] - nx, q[1] - ny}} {
			verts = append(verts, gpu.Vertex{DstX: v[0], DstY: v[1], ColorR: cr, ColorG: cg,
				ColorB: cb, ColorA: ca})
		}
		indices = append(indices, at, at+1, at+2, at+1, at+2, at+3)
	}
	if len(indices) > 0 {
		gpu.Colored(i.gpu(), verts, indices, gpu.SourceOver)
	}
}
