package topography

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
)

// ramp is ground rising 3 a unit away from the eye — up to 300 where x + y is 560, level at 0
// from 660 on — every point of it facing the eye, which looks from where x and y grow.
func ramp(x, y float32) float32 { return min(max(3*(660-(x+y)), 0), 300) }

// rampPoints lie on the slope, where the ground rises 3 a unit.
var rampPoints = [][2]float32{{300, 320}, {310, 300}, {295, 305}, {320, 290}, {305, 330}}

// settle is what the click once did: four rounds of unprojecting at the height of the last answer,
// from sea level.
func settle(unproject func(sx, sy, z float32) (float32, float32), ground func(x, y float32) float32, sx, sy float32) (float32, float32) {
	x, y := unproject(sx, sy, 0)
	for range 4 {
		x, y = unproject(sx, sy, ground(x, y))
	}
	return x, y
}

func testIso(ground func(x, y float32) float32, high float32) *isoCamera {
	c := newIsoCamera(testProjection.viewed(false), geom.NewVec(640, 640), geom.NewAABBAt(geom.NewVec(0, 0), 400, 300), 0)
	c.ground, c.extent = ground, func() (float32, float32) { return 0, high }
	return c
}

func TestIsoCamera_PickFindsTheGroundOnASlopeTooSteepToSettle(t *testing.T) {
	c := testIso(ramp, 300)
	missed := false
	for _, q := range rampPoints {
		z := ramp(q[0], q[1])
		c.CenterOn(float64(q[0]), float64(q[1]), float64(z))
		sx, sy := c.Project(q[0], q[1], z)
		x, y, ok := c.Pick(sx, sy)
		if !ok || math.Abs(float64(x-q[0])) > 1e-2 || math.Abs(float64(y-q[1])) > 1e-2 {
			t.Errorf("point %v at height %v is drawn at (%v, %v) and picked back as (%v, %v) %v", q, z, sx, sy, x, y, ok)
		}
		if ox, oy := settle(c.Unproject, ramp, sx, sy); math.Hypot(float64(ox-q[0]), float64(oy-q[1])) > 1 {
			missed = true
		}
	}
	if !missed {
		t.Error("the old rounds of unprojecting found every point: the slope is not steep enough to test the pick")
	}
}

func TestIsoCamera_PickStopsAtTheRidgeInFront(t *testing.T) {
	ridge := func(x, y float32) float32 {
		if s := x + y; s >= 600 && s <= 640 {
			return 300
		}
		return 0
	}
	c := testIso(ridge, 300)
	c.CenterOn(280, 280, 0)
	sx, sy := c.Project(280, 280, 0) // the valley behind the ridge, as seen from where x and y grow
	x, y, ok := c.Pick(sx, sy)
	if !ok || ridge(x, y) != 300 {
		t.Errorf("the screen point of the hidden valley picked (%v, %v) %v at height %v, want the ridge in front", x, y, ok, ridge(x, y))
	}
}

func TestPerspCamera_PickFindsTheGroundOnASteepSlopeOverTheCurve(t *testing.T) {
	c := testPersp(ramp, func() float32 { return 300 })
	c.bend = 5e-5
	c.look()
	for _, q := range rampPoints {
		z := ramp(q[0], q[1])
		sx, sy := c.Project(q[0], q[1], z)
		x, y, ok := c.Pick(sx, sy)
		if !ok || math.Abs(float64(x-q[0])) > 1e-2 || math.Abs(float64(y-q[1])) > 1e-2 {
			t.Errorf("point %v at height %v is drawn at (%v, %v) and picked back as (%v, %v) %v", q, z, sx, sy, x, y, ok)
		}
	}
	at := c.target()
	if !near(at[2], ramp(at[0], at[1])) {
		t.Errorf("the middle of the screen looks at %v, off the ground %v there", at, ramp(at[0], at[1]))
	}
	if sx, sy := c.Project(at[0], at[1], at[2]); math.Abs(float64(sx-200)) > 0.05 || math.Abs(float64(sy-150)) > 0.05 {
		t.Errorf("the middle's ground point is drawn at (%v, %v), want (200, 150)", sx, sy)
	}
}

func TestPerspCamera_InsideAUnitPickReachesUpASlopeAndMissesTheSky(t *testing.T) {
	slope := func(x, y float32) float32 { return min(max(2*(600-(x+y)), 0), 200) }
	c := testPersp(slope, func() float32 { return 200 })
	c.enterInside([3]float32{330, 330, 5}, 0, 0) // looking along the ground towards the slope ahead
	c.Tilt(-0.3)
	x, y, ok := c.Pick(200, 150)
	if !ok || slope(x, y) <= 5 {
		t.Fatalf("looking up the slope the middle picked (%v, %v) %v at height %v, want the slope above the eye", x, y, ok, slope(x, y))
	}
	if sx, sy := c.Project(x, y, slope(x, y)); math.Abs(float64(sx-200)) > 0.05 || math.Abs(float64(sy-150)) > 0.05 {
		t.Errorf("the picked point is drawn at (%v, %v), want the middle", sx, sy)
	}
	c.Turn(math.Pi) // the slope behind: level ground ahead, and the eye looking over it at the sky
	if x, y, ok := c.Pick(200, 20); ok {
		t.Errorf("a screen point in the sky picked (%v, %v)", x, y)
	}
}
