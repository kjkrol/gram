package painter

import (
	"github.com/kjkrol/gram/render"
)

// Light is the light on the tile's top at its corners: even white, the GPU lighting the ground.
func (t *tile) Light() render.Shade { return render.Even(1) }

// all of it: the GPU casts the shadows
func (t *tile) sunlit() [4]float32 { return [4]float32{1, 1, 1, 1} }
