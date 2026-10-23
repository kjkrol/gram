package look_test

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
)

// the left and the right half of a board 128 wide
type halves struct{}

func (halves) Len() int  { return 2 }
func (halves) Cols() int { return 2 }
func (halves) Box(i int) geom.AABB {
	return geom.NewAABB(geom.NewVec(float64(i)*64, 0), geom.NewVec(float64(i+1)*64, 128))
}
func (halves) Of(x, _ float64) int { return min(max(int(x/64), 0), 1) }
func (halves) Off(int) bool        { return false }

// seeing the left half a unit a pixel, the right a tenth of that, from the left edge's middle
type sighted struct {
	camera.Camera
	d camera.Detail
}

func (s sighted) Detail() camera.Detail  { return s.d }
func (sighted) Visible(camera.AABB) bool { return true }
func (sighted) Focus() (x, y, z float32) { return 0, 64, 0 }
func (sighted) ScaleAt(x, _, _ float32) float32 {
	return map[bool]float32{true: 1, false: 0.1}[x < 64]
}
