package topography

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
)

// testViews is the topography's camera over a 640 x 640 world drawn to 400 x 300, isometric to
// begin with, reaching the perspective view when reaches.
func testViews(reaches bool) *viewCamera {
	return newCamera(testProjection, 640, 640, 0, contract.Config{ViewportWidth: 400, ViewportHeight: 300}, 0, reaches, nil, nil, 0)
}

// middle is the ground point under the middle of the screen.
func middle(c *viewCamera) (x, y float32) { return c.Unproject(200, 150, 0) }

func TestViewCamera_NextGoesRoundTheViewsKeepingTheMiddleAndTheScale(t *testing.T) {
	c := testViews(true)
	c.CenterOn(330, 310, 0)
	c.Turn(0.3)
	x0, y0 := middle(c)
	for _, step := range []struct {
		name          string
		persp, relief bool
	}{{"perspective", true, true}, {"from above", false, false}, {"isometric", false, true}, {"perspective", true, true}} {
		before := c.iso.scale()
		c.next()
		if c.inPersp != step.persp || c.relief() != step.relief || (step.persp && c.cur != contract.Camera(c.persp)) || (!step.persp && c.cur != contract.Camera(c.iso)) {
			t.Fatalf("%s: in perspective %v, in relief %v, current %T", step.name, c.inPersp, c.relief(), c.cur)
		}
		if x, y := middle(c); !near(x, x0) || !near(y, y0) {
			t.Errorf("%s: the middle of the screen shows (%v, %v), want (%v, %v) still", step.name, x, y, x0, y0)
		}
		if step.persp {
			if _, ok := c.Projection().(perspective); !ok {
				t.Errorf("%s: Projection is %T", step.name, c.Projection())
			}
			if s := c.persp.Zoom(); !near(s/before, 1) {
				t.Errorf("%s: a world unit spans %v screen units in the middle, want the isometric view's %v", step.name, s, before)
			}
			if !near(c.Heading(), c.iso.heading) || !near(c.Pitch(), c.iso.pitch) {
				t.Errorf("%s: the heading is %v and the pitch %v, want the isometric view's %v and %v", step.name, c.Heading(), c.Pitch(), c.iso.heading, c.iso.pitch)
			}
		}
		if step.name == "isometric" && !near(c.iso.Zoom(), 1) {
			t.Errorf("round the views and back the isometric zoom is %v, want 1 as it began", c.iso.Zoom())
		}
	}
	unreached := testViews(false)
	unreached.next()
	unreached.next()
	if unreached.inPersp || !unreached.relief() {
		t.Errorf("a camera not reaching the perspective went from above to: perspective %v, relief %v; want isometric", unreached.inPersp, unreached.relief())
	}
}

func TestViewCamera_LookFromGoesIntoPerspectiveWhereReached(t *testing.T) {
	c := testViews(true)
	if !c.LookFrom(100, 100, 50) || !c.inPersp {
		t.Fatal("LookFrom on the isometric view did not go into the perspective one")
	}
	if e := c.persp.proj.eye; !near(e[0], 100) || !near(e[1], 100) || !near(e[2], c.persp.ceiling()) {
		t.Errorf("the eye is at %v, want (100, 100) at the ceiling %v: no lower than it flies", e, c.persp.ceiling())
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
	bx, by := c.Project(300, 280, 4)
	other := testViews(true)
	saved, restored := c.Persisted(), other.Persisted()
	if len(saved) != 11 || len(restored) != 11 {
		t.Fatalf("Persisted has %d and %d values, want 11: every view's and which is in", len(saved), len(restored))
	}
	for i := range saved {
		switch v := saved[i].(type) {
		case *geom.Vec:
			*restored[i].(*geom.Vec) = *v
		case *float32:
			*restored[i].(*float32) = *v
		case *bool:
			*restored[i].(*bool) = *v
		default:
			t.Fatalf("Persisted()[%d] is %T", i, v)
		}
	}
	other.Restore()
	if !other.inPersp || other.cur != contract.Camera(other.persp) {
		t.Fatal("restored camera is not in the perspective view")
	}
	if ax, ay := other.Project(300, 280, 4); !near(ax, bx) || !near(ay, by) {
		t.Errorf("restored camera draws the point at (%v, %v), want (%v, %v)", ax, ay, bx, by)
	}
	unreached := testViews(false)
	for i, p := range unreached.Persisted() {
		if b, ok := p.(*bool); ok && i == 10 {
			*b = true
		}
	}
	unreached.Restore()
	if unreached.inPersp {
		t.Error("a save in perspective loaded by a game not reaching it stayed in perspective")
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

// Inside a unit whose eye says how wide it sees, the screen's width fills that field and the
// height follows the screen's shape; out again, the camera's own field of view is back.
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
