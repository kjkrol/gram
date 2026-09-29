package camera

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/camera"
)

// The top-down camera's transform for the GPU draws a world point where the camera does.
func TestBasicCamera_ItsTransformDrawsWhereItProjects(t *testing.T) {
	c := NewFromSpace(1000, 800, 0, camera.AABB{})
	c.SetViewport(400, 300)
	c.ZoomIn(2, 100, 100)
	c.MoveTo(120, 90)
	f, ok := c.(camera.Rays).Rays()
	if !ok {
		t.Fatal("no rays")
	}
	tr, ok := camera.TransformOf(f, 400, 300, 0, 1<<17)
	if !ok {
		t.Fatal("no transform")
	}
	for _, p := range [][2]float32{{120, 90}, {200, 150}, {300, 210}} {
		wx, wy := c.ToScreen(p[0], p[1])
		sx, sy, _, ok := tr.Apply(p[0], p[1], 0, 400, 300)
		if !ok || math.Abs(float64(sx-wx)) > 0.01 || math.Abs(float64(sy-wy)) > 0.01 {
			t.Errorf("%v drawn at (%v, %v), the camera at (%v, %v)", p, sx, sy, wx, wy)
		}
	}
}
