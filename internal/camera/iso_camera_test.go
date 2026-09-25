package camera_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/camera"
)

var iso = contract.Isometric{Cell: 32, TileW: 64, TileH: 32, HeightUnit: 2}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func isoCamera(t *testing.T, edges aabbworld.Edges) contract.Camera {
	t.Helper()
	return camera.NewFromSpaceWithConfig(640, 640, edges, contract.Config{ViewportWidth: 400, ViewportHeight: 300, Projection: iso})
}

func TestCameras_ReportTheirViewportInPixels(t *testing.T) {
	for name, cam := range map[string]contract.Camera{
		"isometric": isoCamera(t, 0),
		"top-down":  camera.NewFromSpaceWithConfig(640, 640, 0, contract.Config{ViewportWidth: 400, ViewportHeight: 300}),
	} {
		cam.ZoomIn(2, 320, 320)
		if w, h := cam.Viewport(); w != 400 || h != 300 {
			t.Errorf("%s: Viewport = %v x %v after zooming, want the 400 x 300 screen", name, w, h)
		}
	}
}

func TestIsoCamera_RefusesAWrappingWorld(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("an isometric camera took a torus")
		}
	}()
	isoCamera(t, aabbworld.Torus)
}

func TestIsoCamera_DrawsThroughItsProjectionAndPansInPixels(t *testing.T) {
	cam := isoCamera(t, 0)
	if _, ok := cam.Projection().(contract.Isometric); !ok {
		t.Fatalf("Projection is %T, want Isometric", cam.Projection())
	}
	cam.MoveTo(320, 320)
	if sx, sy := cam.Project(320, 320, 0); !near(sx, 0) || !near(sy, 0) {
		t.Errorf("after MoveTo the point sits at (%v, %v), want the screen's corner", sx, sy)
	}
	bx, by := cam.Project(320, 320, 0)
	cam.Pan(10, -5)
	if ax, ay := cam.Project(320, 320, 0); !near(ax, bx-10) || !near(ay, by+5) {
		t.Errorf("Pan(10, -5) moved the point from (%v, %v) to (%v, %v)", bx, by, ax, ay)
	}
	px, py := cam.Project(100, 200, 0)
	x, y := cam.Unproject(px, py, 0)
	if !near(x, 100) || !near(y, 200) {
		t.Errorf("Unproject(Project(100, 200)) = (%v, %v)", x, y)
	}
}

func TestIsoCamera_ZoomKeepsTheAnchorAndBoundsStayInTheWorld(t *testing.T) {
	cam := isoCamera(t, 0)
	cam.MoveTo(320, 320)
	bx, by := cam.Project(330, 300, 0)
	cam.ZoomIn(2, 330, 300)
	if ax, ay := cam.Project(330, 300, 0); !near(ax, bx) || !near(ay, by) || cam.Zoom() != 2 {
		t.Errorf("after ZoomIn the anchor moved from (%v, %v) to (%v, %v) at zoom %v", bx, by, ax, ay, cam.Zoom())
	}
	b := cam.Bounds()
	if b.TopLeft.X < 0 || b.TopLeft.Y < 0 || b.BottomRight.X > 640 || b.BottomRight.Y > 640 || b.TopLeft.X >= b.BottomRight.X {
		t.Errorf("Bounds %v, want a rectangle inside the 640x640 world", b)
	}
	cx, cy := cam.Unproject(200, 150, 0)
	inside := func(x, y float32) bool {
		return float64(x) >= b.TopLeft.X && float64(x) <= b.BottomRight.X && float64(y) >= b.TopLeft.Y && float64(y) <= b.BottomRight.Y
	}
	if !inside(cx, cy) {
		t.Errorf("Bounds %v does not contain the ground under the screen centre (%v, %v)", b, cx, cy)
	}
	if !cam.Visible(geom.NewAABBAt(geom.NewVec(float64(cx)-5, float64(cy)-5), 10, 10)) {
		t.Error("a box under the screen centre is not Visible")
	}
	if cam.Visible(geom.NewAABBAt(geom.NewVec(0, 0), 1, 1)) && !inside(0, 0) {
		t.Error("a box far off the screen is Visible")
	}
}

func TestIsoCamera_PersistedRoundTrip(t *testing.T) {
	cam := isoCamera(t, 0)
	cam.MoveTo(300, 280)
	cam.ZoomIn(1.5, 320, 320)
	cam.Pan(7, 3)
	bx, by := cam.Project(320, 320, 0)
	saved := make([]any, 0, 2)
	for _, p := range cam.Persisted() {
		switch v := p.(type) {
		case *geom.Vec:
			saved = append(saved, *v)
		case *float32:
			saved = append(saved, *v)
		}
	}

	other := isoCamera(t, 0)
	for i, p := range other.Persisted() {
		switch v := p.(type) {
		case *geom.Vec:
			*v = saved[i].(geom.Vec)
		case *float32:
			*v = saved[i].(float32)
		}
	}
	other.Restore()
	if ax, ay := other.Project(320, 320, 0); !near(ax, bx) || !near(ay, by) || other.Zoom() != cam.Zoom() {
		t.Errorf("restored camera draws the point at (%v, %v) zoom %v, want (%v, %v) zoom %v", ax, ay, other.Zoom(), bx, by, cam.Zoom())
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

func TestCameras_CenterOnPutsThePointInTheMiddleOfTheScreen(t *testing.T) {
	for name, cam := range map[string]contract.Camera{
		"isometric": isoCamera(t, 0),
		"top-down":  camera.NewFromSpaceWithConfig(640, 640, 0, contract.Config{ViewportWidth: 400, ViewportHeight: 300}),
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
	cam := camera.NewFromSpaceWithConfig(640, 640, 0, contract.Config{ViewportWidth: 400, ViewportHeight: 300})
	cam.CenterOn(0, 0, 0)
	if b := cam.Bounds(); b.TopLeft.X != 0 || b.TopLeft.Y != 0 {
		t.Errorf("centred on the corner the window starts at %v, want it held inside the world at (0, 0)", b.TopLeft)
	}
}

func TestCameras_SetViewportKeepsTheMiddleAndCoversTheScreenWithTheWorld(t *testing.T) {
	for name, cam := range map[string]contract.Camera{
		"isometric": isoCamera(t, 0),
		"top-down":  camera.NewFromSpaceWithConfig(640, 640, 0, contract.Config{ViewportWidth: 400, ViewportHeight: 300}),
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
	small := camera.NewFromSpaceWithConfig(640, 640, 0, contract.Config{ViewportWidth: 640, ViewportHeight: 640})
	small.SetViewport(1280, 960)
	if z := small.Zoom(); !near(z, 2) {
		t.Errorf("a 640-unit world in a 1280-pixel window is at zoom %v, want 2: scaled up to cover it", z)
	}
	large := camera.NewFromSpaceWithConfig(4000, 4000, 0, contract.Config{ViewportWidth: 400, ViewportHeight: 300})
	large.SetViewport(1200, 900)
	if b := large.Bounds(); large.Zoom() != 1 || b.BottomRight.X-b.TopLeft.X != 1200 {
		t.Errorf("a large world in a larger window: zoom %v, bounds %v; want zoom 1 showing 1200 units", large.Zoom(), b)
	}
}
