package camera_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
)

var iso = camera.Isometric{Cell: 32, TileW: 64, TileH: 32, HeightUnit: 2}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func TestIsometric_ACellIsADiamondAndHeightLiftsAPoint(t *testing.T) {
	if sx, sy := iso.Project(32, 0, 0); !near(sx, 32) || !near(sy, 16) {
		t.Errorf("one cell along x projects to (%v, %v), want (32, 16): down-right", sx, sy)
	}
	if sx, sy := iso.Project(0, 32, 0); !near(sx, -32) || !near(sy, 16) {
		t.Errorf("one cell along y projects to (%v, %v), want (-32, 16): down-left", sx, sy)
	}
	if _, sy := iso.Project(0, 0, 10); !near(sy, -20) {
		t.Errorf("a point 10 up projects to y %v, want -20: two units a height unit", sy)
	}
}

func TestIsometric_UnprojectInvertsProject(t *testing.T) {
	for _, p := range [][3]float32{{0, 0, 0}, {100, 40, 0}, {13.5, 250, 7}, {-20, 5, 30}} {
		sx, sy := iso.Project(p[0], p[1], p[2])
		x, y := iso.Unproject(sx, sy, p[2])
		if !near(x, p[0]) || !near(y, p[1]) {
			t.Errorf("point %v went to (%v, %v) and came back as (%v, %v)", p, sx, sy, x, y)
		}
	}
}

func TestIsometric_DepthIsTheRowOfTheCellUnderThePoint(t *testing.T) {
	back, front := iso.Depth(0, 0, 0), iso.Depth(32, 32, 0)
	if back >= front {
		t.Errorf("depth at the origin %v is not behind (32, 32) %v", back, front)
	}
	if iso.Depth(32, 0, 0) != iso.Depth(0, 32, 0) {
		t.Error("two cells on the same row differ in depth")
	}
	if tile, standing := iso.Depth(48, 48, 0), iso.Depth(40, 60, 30); standing != tile || standing >= iso.Depth(64, 48, 0) {
		t.Errorf("a thing anywhere in a cell has depth %v, want its tile's %v, before the next row", standing, tile)
	}
}

func isoCamera(t *testing.T, edges aabbworld.Edges) camera.Camera {
	t.Helper()
	return camera.NewFromSpaceWithConfig(640, 640, edges, camera.Config{ViewportWidth: 400, ViewportHeight: 300, Projection: iso})
}

func TestCameras_ReportTheirViewportInPixels(t *testing.T) {
	for name, cam := range map[string]camera.Camera{
		"isometric": isoCamera(t, 0),
		"top-down":  camera.NewFromSpaceWithConfig(640, 640, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300}),
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
	if _, ok := cam.Projection().(camera.Isometric); !ok {
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
	if _, ok := cam.Projection().(camera.TopDown); !ok {
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
	for name, cam := range map[string]camera.Camera{
		"isometric": isoCamera(t, 0),
		"top-down":  camera.NewFromSpaceWithConfig(640, 640, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300}),
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
	cam := camera.NewFromSpaceWithConfig(640, 640, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300})
	cam.CenterOn(0, 0, 0)
	if b := cam.Bounds(); b.TopLeft.X != 0 || b.TopLeft.Y != 0 {
		t.Errorf("centred on the corner the window starts at %v, want it held inside the world at (0, 0)", b.TopLeft)
	}
}
