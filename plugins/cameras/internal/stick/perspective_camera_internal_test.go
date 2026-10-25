package stick

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/vec"
)

func testPersp(ground func(x, y float32) float32, highest func() float32) *perspCamera {
	var extent func() (float32, float32)
	if highest != nil {
		extent = func() (float32, float32) { return 0, highest() }
	}
	c := newPerspCamera(testOptics, 0, float32(testOptics.isoPitch()), 0, geom.NewVec(640, 640),
		geom.NewAABBAt(geom.NewVec(0, 0), 400, 300), ground, extent)
	c.CenterOn(320, 320, 0)
	return c
}

// the length the eye stands up the stick from its foot
func stickOf(c *perspCamera) float32 { return vec.Norm(vec.Sub(c.eye(), c.foot)) }

func TestPerspCamera_TheFootIsInTheMiddleTheEyeUpTheStick(t *testing.T) {
	c := testPersp(nil, nil)
	if _, ok := c.Projection().(perspective); !ok {
		t.Fatalf("Projection is %T, want perspective", c.Projection())
	}
	for _, foot := range [][3]float32{{320, 320, 0}, {330, 310, 12}} {
		c.CenterOn(float32(foot[0]), float32(foot[1]), float32(foot[2]))
		if sx, sy := c.Project(foot[0], foot[1], foot[2]); !near(sx, 200) || !near(sy, 150) {
			t.Errorf("the foot %v is drawn at (%v, %v), want the middle (200, 150)", foot, sx, sy)
		}
		want := vec.Add(foot, vec.Scale(c.out(), c.length))
		if e := c.eye(); vec.Norm(vec.Sub(e, want)) > 1e-3 {
			t.Errorf("the eye stands at %v, want %v: the stick's length up from the foot", e, want)
		}
	}
}

func TestPerspCamera_PanSlidesTheFootAtItsHeightOverTheWorld(t *testing.T) {
	c := testPersp(nil, nil)
	c.CenterOn(320, 320, 12)
	x, y := c.Unproject(210, 145, 12)
	c.Pan(10, -5)
	if sx, sy := c.Project(x, y, 12); !near(sx, 200) || !near(sy, 150) || c.foot[2] != 12 {
		t.Errorf("after Pan(10, -5) the point drawn at (210, 145) before is at (%v, %v), the foot "+
			"%v up; want the middle, 12", sx, sy, c.foot[2])
	}
	foot := c.foot
	c.Pan(0, -10000) // over the horizon: nothing to slide to
	if c.foot != foot {
		t.Errorf("a Pan over the horizon moved the foot from %v to %v", foot, c.foot)
	}
	c.Translate(10, -20)
	if !near(c.foot[0], foot[0]+10) || !near(c.foot[1], foot[1]-20) {
		t.Errorf("after Translate(10, -20) the foot is at %v, want %v moved", c.foot, foot)
	}
	for range 40 {
		c.Pan(200, 200)
	}
	c.Translate(1e6, -1e6)
	if c.foot[0] != 640 || c.foot[1] != 0 {
		t.Errorf("translated far to the north-east the foot is at %v, want the world's corner",
			c.foot)
	}
}

func TestPerspCamera_TurnAndTiltGoRoundTheFoot(t *testing.T) {
	c := testPersp(nil, nil)
	length := stickOf(c)
	c.Turn(1)
	if !near(c.Heading(), 1) || !near(stickOf(c), length) {
		t.Errorf("after Turn(1) the heading is %v, the eye %v off the foot; want 1, %v",
			c.Heading(), stickOf(c), length)
	}
	c.Tilt(0.2)
	if !near(c.Pitch(), defaultMinPitch+0.2) || !near(stickOf(c), length) {
		t.Errorf("after Tilt(0.2) the pitch is %v, the eye %v off the foot; want %v, %v",
			c.Pitch(), stickOf(c), defaultMinPitch+0.2, length)
	}
	if sx, sy := c.Project(320, 320, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("turned and tilted the foot is drawn at (%v, %v), want the middle", sx, sy)
	}
	c.Tilt(-5)
	if !near(c.Pitch(), defaultMinPitch) {
		t.Errorf("flattened far the pitch is %v, want held at the floor %v", c.Pitch(),
			defaultMinPitch)
	}
	c.Tilt(5)
	if e := c.eye(); !near(c.Pitch(), maxPitch) || !near(e[0], 320) || !near(e[1], 320) ||
		!near(e[2], length) {
		t.Errorf("steepened far the pitch is %v, the eye at %v; want straight over the foot, %v up",
			c.Pitch(), e, length)
	}
	if c.tilts != 3 {
		t.Errorf("the tilts count %d, want 3", c.tilts)
	}
}

func TestPerspCamera_TurnAndTiltStandTheStickOnTheGroundSeen(t *testing.T) {
	plateau := func(_, _ float32) float32 { return 40 }
	c := testPersp(plateau, func() float32 { return 40 })
	c.CenterOn(320, 320, 0) // the foot under the plateau
	x, y, z, _ := c.Pick(200, 150)
	for _, move := range []func(){func() { c.Turn(1) }, func() { c.Tilt(0.3) }} {
		move()
		if sx, sy := c.Project(x, y, z); !near(sx, 200) || !near(sy, 150) {
			t.Errorf("the ground seen in the middle (%v, %v, %v) is drawn at (%v, %v), want the "+
				"middle", x, y, z, sx, sy)
		}
	}
	if c.foot[2] != 40 {
		t.Errorf("the foot stands %v up, want on the plateau, 40", c.foot[2])
	}
}

func TestPerspCamera_ZoomShortensTheStickAboutTheCursor(t *testing.T) {
	c := testPersp(nil, nil)
	length := c.length
	ax, ay := c.Unproject(250, 180, 0)
	c.ZoomIn(2, ax, ay)
	sx, sy := c.Project(ax, ay, 0)
	if !near(sx, 250) || !near(sy, 180) || !near(c.length, length/2) {
		t.Errorf("zoomed in twice the anchor is drawn at (%v, %v), the stick %v long; want (250, "+
			"180), %v", sx, sy, c.length, length/2)
	}
	c.ZoomOut(2, ax, ay)
	if !near(c.length, length) {
		t.Errorf("zoomed back out the stick is %v long, want %v", c.length, length)
	}
	c.ZoomIn(1000, ax, ay)
	if !near(c.length, 32) {
		t.Errorf("zoomed far in the stick is %v long, want a cell, 32", c.length)
	}
	c.ZoomOut(1e6, ax, ay)
	if diagonal := float32(f32.Hypot(640, 640)); !near(c.length, diagonal) {
		t.Errorf("zoomed far out the stick is %v long, want the world's diagonal %v", c.length,
			diagonal)
	}
}

func TestPerspCamera_ZoomStopsWhereTheEyeWouldNotBeFree(t *testing.T) {
	// a plateau 30 high from where x and y sum to 700, round the eye
	behind := func(x, y float32) float32 {
		if x+y >= 700 {
			return 30
		}
		return 0
	}
	c := testPersp(behind, func() float32 { return 30 })
	length := c.length
	c.ZoomIn(2, 320, 320)
	if c.length <= length/2 || c.length >= length || c.need(c.rest()) != 0 {
		t.Errorf("zoomed in twice towards the plateau the stick is %v long, the eye %v short of "+
			"free; want between %v and %v, free", c.length, c.need(c.rest()), length/2, length)
	}
}

func TestPerspCamera_TheSpringLiftsTheEyeOverAHillAtOnceAndSettlesBack(t *testing.T) {
	// a hill 200 high where x and y sum to 760 to 840, behind the foot where the eye stands
	hill := func(x, y float32) float32 {
		if s := x + y; s >= 760 && s <= 840 {
			return 200
		}
		return 0
	}
	c := testPersp(hill, func() float32 { return 200 })
	rest := c.rest()
	if e := c.eye(); e[2] < 208 || !near(e[0], rest[0]) || !near(e[1], rest[1]) {
		t.Errorf("over the hill the eye stands at %v, want lifted over it a quarter of a cell, "+
			"straight up from %v", e, rest)
	}
	if sx, sy := c.Project(320, 320, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("lifted, the foot is drawn at (%v, %v), want the middle", sx, sy)
	}
	c.CenterOn(200, 200, 0) // the eye clear of the hill
	lifted := c.eye()[2] - c.rest()[2]
	c.settle(time.Second / 60)
	if got := c.eye()[2] - c.rest()[2]; got <= 0 || got >= lifted {
		t.Errorf("clear of the hill a tick on the eye is lifted %v, want easing down from %v",
			got, lifted)
	}
	c.settle(3 * time.Second)
	if got := c.eye()[2] - c.rest()[2]; got != 0 {
		t.Errorf("clear of the hill three seconds on the eye is lifted %v, want none", got)
	}
}

func TestPerspCamera_LookFromPutsTheEyeAndLookAtTheFoot(t *testing.T) {
	c := testPersp(nil, nil)
	c.LookFrom(320, 520, 30)
	if e := c.eye(); !near(e[0], 320) || !near(e[1], 520) || !near(e[2], 30) {
		t.Errorf("after LookFrom the eye is at %v, want (320, 520, 30)", e)
	}
	if sx, sy := c.Project(320, 320, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("after LookFrom the foot is drawn at (%v, %v), want the middle", sx, sy)
	}
	if c.Pitch() >= c.minPitch {
		t.Errorf("after LookFrom the pitch is %v, want below the floor %v until the next Tilt",
			c.Pitch(), c.minPitch)
	}
	eye := c.eye()
	c.LookAt(400, 320, 10)
	if e := c.eye(); vec.Norm(vec.Sub(e, eye)) > 1e-3 {
		t.Errorf("after LookAt the eye moved from %v to %v", eye, e)
	}
	if sx, sy := c.Project(400, 320, 10); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("after LookAt the point is drawn at (%v, %v), want the middle", sx, sy)
	}
	c.Tilt(0.01)
	if c.Pitch() < c.minPitch {
		t.Errorf("after the next Tilt the pitch is %v, want held to the floor %v again",
			c.Pitch(), c.minPitch)
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
	behind := vec.Add(e, vec.Scale(c.proj.forward, -50))
	if c.Visible(geom.NewAABBAt(geom.NewVec(float32(behind[0])-2, float32(behind[1])-2), 4, 4)) {
		t.Error("a box behind the eye is Visible")
	}
	if sx, sy := c.Project(behind[0], behind[1], behind[2]); f32.Abs(float32(sx-200)) < 1e4 &&
		f32.Abs(float32(sy-150)) < 1e4 {
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
	if len(c.Persisted()) != 4 {
		t.Fatalf("Persisted has %d values, want 4: the foot, the length, the heading, the pitch",
			len(c.Persisted()))
	}
	carry(t, c.Persisted(), other.Persisted())
	other.Restore()
	if ax, ay := other.Project(300, 280, 4); !near(ax, bx) || !near(ay, by) {
		t.Errorf("restored camera draws the point at (%v, %v), want (%v, %v)", ax, ay, bx, by)
	}
}

func TestPerspCamera_InsideAUnitTheEyeIsTheUnitsLookingWhereItIsTurned(t *testing.T) {
	c := testPersp(func(_, _ float32) float32 { return 0 }, nil)
	free := c.pose()
	c.enterInside([3]float32{300, 300, 20}, 0, 0)
	if e := c.eye(); e != [3]float32{300, 300, 20} || !near(c.Pitch(), 0) {
		t.Fatalf("inside, the eye stands at %v looking down at %v, want (300, 300, 20) along the ground", e, c.Pitch())
	}
	// 100 ahead, up the screen at heading 0
	ax, ay := float32(300-100*math.Sqrt2/2), float32(300-100*math.Sqrt2/2)
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
	if c.inside || c.narrow != 1 || vec.Norm(vec.Sub(c.eye(), c.rest())) > 1e-3 ||
		!near(c.Pitch(), free.pitch) {
		t.Errorf("set back free the eye is inside %v, narrowed %v, at %v looking down at %v; want free as it was", c.inside, c.narrow, c.eye(), c.Pitch())
	}
	c.enterInside([3]float32{300, 300, 20}, 0, 0)
	c.Tilt(-1)
	c.Restore()
	if c.inside || c.Pitch() < c.minPitch {
		t.Error("a restored camera is inside, or looks at the sky: a save keeps the view, not the eye in a unit")
	}
}

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
