package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/vector"
)

// Canvas is a sprite being painted on the CPU before its atlas goes to the GPU: an RGBA picture,
// premultiplied, with the few shapes sprites are drawn with.
type Canvas struct{ *image.RGBA }

func newCanvas(w, h int) *Canvas { return &Canvas{image.NewRGBA(image.Rect(0, 0, w, h))} }

// Fill puts c in place of every pixel.
func (c *Canvas) Fill(col color.Color) {
	draw.Draw(c.RGBA, c.Bounds(), image.NewUniform(col), image.Point{}, draw.Src)
}

// FillRect lays c over the rectangle x, y, w by h, its edges on whole pixels.
func (c *Canvas) FillRect(x, y, w, h float32, col color.Color) {
	r := image.Rect(int(math.Round(float64(x))), int(math.Round(float64(y))), int(math.Round(float64(x+w))), int(math.Round(float64(y+h))))
	draw.Draw(c.RGBA, r.Intersect(c.Bounds()), image.NewUniform(col), image.Point{}, draw.Over)
}

// FillCircle lays c over the disc round (cx, cy), radius r, smoothed at its edge.
func (c *Canvas) FillCircle(cx, cy, r float32, col color.Color) {
	z := c.rasterizer()
	const k = 0.5522847498 // a quarter circle's Bézier control distance, over the radius
	z.MoveTo(cx+r, cy)
	z.CubeTo(cx+r, cy+k*r, cx+k*r, cy+r, cx, cy+r)
	z.CubeTo(cx-k*r, cy+r, cx-r, cy+k*r, cx-r, cy)
	z.CubeTo(cx-r, cy-k*r, cx-k*r, cy-r, cx, cy-r)
	z.CubeTo(cx+k*r, cy-r, cx+r, cy-k*r, cx+r, cy)
	z.ClosePath()
	z.Draw(c.RGBA, c.Bounds(), image.NewUniform(col), image.Point{})
}

// StrokeLine lays c over the line from (x0, y0) to (x1, y1), width wide, smoothed at its edges.
func (c *Canvas) StrokeLine(x0, y0, x1, y1, width float32, col color.Color) {
	dx, dy := x1-x0, y1-y0
	n := float32(math.Hypot(float64(dx), float64(dy)))
	if n == 0 {
		return
	}
	nx, ny := -dy/n*width/2, dx/n*width/2
	z := c.rasterizer()
	z.MoveTo(x0+nx, y0+ny)
	z.LineTo(x1+nx, y1+ny)
	z.LineTo(x1-nx, y1-ny)
	z.LineTo(x0-nx, y0-ny)
	z.ClosePath()
	z.Draw(c.RGBA, c.Bounds(), image.NewUniform(col), image.Point{})
}

func (c *Canvas) rasterizer() *vector.Rasterizer {
	b := c.Bounds()
	z := vector.NewRasterizer(b.Dx(), b.Dy())
	z.DrawOp = draw.Over
	return z
}
