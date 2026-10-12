package main

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/kjkrol/gram/render"
)

// game time: the tactical pause is a freeze-frame
const (
	gaitFrames = 4
)

// authored facing east within the inscribed circle; a placeholder for a sheet (sheetFrames)
func bugFrame(i int) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		s := float32(size)
		swing := [gaitFrames]float32{-3, 0, 3, 0}[i%gaitFrames]
		for k := float32(-1); k <= 1; k++ {
			x := s/2 + k*s/5
			dst.FillRect(x+swing, s/5, 2, s/5, legColor)
			dst.FillRect(x-swing, s-s/5-s/5, 2, s/5, legColor)
		}
		render.Dot(s/4, bodyColor)(dst, size)
		render.Arrow(0, 3, headColor)(dst, size)
	}
}

// sheetFrames cuts the frames out of a sprite sheet laid left to right, each frameW x frameH:
// what bugFrame gives way to once the game's own PNG lands (//go:embed it, image.Decode it, and
// hand the image here).
func sheetFrames(sheet image.Image, frameW, frameH int) func(frame int) render.SpriteDrawer {
	return func(frame int) render.SpriteDrawer {
		return func(dst *render.Canvas, size int) {
			src := image.Rect(frame*frameW, 0, (frame+1)*frameW, frameH)
			draw.Draw(dst.RGBA, dst.RGBA.Bounds(), sheet, src.Min, draw.Over)
		}
	}
}

var (
	bodyColor = color.RGBA{R: 120, G: 85, B: 50, A: 255}
	headColor = color.RGBA{R: 200, G: 170, B: 90, A: 255}
	legColor  = color.RGBA{R: 60, G: 45, B: 30, A: 255}
)
