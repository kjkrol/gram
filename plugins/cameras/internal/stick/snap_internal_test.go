package stick

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
)

// isometric over hills as high as top says
func hillsUnder(top *float32) *viewCamera {
	return sized(newCamera(testOptics, Isometrically, 640, 640, 0, contract.Config{}, 0, true,
		func(x, y float32) float32 { return *top }, func() (float32, float32) { return 0, *top },
		0), 400, 300)
}

// what a camera draws: a point's place, its window, its sight
type seeing struct {
	px, py  float32
	bounds  contract.AABB
	visible bool
	rays    contract.RayField
	ex, ey  float32
	ez      float32
	vw, vh  float32
	zoom    float32
	inPersp bool
}

func seen(c *viewCamera) seeing {
	s := seeing{bounds: c.Bounds(), zoom: c.Zoom(), inPersp: c.inPersp}
	s.px, s.py = c.Project(300, 300, 20)
	s.visible = c.Visible(geom.NewAABBAt(geom.NewVec(320, 300), 10, 10))
	s.rays, _ = c.Rays()
	s.ex, s.ey, s.ez, _ = c.Eye()
	s.vw, s.vh = c.Viewport()
	return s
}

func TestViewCamera_ACopyDrawsAsItStoodWhileTheCameraMovesOn(t *testing.T) {
	top := float32(100)
	c := hillsUnder(&top)
	c.CenterOn(330, 310, 0)
	if c.Drawn() != contract.Camera(c) || c.Origin() != contract.Camera(c) {
		t.Fatal("before any copy the camera draws itself and is its own origin")
	}
	for _, persp := range []bool{false, true} {
		if persp {
			c.next()
		}
		c.Snap()
		d := c.Drawn().(*viewCamera)
		if d == c || d.Origin() != contract.Camera(c) || d.ortho == c.ortho ||
			d.persp == c.persp || (d.cur != contract.Camera(d.ortho) &&
			d.cur != contract.Camera(d.persp)) {
			t.Fatalf("perspective %v: snapped, the copy shares the camera's sticks or looks "+
				"through them", persp)
		}
		want := seen(c)
		if got := seen(d); got != want {
			t.Errorf("perspective %v: the copy sees %+v, want the camera's %+v", persp, got, want)
		}
		top = 300
		c.Pan(40, 0)
		if got := seen(d); got != want {
			t.Errorf("perspective %v: the camera moved on and the hills grew, the copy sees "+
				"%+v, want it as it stood, %+v", persp, got, want)
		}
		live := seen(c)
		d.Crop(contract.Window{W: 100, H: 80}, 0.5)
		if got := seen(c); got != live || c.Cropped() {
			t.Errorf("perspective %v: the copy cropped, the camera sees %+v cropped %v, want "+
				"%+v whole", persp, got, c.Cropped(), live)
		}
		d.Uncrop()
		top = 100
	}
}
