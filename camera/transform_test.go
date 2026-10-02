package camera

import (
	"math"
	"testing"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-2 }

// A parallel projection's transform draws a point where its ray through the screen starts over it,
// deeper the further along the rays.
func TestTransform_AParallelProjectionDrawsAPointUnderItsRay(t *testing.T) {
	f := RayField{Origin: [3]float32{10, 20, 100}, DX: [3]float32{0.5, 0.25, 0}, DY: [3]float32{-0.25, 0.5, 0.1}, Dir: [3]float32{0.2, -0.3, -1}}
	tr, ok := TransformOf(f, 640, 480, 0, 500)
	if !ok {
		t.Fatal("no transform")
	}
	for _, s := range [][3]float32{{0, 0, 0}, {320, 240, 50}, {600, 20, 200}} {
		p := [3]float32{f.Origin[0] + s[0]*f.DX[0] + s[1]*f.DY[0] + s[2]*f.Dir[0], f.Origin[1] + s[0]*f.DX[1] + s[1]*f.DY[1] + s[2]*f.Dir[1], f.Origin[2] + s[0]*f.DX[2] + s[1]*f.DY[2] + s[2]*f.Dir[2]}
		sx, sy, d, ok := tr.Apply(p[0], p[1], p[2], 640, 480)
		if !ok || !near(sx, s[0]) || !near(sy, s[1]) || !near(d, (500-s[2])/500) {
			t.Errorf("screen (%v, %v) %v along drew at (%v, %v) depth %v", s[0], s[1], s[2], sx, sy, d)
		}
	}
}

// A perspective's transform draws a point where the ray through the screen reaches it, the
// depth falling from 1 at near towards 0 far off; a point behind the eye is not drawn.
func TestTransform_APerspectiveDrawsAPointOnItsRay(t *testing.T) {
	f := RayField{Origin: [3]float32{5, 5, 50}, Dir: [3]float32{0.6, 0.8, -0.2}, DDX: [3]float32{0.002, -0.0015, 0}, DDY: [3]float32{0, 0, -0.0025}}
	tr, ok := TransformOf(f, 800, 600, 2, 0)
	if !ok {
		t.Fatal("no transform")
	}
	for _, s := range [][3]float32{{0, 0, 10}, {400, 300, 2}, {790, 10, 300}} {
		d := [3]float32{f.Dir[0] + s[0]*f.DDX[0] + s[1]*f.DDY[0], f.Dir[1] + s[0]*f.DDX[1] + s[1]*f.DDY[1], f.Dir[2] + s[0]*f.DDX[2] + s[1]*f.DDY[2]}
		p := [3]float32{f.Origin[0] + s[2]*d[0], f.Origin[1] + s[2]*d[1], f.Origin[2] + s[2]*d[2]}
		sx, sy, depth, ok := tr.Apply(p[0], p[1], p[2], 800, 600)
		if !ok || !near(sx, s[0]) || !near(sy, s[1]) || !near(depth, 2/s[2]) {
			t.Errorf("screen (%v, %v) %v along drew at (%v, %v) depth %v, want depth %v", s[0], s[1], s[2], sx, sy, depth, 2/s[2])
		}
	}
	if _, _, _, ok := tr.Apply(5-60, 5-80, 50, 800, 600); ok {
		t.Error("a point behind the eye was drawn")
	}
}

// A world point taken through the transform and back through its Unproject comes back where it
// stood, through a parallel projection and a perspective.
func TestTransform_UnprojectTakesTheClipPointBack(t *testing.T) {
	fields := map[string]RayField{
		"parallel":    {Origin: [3]float32{10, 20, 500}, DX: [3]float32{0.5, 0, 0}, DY: [3]float32{0, 0.4, -0.2}, Dir: [3]float32{0.1, 0.5, -1}},
		"perspective": {Origin: [3]float32{10, 20, 30}, DDX: [3]float32{0.002, 0, 0}, DDY: [3]float32{0, 0.001, -0.0017}, Dir: [3]float32{-0.4, 0.8, -0.3}},
	}
	for name, f := range fields {
		tr, ok := SceneTransform(f, 400, 300)
		if !ok {
			t.Fatalf("%s: no transform", name)
		}
		inv, ok := tr.Unproject()
		if !ok {
			t.Fatalf("%s: no inverse", name)
		}
		p := [4]float32{40, 90, 5, 1}
		var c, back [4]float32
		for r := range 4 {
			for k := range 4 {
				c[r] += tr.M[4*k+r] * p[k]
			}
		}
		for r := range 4 {
			for k := range 4 {
				back[r] += inv[4*k+r] * c[k]
			}
		}
		for k := range 3 {
			if d := back[k]/back[3] - p[k]; d > 1e-2 || d < -1e-2 {
				t.Errorf("%s: %v came back as %v", name, p, [3]float32{back[0] / back[3], back[1] / back[3], back[2] / back[3]})
				break
			}
		}
	}
}
