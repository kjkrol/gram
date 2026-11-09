package main

import (
	"math"

	"github.com/kjkrol/gram/plugins/forest"
	"github.com/kjkrol/gram/render"
)

// Madeira's kinds of trees, each drawn from above and from the side, its side spanning its crown
// across and its height down: the laurel of the laurisilva, broad and dark; the maritime pine,
// tall and conical; the tree heather, low and bushy.
var (
	laurel = forest.Spec{
		Height: scale.Figure(9),
		Crown:  30,
		Top:    laurelTop,
		Side:   laurelSide,
	}
	pine = forest.Spec{
		Height: scale.Figure(15),
		Crown:  18,
		Top:    pineTop,
		Side:   pineSide,
	}
	heather = forest.Spec{
		Height: scale.Figure(5),
		Crown:  22,
		Top:    heatherTop,
		Side:   heatherSide,
	}
)

func laurelTop(dst *render.Canvas, size int) {
	s := float32(size)
	dst.FillCircle(s/2, s/2, s/2, laurelCrown)
	dst.FillCircle(s*0.36, s*0.36, s*0.18, laurelLeaves)
	dst.FillCircle(s*0.64, s*0.32, s*0.12, laurelLeaves)
	dst.FillCircle(s*0.32, s*0.64, s*0.1, laurelLeaves)
	seeThrough(dst)
}

func laurelSide(dst *render.Canvas, size int) {
	s := float32(size)
	dst.FillCircle(s*0.3, s*0.42, s*0.28, laurelCrown)
	dst.FillCircle(s*0.7, s*0.42, s*0.28, laurelCrown)
	dst.FillCircle(s*0.5, s*0.3, s*0.3, laurelCrown)
	dst.FillCircle(s*0.36, s*0.24, s*0.12, laurelLeaves)
	dst.FillCircle(s*0.64, s*0.38, s*0.09, laurelLeaves)
	seeThrough(dst)
	dst.FillRect(s*0.45, s*0.68, s*0.1, s*0.32, laurelTrunk)
}

func pineTop(dst *render.Canvas, size int) {
	s := float32(size)
	dst.FillCircle(s/2, s/2, s/2, pineCrown)
	for i := range 8 {
		sin, cos := math.Sincos(float64(i) * math.Pi / 4)
		dst.StrokeLine(s/2, s/2, s/2+float32(cos)*s*0.42, s/2+float32(sin)*s*0.42, s/16,
			pineLeaves)
	}
	dst.FillCircle(s/2, s/2, s*0.1, pineLeaves)
	seeThrough(dst)
}

func pineSide(dst *render.Canvas, size int) {
	s := float32(size)
	// its boughs in tiers, narrowing to its top
	for i, r := range []float32{0.38, 0.31, 0.24, 0.17, 0.1} {
		y := s * (0.52 - float32(i)*0.11)
		dst.FillCircle(s/2, y, s*r, pineCrown)
		dst.FillCircle(s/2-s*r/3, y-s*r/3, s*r/3, pineLeaves)
	}
	seeThrough(dst)
	dst.FillRect(s*0.46, s*0.86, s*0.08, s*0.14, pineTrunk)
}

func heatherTop(dst *render.Canvas, size int) {
	s := float32(size)
	dst.FillCircle(s/2, s/2, s*0.4, heatherCrown)
	for i := range 6 {
		sin, cos := math.Sincos(float64(i) * math.Pi / 3)
		dst.FillCircle(s/2+float32(cos)*s*0.3, s/2+float32(sin)*s*0.3, s*0.18, heatherCrown)
	}
	dst.FillCircle(s*0.38, s*0.36, s*0.12, heatherLeaves)
	dst.FillCircle(s*0.62, s*0.6, s*0.08, heatherLeaves)
	seeThrough(dst)
}

func heatherSide(dst *render.Canvas, size int) {
	s := float32(size)
	dst.FillCircle(s*0.25, s*0.58, s*0.24, heatherCrown)
	dst.FillCircle(s*0.75, s*0.58, s*0.24, heatherCrown)
	dst.FillCircle(s*0.5, s*0.42, s*0.32, heatherCrown)
	dst.FillCircle(s*0.5, s*0.62, s*0.25, heatherCrown)
	dst.FillCircle(s*0.4, s*0.3, s*0.1, heatherLeaves)
	dst.FillCircle(s*0.66, s*0.48, s*0.08, heatherLeaves)
	dst.FillCircle(s*0.24, s*0.52, s*0.07, heatherLeaves)
	seeThrough(dst)
	dst.StrokeLine(s*0.45, s*0.84, s*0.4, s, s*0.07, heatherTrunk)
	dst.StrokeLine(s*0.55, s*0.84, s*0.62, s, s*0.07, heatherTrunk)
}

// seeThrough has a crown three quarters opaque, what walks under it showing through:
// premultiplied, every channel scaled alike
func seeThrough(dst *render.Canvas) {
	for i := range dst.Pix {
		dst.Pix[i] = uint8(uint16(dst.Pix[i]) * 3 / 4)
	}
}
