package compose

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render/internal/atlas"
)

// render's Even, Box, ProjectCorners and Solid, which a test of compose cannot import

func even(v float32) Shade {
	l := Light{v, v, v}
	return Shade{l, l, l, l}
}

func box(x0, y0, x1, y1 float32) World { return World{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} }

func projectCorners(cam camera.Camera, x0, y0, x1, y1, z float32) Corners {
	var out Corners
	for i, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		out[i][0], out[i][1] = cam.Project(p[0], p[1], z)
	}
	return out
}

func solid(c color.RGBA) atlas.SpriteDrawer {
	return func(dst *atlas.Canvas, size int) { dst.FillRect(0, 0, float32(size), float32(size), c) }
}
