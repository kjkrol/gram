package painter

import (
	"github.com/kjkrol/gram/render"
)

// Light is the light on the tile's top at its corners: even white, for the tiles are dressed only
// to be composed once, from above, for the ground drawn on the GPU, which lights them there.
func (t *tile) Light() render.Shade { return render.Even(1) }

// sunlit is how much of the sun reaches each of the tile's corners: all of it, the GPU casting the
// shadows.
func (t *tile) sunlit() [4]float32 { return [4]float32{1, 1, 1, 1} }

// FaceLight is the light on an upright face of the tile: white, as its top's.
func (t *tile) FaceLight(int, int) render.Light { return render.Light{1, 1, 1} }
