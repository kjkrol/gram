package topdown

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
)

func TestBasicCamera_AtAResolutionItsPixelsAreTheShownOnesTimesIt(t *testing.T) {
	c := newBasicCamera(geom.NewVec(1000, 1000), testViewport(100, 100, 400, 300), 0)
	c.ZoomIn(2, 300, 250)
	bounds := c.Bounds()
	bx, by := c.Project(230, 210, 0)
	rays, _ := c.Rays()

	c.SetResolution(0.5)

	if w, h := c.Viewport(); w != 200 || h != 150 || c.Zoom() != 1 || c.Resolution() != 0.5 {
		t.Errorf("at 0.5 the viewport is %v x %v at zoom %v, want 200 x 150 at 1: half the shown",
			w, h, c.Zoom())
	}
	ax, ay := c.Project(230, 210, 0)
	if ax != bx/2 || ay != by/2 {
		t.Errorf("(230, 210) is drawn at (%v, %v), want (%v, %v): half of where it was shown",
			ax, ay, bx/2, by/2)
	}
	if x, y := c.Unproject(ax, ay, 0); math.Abs(float64(x-230)) > 1e-3 ||
		math.Abs(float64(y-210)) > 1e-3 {
		t.Errorf("unprojected back, the point is (%v, %v), want (230, 210)", x, y)
	}
	if r, _ := c.Rays(); r.DX[0] != 2*rays.DX[0] || r.DY[1] != 2*rays.DY[1] {
		t.Errorf("a camera pixel is %v world units, want %v: twice a shown one", r.DX[0],
			2*rays.DX[0])
	}
	if got := c.Bounds(); got != bounds {
		t.Errorf("the window is %v, want %v as before: the resolution changes nothing of it",
			got, bounds)
	}
	if zoom := *c.Persisted()[2].(*float32); zoom != 2 {
		t.Errorf("the zoom saved is %v, want 2: in shown pixels", zoom)
	}
	c.SetMaxZoom(4)
	c.ZoomIn(100, 300, 250)
	if z := c.Zoom(); z != 2 {
		t.Errorf("zoomed far in the zoom is %v, want 2: a limit of 4 in shown pixels", z)
	}
}

func TestBasicCamera_CroppedItDrawsTheMiddleOfItsFrameAndComesBackExactly(t *testing.T) {
	c := newBasicCamera(geom.NewVec(1000, 1000), testViewport(100, 100, 400, 300), 0)
	c.ZoomIn(2, 300, 250)
	viewport, res, effective, zoom := c.viewportSize, c.res, c.effective, c.Zoom()
	rays, _ := c.Rays()

	c.Crop(200, 150, 0.5)

	if w, h := c.Viewport(); w != 100 || h != 75 || c.Zoom() != 1 || !c.Cropped() {
		t.Errorf("cropped to 200 x 150 at 0.5 the viewport is %v x %v at zoom %v, cropped %v; "+
			"want 100 x 75 at 1, cropped", w, h, c.Zoom(), c.Cropped())
	}
	if got, want := c.Bounds(), geom.NewAABBAt(geom.NewVec(250, 212.5), 100, 75); got != want {
		t.Errorf("the window is %v, want %v: the middle quarter of the frame's", got, want)
	}
	if sx, sy := c.Project(300, 250, 0); sx != 50 || sy != 37.5 {
		t.Errorf("the middle is drawn at (%v, %v), want (50, 37.5)", sx, sy)
	}
	if r, _ := c.Rays(); r.DX[0] != 2*rays.DX[0] || r.DY[1] != 2*rays.DY[1] {
		t.Errorf("a camera pixel is %v world units, want %v: twice a shown one", r.DX[0],
			2*rays.DX[0])
	}

	c.Crop(400, 300, 0.5)
	if w, h := c.Viewport(); w != 200 || h != 150 || c.Cropped() {
		t.Errorf("cropped to the whole frame at 0.5 the viewport is %v x %v, cropped %v; "+
			"want 200 x 150, not cropped", w, h, c.Cropped())
	}

	c.Uncrop()
	if c.viewportSize != viewport || c.res != res || c.effective != effective ||
		c.Zoom() != zoom || c.Cropped() {
		t.Errorf("uncropped the camera is %v at %v over %v, zoom %v; want %v at %v over %v, "+
			"zoom %v: exactly as before", c.viewportSize, c.res, c.effective.AABB, c.Zoom(),
			viewport, res, effective.AABB, zoom)
	}
}
