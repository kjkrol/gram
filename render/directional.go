package render

import (
	"image/color"
	"math"
)

func endpointAt(size int, angleDeg float64) (x, y float32) {
	cx, cy := float32(size)/2, float32(size)/2
	rad := angleDeg * math.Pi / 180
	dx, dy := float32(math.Cos(rad)), -float32(math.Sin(rad))
	half := float32(size) / 2
	scale := half / max(abs32(dx), abs32(dy))
	return cx + dx*scale, cy + dy*scale
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func Arrow(angleDeg float64, width float32, c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		x0, y0 := endpointAt(size, angleDeg)
		cx, cy := float32(size)/2, float32(size)/2
		dst.StrokeLine(x0, y0, cx, cy, width, c)
	}
}

func Dot(radius float32, c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		dst.FillCircle(float32(size)/2, float32(size)/2, radius, c)
	}
}
