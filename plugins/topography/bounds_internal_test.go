package topography

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
)

// inBounds reports whether the world point (x, y) lies in b, give or take a hundredth.
func inBounds(b contract.AABB, x, y float32) bool {
	return float64(x) >= b.TopLeft.X-1e-2 && float64(x) <= b.BottomRight.X+1e-2 && float64(y) >= b.TopLeft.Y-1e-2 && float64(y) <= b.BottomRight.Y+1e-2
}

// onWorld reports whether (x, y) lies on the 640 x 640 world of the tests.
func onWorld(x, y float32) bool { return x >= 0 && x <= 640 && y >= 0 && y <= 640 }

// The isometric camera's Bounds hold every ground point the screen shows at any height of the
// relief: high ground from beyond the screen's lower edge at sea level included.
func TestIsoCamera_BoundsHoldTheGroundTheScreenShowsAtAnyHeight(t *testing.T) {
	c := testCamera(t, 0).(*viewCamera).iso
	c.ZoomIn(3, 160, 160)
	c.CenterOn(160, 160, 0) // room on the world below the screen
	c.extent = func() (float32, float32) { return 0, 150 }
	b := c.Bounds()
	low, high := c.layer()
	for sy := float32(0); sy <= 300; sy += 30 {
		for sx := float32(0); sx <= 400; sx += 40 {
			for _, z := range []float32{low, 100, high} {
				if x, y := c.Unproject(sx, sy, z); onWorld(x, y) && !inBounds(b, x, y) {
					t.Errorf("the ground %v up drawn at (%v, %v) lies at (%v, %v), outside Bounds %v", z, sx, sy, x, y, b)
				}
			}
		}
	}
	x, y := c.Unproject(200, 290, 150) // a hilltop drawn at the bottom of the screen
	if sx, sy := c.Project(x, y, 0); sy <= 300 || !onWorld(x, y) {
		t.Fatalf("the hill at (%v, %v) has its foot drawn at (%v, %v), want on the world and below the screen for the case to hold", x, y, sx, sy)
	}
	box := geom.NewAABBAt(geom.NewVec(float64(x)-2, float64(y)-2), 4, 4)
	if !c.Visible(box) || !inBounds(b, x, y) {
		t.Error("a unit on a hill 150 up drawn at the bottom of the screen is not Visible, or its ground not in Bounds")
	}
	c.extent = nil
	if inBounds(c.Bounds(), x, y) || c.Visible(box) {
		t.Error("over level ground the hill's point counts as shown: the case shows nothing")
	}
}

// The perspective camera's Bounds hold every ground point the screen shows, from above and from
// inside a unit, the head raised or bowed; looking over everything they stay a rectangle at the eye.
func TestPerspCamera_BoundsHoldTheGroundTheScreenShowsAtAnyHeight(t *testing.T) {
	check := func(name string, c *perspCamera) {
		t.Helper()
		b := c.Bounds()
		if b.TopLeft.X > b.BottomRight.X || b.TopLeft.Y > b.BottomRight.Y {
			t.Errorf("%s: Bounds %v turned inside out", name, b)
		}
		low, high := c.layer()
		e := c.proj.eye
		for sy := float32(0); sy <= 300; sy += 20 {
			for sx := float32(0); sx <= 400; sx += 40 {
				d := c.proj.ray(sx-200, sy-150)
				for _, z := range []float32{low, (low + high) / 2, high} {
					if d[2] == 0 {
						continue
					}
					tt := (z - e[2]) / d[2]
					if tt <= 0 || tt > c.proj.far {
						continue // not ahead of the eye, or past far
					}
					x, y := e[0]+d[0]*tt, e[1]+d[1]*tt
					if onWorld(x, y) && !inBounds(b, x, y) {
						t.Errorf("%s: the ground %v up drawn at (%v, %v) lies at (%v, %v), outside Bounds %v", name, z, sx, sy, x, y, b)
						return
					}
				}
			}
		}
	}
	hills := func(x, y float32) float32 { return 100 }
	free := testPersp(hills, func() float32 { return 250 })
	check("free, at the ceiling", free)
	free.Tilt(1)
	check("free, bowed", free)
	for _, pitch := range []float32{0, -0.2, -0.45, 0.3, -1.4} {
		c := testPersp(hills, func() float32 { return 250 })
		c.enterInside([3]float32{320, 320, 154}, 0, 0)
		c.Tilt(pitch)
		check("inside a unit", c)
		if b := c.Bounds(); !inBounds(b, 320, 320) {
			t.Errorf("inside a unit at pitch %v the eye's own ground is outside Bounds %v", pitch, b)
		}
	}
	level := testPersp(nil, nil) // low over level ground, the screen's sides crossing the horizon
	level.enterInside([3]float32{620, 620, 40}, 0, 0)
	level.Tilt(0.25)
	check("inside a unit, looking far over level ground", level)
	c := testPersp(hills, func() float32 { return 250 })
	c.enterInside([3]float32{320, 320, 154}, 0, 0)
	ahead := facing(0)
	if x, y := 320+float32(ahead.X)*150, 320+float32(ahead.Y)*150; !inBounds(c.Bounds(), x, y) {
		t.Errorf("inside a unit looking level, the ground 150 ahead (%v, %v) is outside Bounds %v", x, y, c.Bounds())
	}
	x, y, _ := c.cast(200, 280, 100) // the hillside near the bottom of the screen
	if !c.Visible(geom.NewAABBAt(geom.NewVec(float64(x)-2, float64(y)-2), 4, 4)) {
		t.Error("inside a unit, a walker on the hill 100 up near the bottom of the screen is not Visible")
	}
	up := testPersp(nil, nil)
	up.enterInside([3]float32{320, 320, 500}, 0, 0) // over everything, looking up
	up.Tilt(-1.4)
	if b := up.Bounds(); !inBounds(b, 320, 320) || b.BottomRight.X-b.TopLeft.X > 1e-3 || b.BottomRight.Y-b.TopLeft.Y > 1e-3 {
		t.Errorf("looking up from over everything, Bounds %v, want the eye's point alone", b)
	}
}
