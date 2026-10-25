package stick

import (
	"bytes"
	"encoding/gob"
	"math"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
)

func testCamera(t *testing.T, edges aabbworld.Edges) contract.Camera {
	t.Helper()
	return sized(newCamera(testOptics, Isometrically, 640, 640, edges, contract.Config{}, 0, true,
		nil, nil, 0), 400, 300)
}

// what one camera's Persisted holds, gob-encoded and decoded into another's
func carry(t *testing.T, from, to []any) {
	t.Helper()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, v := range from {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	dec := gob.NewDecoder(&buf)
	for _, v := range to {
		if err := dec.Decode(v); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOrthoCamera_RefusesAWrappingWorld(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a camera in relief took a torus")
		}
	}()
	testCamera(t, aabbworld.Torus)
}

func TestOrthoCamera_BeginsWhereItsStartSays(t *testing.T) {
	iso := testCamera(t, 0).(*viewCamera).ortho
	if !near(iso.Zoom(), 64/(math.Sqrt2*32)) || iso.turn != eighth ||
		!near(float32(iso.pitch), float32(testOptics.isoPitch())) {
		t.Errorf("isometrically the zoom is %v, the turn %v, the pitch %v; want %v, π/4, 30°",
			iso.Zoom(), iso.turn, iso.pitch, 64/(math.Sqrt2*32))
	}
	above := newOrthoCamera(testOptics, FromAbove, geom.NewVec(640, 640),
		geom.NewAABBAt(geom.NewVec(100, 50), 400, 300), 0)
	if sx, sy := above.Project(130, 70, 40); above.Zoom() != 1 || sx != 30 || sy != 20 {
		t.Errorf("from above (130, 70) 40 up is drawn at (%v, %v) at zoom %v, want (30, 20) at 1: "+
			"the map's window", sx, sy, above.Zoom())
	}
}

func TestOrthoCamera_DrawsThroughItsProjectionAndPansInPixels(t *testing.T) {
	cam := testCamera(t, 0)
	if _, ok := cam.Projection().(ortho); !ok {
		t.Fatalf("Projection is %T, want ortho", cam.Projection())
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
	if x, y := cam.Unproject(px, py, 0); !near(x, 100) || !near(y, 200) {
		t.Errorf("Unproject(Project(100, 200)) = (%v, %v)", x, y)
	}
}

func TestOrthoCamera_PanSlidesTheFootAtItsHeight(t *testing.T) {
	c := testCamera(t, 0).(*viewCamera).ortho
	c.ZoomIn(3, 320, 320) // room to slide in, the world's edge holding nothing
	c.CenterOn(300, 320, 40)
	fx, fy := c.Unproject(230, 130, 40)
	c.Pan(30, -20)
	if c.foot[2] != 40 {
		t.Errorf("panned, the foot is %v up, want 40 still", c.foot[2])
	}
	x, y := float32(c.foot[0]), float32(c.foot[1])
	if sx, sy := c.Project(x, y, 40); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("the foot (%v, %v) 40 up is drawn at (%v, %v), want the middle", x, y, sx, sy)
	}
	if !near(fx, x) || !near(fy, y) {
		t.Errorf("the foot went to (%v, %v), want what was drawn at (230, 130): (%v, %v)",
			x, y, fx, fy)
	}
}

func TestOrthoCamera_ZoomKeepsTheAnchorAndBoundsStayInTheWorld(t *testing.T) {
	cam := testCamera(t, 0)
	cam.MoveTo(100, 300)
	bx, by := cam.Project(330, 300, 0)
	before := cam.Zoom()
	cam.ZoomIn(2, 330, 300)
	if ax, ay := cam.Project(330, 300, 0); !near(ax, bx) || !near(ay, by) ||
		!near(cam.Zoom(), 2*before) {
		t.Errorf("after ZoomIn the anchor moved from (%v, %v) to (%v, %v) at zoom %v",
			bx, by, ax, ay, cam.Zoom())
	}
	// over ground the anchor is the point as it is drawn, on the ground's height
	hilly := sized(newCamera(testOptics, Isometrically, 640, 640, 0, contract.Config{}, 0, true,
		func(x, y float32) float32 { return 100 }, func() (float32, float32) { return 0, 100 },
		0), 400, 300)
	hilly.MoveTo(100, 300)
	hx, hy := hilly.Project(330, 300, 100)
	hilly.ZoomIn(2, 330, 300)
	if ax, ay := hilly.Project(330, 300, 100); !near(ax, hx) || !near(ay, hy) {
		t.Errorf("over ground 100 high the anchor moved from (%v, %v) to (%v, %v), want the "+
			"hill's top still under the cursor", hx, hy, ax, ay)
	}
	b := cam.Bounds()
	if b.TopLeft.X < 0 || b.TopLeft.Y < 0 || b.BottomRight.X > 640 || b.BottomRight.Y > 640 ||
		b.TopLeft.X >= b.BottomRight.X {
		t.Errorf("Bounds %v, want a rectangle inside the 640x640 world", b)
	}
	cx, cy := cam.Unproject(200, 150, 0)
	if !inBounds(b, cx, cy) {
		t.Errorf("Bounds %v does not contain the ground under the screen centre (%v, %v)",
			b, cx, cy)
	}
	if !cam.Visible(geom.NewAABBAt(geom.NewVec(float32(cx)-5, float32(cy)-5), 10, 10)) {
		t.Error("a box under the screen centre is not Visible")
	}
	if cam.Visible(geom.NewAABBAt(geom.NewVec(0, 0), 1, 1)) && !inBounds(b, 0, 0) {
		t.Error("a box far off the screen is Visible")
	}
}

func TestOrthoCamera_PersistedRoundTrip(t *testing.T) {
	cam := testCamera(t, 0).(*viewCamera).ortho
	cam.MoveTo(300, 280)
	cam.ZoomIn(1.5, 320, 320)
	cam.Pan(7, 3)
	cam.Turn(0.7)
	cam.Tilt(0.2)
	bx, by := cam.Project(320, 320, 0)
	other := testCamera(t, 0).(*viewCamera).ortho
	carry(t, cam.Persisted(), other.Persisted())
	other.Restore()
	if ax, ay := other.Project(320, 320, 0); !near(ax, bx) || !near(ay, by) ||
		other.Zoom() != cam.Zoom() || other.Pitch() != cam.Pitch() {
		t.Errorf("restored camera draws the point at (%v, %v) zoom %v pitch %v, want (%v, %v) "+
			"zoom %v pitch %v", ax, ay, other.Zoom(), other.Pitch(), bx, by, cam.Zoom(),
			cam.Pitch())
	}
}

func TestOrthoCamera_ReportsItsViewportInPixels(t *testing.T) {
	cam := testCamera(t, 0)
	cam.ZoomIn(2, 320, 320)
	if w, h := cam.Viewport(); w != 400 || h != 300 {
		t.Errorf("Viewport = %v x %v after zooming, want the 400 x 300 screen", w, h)
	}
}

func TestOrthoCamera_CenterOnPutsThePointInTheMiddleOfTheScreen(t *testing.T) {
	cam := testCamera(t, 0)
	for _, zoom := range []float32{1, 2} {
		cam.ZoomIn(zoom, 320, 320)
		cam.CenterOn(330, 310, 12)
		if sx, sy := cam.Project(330, 310, 12); !near(sx, 200) || !near(sy, 150) {
			t.Errorf("at zoom %v the point is drawn at (%v, %v), want the middle (200, 150)",
				cam.Zoom(), sx, sy)
		}
	}
}

func TestOrthoCamera_SetViewportKeepsTheMiddle(t *testing.T) {
	cam := testCamera(t, 0)
	cam.CenterOn(330, 310, 0)
	cam.SetViewport(440, 330)
	if w, h := cam.Viewport(); w != 440 || h != 330 {
		t.Errorf("Viewport %v x %v after SetViewport(440, 330)", w, h)
	}
	if sx, sy := cam.Project(330, 310, 0); !near(sx, 220) || !near(sy, 165) {
		t.Errorf("the point in the middle moved to (%v, %v), want (220, 165)", sx, sy)
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

func TestOrthoCamera_TheScreenStaysOverTheWorld(t *testing.T) {
	cam := testCamera(t, 0).(*viewCamera)
	pans := [][2]float32{{-1e4, 0}, {1e4, 0}, {0, -1e4}, {0, 1e4}, {1e4, 1e4}, {-1e4, 1e4}}
	for _, pan := range pans {
		cam.Pan(pan[0], pan[1])
		if !cornersOver(cam, 640) {
			t.Errorf("after Pan%v the screen's corners see %v, beyond the world", pan, cam.Bounds())
		}
	}
	cam.CenterOn(320, 320, 0)
	cam.ZoomOut(1000, 320, 320)
	// at the 2:1 view's zoom the screen's footprint is 500 across and 500 down
	if want := testOptics.tile() * 500 / 640; !near(cam.Zoom(), want) || !cornersOver(cam, 640) {
		t.Errorf("zoomed far out the zoom is %v with the screen over %v, want %v with the screen "+
			"over the world", cam.Zoom(), cam.Bounds(), want)
	}
	cam.Turn(-eighth)
	cam.Tilt(math.Pi)
	cam.ZoomOut(1000, 320, 320)
	if !near(cam.Zoom(), 400.0/640) || !cornersOver(cam, 640) {
		t.Errorf("from above zoomed far out the zoom is %v, want 400/640: the screen just "+
			"covered by the world", cam.Zoom())
	}
	cam.Pan(1e4, 1e4)
	if !cornersOver(cam, 640) {
		t.Errorf("from above panned far the screen sees %v, beyond the world", cam.Bounds())
	}
}

func TestOrthoCamera_TurnKeepsTheFootAndTurnsTheView(t *testing.T) {
	for name, pitch := range map[string]float32{"isometrically": 0, "from above": math.Pi} {
		cam := testCamera(t, 0).(*viewCamera)
		cam.Tilt(pitch)
		cam.CenterOn(330, 310, 20)
		cam.Turn(1)
		if sx, sy := cam.Project(330, 310, 20); !near(sx, 200) || !near(sy, 150) {
			t.Errorf("%s after Turn the foot is drawn at (%v, %v), want (200, 150)", name, sx, sy)
		}
		if !near(cam.Heading(), 1) || !near(float32(cam.ortho.turn), 1+eighth) {
			t.Errorf("%s: heading %v, turn %v, want 1 and 1+π/4", name, cam.Heading(),
				cam.ortho.turn)
		}
		cam.Turn(2 * math.Pi)
		if !near(cam.Heading(), 1) {
			t.Errorf("%s: a whole turn more gives heading %v, want 1 again", name, cam.Heading())
		}
		b := cam.Bounds()
		for _, corner := range [4][2]float32{{0, 0}, {400, 0}, {0, 300}, {400, 300}} {
			x, y := cam.Unproject(corner[0], corner[1], 0)
			if x, y = min(max(x, 0), 640), min(max(y, 0), 640); !inBounds(b, x, y) {
				t.Errorf("%s: the screen's corner %v shows the ground (%v, %v), outside Bounds %v",
					name, corner, x, y, b)
			}
		}
	}
}

func TestOrthoCamera_StepsOfATurnLandOnAnEighthExactly(t *testing.T) {
	cam := newOrthoCamera(testOptics, FromAbove, geom.NewVec(640, 640),
		geom.NewAABBAt(geom.NewVec(0, 0), 400, 300), 0)
	for range 18 {
		cam.Turn(math.Pi / 72)
	}
	if cam.turn != eighth || cam.proj.sin != cam.proj.cos {
		t.Errorf("18 steps of 2.5° turn the view %v, sin %v, cos %v; want π/4, the two equal",
			cam.turn, cam.proj.sin, cam.proj.cos)
	}
}

func TestOrthoCamera_TiltsSteeperUpToStraightDownAndFlatterDownToMinPitch(t *testing.T) {
	cam := testCamera(t, 0).(*viewCamera)
	cam.CenterOn(330, 310, 0)
	cam.Tilt(0.3)
	if sx, sy := cam.Project(330, 310, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("after Tilt the foot is drawn at (%v, %v), want (200, 150)", sx, sy)
	}
	if !near(cam.Pitch(), float32(testOptics.isoPitch())+0.3) {
		t.Errorf("tilted 0.3 the pitch is %v, want 0.3 steeper than 30°", cam.Pitch())
	}
	cam.Tilt(5)
	if cam.ortho.pitch != maxPitch || cam.Projection().Sorts() {
		t.Errorf("tilted far the pitch is %v, sorting %v; want straight down, not sorting",
			cam.ortho.pitch, cam.Projection().Sorts())
	}
	if _, lift := cam.Project(330, 310, 50); !near(lift, 150) {
		t.Errorf("straight down a point 50 up is drawn at y %v, want 150: not lifted", lift)
	}
	cam.Tilt(-5)
	if !near(cam.Pitch(), defaultMinPitch) || !cam.Projection().Sorts() {
		t.Errorf("tilted far the other way the pitch is %v, want held at %v",
			cam.Pitch(), defaultMinPitch)
	}
}

func TestOrthoCamera_LeansAndTurnsAboutTheGroundSeenInTheMiddle(t *testing.T) {
	c := sized(newCamera(testOptics, Isometrically, 3200, 3200, 0, contract.Config{}, 0, true,
		func(x, y float32) float32 { return 100 }, func() (float32, float32) { return 0, 100 },
		0), 400, 300)
	c.CenterOn(1600, 1600, 0) // the foot at sea level, under the ground
	x, y, z, _ := c.Pick(200, 150)
	for _, move := range []func(){
		func() { c.Tilt(0.4) },
		func() { c.Turn(0.7) },
		func() { c.Tilt(5) },
		func() { c.Tilt(-0.9) },
	} {
		move()
		if sx, sy := c.Project(x, y, z); !near(sx, 200) || !near(sy, 150) {
			t.Errorf("at pitch %v, heading %v the ground seen in the middle (%v, %v, %v) is drawn "+
				"at (%v, %v), want (200, 150)", c.Pitch(), c.Heading(), x, y, z, sx, sy)
		}
	}
}
