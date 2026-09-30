package cameras

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/camera"
)

// In every view the camera's transform for the GPU draws a world point where the camera
// projects it.
func TestViewCamera_ItsTransformDrawsWhereItProjects(t *testing.T) {
	cam := newCamera(testProjection, 3200, 3200, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300}, 0, true, nil, nil, 0)
	cam.CenterOn(1600, 1600, 0)
	for view := range 3 {
		f, ok := cam.Rays()
		if !ok {
			t.Fatalf("view %d: no rays", view)
		}
		tr, ok := camera.TransformOf(f, 400, 300, -1e5, 1e5)
		if cam.inPersp {
			tr, ok = camera.TransformOf(f, 400, 300, 1, 0)
		}
		if !ok {
			t.Fatalf("view %d: no transform", view)
		}
		for _, p := range [][3]float32{{1600, 1600, 0}, {1580, 1650, 20}, {1650, 1560, 45}} {
			wx, wy := cam.Project(p[0], p[1], p[2])
			sx, sy, _, ok := tr.Apply(p[0], p[1], p[2], 400, 300)
			if !ok || math.Abs(float64(sx-wx)) > 0.05 || math.Abs(float64(sy-wy)) > 0.05 {
				t.Errorf("view %d (perspective %v): %v drawn at (%v, %v), projected at (%v, %v)", view, cam.inPersp, p, sx, sy, wx, wy)
			}
		}
		cam.next()
	}
}
