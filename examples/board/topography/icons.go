package main

import (
	"image"
	"image/color"
	"math"

	"github.com/kjkrol/gram/render"
	"golang.org/x/image/vector"
)

// an icon's size and its disc's, in pixels; a figure's head, its torso's sides and height
const (
	iconPx     = 24
	discR      = 11
	headR      = 2.2
	torsoWide  = 5.5
	torsoNeck  = 3.3
	torsoTall  = 9
	torsoRound = 1.2
	packR      = 2
)

var (
	discColor = color.RGBA{R: 245, G: 240, B: 225, A: 178}
	iconRim   = color.RGBA{R: 40, G: 35, B: 30, A: 255}
	packColor = color.RGBA{R: 90, G: 60, B: 35, A: 255}
)

// travellerIcon is one figure in c on a pale disc: a traveller seen from afar.
func travellerIcon(c color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		k := float32(size) / iconPx
		disc(dst, k)
		silhouette(dst, k, 12, 19, 1, c, false)
	}
}

// caravanIcon is three figures in c on a pale disc, the middle one taller, the last one laden.
func caravanIcon(c color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		k := float32(size) / iconPx
		disc(dst, k)
		silhouette(dst, k, 6, 19, 1, c, false)
		silhouette(dst, k, 18, 19, 1, c, true)
		silhouette(dst, k, 12, 17, 1.2, c, false)
	}
}

func disc(dst *render.Canvas, k float32) {
	z := newPath(dst)
	circle(z, 12*k, 12*k, (discR-0.5)*k, false)
	paint(dst, z, discColor)
	z = newPath(dst)
	circle(z, 12*k, 12*k, discR*k, false)
	circle(z, 12*k, 12*k, (discR-1)*k, true)
	paint(dst, z, iconRim)
}

// silhouette draws a figure scaled by s whose feet stand at (x, feet) in an icon's pixels, k the
// sprite's pixels to an icon's, a pack at its shoulder if laden, rimmed a pixel round
func silhouette(dst *render.Canvas, k, x, feet, s float32, c color.RGBA, laden bool) {
	top := feet - torsoTall*s
	head := top - 0.5*s - headR*s
	for _, rim := range []bool{true, false} {
		grow, col := float32(0), c
		if rim {
			grow, col = 1, iconRim
		}
		z := newPath(dst)
		circle(z, x*k, head*k, (headR*s+grow)*k, false)
		paint(dst, z, col)
		z = newPath(dst)
		half, neck := torsoWide*s/2+grow, torsoNeck*s/2+grow
		rounded(z, k, torsoRound*s, [][2]float32{
			{x - neck, top - grow},
			{x + neck, top - grow},
			{x + half, feet + grow},
			{x - half, feet + grow},
		})
		paint(dst, z, col)
		if laden {
			z = newPath(dst)
			circle(z, (x+neck)*k, (top+1)*k, (packR+grow)*k, false)
			if rim {
				paint(dst, z, iconRim)
			} else {
				paint(dst, z, packColor)
			}
		}
	}
}

func newPath(dst *render.Canvas) *vector.Rasterizer {
	b := dst.Bounds()
	return vector.NewRasterizer(b.Dx(), b.Dy())
}

// paint lays c, a straight colour, over what the path z went round
func paint(dst *render.Canvas, z *vector.Rasterizer, c color.RGBA) {
	z.Draw(dst.RGBA, dst.Bounds(), image.NewUniform(color.NRGBA(c)), image.Point{})
}

// circle goes round (cx, cy) at r, the other way round if back, so it cuts a hole in another
func circle(z *vector.Rasterizer, cx, cy, r float32, back bool) {
	const q = 0.5522847498 // a quarter circle's Bézier control distance, over the radius
	s := float32(1)
	if back {
		s = -1
	}
	z.MoveTo(cx+r, cy)
	z.CubeTo(cx+r, cy+s*q*r, cx+q*r, cy+s*r, cx, cy+s*r)
	z.CubeTo(cx-q*r, cy+s*r, cx-r, cy+s*q*r, cx-r, cy)
	z.CubeTo(cx-r, cy-s*q*r, cx-q*r, cy-s*r, cx, cy-s*r)
	z.CubeTo(cx+q*r, cy-s*r, cx+r, cy-s*q*r, cx+r, cy)
	z.ClosePath()
}

// rounded goes round the polygon of corners, in an icon's pixels, each corner rounded by r
func rounded(z *vector.Rasterizer, k, r float32, corners [][2]float32) {
	n := len(corners)
	toward := func(from, to [2]float32) (float32, float32) {
		dx, dy := to[0]-from[0], to[1]-from[1]
		d := float32(1)
		if l := float32(math.Hypot(float64(dx), float64(dy))); l > 0 {
			d = r / l
		}
		return (from[0] + dx*d) * k, (from[1] + dy*d) * k
	}
	for i, p := range corners {
		ax, ay := toward(p, corners[(i+n-1)%n])
		bx, by := toward(p, corners[(i+1)%n])
		if i == 0 {
			z.MoveTo(ax, ay)
		} else {
			z.LineTo(ax, ay)
		}
		z.QuadTo(p[0]*k, p[1]*k, bx, by)
	}
	z.ClosePath()
}
