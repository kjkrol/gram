package camera_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/camera"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

// sized is a top-down camera over a side x side world shown w x h pixels, its window at the
// world's top-left corner.
func sized(side uint32, w, h float32) contract.Camera {
	cam := camera.NewFromSpaceWithConfig(side, side, 0, contract.Config{})
	cam.SetViewport(w, h)
	cam.MoveTo(0, 0)
	return cam
}

func TestCameras_ReportTheirViewportInPixels(t *testing.T) {
	for name, cam := range map[string]contract.Camera{
		"top-down": sized(640, 400, 300),
	} {
		cam.ZoomIn(2, 320, 320)
		if w, h := cam.Viewport(); w != 400 || h != 300 {
			t.Errorf("%s: Viewport = %v x %v after zooming, want the 400 x 300 screen", name, w, h)
		}
	}
}

func TestCameras_CenterOnPutsThePointInTheMiddleOfTheScreen(t *testing.T) {
	for name, cam := range map[string]contract.Camera{
		"top-down": sized(640, 400, 300),
	} {
		for _, zoom := range []float32{1, 2} {
			cam.ZoomIn(zoom, 320, 320)
			cam.CenterOn(330, 310, 12)
			w, h := cam.Viewport()
			if sx, sy := cam.Project(330, 310, 12); !near(sx, w/2) || !near(sy, h/2) {
				t.Errorf("%s at zoom %v: the point is drawn at (%v, %v), want the middle (%v, %v)", name, cam.Zoom(), sx, sy, w/2, h/2)
			}
		}
	}
	cam := sized(640, 400, 300)
	cam.CenterOn(0, 0, 0)
	if b := cam.Bounds(); b.TopLeft.X != 0 || b.TopLeft.Y != 0 {
		t.Errorf("centred on the corner the window starts at %v, want it held inside the world at (0, 0)", b.TopLeft)
	}
}

func TestCameras_SetViewportKeepsTheMiddleAndCoversTheScreenWithTheWorld(t *testing.T) {
	for name, cam := range map[string]contract.Camera{
		"top-down": sized(640, 400, 300),
	} {
		cam.CenterOn(330, 310, 0)
		cam.SetViewport(500, 400)
		if w, h := cam.Viewport(); w != 500 || h != 400 {
			t.Errorf("%s: Viewport %v x %v after SetViewport(500, 400)", name, w, h)
		}
		if sx, sy := cam.Project(330, 310, 0); !near(sx, 250) || !near(sy, 200) {
			t.Errorf("%s: the point in the middle moved to (%v, %v), want (250, 200)", name, sx, sy)
		}
	}
	small := sized(640, 640, 640)
	small.SetViewport(1280, 960)
	if z := small.Zoom(); !near(z, 2) {
		t.Errorf("a 640-unit world in a 1280-pixel window is at zoom %v, want 2: scaled up to cover it", z)
	}
	large := sized(4000, 400, 300)
	large.SetViewport(1200, 900)
	if b := large.Bounds(); large.Zoom() != 1 || b.BottomRight.X-b.TopLeft.X != 1200 {
		t.Errorf("a large world in a larger window: zoom %v, bounds %v; want zoom 1 showing 1200 units", large.Zoom(), b)
	}
}

func TestTopDownCamera_ProjectIsToScreen(t *testing.T) {
	cam := camera.NewFromSpace(1000, 1000, 0, geom.NewAABBAt(geom.NewVec(100, 50), 400, 300))
	if _, ok := cam.Projection().(contract.TopDown); !ok {
		t.Fatalf("Projection is %T, want TopDown", cam.Projection())
	}
	sx, sy := cam.ToScreen(150, 80)
	if px, py := cam.Project(150, 80, 40); px != sx || py != sy {
		t.Errorf("Project = (%v, %v), want ToScreen's (%v, %v): height is not drawn", px, py, sx, sy)
	}
	if cam.Depth(0, 10, 0) >= cam.Depth(0, 20, 0) {
		t.Error("a point further up the screen is not drawn first")
	}
}
