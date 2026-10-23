package instanced

import (
	"image/color"

	"github.com/kjkrol/gram/render/internal/atlas"
	"github.com/kjkrol/gram/render/internal/gfx"
)

var white = color.RGBA{255, 255, 255, 255}

// render's Solid, which a test of instanced cannot import
func solid(c color.RGBA) atlas.SpriteDrawer {
	return func(dst *atlas.Canvas, size int) { dst.FillRect(0, 0, float32(size), float32(size), c) }
}

func pixelsOf(screen *gfx.Image) func(x, y int) [4]byte {
	w := screen.Bounds().Dx()
	pix := make([]byte, 4*w*screen.Bounds().Dy())
	screen.ReadPixels(pix)
	return func(x, y int) [4]byte { i := 4 * (y*w + x); return [4]byte(pix[i : i+4]) }
}
