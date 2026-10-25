package stick

import (
	"testing"

	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/vec"
)

func sameBox(a, b contract.AABB) bool {
	return f32.Abs(a.TopLeft.X-b.TopLeft.X) < 1e-2 && f32.Abs(a.TopLeft.Y-b.TopLeft.Y) < 1e-2 &&
		f32.Abs(a.BottomRight.X-b.BottomRight.X) < 1e-2 &&
		f32.Abs(a.BottomRight.Y-b.BottomRight.Y) < 1e-2
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

func TestOrthoCamera_CroppedItDrawsTheMiddleOfItsFrameAndComesBackExactly(t *testing.T) {
	cam := testCamera(t, 0).(*viewCamera).ortho
	cam.CenterOn(320, 320, 0)
	cam.ZoomIn(1.5, 320, 320)
	viewport, res, pan, foot, scale := cam.viewportSize, cam.res, cam.pan, cam.foot, cam.scale
	zoom := cam.Zoom()
	x0, y0 := cam.Unproject(100, 75, 0)
	x1, y1 := cam.Unproject(300, 225, 0)
	px, py := cam.Unproject(240, 150, 4)

	cam.Crop(contract.Window{X: 100, Y: 75, W: 200, H: 150}, 0.5)

	if w, h := cam.Viewport(); w != 100 || h != 75 || !near(cam.Zoom(), zoom/2) || !cam.Cropped() {
		t.Errorf("cropped to 200 x 150 at 0.5 the viewport is %v x %v at zoom %v, cropped %v; "+
			"want 100 x 75 at %v, cropped", w, h, cam.Zoom(), cam.Cropped(), zoom/2)
	}
	f := cam.foot
	if sx, sy := cam.Project(float32(f[0]), float32(f[1]), float32(f[2])); !near(sx, 50) ||
		!near(sy, 37.5) {
		t.Errorf("the foot is drawn at (%v, %v), want (50, 37.5): the middle", sx, sy)
	}
	if sx, sy := cam.Project(px, py, 4); !near(sx, 70) || !near(sy, 37.5) {
		t.Errorf("a point 40 shown pixels right of the middle is drawn at (%v, %v), want "+
			"(70, 37.5): 20 camera pixels right of it", sx, sy)
	}
	if x, y := cam.Unproject(0, 0, 0); !near(x, x0) || !near(y, y0) {
		t.Errorf("the corner sees (%v, %v), want (%v, %v): a quarter into the frame", x, y, x0, y0)
	}
	if x, y := cam.Unproject(100, 75, 0); !near(x, x1) || !near(y, y1) {
		t.Errorf("the far corner sees (%v, %v), want (%v, %v): three quarters into the frame",
			x, y, x1, y1)
	}
	if cam.scale != scale || cam.foot != foot || *cam.Persisted()[3].(*float32) != scale {
		t.Errorf("the scale is %v, the foot %v; want %v and %v as before, in shown pixels",
			cam.scale, cam.foot, scale, foot)
	}

	cam.Crop(contract.Window{W: 400, H: 300}, 0.5)
	if w, h := cam.Viewport(); w != 200 || h != 150 || cam.Cropped() {
		t.Errorf("cropped to the whole frame at 0.5 the viewport is %v x %v, cropped %v; "+
			"want 200 x 150, not cropped", w, h, cam.Cropped())
	}

	cam.Uncrop()
	if cam.viewportSize != viewport || cam.res != res || cam.pan != pan || cam.Zoom() != zoom ||
		cam.Cropped() {
		t.Errorf("uncropped the camera is %v at %v panned %v, zoom %v; want %v at %v panned %v, "+
			"zoom %v: exactly as before", cam.viewportSize, cam.res, cam.pan, cam.Zoom(),
			viewport, res, pan, zoom)
	}
}

func TestPerspCamera_CroppedTheFocalLengthIsTheFramesAndTheLimitsStay(t *testing.T) {
	c := testPersp(nil, nil)
	c.SetMaxZoom(6)
	c.SetMinZoom(1)
	f0, short, long := c.focal(), c.bounded(0), c.bounded(1e9)
	at, foot, length, proj := c.at, c.foot, c.length, c.proj
	px, py := c.Unproject(240, 150, 0)
	bx, by := c.Project(px, py, 0)

	c.Crop(contract.Window{X: 100, Y: 75, W: 200, H: 150}, 0.5)

	if w, h := c.Viewport(); w != 100 || h != 75 || !c.Cropped() {
		t.Errorf("cropped to 200 x 150 at 0.5 the viewport is %v x %v, cropped %v; want 100 x 75, "+
			"cropped", w, h, c.Cropped())
	}
	if f := c.focal(); !near(f, f0*0.5) {
		t.Errorf("the focal length is %v, want %v: the frame's at 0.5", f, f0*0.5)
	}
	if sx, sy := c.Project(px, py, 0); !near(sx, 70) || !near(sy, 37.5) {
		t.Errorf("a point 40 shown pixels right of the middle is drawn at (%v, %v), want "+
			"(70, 37.5): 20 camera pixels right of it", sx, sy)
	}
	if c.bounded(0) != short || c.bounded(1e9) != long {
		t.Errorf("the stick is bounded to %v..%v, want %v..%v as before: the zoom's limits are "+
			"the frame's", c.bounded(0), c.bounded(1e9), short, long)
	}
	if c.at != at || c.foot != foot || c.length != length {
		t.Errorf("the camera looks at %v from %v up %v; want %v from %v up %v as before",
			c.at, c.foot, c.length, at, foot, length)
	}

	c.Uncrop()
	if c.focal() != f0 || c.proj != proj || c.Cropped() {
		t.Errorf("uncropped the focal length is %v, cropped %v; want %v, not cropped, the "+
			"projection as before", c.focal(), c.Cropped(), f0)
	}
	if sx, sy := c.Project(px, py, 0); sx != bx || sy != by {
		t.Errorf("uncropped the point is drawn at (%v, %v), want (%v, %v) exactly", sx, sy, bx, by)
	}
}

func TestViewCamera_CroppedInPerspectiveTheMiddleLooksTheSameWay(t *testing.T) {
	c := testViews(true)
	c.CenterOn(330, 310, 0)
	c.next()
	if !c.inPersp {
		t.Fatal("after Tab the view is not the perspective")
	}
	w, h := c.Viewport()
	dx, dy, dz, _ := c.Ray(w/2, h/2)

	c.Crop(contract.Window{X: 100, Y: 75, W: 200, H: 150}, 0.5)

	cw, ch := c.Viewport()
	if cw != 100 || ch != 75 || !contract.IsCropped(c) {
		t.Errorf("cropped the viewport is %v x %v, cropped %v; want 100 x 75, cropped", cw, ch,
			contract.IsCropped(c))
	}
	if x, y, z, _ := c.Ray(cw/2, ch/2); x != dx || y != dy || z != dz {
		t.Errorf("the middle looks along (%v, %v, %v), want (%v, %v, %v) as before the crop",
			x, y, z, dx, dy, dz)
	}

	c.Uncrop()
	if w, h := c.Viewport(); w != 400 || h != 300 || contract.IsCropped(c) {
		t.Errorf("uncropped the viewport is %v x %v, cropped %v; want 400 x 300, not cropped",
			w, h, contract.IsCropped(c))
	}
}

// the middle of three layers footed in a 400x300 frame
var footedWindow = contract.Window{X: 80, Y: 120, W: 240, H: 180}

func TestOrthoCamera_CroppedToAFootedWindowTheFootStaysWhereItWasAndTheCornersSeeTheWindow(
	t *testing.T,
) {
	cam := testCamera(t, 0).(*viewCamera).ortho
	cam.CenterOn(320, 320, 0)
	cam.ZoomIn(1.5, 320, 320)
	viewport, res, pan, zoom := cam.viewportSize, cam.res, cam.pan, cam.Zoom()
	x0, y0 := cam.Unproject(80, 120, 0)
	x1, y1 := cam.Unproject(320, 300, 0)

	cam.Crop(footedWindow, 0.5)

	if w, h := cam.Viewport(); w != 120 || h != 90 || !near(cam.Zoom(), zoom/2) || !cam.Cropped() {
		t.Errorf("cropped to 240 x 180 at 0.5 the viewport is %v x %v at zoom %v, cropped %v; "+
			"want 120 x 90 at %v, cropped", w, h, cam.Zoom(), cam.Cropped(), zoom/2)
	}
	if fw, fh := cam.Frame(); cam.Window() != footedWindow || fw != 400 || fh != 300 {
		t.Errorf("the window is %v of a frame %v x %v, want %v of 400 x 300", cam.Window(), fw,
			fh, footedWindow)
	}
	f := cam.foot
	if sx, sy := cam.Project(float32(f[0]), float32(f[1]), float32(f[2])); !near(sx, 60) ||
		!near(sy, 15) {
		t.Errorf("the foot is drawn at (%v, %v), want (60, 15): the frame's middle, high in the "+
			"window", sx, sy)
	}
	if x, y := cam.Unproject(0, 0, 0); !near(x, x0) || !near(y, y0) {
		t.Errorf("the window's corner sees (%v, %v), want (%v, %v): the frame's (80, 120)", x, y,
			x0, y0)
	}
	if x, y := cam.Unproject(120, 90, 0); !near(x, x1) || !near(y, y1) {
		t.Errorf("the window's far corner sees (%v, %v), want (%v, %v): the frame's (320, 300), "+
			"on its bottom edge", x, y, x1, y1)
	}

	cam.Uncrop()
	if cam.viewportSize != viewport || cam.res != res || cam.pan != pan || cam.Zoom() != zoom ||
		cam.Cropped() || cam.Window() != (contract.Window{W: 400, H: 300}) {
		t.Errorf("uncropped the camera is %v at %v panned %v, zoom %v, window %v; want %v at %v "+
			"panned %v, zoom %v, the whole frame: exactly as before", cam.viewportSize, cam.res,
			cam.pan, cam.Zoom(), cam.Window(), viewport, res, pan, zoom)
	}
}

func TestPerspCamera_CroppedOffCentreTheLineOfSightThroughTheMiddleStays(t *testing.T) {
	c := testPersp(nil, nil)
	f0, at, proj := c.focal(), c.at, c.proj
	dx, dy, dz, _ := c.Ray(200, 150)
	px, py := c.Unproject(240, 150, 0)
	x0, y0 := c.Unproject(80, 120, 0)
	x1, y1 := c.Unproject(320, 300, 0)

	c.Crop(footedWindow, 0.5)

	if w, h := c.Viewport(); w != 120 || h != 90 || !c.Cropped() || c.Window() != footedWindow {
		t.Errorf("cropped the viewport is %v x %v, cropped %v, the window %v; want 120 x 90, "+
			"cropped, %v", w, h, c.Cropped(), c.Window(), footedWindow)
	}
	if f := c.focal(); !near(f, f0*0.5) {
		t.Errorf("the focal length is %v, want %v: the frame's at 0.5", f, f0*0.5)
	}
	if x, y, z, _ := c.Ray(60, 15); !near(x, dx) || !near(y, dy) || !near(z, dz) {
		t.Errorf("the frame's middle, (60, 15) of the window, looks along (%v, %v, %v), want "+
			"(%v, %v, %v) as before the crop", x, y, z, dx, dy, dz)
	}
	fw := c.proj.forward
	if sx, sy, ok := c.Vanish(fw[0], fw[1], fw[2]); !ok || !near(sx, 60) || !near(sy, 15) {
		t.Errorf("the way looked vanishes at (%v, %v), want (60, 15): the frame's middle", sx, sy)
	}
	if m, sky := c.middle(); sky || vec.Norm(vec.Sub(m, at)) > 1e-2 {
		t.Errorf("the middle sees %v, want %v as before the crop", m, at)
	}
	if sx, sy := c.Project(px, py, 0); !near(sx, 80) || !near(sy, 15) {
		t.Errorf("what the frame's (240, 150) sees is drawn at (%v, %v), want (80, 15)", sx, sy)
	}
	if x, y := c.Unproject(0, 0, 0); !near(x, x0) || !near(y, y0) {
		t.Errorf("the window's corner sees (%v, %v), want (%v, %v): the frame's (80, 120)", x, y,
			x0, y0)
	}
	if x, y := c.Unproject(120, 90, 0); !near(x, x1) || !near(y, y1) {
		t.Errorf("the window's far corner sees (%v, %v), want (%v, %v): the frame's (320, 300)",
			x, y, x1, y1)
	}
	b := c.Bounds()
	for _, p := range [][2]float32{{x0, y0}, {x1, y1}} {
		if float32(p[0]) < b.TopLeft.X-1e-2 || float32(p[0]) > b.BottomRight.X+1e-2 ||
			float32(p[1]) < b.TopLeft.Y-1e-2 || float32(p[1]) > b.BottomRight.Y+1e-2 {
			t.Errorf("the window's bounds %v leave out %v, a corner of it on the ground", b, p)
		}
	}

	c.Uncrop()
	if c.focal() != f0 || c.proj != proj || c.Cropped() ||
		c.Window() != (contract.Window{W: 400, H: 300}) {
		t.Errorf("uncropped the focal length is %v, cropped %v, the window %v; want %v, not "+
			"cropped, the whole frame, the projection as before", c.focal(), c.Cropped(),
			c.Window(), f0)
	}
}

func TestViewCamera_CroppedOffCentreUnprojectAndProjectAgreeInBothViews(t *testing.T) {
	c := testViews(true)
	c.CenterOn(330, 310, 0)
	frame := [][2]float32{{80, 120}, {200, 150}, {320, 300}, {130, 250}}
	for _, persp := range []bool{false, true} {
		if c.inPersp != persp {
			c.next()
		}
		seen := make([][2]float32, len(frame))
		for i, p := range frame {
			seen[i][0], seen[i][1] = c.Unproject(p[0], p[1], 4)
		}

		c.Crop(footedWindow, 0.5)

		if !contract.IsCropped(c) || c.Window() != footedWindow {
			t.Errorf("perspective %v: cropped %v to %v, want cropped to %v", persp,
				contract.IsCropped(c), c.Window(), footedWindow)
		}
		for i, p := range frame {
			sx, sy := (p[0]-footedWindow.X)*0.5, (p[1]-footedWindow.Y)*0.5
			x, y := c.Unproject(sx, sy, 4)
			if !near(x, seen[i][0]) || !near(y, seen[i][1]) {
				t.Errorf("perspective %v: (%v, %v) of the window sees (%v, %v), want (%v, %v): "+
					"what the frame's %v saw", persp, sx, sy, x, y, seen[i][0], seen[i][1], p)
			}
			if bx, by := c.Project(x, y, 4); !near(bx, sx) || !near(by, sy) {
				t.Errorf("perspective %v: (%v, %v) 4 up is drawn at (%v, %v), want (%v, %v)",
					persp, x, y, bx, by, sx, sy)
			}
		}
		c.Uncrop()
	}
}
