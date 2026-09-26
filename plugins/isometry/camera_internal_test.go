package isometry

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
)

var testProjection = projection{Cell: 32, TileW: 64, TileH: 32, HeightUnit: 2}.withDefaults()

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func testCamera(t *testing.T, edges aabbworld.Edges) contract.Camera {
	t.Helper()
	return newCamera(testProjection, 640, 640, edges, contract.Config{ViewportWidth: 400, ViewportHeight: 300})
}

func TestCamera_RefusesAWrappingWorld(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("an isometric camera took a torus")
		}
	}()
	testCamera(t, aabbworld.Torus)
}

func TestCamera_DrawsThroughItsProjectionAndPansInPixels(t *testing.T) {
	cam := testCamera(t, 0)
	if _, ok := cam.Projection().(projection); !ok {
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

func TestCamera_ZoomKeepsTheAnchorAndBoundsStayInTheWorld(t *testing.T) {
	cam := testCamera(t, 0)
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

func TestCamera_PersistedRoundTrip(t *testing.T) {
	cam := testCamera(t, 0)
	cam.MoveTo(300, 280)
	cam.ZoomIn(1.5, 320, 320)
	cam.Pan(7, 3)
	cam.(*isoCamera).Turn(0.7)
	bx, by := cam.Project(320, 320, 0)
	saved := make([]any, 0, 3)
	for _, p := range cam.Persisted() {
		switch v := p.(type) {
		case *geom.Vec:
			saved = append(saved, *v)
		case *float32:
			saved = append(saved, *v)
		}
	}

	other := testCamera(t, 0)
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

func TestCamera_ReportTheirViewportInPixels(t *testing.T) {
	for name, cam := range map[string]contract.Camera{
		"isometric": testCamera(t, 0),
	} {
		cam.ZoomIn(2, 320, 320)
		if w, h := cam.Viewport(); w != 400 || h != 300 {
			t.Errorf("%s: Viewport = %v x %v after zooming, want the 400 x 300 screen", name, w, h)
		}
	}
}

func TestCamera_CenterOnPutsThePointInTheMiddleOfTheScreen(t *testing.T) {
	for name, cam := range map[string]contract.Camera{
		"isometric": testCamera(t, 0),
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
}

func TestCamera_SetViewportKeepsTheMiddleAndCoversTheScreenWithTheWorld(t *testing.T) {
	for name, cam := range map[string]contract.Camera{
		"isometric": testCamera(t, 0),
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
}

func TestCamera_TurnKeepsTheMiddleAndTurnsTheView(t *testing.T) {
	cam := testCamera(t, 0)
	cam.CenterOn(330, 310, 0)
	turner := cam.(*isoCamera)
	turner.Turn(1)
	if sx, sy := cam.Project(330, 310, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("after Turn the middle point is drawn at (%v, %v), want (200, 150)", sx, sy)
	}
	if !near(turner.Heading(), 1) || !near(cam.Projection().(projection).Heading, 1) {
		t.Errorf("heading %v, projection's %v, want 1", turner.Heading(), cam.Projection().(projection).Heading)
	}
	turner.Turn(2 * math.Pi)
	if !near(turner.Heading(), 1) {
		t.Errorf("a whole turn more gives heading %v, want 1 again", turner.Heading())
	}
	b := cam.Bounds()
	for _, corner := range [4][2]float32{{0, 0}, {400, 0}, {0, 300}, {400, 300}} {
		x, y := cam.Unproject(corner[0], corner[1], 0)
		x, y = min(max(x, 0), 640), min(max(y, 0), 640)
		if float64(x) < b.TopLeft.X-1e-3 || float64(x) > b.BottomRight.X+1e-3 || float64(y) < b.TopLeft.Y-1e-3 || float64(y) > b.BottomRight.Y+1e-3 {
			t.Errorf("the screen's corner %v shows the ground (%v, %v), outside Bounds %v", corner, x, y, b)
		}
	}
}

func TestCamera_TiltKeepsTheMiddleStaysInRangeAndIsSaved(t *testing.T) {
	cam := testCamera(t, 0).(*isoCamera)
	cam.CenterOn(330, 310, 0)
	cam.Tilt(-0.3)
	if sx, sy := cam.Project(330, 310, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("after Tilt the middle point is drawn at (%v, %v), want (200, 150)", sx, sy)
	}
	cam.Tilt(-5)
	if !near(cam.Pitch(), minPitch) {
		t.Errorf("tilted far down the pitch is %v, want held at %v", cam.Pitch(), minPitch)
	}
	cam.Tilt(0.4)
	pitch := cam.Pitch()
	saved := *cam.Persisted()[3].(*float32)
	other := testCamera(t, 0).(*isoCamera)
	*other.Persisted()[3].(*float32) = saved
	other.Restore()
	if !near(other.Pitch(), pitch) {
		t.Errorf("restored pitch %v, want %v", other.Pitch(), pitch)
	}
}
