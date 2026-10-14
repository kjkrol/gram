package cameras

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
	return sized(newCamera(testProjection, 640, 640, edges, contract.Config{}, 0, true, nil, nil, 0), 400, 300)
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
	cam.MoveTo(100, 300)
	if sx, sy := cam.Project(100, 300, 0); !near(sx, 0) || !near(sy, 0) {
		t.Errorf("after MoveTo the point sits at (%v, %v), want the screen's corner", sx, sy)
	}
	bx, by := cam.Project(100, 300, 0)
	cam.Pan(10, -5)
	if ax, ay := cam.Project(100, 300, 0); !near(ax, bx-10) || !near(ay, by+5) {
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
	cam.MoveTo(100, 300)
	bx, by := cam.Project(330, 300, 0)
	cam.ZoomIn(2, 330, 300)
	if ax, ay := cam.Project(330, 300, 0); !near(ax, bx) || !near(ay, by) || cam.Zoom() != 2 {
		t.Errorf("after ZoomIn the anchor moved from (%v, %v) to (%v, %v) at zoom %v", bx, by, ax, ay, cam.Zoom())
	}
	// over ground the anchor is the point as it is drawn, on the ground's height
	hilly := sized(newCamera(testProjection, 640, 640, 0, contract.Config{}, 0, true,
		func(x, y float32) float32 { return 100 }, func() (float32, float32) { return 0, 100 }, 0), 400, 300)
	hilly.MoveTo(100, 300)
	hx, hy := hilly.Project(330, 300, 100)
	hilly.ZoomIn(2, 330, 300)
	if ax, ay := hilly.Project(330, 300, 100); !near(ax, hx) || !near(ay, hy) {
		t.Errorf("over ground 100 high the anchor moved from (%v, %v) to (%v, %v), want the hill's top still under the cursor", hx, hy, ax, ay)
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
	cam.(*viewCamera).Turn(0.7)
	bx, by := cam.Project(320, 320, 0)
	saved := make([]any, 0, 11)
	for _, p := range cam.Persisted() {
		switch v := p.(type) {
		case *geom.Vec:
			saved = append(saved, *v)
		case *float32:
			saved = append(saved, *v)
		case *bool:
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
		case *bool:
			*v = saved[i].(bool)
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
		cam.SetViewport(440, 330)
		if w, h := cam.Viewport(); w != 440 || h != 330 {
			t.Errorf("%s: Viewport %v x %v after SetViewport(440, 330)", name, w, h)
		}
		if sx, sy := cam.Project(330, 310, 0); !near(sx, 220) || !near(sy, 165) {
			t.Errorf("%s: the point in the middle moved to (%v, %v), want (220, 165)", name, sx, sy)
		}
	}
}

func cornersOver(cam contract.Camera, size float32) bool {
	w, h := cam.Viewport()
	for _, corner := range [4][2]float32{{0, 0}, {w, 0}, {0, h}, {w, h}} {
		x, y := cam.Unproject(corner[0], corner[1], 0)
		if x < -1e-2 || x > size+1e-2 || y < -1e-2 || y > size+1e-2 {
			return false
		}
	}
	return true
}

func TestCamera_TheScreenStaysOverTheWorld(t *testing.T) {
	cam := testCamera(t, 0)
	for _, pan := range [][2]float32{{-1e4, 0}, {1e4, 0}, {0, -1e4}, {0, 1e4}, {1e4, 1e4}, {-1e4, 1e4}} {
		cam.Pan(pan[0], pan[1])
		if !cornersOver(cam, 640) {
			t.Errorf("after Pan%v the screen's corners see %v, beyond the world", pan, cam.Bounds())
		}
	}
	cam.CenterOn(320, 320, 0)
	cam.ZoomOut(1000, 320, 320)
	// the screen's footprint at zoom 1 is 500 across and 500 down: half its width plus its height
	if !near(cam.Zoom(), 500.0/640) || !cornersOver(cam, 640) {
		t.Errorf("zoomed far out the zoom is %v with the screen over %v, want 500/640 with the screen over the world", cam.Zoom(), cam.Bounds())
	}
	cam.(*viewCamera).iso.SetIsometric(false)
	cam.ZoomOut(1000, 320, 320)
	if !near(cam.Zoom(), 400.0/640) || !cornersOver(cam, 640) {
		t.Errorf("from above zoomed far out the zoom is %v, want 400/640: the screen just covered by the world", cam.Zoom())
	}
	cam.Pan(1e4, 1e4)
	if !cornersOver(cam, 640) {
		t.Errorf("from above panned far the screen sees %v, beyond the world", cam.Bounds())
	}
}

func TestCamera_TurnKeepsTheMiddleAndTurnsTheView(t *testing.T) {
	cam := testCamera(t, 0)
	cam.CenterOn(330, 310, 0)
	turner := cam.(*viewCamera)
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
	cam := testCamera(t, 0).(*viewCamera)
	cam.CenterOn(330, 310, 0)
	cam.Tilt(-0.3)
	if sx, sy := cam.Project(330, 310, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("after Tilt the middle point is drawn at (%v, %v), want (200, 150)", sx, sy)
	}
	cam.Tilt(-5)
	if !near(cam.Pitch(), defaultMinPitch) {
		t.Errorf("tilted far down the pitch is %v, want held at %v", cam.Pitch(), defaultMinPitch)
	}
	cam.Tilt(0.4)
	pitch := cam.Pitch()
	saved := *cam.Persisted()[3].(*float32)
	other := testCamera(t, 0).(*viewCamera)
	*other.Persisted()[3].(*float32) = saved
	other.Restore()
	if !near(other.Pitch(), pitch) {
		t.Errorf("restored pitch %v, want %v", other.Pitch(), pitch)
	}
}

func turned(cam contract.Camera, h float32) float32 {
	return float32(math.Remainder(float64(cam.(*viewCamera).Heading()-h), 2*math.Pi))
}

func TestCamera_APanTheEdgeStopsTurnsAndBowsTheView(t *testing.T) {
	for _, c := range []struct {
		name      string
		to, push  [2]float32
		turn, bow float32
	}{
		{"left", [2]float32{-1e4, 0}, [2]float32{-8, 0}, -0.08, 0},
		{"right", [2]float32{1e4, 0}, [2]float32{8, 0}, 0.08, 0},
		{"forward", [2]float32{0, -1e4}, [2]float32{0, -8}, 0, 0.04},
		{"back", [2]float32{0, 1e4}, [2]float32{0, 8}, 0, 0},
	} {
		cam := testCamera(t, 0).(*viewCamera)
		cam.Pan(c.to[0], c.to[1])
		cam.edgeTurn, cam.edgeTilt = 0.01, 0.005
		h, p := cam.Heading(), cam.Pitch()
		cam.Pan(c.push[0], c.push[1])
		if got := turned(cam, h); !near(got, c.turn) {
			t.Errorf("pushed %s at the edge: turned %v, want %v", c.name, got, c.turn)
		}
		if got := cam.Pitch() - p; !near(got, c.bow) {
			t.Errorf("pushed %s at the edge: bowed %v, want %v", c.name, got, c.bow)
		}
	}
	cam := testCamera(t, 0).(*viewCamera)
	cam.Pan(-1e4, 0)
	cam.edgeTurn = 0.01
	cam.Pan(8, 0)
	if got := turned(cam, cam.Heading()); got != 0 {
		t.Errorf("pushed away from the edge it turned %v", got)
	}
	cam.iso.SetIsometric(false)
	h := cam.Heading()
	cam.Pan(-1e4, 0)
	if got := turned(cam, h); got != 0 {
		t.Errorf("from above pushed at the edge it turned %v", got)
	}
}

func TestCamera_InPerspectiveAPanTheEdgeStopsTurnsAndBows(t *testing.T) {
	for _, c := range []struct {
		name      string
		push      [2]float32
		turn, bow bool
	}{
		{"left", [2]float32{-8, 0}, true, false},
		{"forward", [2]float32{0, -8}, false, true},
	} {
		cam := testCamera(t, 0).(*viewCamera)
		cam.next()
		if !cam.inPersp {
			t.Fatal("the camera did not go into perspective")
		}
		for range 400 {
			cam.Pan(c.push[0]*8, c.push[1]*8)
		}
		cam.edgeTurn, cam.edgeTilt = 0.01, 0.005
		h, p := cam.Heading(), cam.Pitch()
		cam.Pan(c.push[0], c.push[1])
		if got := turned(cam, h); (got != 0) != c.turn {
			t.Errorf("pushed %s at the edge: turned %v, want a turn %v", c.name, got, c.turn)
		}
		if got := cam.Pitch() - p; (got > 0) != c.bow {
			t.Errorf("pushed %s at the edge: bowed %v, want a bow %v", c.name, got, c.bow)
		}
	}
}

func TestCamera_DrawnBackTheViewLiesDownAsFarAsTheEdgeBowedIt(t *testing.T) {
	cam := testCamera(t, 0).(*viewCamera)
	cam.Pan(0, -1e4)
	cam.edgeTurn, cam.edgeTilt = 0.01, 0.005
	p := cam.Pitch()
	cam.Pan(0, -8)
	if got := cam.Pitch() - p; !near(got, 0.04) {
		t.Fatalf("pushed forward at the edge: bowed %v, want 0.04", got)
	}
	cam.Pan(0, 4)
	if got := cam.Pitch() - p; !near(got, 0.02) {
		t.Errorf("drawn back 4: still bowed %v, want 0.02, half of it laid down", got)
	}
	cam.Pan(0, 40)
	if got := cam.Pitch() - p; !near(got, 0) {
		t.Errorf("drawn back further: still bowed %v, want the pitch it started from, no lower", got)
	}
}

func TestCamera_ScrolledInACornerTheViewTurnsAndAtASideItPans(t *testing.T) {
	cam := testCamera(t, 0).(*viewCamera)
	cam.cornerTurn = 0.0025
	h := cam.Heading()
	cam.Scroll(-8, -8, true)
	if got := turned(cam, h); !near(got, -0.02) {
		t.Errorf("scrolled in the top-left corner: turned %v, want -0.02, as Q at a quarter", got)
	}
	cam.Scroll(8, 8, true)
	if got := turned(cam, h); !near(got, 0) {
		t.Errorf("scrolled in the bottom-right corner after: turned %v, want back to 0, as E", got)
	}
	at := cam.State().Viewport
	cam.Scroll(0, 8, false)
	if s := cam.State().Viewport; s == at || !near(turned(cam, h), 0) {
		t.Errorf("scrolled at the bottom side: moved %v, turned %v; want a pan, no turn", s != at,
			turned(cam, h))
	}
}
