package topography

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
)

// testPersp is a perspective camera over a 640 x 640 world drawn to 400 x 300, at the 2:1 view's
// pitch and heading, looking at (320, 320) on the ground from the ceiling, over ground and its
// highest point.
func testPersp(ground func(x, y float32) float32, highest func() float32) *perspCamera {
	var extent func() (float32, float32)
	if highest != nil {
		extent = func() (float32, float32) { return 0, highest() }
	}
	c := newPerspCamera(testProjection, 0, geom.NewVec(640, 640), geom.NewAABBAt(geom.NewVec(0, 0), 400, 300), ground, extent)
	c.CenterOn(320, 320, 0)
	return c
}

func TestPerspCamera_CenterOnPutsThePointInTheMiddleFromWhereverItFlies(t *testing.T) {
	c := testPersp(nil, nil)
	if _, ok := c.Projection().(perspective); !ok {
		t.Fatalf("Projection is %T, want perspective", c.Projection())
	}
	if sx, sy := c.Project(320, 320, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("the point is drawn at (%v, %v), want the middle (200, 150)", sx, sy)
	}
	if e := c.eye(); !near(e[2], c.ceiling()) || !near(c.ceiling(), 2*32) {
		t.Errorf("the eye flies at %v, want the ceiling: two cells over level ground, 64", e)
	}
	c.alt = 200
	c.CenterOn(330, 310, 12)
	if sx, sy := c.Project(330, 310, 12); !near(sx, 200) || !near(sy, 150) || !near(c.eye()[2], 200) {
		t.Errorf("from 200 up the point is drawn at (%v, %v) and the eye flies at %v, want the middle and 200", sx, sy, c.eye()[2])
	}
	if at := c.target(); !near(at[2], 0) {
		t.Errorf("the middle of the screen looks at %v, want a point on the ground, at sea level", at)
	}
}

func TestPerspCamera_FliesOverTheHighestGround(t *testing.T) {
	high := float32(0)
	c := testPersp(nil, func() float32 { return high })
	if e := c.eye(); !near(e[2], 64) {
		t.Fatalf("over level ground the eye flies at %v, want 64", e)
	}
	high = 250
	c.CenterOn(320, 320, 0)
	if e := c.eye(); !near(e[2], 314) {
		t.Errorf("the ground raised to 250, the eye flies at %v, want 314: two cells over it", e)
	}
	if sx, sy := c.Project(320, 320, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("the point is drawn at (%v, %v), want the middle still", sx, sy)
	}
	c.ZoomIn(100, 320, 320)
	if e := c.eye(); e[2] < 314-1e-3 || c.narrow <= 1 {
		t.Errorf("zoomed far in the eye flies at %v with the view narrowed %v times, want no lower than 314 and narrowed", e, c.narrow)
	}
	c.Tilt(-5)
	c.Pan(1000, 1000)
	if e := c.eye(); e[2] < 314-1e-3 {
		t.Errorf("tilted and panned the eye flies at %v, want no lower than 314", e)
	}
}

func TestPerspCamera_PanMovesTheEyeAlongTheGroundAsTheMiddleMoves(t *testing.T) {
	c := testPersp(nil, nil)
	x, y := c.Unproject(210, 145, 0)
	e0 := c.eye()
	c.Pan(10, -5)
	if sx, sy := c.Project(x, y, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("after Pan(10, -5) the point drawn at (210, 145) before is at (%v, %v), want the middle", sx, sy)
	}
	if e := c.eye(); !near(e[2], e0[2]) || (near(e[0], e0[0]) && near(e[1], e0[1])) {
		t.Errorf("after Pan the eye went from %v to %v, want moved along the ground, as high", e0, e)
	}
	e1 := c.eye()
	c.Pan(0, -10000) // over the horizon: nothing to look at there
	if c.eye() != e1 {
		t.Errorf("a Pan over the horizon moved the eye from %v to %v", e1, c.eye())
	}
	c.Translate(10, -20)
	if e := c.eye(); !near(e[0], e1[0]+10) || !near(e[1], e1[1]-20) || !near(e[2], e1[2]) {
		t.Errorf("after Translate(10, -20) the eye is at %v, want %v moved 10, -20", e, e1)
	}
}

func TestPerspCamera_TurnGoesRoundTheMiddleAsHigh(t *testing.T) {
	c := testPersp(nil, nil)
	at, e0 := c.target(), c.eye()
	r := math.Hypot(float64(e0[0]-at[0]), float64(e0[1]-at[1]))
	c.Turn(1)
	e := c.eye()
	if !near(c.Heading(), 1) || !near(e[2], e0[2]) || !near(float32(math.Hypot(float64(e[0]-at[0]), float64(e[1]-at[1]))), float32(r)) {
		t.Errorf("after Turn(1) the heading is %v and the eye at %v, want 1 and the eye %v from %v as high as %v", c.Heading(), e, r, at, e0[2])
	}
	if sx, sy := c.Project(at[0], at[1], at[2]); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("after Turn the point in the middle is drawn at (%v, %v), want the middle", sx, sy)
	}
	if got := c.target(); math.Abs(float64(got[0]-at[0])) > 0.01 || math.Abs(float64(got[1]-at[1])) > 0.01 {
		t.Errorf("after Turn the middle looks at %v, want %v still", got, at)
	}
	c.Turn(2 * math.Pi)
	if !near(c.Heading(), 1) {
		t.Errorf("a whole turn more gives heading %v, want 1 again", c.Heading())
	}
}

func TestPerspCamera_TiltMovesTheHeadNotTheEye(t *testing.T) {
	c := testPersp(nil, nil)
	e0, at0 := c.eye(), c.target()
	c.Tilt(0.2)
	if e := c.eye(); e != e0 || !near(c.Pitch(), defaultMinPitch+0.2) {
		t.Errorf("after Tilt(0.2) the eye is at %v looking down at %v, want %v still, at %v: the head bows", e, c.Pitch(), e0, defaultMinPitch+0.2)
	}
	if at := c.target(); math.Hypot(float64(at[0]-e0[0]), float64(at[1]-e0[1])) >= math.Hypot(float64(at0[0]-e0[0]), float64(at0[1]-e0[1])) {
		t.Errorf("bowed, the middle looks at %v, want nearer the eye than %v", at, at0)
	}
	c.Tilt(-5)
	if !near(c.Pitch(), defaultMinPitch) || c.eye() != e0 {
		t.Errorf("raised far the pitch is %v and the eye at %v, want held at the floor %v, the eye %v", c.Pitch(), c.eye(), defaultMinPitch, e0)
	}
	c.Tilt(5)
	if !near(c.Pitch(), maxPitch) || c.eye() != e0 {
		t.Errorf("bowed far the pitch is %v and the eye at %v, want straight down %v, the eye %v", c.Pitch(), c.eye(), maxPitch, e0)
	}
	if at := c.target(); !near(at[0], e0[0]) || !near(at[1], e0[1]) {
		t.Errorf("straight down the middle looks at %v, want under the eye %v", at, e0)
	}
	if c.tilts != 3 {
		t.Errorf("the tilts count %d, want 3", c.tilts)
	}
}

func TestPerspCamera_ZoomComesInToTheCeilingThenNarrowsAndOutTheOtherWayRound(t *testing.T) {
	c := testPersp(nil, nil)
	e0, z0 := c.eye(), c.Zoom()
	c.ZoomIn(2, 320, 320)
	if e := c.eye(); e != e0 || !near(c.narrow, 2) || !near(c.Zoom()/z0, 2) {
		t.Errorf("at the ceiling, zoomed in twice the eye is at %v narrowed %v for a zoom %v times, want %v, 2 and 2", e, c.narrow, c.Zoom()/z0, e0)
	}
	c.ZoomOut(2, 320, 320)
	if e := c.eye(); e != e0 || !near(c.narrow, 1) || !near(c.Zoom(), z0) {
		t.Errorf("zoomed back out the eye is at %v narrowed %v, want %v unnarrowed", e, c.narrow, e0)
	}
	bx, by := c.Project(330, 300, 0)
	c.ZoomOut(2, 330, 300)
	if e := c.eye(); !near(e[2], 128) || !near(c.narrow, 1) || !near(c.Zoom()/z0, 0.5) {
		t.Errorf("zoomed out again the eye flies at %v narrowed %v for a zoom %v times, want twice as high, 128, unnarrowed, half", e, c.narrow, c.Zoom()/z0)
	}
	if ax, ay := c.Project(330, 300, 0); !near(ax, bx) || !near(ay, by) {
		t.Errorf("zoomed out the anchor moved from (%v, %v) to (%v, %v)", bx, by, ax, ay)
	}
	c.ZoomIn(2, 330, 300)
	if ax, ay := c.Project(330, 300, 0); !near(ax, bx) || !near(ay, by) || !near(c.eye()[2], 64) || !near(c.narrow, 1) {
		t.Errorf("zoomed back in the anchor is at (%v, %v), the eye flies at %v narrowed %v; want (%v, %v), 64 and unnarrowed", ax, ay, c.eye()[2], c.narrow, bx, by)
	}
	c.ZoomIn(2, 330, 300)
	if !near(c.eye()[2], 64) || !near(c.narrow, 2) {
		t.Errorf("zoomed in at the ceiling the eye flies at %v narrowed %v, want 64 and twice", c.eye()[2], c.narrow)
	}
	c.ZoomIn(100, 330, 300)
	if c.narrow > narrowest {
		t.Errorf("zoomed hard the view is narrowed %v times, want no more than %v", c.narrow, narrowest)
	}
}

// At the ceiling zooming in narrows the view about the middle of the screen; the head turns so
// the ground under the cursor stays where it is drawn, the view swinging towards it.
func TestPerspCamera_ZoomAtTheCeilingKeepsTheAnchorByTurningTheHead(t *testing.T) {
	c := testPersp(nil, nil)
	heading, pitch := c.heading, c.pitch
	bx, by := c.Project(330, 300, 0)
	c.ZoomIn(2, 330, 300)
	if ax, ay := c.Project(330, 300, 0); math.Abs(float64(ax-bx)) > 0.5 || math.Abs(float64(ay-by)) > 0.5 || !near(c.narrow, 2) {
		t.Errorf("zoomed in at the ceiling the anchor moved from (%v, %v) to (%v, %v), narrowed %v; want it still, narrowed twice", bx, by, ax, ay, c.narrow)
	}
	if c.heading == heading && c.pitch == pitch {
		t.Error("the head did not turn towards the anchor")
	}
	c.ZoomOut(2, 330, 300)
	if ax, ay := c.Project(330, 300, 0); math.Abs(float64(ax-bx)) > 0.5 || math.Abs(float64(ay-by)) > 0.5 || !near(c.narrow, 1) {
		t.Errorf("widened back the anchor is at (%v, %v), want (%v, %v) still", ax, ay, bx, by)
	}
}

// The ground point in the middle of the screen stays over the world however the eye is moved, and
// zooming out lifts the eye no higher than where the middle of the screen shows the world's
// diagonal across at the flattest pitch.
func TestPerspCamera_TheMiddleStaysOverTheWorldAndTheEyeNoHigherThanShowsItAcross(t *testing.T) {
	c := testPersp(nil, nil)
	over := func(what string) {
		if at := c.target(); at[0] < -1e-2 || at[0] > 640+1e-2 || at[1] < -1e-2 || at[1] > 640+1e-2 {
			t.Errorf("%s the middle of the screen looks at %v, beyond the 640x640 world", what, at)
		}
	}
	for range 40 {
		c.Pan(200, 200)
	}
	over("panned far")
	c.Translate(1e6, -1e6)
	over("translated far")
	if at := c.target(); !near(at[0], 640) || !near(at[1], 0) {
		t.Errorf("translated far to the north-east the middle looks at %v, want the world's corner (640, 0)", at)
	}
	c.CenterOn(320, 320, 0)
	c.ZoomOut(1000, 320, 320)
	w, _ := c.Viewport()
	diagonal := float32(math.Hypot(640, 640))
	if across := w / c.Zoom(); across > diagonal+1 || across < diagonal-1 || c.eye()[2] < c.ceiling() {
		t.Errorf("zoomed far out the middle of the screen shows %v across from %v up, want the diagonal %v", across, c.eye()[2], diagonal)
	}
	if !near(c.eye()[2], c.maxAlt()) || !near(c.narrow, 1) {
		t.Errorf("zoomed far out the eye flies at %v narrowed %v, want maxAlt %v unnarrowed", c.eye()[2], c.narrow, c.maxAlt())
	}
	over("zoomed far out")
}

func TestPerspCamera_LookFromStaysOverTheCeilingAndLookAtKeepsTheEye(t *testing.T) {
	c := testPersp(nil, nil)
	c.LookFrom(320, 520, 30)
	if e := c.eye(); !near(e[0], 320) || !near(e[1], 520) || !near(e[2], 64) {
		t.Errorf("after LookFrom 30 up the eye is at %v, want (320, 520) at the ceiling 64", e)
	}
	if sx, sy := c.Project(320, 320, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("after LookFrom the point looked at is drawn at (%v, %v), want the middle", sx, sy)
	}
	if c.Pitch() >= c.minPitch {
		t.Errorf("after LookFrom the pitch is %v, want below the floor %v until the next Tilt", c.Pitch(), c.minPitch)
	}
	eye := c.eye()
	c.LookAt(400, 320, 10)
	if e := c.eye(); e != eye {
		t.Errorf("after LookAt the eye moved from %v to %v", eye, e)
	}
	if sx, sy := c.Project(400, 320, 10); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("after LookAt the point is drawn at (%v, %v), want the middle", sx, sy)
	}
	c.Tilt(0.01)
	if c.Pitch() < c.minPitch {
		t.Errorf("after the next Tilt the pitch is %v, want held to the floor %v again", c.Pitch(), c.minPitch)
	}
}

func TestPerspCamera_BoundsHoldTheGroundShownAndVisibleTheBoxesOnTheScreen(t *testing.T) {
	c := testPersp(nil, nil)
	b := c.Bounds()
	if b.TopLeft.X < 0 || b.TopLeft.Y < 0 || b.BottomRight.X > 640 || b.BottomRight.Y > 640 || b.TopLeft.X >= b.BottomRight.X {
		t.Errorf("Bounds %v, want a rectangle inside the 640x640 world", b)
	}
	if b.TopLeft.X > 320 || b.BottomRight.X < 320 || b.TopLeft.Y > 320 || b.BottomRight.Y < 320 {
		t.Errorf("Bounds %v does not contain the ground under the middle of the screen", b)
	}
	if !c.Visible(geom.NewAABBAt(geom.NewVec(315, 315), 10, 10)) {
		t.Error("a box under the middle of the screen is not Visible")
	}
	e := c.eye()
	behind := add(e, scale(c.proj.forward, -50))
	if c.Visible(geom.NewAABBAt(geom.NewVec(float64(behind[0])-2, float64(behind[1])-2), 4, 4)) {
		t.Error("a box behind the eye is Visible")
	}
	if sx, sy := c.Project(behind[0], behind[1], behind[2]); math.Abs(float64(sx-200)) < 1e4 && math.Abs(float64(sy-150)) < 1e4 {
		t.Errorf("a point behind the eye is drawn at (%v, %v), want far off the screen", sx, sy)
	}
}

func TestPerspCamera_SetViewportKeepsTheEyeAndTheFieldOfView(t *testing.T) {
	c := testPersp(nil, nil)
	e := c.eye()
	top, _ := c.Unproject(200, 0, 0)
	c.SetViewport(500, 400)
	if w, h := c.Viewport(); w != 500 || h != 400 {
		t.Errorf("Viewport %v x %v after SetViewport(500, 400)", w, h)
	}
	if sx, sy := c.Project(320, 320, 0); !near(sx, 250) || !near(sy, 200) || c.eye() != e {
		t.Errorf("the point in the middle moved to (%v, %v), the eye to %v; want (250, 200) and %v", sx, sy, c.eye(), e)
	}
	if x, _ := c.Unproject(250, 0, 0); !near(x, top) {
		t.Errorf("the top of the screen looks at x %v, want %v: the same field of view over a taller screen", x, top)
	}
}

func TestPerspCamera_PersistedRoundTrip(t *testing.T) {
	c := testPersp(nil, nil)
	c.Turn(0.7)
	c.Tilt(0.2)
	c.ZoomIn(1.5, 320, 320)
	c.Pan(7, 3)
	bx, by := c.Project(300, 280, 4)
	other := testPersp(nil, nil)
	saved, restored := c.Persisted(), other.Persisted()
	if len(saved) != 5 {
		t.Fatalf("Persisted has %d values, want 5", len(saved))
	}
	for i := range saved {
		switch v := saved[i].(type) {
		case *geom.Vec:
			*restored[i].(*geom.Vec) = *v
		case *float32:
			*restored[i].(*float32) = *v
		default:
			t.Fatalf("Persisted()[%d] is %T", i, v)
		}
	}
	other.Restore()
	if ax, ay := other.Project(300, 280, 4); !near(ax, bx) || !near(ay, by) || !near(other.Zoom(), c.Zoom()) {
		t.Errorf("restored camera draws the point at (%v, %v) zoom %v, want (%v, %v) zoom %v", ax, ay, other.Zoom(), bx, by, c.Zoom())
	}
}

func TestPerspCamera_InsideAUnitTheEyeIsTheUnitsLookingWhereItIsTurned(t *testing.T) {
	c := testPersp(func(_, _ float32) float32 { return 0 }, nil)
	free := c.pose()
	c.enterInside([3]float32{300, 300, 20}, 0, 0)
	if e := c.eye(); e != [3]float32{300, 300, 20} || !near(c.Pitch(), 0) {
		t.Fatalf("inside, the eye stands at %v looking down at %v, want (300, 300, 20) along the ground", e, c.Pitch())
	}
	ax, ay := float32(300-100*math.Sqrt2/2), float32(300-100*math.Sqrt2/2) // 100 ahead, up the screen at heading 0
	if sx, sy := c.Project(ax, ay, 20); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("the point ahead at the eye's height is drawn at (%v, %v), want the middle", sx, sy)
	}
	heading := c.Heading()
	c.Pan(10, -8)
	if c.eye() != [3]float32{300, 300, 20} || c.Heading() != heading || c.Pitch() != 0 {
		t.Errorf("after Pan inside the eye is at %v looking from %v at %v, want nothing moved", c.eye(), c.Heading(), c.Pitch())
	}
	c.Turn(0.5)
	if !near(c.Heading(), heading+0.5) || c.eye() != [3]float32{300, 300, 20} {
		t.Errorf("after Turn(0.5) inside the heading is %v and the eye at %v, want %v on the spot", c.Heading(), c.eye(), heading+0.5)
	}
	c.Tilt(-1)
	if c.Pitch() >= 0 {
		t.Errorf("raised the pitch is %v, want negative: the sky, no floor inside", c.Pitch())
	}
	if _, sy := c.Project(300, 300, 1000); sy >= 150 {
		t.Errorf("looking up, a point high over the eye is drawn at y %v, want the upper half", sy)
	}
	c.Tilt(5)
	if !near(c.Pitch(), maxPitch) {
		t.Errorf("bowed far inside the pitch is %v, want straight down %v", c.Pitch(), maxPitch)
	}
	zoom := c.Zoom()
	c.ZoomIn(2, 0, 0)
	if e := c.eye(); !near(c.Zoom()/zoom, 2) || e != [3]float32{300, 300, 20} {
		t.Errorf("zoomed in inside the zoom is %v times what it was and the eye at %v, want twice, narrowed, and the eye where it was", c.Zoom()/zoom, e)
	}
	c.CenterOn(310, 300, 25)
	if e := c.eye(); e != [3]float32{310, 300, 25} {
		t.Errorf("after CenterOn the eye stands at %v, want (310, 300, 25)", e)
	}
	if at := c.underEye(); at != [3]float32{310, 300, 0} {
		t.Errorf("the ground under the eye is %v, want (310, 300, 0)", at)
	}
	c.setPose(free)
	if c.inside || c.narrow != 1 || !near(c.eye()[2], 64) || !near(c.Pitch(), free.pitch) {
		t.Errorf("set back free the eye is inside %v, narrowed %v, at %v looking down at %v; want free as it was", c.inside, c.narrow, c.eye(), c.Pitch())
	}
	c.enterInside([3]float32{300, 300, 20}, 0, 0)
	c.Tilt(-1)
	c.Restore()
	if c.inside || c.Pitch() < c.minPitch {
		t.Error("a restored camera is inside, or looks at the sky: a save keeps the view, not the eye in a unit")
	}
}

// Ray is the way a screen point looks: the middle of the screen the way the eye looks, the top of
// it higher, all of length 1.
func TestPerspCamera_RayIsTheWayAScreenPointLooks(t *testing.T) {
	c := testPersp(nil, nil)
	dx, dy, dz, ok := c.Ray(200, 150)
	if f := c.proj.forward; !ok || !near(dx, f[0]) || !near(dy, f[1]) || !near(dz, f[2]) || !near(dx*dx+dy*dy+dz*dz, 1) {
		t.Errorf("the middle of the screen looks along (%v, %v, %v) %v, want the eye's way %v, of length 1", dx, dy, dz, ok, f)
	}
	_, _, up, _ := c.Ray(200, 0)
	_, _, down, _ := c.Ray(200, 300)
	if !(up > dz && down < dz) {
		t.Errorf("the top of the screen looks %v up and the bottom %v, want higher and lower than the middle's %v", up, down, dz)
	}
}
