package look_test

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
)

// the left and the right half of a board 128 wide, each at the level ladder says at its edge
// nearest the camera's focus
type halves struct{ ladder lod.Ladder }

func (halves) Len() int  { return 2 }
func (halves) Cols() int { return 2 }
func (halves) Box(i int) geom.AABB {
	return geom.NewAABB(geom.NewVec(float32(i)*64, 0), geom.NewVec(float32(i+1)*64, 128))
}
func (halves) Of(x, _ float32) int  { return min(max(int(x/64), 0), 1) }
func (h halves) Ladder() lod.Ladder { return h.ladder }

func (h halves) Level(cam camera.Camera, i int) lod.Level {
	x := float32(i) * 64
	return h.ladder.Widened(camera.SightOf(cam)).At(camera.ScaleShown(cam, x, 64, 0), x)
}

// seeing the left half a unit a pixel, the right a tenth of that, from the left edge's middle
type sighted struct{ camera.Camera }

func (sighted) Visible(camera.AABB) bool { return true }
func (sighted) Focus() (x, y, z float32) { return 0, 64, 0 }
func (sighted) ScaleAt(x, _, _ float32) float32 {
	return map[bool]float32{true: 1, false: 0.1}[x < 64]
}
