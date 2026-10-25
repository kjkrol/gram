package stick

import (
	"bytes"
	"encoding/gob"
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/vec"
)

func sized(c *viewCamera, w, h float32) *viewCamera {
	c.SetViewport(w, h)
	c.MoveTo(0, 0)
	return c
}

// isometric over a size x size world
func testViewsOver(size uint32, reaches bool) *viewCamera {
	return sized(newCamera(testOptics, Isometrically, size, size, 0, contract.Config{}, 0, reaches,
		nil, nil, 0), 400, 300)
}

func testViews(reaches bool) *viewCamera { return testViewsOver(640, reaches) }

func middle(c *viewCamera) (x, y float32) { return c.Unproject(200, 150, 0) }

func TestViewCamera_NextSwitchesTheProjectionKeepingTheStick(t *testing.T) {
	c := testViews(true)
	c.CenterOn(330, 310, 0)
	c.Turn(0.3)
	c.Tilt(0.2)
	x0, y0 := middle(c)
	heading, pitch, scale := c.Heading(), c.Pitch(), c.Zoom()
	for _, persp := range []bool{true, false, true} {
		c.next()
		if c.inPersp != persp || (persp && c.cur != contract.Camera(c.persp)) ||
			(!persp && c.cur != contract.Camera(c.ortho)) {
			t.Fatalf("in perspective %v, want %v; current %T", c.inPersp, persp, c.cur)
		}
		if x, y := middle(c); !near(x, x0) || !near(y, y0) {
			t.Errorf("perspective %v: the middle of the screen shows (%v, %v), want (%v, %v)",
				persp, x, y, x0, y0)
		}
		if !near(c.Heading(), heading) || !near(c.Pitch(), pitch) || !near(c.Zoom()/scale, 1) {
			t.Errorf("perspective %v: heading %v, pitch %v, zoom %v; want %v, %v, %v as before",
				persp, c.Heading(), c.Pitch(), c.Zoom(), heading, pitch, scale)
		}
		if _, ok := c.Projection().(perspective); ok != persp {
			t.Errorf("perspective %v: Projection is %T", persp, c.Projection())
		}
	}
	unreached := testViews(false)
	unreached.CenterOn(330, 310, 0)
	bx, by := unreached.Project(300, 300, 0)
	unreached.next()
	if ax, ay := unreached.Project(300, 300, 0); unreached.inPersp || ax != bx || ay != by {
		t.Errorf("a camera not reaching the perspective switched: perspective %v, (300, 300) "+
			"from (%v, %v) to (%v, %v)", unreached.inPersp, bx, by, ax, ay)
	}
}

func TestViewCamera_NextOverHighGroundKeepsTheGroundSeenInTheMiddle(t *testing.T) {
	c := sized(newCamera(testOptics, Isometrically, 3200, 3200, 0, contract.Config{}, 0, true,
		func(x, y float32) float32 { return 100 }, func() (float32, float32) { return 0, 100 },
		0), 400, 300)
	c.CenterOn(1600, 1600, 0) // the foot at sea level, under the ground
	x, y, z, _ := c.Pick(200, 150)
	scale := c.Zoom()
	for _, persp := range []bool{true, false} {
		c.next()
		if sx, sy := c.Project(x, y, z); c.inPersp != persp || !near(sx, 200) || !near(sy, 150) {
			t.Errorf("perspective %v (%v): the ground seen in the middle is drawn at (%v, %v), "+
				"want (200, 150)", persp, c.inPersp, sx, sy)
		}
		if s := c.Zoom(); !near(s/scale, 1) {
			t.Errorf("perspective %v: the zoom is %v, want %v as before", persp, s, scale)
		}
	}
}

func TestViewCamera_LookFromGoesIntoPerspectiveWhereReached(t *testing.T) {
	c := testViews(true)
	if !c.LookFrom(100, 100, 50) || !c.inPersp {
		t.Fatal("LookFrom on the isometric view did not go into the perspective one")
	}
	if e := c.persp.proj.eye; !near(e[0], 100) || !near(e[1], 100) || !near(e[2], 50) {
		t.Errorf("the eye is at %v, want (100, 100, 50)", e)
	}
	unreached := testViews(false)
	if unreached.LookFrom(100, 100, 50) || unreached.LookAt(1, 2, 3) || unreached.enterInside([3]float32{1, 1, 1}, 0, 0) || unreached.inPersp {
		t.Error("a camera not reaching the perspective moved its eye")
	}
}

func TestViewCamera_PersistedRoundTripKeepsTheView(t *testing.T) {
	c := testViews(true)
	c.next()
	c.Turn(0.7)
	c.ZoomIn(1.5, 320, 320)
	c.SetDetail(contract.Detail{Sight: 900})
	c.See(2)
	bx, by := c.Project(300, 280, 4)
	other := testViews(true)
	saved, restored := c.Persisted(), other.Persisted()
	if len(saved) != 12 || len(restored) != 12 {
		t.Fatalf("Persisted has %d and %d values, want 12: the form, every view's, which is in, "+
			"the fastening and the sight", len(saved), len(restored))
	}
	carry(t, saved, restored)
	other.Restore()
	if other.Detail().Sight != 1800 {
		t.Errorf("restored camera sees %v far, want the 1800 it was saved seeing",
			other.Detail().Sight)
	}
	if !other.inPersp || other.cur != contract.Camera(other.persp) {
		t.Fatal("restored camera is not in the perspective view")
	}
	if ax, ay := other.Project(300, 280, 4); !near(ax, bx) || !near(ay, by) {
		t.Errorf("restored camera draws the point at (%v, %v), want (%v, %v)", ax, ay, bx, by)
	}
	unreached := testViews(false)
	carry(t, saved, unreached.Persisted())
	unreached.Restore()
	if unreached.inPersp {
		t.Error("a save in perspective loaded by a game not reaching it stayed in perspective")
	}
}

type otherForm struct{}

func (otherForm) GobEncode() ([]byte, error) { return []byte("stick 1"), nil }

func TestViewCamera_RefusesASaveOfAnotherFormAtItsFirstValue(t *testing.T) {
	for name, first := range map[string]any{
		"the views before the stick": &geom.Vec{X: 10, Y: 20},
		"the stick before its eye":   otherForm{},
	} {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(first); err != nil {
			t.Fatal(err)
		}
		c := testViews(true)
		if err := gob.NewDecoder(&buf).Decode(c.Persisted()[0]); err == nil {
			t.Errorf("%s: a save beginning %v is taken as the stick's", name, first)
		}
	}
}

func TestViewCamera_SetViewportAndZoomLimitsReachEveryView(t *testing.T) {
	c := testViews(true)
	c.SetViewport(500, 400)
	c.SetMaxZoom(6)
	c.next()
	if w, h := c.Viewport(); w != 500 || h != 400 {
		t.Errorf("in perspective the viewport is %v x %v, want 500 x 400", w, h)
	}
	c.ZoomIn(100, 320, 320)
	if z := c.Zoom(); z > 6+1e-3 {
		t.Errorf("in perspective zoomed far in the zoom is %v, want capped at 6", z)
	}
}

func TestViewCamera_InsideAUnitTheScreenIsAsWideAsItsEyeSees(t *testing.T) {
	c := testViews(true)
	own := c.persp.focal()
	c.enterInside([3]float32{320, 320, 7}, 0, math.Pi/2)
	if f := c.persp.focal(); !near(f, 200) {
		t.Errorf("seeing 90° across a 400-wide screen the focal length is %v, want 200", f)
	}
	if x, _ := c.Unproject(0, 150, 7); !near(x, 320-200) && !near(x, 320+200) && x > 120 && x < 520 {
		t.Errorf("the screen's edge looks at x %v, want 200 off along the ground from the eye at 320", x)
	}
	c.leaveInside()
	if f := c.persp.focal(); !near(f, own) {
		t.Errorf("out of the unit the focal length is %v, want the camera's own %v", f, own)
	}
	c.enterInside([3]float32{320, 320, 7}, 0, 0)
	if f := c.persp.focal(); !near(f, own) {
		t.Errorf("inside a unit whose eye says nothing the focal length is %v, want the camera's own %v", f, own)
	}
}

func TestViewCamera_FocusIsTheFootOfTheStickOrTheEyeInAUnit(t *testing.T) {
	c := testViews(true)
	c.CenterOn(330, 310, 0)
	for _, persp := range []bool{false, true} {
		if x, y, _ := c.Focus(); !near(x, 330) || !near(y, 310) {
			t.Errorf("perspective %v: Focus() at (%v, %v), want the foot (330, 310)", persp, x, y)
		}
		c.next()
	}
	c.next()
	c.enterInside([3]float32{100, 200, 7}, 0, 0)
	if x, y, z := c.Focus(); x != 100 || y != 200 || z != 7 {
		t.Errorf("inside a unit Focus() = (%v, %v, %v), want its eye (100, 200, 7)", x, y, z)
	}
}

func TestViewCamera_AheadRunsUpTheScreenInEitherViewAndAfterATurn(t *testing.T) {
	c := testViews(true)
	c.CenterOn(330, 310, 0)
	for _, turn := range []float32{0, 0.7} {
		c.Turn(turn)
		for _, persp := range []bool{false, true} {
			x, y, _ := c.Focus()
			ax, ay := c.Ahead()
			sx0, sy0 := c.Project(x, y, 0)
			sx1, sy1 := c.Project(x+20*ax, y+20*ay, 0)
			if !near(sx0, sx1) || sy1 >= sy0 {
				t.Errorf("turned %v, perspective %v: ahead (%v, %v) of the focus lies at (%v, %v) "+
					"on the screen from (%v, %v), want straight up", turn, persp, ax, ay, sx1, sy1,
					sx0, sy0)
			}
			c.next()
		}
	}
}

func TestViewCamera_RaysRunThroughWhatTheScreenPointsSee(t *testing.T) {
	c := testViews(true)
	for _, view := range []string{"isometric", "perspective", "from above"} {
		switch view {
		case "perspective":
			c.next()
		case "from above":
			c.next()
			c.Tilt(math.Pi)
		}
		field, ok := c.Rays()
		if !ok {
			t.Fatalf("%s: no rays", view)
		}
		for _, s := range [][2]float32{{0, 0}, {200, 150}, {399, 20}, {30, 299}} {
			o, d := field.At(s[0], s[1])
			if c.inPersp {
				dx, dy, dz, _ := c.Ray(s[0], s[1])
				n := vec.Norm(d)
				ex, ey, ez, _ := c.Eye()
				if !near(d[0]/n, dx) || !near(d[1]/n, dy) || !near(d[2]/n, dz) ||
					o != [3]float32{ex, ey, ez} {
					t.Errorf("%s: the ray through %v is %v from %v, want %v from the eye %v",
						view, s, d, o, [3]float32{dx, dy, dz}, [3]float32{ex, ey, ez})
				}
				continue
			}
			for _, z := range []float32{0, 40} {
				k := (z - o[2]) / d[2]
				x, y := c.Unproject(s[0], s[1], z)
				px, py := o[0]+k*d[0], o[1]+k*d[1]
				if f32.Abs(float32(px-x)) > 0.05 || f32.Abs(float32(py-y)) > 0.05 {
					t.Errorf("%s: the ray through %v meets height %v at (%v, %v), the screen "+
						"point sees (%v, %v)", view, s, z, px, py, x, y)
				}
			}
		}
	}
}
