package stick

import (
	"math"
	"testing"

	contract "github.com/kjkrol/gram/camera"
)

func sameBox(a, b contract.AABB) bool {
	return math.Abs(a.TopLeft.X-b.TopLeft.X) < 1e-2 && math.Abs(a.TopLeft.Y-b.TopLeft.Y) < 1e-2 &&
		math.Abs(a.BottomRight.X-b.BottomRight.X) < 1e-2 &&
		math.Abs(a.BottomRight.Y-b.BottomRight.Y) < 1e-2
}

func TestOrthoCamera_AtAResolutionItsPixelsAreTheShownOnesTimesIt(t *testing.T) {
	cam := testCamera(t, 0).(*viewCamera).ortho
	cam.CenterOn(320, 320, 0)
	cam.ZoomIn(1.5, 320, 320)
	bounds, zoom, scale := cam.Bounds(), cam.Zoom(), cam.scale
	bx, by := cam.Project(300, 280, 4)

	cam.SetResolution(0.5)

	if w, h := cam.Viewport(); w != 200 || h != 150 || !near(cam.Zoom(), zoom/2) {
		t.Errorf("at 0.5 the viewport is %v x %v at zoom %v, want 200 x 150 at %v: half the shown",
			w, h, cam.Zoom(), zoom/2)
	}
	ax, ay := cam.Project(300, 280, 4)
	if !near(ax, bx/2) || !near(ay, by/2) {
		t.Errorf("(300, 280) 4 up is drawn at (%v, %v), want (%v, %v): half of where it was shown",
			ax, ay, bx/2, by/2)
	}
	if x, y := cam.Unproject(ax, ay, 4); !near(x, 300) || !near(y, 280) {
		t.Errorf("unprojected back, the point is (%v, %v), want (300, 280)", x, y)
	}
	if got := cam.Bounds(); !sameBox(got, bounds) {
		t.Errorf("the window is %v, want %v as before: the resolution changes nothing of it",
			got, bounds)
	}
	if saved := *cam.Persisted()[3].(*float32); saved != scale {
		t.Errorf("the scale saved is %v, want %v: in shown pixels", saved, scale)
	}
	x0, y0 := cam.Unproject(100, 100, 0)
	cam.Pan(10, 0)
	if sx, sy := cam.Project(x0, y0, 0); !near(sx, 90) || !near(sy, 100) {
		t.Errorf("after a pan of 10 camera pixels the point is drawn at (%v, %v), want (90, 100)",
			sx, sy)
	}
	cam.SetMaxZoom(4)
	cam.ZoomIn(100, 320, 320)
	if z := cam.Zoom(); !near(z, 2) {
		t.Errorf("zoomed far in the zoom is %v, want 2: a limit of 4 in shown pixels", z)
	}
}

func TestPerspCamera_AtAResolutionTheWindowStaysAndTheLimitsAreInShownPixels(t *testing.T) {
	c := testPersp(nil, nil)
	c.SetMaxZoom(6)
	bx, by := c.Project(330, 310, 12)

	c.SetResolution(0.5)

	if w, h := c.Viewport(); w != 200 || h != 150 {
		t.Errorf("at 0.5 the viewport is %v x %v, want 200 x 150: half the shown", w, h)
	}
	if ax, ay := c.Project(330, 310, 12); !near(ax, bx/2) || !near(ay, by/2) {
		t.Errorf("(330, 310) 12 up is drawn at (%v, %v), want (%v, %v): half of where it was "+
			"shown", ax, ay, bx/2, by/2)
	}
	c.SetViewport(400, 300)
	if w, h := c.Viewport(); w != 200 || h != 150 {
		t.Errorf("shown at 400 x 300 again the viewport is %v x %v, want 200 x 150", w, h)
	}
	c.ZoomIn(100, 320, 320)
	if z := c.Zoom(); !near(z, 3) {
		t.Errorf("zoomed far in the zoom is %v, want 3: a limit of 6 in shown pixels", z)
	}
	c.SetMinZoom(1)
	c.ZoomOut(1000, 320, 320)
	if z := c.Zoom(); !near(z, 0.5) {
		t.Errorf("zoomed far out the zoom is %v, want 0.5: a floor of 1 in shown pixels", z)
	}
}

func TestViewCamera_AtAResolutionTheViewsSwitchKeepingTheScale(t *testing.T) {
	c := testViews(true)
	c.CenterOn(330, 310, 0)
	shown, _ := c.Rays()

	c.SetResolution(0.5)

	if c.Resolution() != 0.5 || c.persp.res != 0.5 {
		t.Fatalf("the resolution is %v, the perspective's %v; want 0.5 in both views",
			c.Resolution(), c.persp.res)
	}
	if r, _ := c.Rays(); !near(r.DX[0], 2*shown.DX[0]) || !near(r.DX[1], 2*shown.DX[1]) {
		t.Errorf("a camera pixel steps %v, want %v: twice a shown one", r.DX, shown.DX)
	}
	w, h := c.Viewport()
	x0, y0 := c.Unproject(w/2, h/2, 0)
	scale := c.Zoom()
	for _, persp := range []bool{true, false, true} {
		c.next()
		if c.inPersp != persp {
			t.Fatalf("after Tab the perspective is %v, want %v", c.inPersp, persp)
		}
		if w, h := c.Viewport(); w != 200 || h != 150 {
			t.Errorf("perspective %v: the viewport is %v x %v, want 200 x 150", persp, w, h)
		}
		if s := c.Zoom(); !near(s/scale, 1) {
			t.Errorf("perspective %v: the zoom is %v, want %v as before", persp, s, scale)
		}
		if x, y := c.Unproject(w/2, h/2, 0); !near(x, x0) || !near(y, y0) {
			t.Errorf("perspective %v: the middle sees (%v, %v), want (%v, %v) as before",
				persp, x, y, x0, y0)
		}
	}
}
