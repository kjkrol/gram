package main

import (
	"image/color"
	"math"

	"github.com/kjkrol/gram/plugins/forest"
	"github.com/kjkrol/gram/render"
)

// foliage is what a tree is painted in: its crown, the lit leaves on it and its trunk
type foliage struct {
	crown  color.RGBA
	leaves color.RGBA
	trunk  color.RGBA
}

// Madeira's trees, each drawn from above and from the side, its side spanning its crown across
// and its height down, sized in metres of life, no crown wider than a cell. Of the laurisilva:
// the til, the laurel, the vinhático, the barbusano and the faia; of the heights: the tree
// heather and the Madeira cedar; brought in: the maritime pine, the eucalyptus, the acacia and the chestnut; of the
// coast and the terraces: the dragon tree, the palm, the banana and the vine.
var trees = map[string]forest.Spec{
	TilKind: broadleaf(25, 6.5, foliage{
		crown:  color.RGBA{R: 22, G: 70, B: 38, A: 255},
		leaves: color.RGBA{R: 50, G: 104, B: 52, A: 255},
		trunk:  color.RGBA{R: 62, G: 48, B: 36, A: 255},
	}),
	LaurelKind: broadleaf(12, 5.5, foliage{
		crown:  color.RGBA{R: 30, G: 84, B: 42, A: 255},
		leaves: color.RGBA{R: 64, G: 122, B: 58, A: 255},
		trunk:  color.RGBA{R: 72, G: 54, B: 38, A: 255},
	}),
	VinhaticoKind: broadleaf(20, 6, foliage{
		crown:  color.RGBA{R: 44, G: 96, B: 48, A: 255},
		leaves: color.RGBA{R: 150, G: 96, B: 60, A: 255},
		trunk:  color.RGBA{R: 96, G: 64, B: 44, A: 255},
	}),
	BarbusanoKind: broadleaf(15, 5.5, foliage{
		crown:  color.RGBA{R: 40, G: 76, B: 36, A: 255},
		leaves: color.RGBA{R: 86, G: 112, B: 48, A: 255},
		trunk:  color.RGBA{R: 58, G: 46, B: 38, A: 255},
	}),
	FaiaKind: broadleaf(8, 5, foliage{
		crown:  color.RGBA{R: 66, G: 110, B: 46, A: 255},
		leaves: color.RGBA{R: 112, G: 150, B: 64, A: 255},
		trunk:  color.RGBA{R: 90, G: 74, B: 56, A: 255},
	}),
	ChestnutKind: broadleaf(18, 6.5, foliage{
		crown:  color.RGBA{R: 58, G: 112, B: 44, A: 255},
		leaves: color.RGBA{R: 132, G: 160, B: 62, A: 255},
		trunk:  color.RGBA{R: 84, G: 62, B: 44, A: 255},
	}),
	AcaciaKind: broadleaf(12, 5, foliage{
		crown:  color.RGBA{R: 70, G: 104, B: 62, A: 255},
		leaves: color.RGBA{R: 214, G: 190, B: 70, A: 255},
		trunk:  color.RGBA{R: 70, G: 58, B: 48, A: 255},
	}),
	HeatherKind: bush(5, 4.5, foliage{
		crown:  color.RGBA{R: 98, G: 112, B: 62, A: 255},
		leaves: color.RGBA{R: 156, G: 160, B: 100, A: 255},
		trunk:  color.RGBA{R: 92, G: 72, B: 52, A: 255},
	}),
	VineKind: bush(1.8, 2.5, foliage{
		crown:  color.RGBA{R: 96, G: 132, B: 52, A: 255},
		leaves: color.RGBA{R: 148, G: 172, B: 70, A: 255},
		trunk:  color.RGBA{R: 86, G: 66, B: 48, A: 255},
	}),
	PineKind: conifer(15, 3.75, foliage{
		crown:  color.RGBA{R: 38, G: 90, B: 70, A: 255},
		leaves: color.RGBA{R: 54, G: 108, B: 82, A: 255},
		trunk:  color.RGBA{R: 84, G: 58, B: 40, A: 255},
	}),
	CedarKind: conifer(12, 5, foliage{
		crown:  color.RGBA{R: 44, G: 82, B: 84, A: 255},
		leaves: color.RGBA{R: 86, G: 124, B: 122, A: 255},
		trunk:  color.RGBA{R: 104, G: 72, B: 56, A: 255},
	}),
	EucalyptusKind: slender(35, 5, foliage{
		crown:  color.RGBA{R: 84, G: 116, B: 96, A: 255},
		leaves: color.RGBA{R: 134, G: 158, B: 136, A: 255},
		trunk:  color.RGBA{R: 196, G: 184, B: 164, A: 255},
	}),
	DragonKind: umbrella(8, 6, foliage{
		crown:  color.RGBA{R: 62, G: 98, B: 66, A: 255},
		leaves: color.RGBA{R: 110, G: 140, B: 92, A: 255},
		trunk:  color.RGBA{R: 150, G: 132, B: 112, A: 255},
	}),
	PalmKind: fronded(12, 5, 0.6, foliage{
		crown:  color.RGBA{R: 52, G: 110, B: 48, A: 255},
		leaves: color.RGBA{R: 98, G: 150, B: 60, A: 255},
		trunk:  color.RGBA{R: 120, G: 92, B: 64, A: 255},
	}),
	BananaKind: fronded(4, 3.5, 0.25, foliage{
		crown:  color.RGBA{R: 72, G: 136, B: 50, A: 255},
		leaves: color.RGBA{R: 136, G: 180, B: 66, A: 255},
		trunk:  color.RGBA{R: 112, G: 124, B: 62, A: 255},
	}),
}

// side draws a tree from the side on a square stretched to its crown across and its height
// down: w is the crown's width in pixels, and a point is given across in crown widths from the
// left and up in crown widths from the ground, so a disc stays round on the tree
type side struct {
	dst    *render.Canvas
	w      float32
	aspect float32 // the tree's height over its crown's width
}

func sideOf(dst *render.Canvas, size int, height, crown float32) side {
	return side{dst: dst, w: float32(size), aspect: height / crown}
}

// disc fills a disc r crown widths round at x across and y up
func (s side) disc(x, y, r float32, col color.RGBA) {
	cy, ry := s.w*(1-y/s.aspect), s.w*r/s.aspect
	for row := float32(math.Floor(float64(cy - ry))); row <= cy+ry; row++ {
		d := (row + 0.5 - cy) / ry
		if d*d >= 1 {
			continue
		}
		half := s.w * r * float32(math.Sqrt(float64(1-d*d)))
		s.dst.FillRect(s.w*x-half, row, 2*half, 1, col)
	}
}

// stem fills a trunk wide crown widths wide at x across, from the ground up to y
func (s side) stem(x, y, wide float32, col color.RGBA) {
	s.dst.FillRect(s.w*(x-wide/2), s.w*(1-y/s.aspect), max(s.w*wide, 1), s.w*y/s.aspect, col)
}

// bough strokes a line from one point to another, wide crown widths wide
func (s side) bough(x0, y0, x1, y1, wide float32, col color.RGBA) {
	s.dst.StrokeLine(s.w*x0, s.w*(1-y0/s.aspect), s.w*x1, s.w*(1-y1/s.aspect),
		max(s.w*wide/s.aspect, 1), col)
}

// broadleaf is a tree of a round crown in three lobes on a trunk
func broadleaf(height, crown float32, f foliage) forest.Spec {
	return forest.Spec{
		Height: scale.Figure(height),
		Crown:  scale.Figure(crown),
		Top: func(dst *render.Canvas, size int) {
			s := float32(size)
			dst.FillCircle(s/2, s/2, s/2, f.crown)
			dst.FillCircle(s*0.36, s*0.36, s*0.18, f.leaves)
			dst.FillCircle(s*0.64, s*0.32, s*0.12, f.leaves)
			dst.FillCircle(s*0.32, s*0.64, s*0.1, f.leaves)
		},
		Side: func(dst *render.Canvas, size int) {
			s := sideOf(dst, size, height, crown)
			top := s.aspect
			s.stem(0.5, top-0.6, 0.1, f.trunk)
			s.disc(0.3, top-0.62, 0.3, f.crown)
			s.disc(0.7, top-0.62, 0.3, f.crown)
			s.disc(0.5, top-0.4, 0.4, f.crown)
			s.disc(0.36, top-0.3, 0.13, f.leaves)
			s.disc(0.66, top-0.55, 0.1, f.leaves)
		},
	}
}

// conifer is a tree of boughs in tiers narrowing to its top
func conifer(height, crown float32, f foliage) forest.Spec {
	return forest.Spec{
		Height: scale.Figure(height),
		Crown:  scale.Figure(crown),
		Top: func(dst *render.Canvas, size int) {
			s := float32(size)
			dst.FillCircle(s/2, s/2, s/2, f.crown)
			for i := range 8 {
				sin, cos := math.Sincos(float64(i) * math.Pi / 4)
				dst.StrokeLine(s/2, s/2, s/2+float32(cos)*s*0.42, s/2+float32(sin)*s*0.42, s/16,
					f.leaves)
			}
			dst.FillCircle(s/2, s/2, s*0.1, f.leaves)
		},
		Side: func(dst *render.Canvas, size int) {
			s := sideOf(dst, size, height, crown)
			top := s.aspect
			s.stem(0.5, top*0.3, 0.09, f.trunk)
			// its tiers from a sixth of its height up, each narrower than the one under it
			const tiers = 6
			for i := range tiers {
				t := float32(i) / (tiers - 1)
				r := 0.48 - 0.38*t
				y := top*(0.17+0.76*t) + r*0.2
				s.disc(0.5, y, r, f.crown)
				s.disc(0.5-r/3, y+r/3, r/3, f.leaves)
			}
		},
	}
}

// bush is a low thing of many stems, wider than tall
func bush(height, crown float32, f foliage) forest.Spec {
	return forest.Spec{
		Height: scale.Figure(height),
		Crown:  scale.Figure(crown),
		Top: func(dst *render.Canvas, size int) {
			s := float32(size)
			dst.FillCircle(s/2, s/2, s*0.4, f.crown)
			for i := range 6 {
				sin, cos := math.Sincos(float64(i) * math.Pi / 3)
				dst.FillCircle(s/2+float32(cos)*s*0.3, s/2+float32(sin)*s*0.3, s*0.18, f.crown)
			}
			dst.FillCircle(s*0.38, s*0.36, s*0.12, f.leaves)
			dst.FillCircle(s*0.62, s*0.6, s*0.08, f.leaves)
		},
		Side: func(dst *render.Canvas, size int) {
			s := sideOf(dst, size, height, crown)
			top := s.aspect
			s.bough(0.45, top*0.3, 0.38, 0, 0.07, f.trunk)
			s.bough(0.55, top*0.3, 0.62, 0, 0.07, f.trunk)
			s.disc(0.26, top*0.45, 0.25, f.crown)
			s.disc(0.74, top*0.45, 0.25, f.crown)
			s.disc(0.5, top*0.6, 0.34, f.crown)
			s.disc(0.4, top*0.72, 0.1, f.leaves)
			s.disc(0.68, top*0.52, 0.08, f.leaves)
		},
	}
}

// slender is a tall tree, a long pale trunk under a narrow crown in loose tufts
func slender(height, crown float32, f foliage) forest.Spec {
	return forest.Spec{
		Height: scale.Figure(height),
		Crown:  scale.Figure(crown),
		Top: func(dst *render.Canvas, size int) {
			s := float32(size)
			dst.FillCircle(s*0.42, s*0.4, s*0.34, f.crown)
			dst.FillCircle(s*0.62, s*0.6, s*0.3, f.crown)
			dst.FillCircle(s*0.36, s*0.34, s*0.14, f.leaves)
			dst.FillCircle(s*0.64, s*0.62, s*0.1, f.leaves)
		},
		Side: func(dst *render.Canvas, size int) {
			s := sideOf(dst, size, height, crown)
			top := s.aspect
			s.stem(0.5, top-0.5, 0.1, f.trunk)
			// tufts down the upper half of its trunk, to either side in turn
			for i := range 7 {
				x := float32(0.34 + 0.32*float32(i%2))
				s.disc(x, top-0.4-float32(i)*top*0.07, 0.32, f.crown)
			}
			s.disc(0.5, top-0.36, 0.36, f.crown)
			s.disc(0.4, top-0.3, 0.12, f.leaves)
			s.disc(0.62, top-0.4-top*0.14, 0.1, f.leaves)
		},
	}
}

// umbrella is a tree of a thick bare trunk forking under a flat, dense crown
func umbrella(height, crown float32, f foliage) forest.Spec {
	return forest.Spec{
		Height: scale.Figure(height),
		Crown:  scale.Figure(crown),
		Top: func(dst *render.Canvas, size int) {
			s := float32(size)
			dst.FillCircle(s/2, s/2, s/2, f.crown)
			for i := range 10 {
				sin, cos := math.Sincos(float64(i) * math.Pi / 5)
				dst.FillCircle(s/2+float32(cos)*s*0.3, s/2+float32(sin)*s*0.3, s*0.07, f.leaves)
			}
			dst.FillCircle(s/2, s/2, s*0.08, f.leaves)
		},
		Side: func(dst *render.Canvas, size int) {
			s := sideOf(dst, size, height, crown)
			top := s.aspect
			s.stem(0.5, top*0.5, 0.16, f.trunk)
			s.bough(0.5, top*0.45, 0.2, top*0.72, 0.12, f.trunk)
			s.bough(0.5, top*0.45, 0.8, top*0.72, 0.12, f.trunk)
			for _, x := range []float32{0.18, 0.34, 0.5, 0.66, 0.82} {
				s.disc(x, top-0.2, 0.18, f.crown)
			}
			s.disc(0.3, top-0.12, 0.07, f.leaves)
			s.disc(0.6, top-0.14, 0.06, f.leaves)
		},
	}
}

// fronded is a plant of long leaves fanning out of the top of its stem, the stem the share
// stem of its height
func fronded(height, crown, stem float32, f foliage) forest.Spec {
	return forest.Spec{
		Height: scale.Figure(height),
		Crown:  scale.Figure(crown),
		Top: func(dst *render.Canvas, size int) {
			s := float32(size)
			for i := range 9 {
				sin, cos := math.Sincos(float64(i) * 2 * math.Pi / 9)
				col := f.crown
				if i%2 == 0 {
					col = f.leaves
				}
				dst.StrokeLine(s/2, s/2, s/2+float32(cos)*s*0.46, s/2+float32(sin)*s*0.46, s/7,
					col)
			}
			dst.FillCircle(s/2, s/2, s*0.1, f.trunk)
		},
		Side: func(dst *render.Canvas, size int) {
			s := sideOf(dst, size, height, crown)
			top := s.aspect
			from := top * stem
			s.stem(0.5, from, 0.09, f.trunk)
			// its leaves fan from the top of the stem, the outer ones drooping
			reach := min(top-from, 0.5)
			for i := range 7 {
				sin, cos := math.Sincos(math.Pi * (float64(i)/6*1.3 - 0.15))
				col := f.crown
				if i%2 == 0 {
					col = f.leaves
				}
				s.bough(0.5, from, 0.5+float32(cos)*0.48, from+float32(sin)*reach, 0.12, col)
			}
		},
	}
}
