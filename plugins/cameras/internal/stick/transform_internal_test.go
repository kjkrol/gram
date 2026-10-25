package stick

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
)

func TestViewCamera_ItsTransformDrawsWhereItProjects(t *testing.T) {
	cam := testViewsOver(3200, true)
	cam.CenterOn(1600, 1600, 0)
	for view := range 3 {
		if view == 2 { // back in the parallel view, from above
			cam.Tilt(math.Pi)
		}
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
			if !ok || f32.Abs(float32(sx-wx)) > 0.05 || f32.Abs(float32(sy-wy)) > 0.05 {
				t.Errorf("view %d (perspective %v): %v drawn at (%v, %v), projected at (%v, %v)", view, cam.inPersp, p, sx, sy, wx, wy)
			}
		}
		cam.next()
	}
}

func TestViewCamera_ItsTransformDrawsWhereItProjectsCroppedOffCentre(t *testing.T) {
	cam := testViewsOver(3200, true)
	cam.CenterOn(1600, 1600, 0)
	for view := range 2 {
		cam.Crop(footedWindow, 0.5)
		f, ok := cam.Rays()
		if !ok {
			t.Fatalf("view %d: no rays", view)
		}
		w, h := cam.Viewport()
		tr, ok := camera.TransformOf(f, w, h, -1e5, 1e5)
		if cam.inPersp {
			tr, ok = camera.TransformOf(f, w, h, 1, 0)
		}
		if !ok {
			t.Fatalf("view %d: no transform", view)
		}
		for _, p := range [][3]float32{{1600, 1600, 0}, {1580, 1650, 20}, {1650, 1560, 45}} {
			wx, wy := cam.Project(p[0], p[1], p[2])
			sx, sy, _, ok := tr.Apply(p[0], p[1], p[2], w, h)
			if !ok || f32.Abs(float32(sx-wx)) > 0.05 || f32.Abs(float32(sy-wy)) > 0.05 {
				t.Errorf("view %d (perspective %v): %v drawn at (%v, %v), projected at (%v, %v)",
					view, cam.inPersp, p, sx, sy, wx, wy)
			}
		}
		cam.Uncrop()
		cam.next()
	}
}
