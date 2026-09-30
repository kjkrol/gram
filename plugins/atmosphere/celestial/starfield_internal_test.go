package celestial

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/camera"
)

// pinhole looks along its unit way ahead, right and up square to it, focal pixels to the screen's
// middle at (50, 50): where a way vanishes on the screen.
type pinhole struct {
	camera.Camera
	ahead, right, up [3]float32
}

func (p pinhole) Vanish(dx, dy, dz float32) (float32, float32, bool) {
	d := [3]float32{dx, dy, dz}
	a := dot(d, p.ahead)
	if a <= 1e-6 {
		return 0, 0, false
	}
	return 50 + 100*dot(d, p.right)/a, 50 - 100*dot(d, p.up)/a, true
}

// looking is a pinhole looking along way, level to the right.
func looking(way [3]float32) pinhole {
	right := unit([3]float32{way[1], -way[0], 0})
	up := [3]float32{right[1]*way[2] - right[2]*way[1], right[2]*way[0] - right[0]*way[2], right[0]*way[1] - right[1]*way[0]}
	return pinhole{ahead: way, right: right, up: up}
}

func unit(a [3]float32) [3]float32 {
	n := float32(math.Sqrt(float64(dot(a, a))))
	return [3]float32{a[0] / n, a[1] / n, a[2] / n}
}

// The star field lays out only the stars over the horizon and ahead of the eye, on the screen:
// looking at the pole, Polaris stands where the pole vanishes; looking down into the ground, none;
// dim, fewer.
func TestStarField_LaysOutTheStarsInSight(t *testing.T) {
	h := Place{NoonWay: South, Latitude: 45}.HeavensAt(0.3, 0.95, 0.4, RealStars)
	var f StarField
	n := f.Place(looking(h.Pole), 100, 100, h, 1)
	if n == 0 {
		t.Fatal("looking at the pole at night, no stars")
	}
	px, py, _ := looking(h.Pole).Vanish(h.Pole[0], h.Pole[1], h.Pole[2])
	polaris := false
	for i := 0; i < len(f.inst); i += 8 {
		x, y := f.inst[i], f.inst[i+1]
		if x < -4 || y < -4 || x > 104 || y > 104 {
			t.Fatalf("a star laid out at (%v, %v), off the screen", x, y)
		}
		if math.Hypot(float64(x-px), float64(y-py)) < 2 && f.inst[i+3] > 0.5 {
			polaris = true
		}
	}
	if !polaris {
		t.Errorf("no bright star within 2 pixels of the pole at (%v, %v)", px, py)
	}
	if n := f.Place(looking(unit([3]float32{0, -1, -1})), 100, 100, h, 1); n != 0 {
		t.Errorf("looking down into the ground, %d stars laid out", n)
	}
	if dim := f.Place(looking(h.Pole), 100, 100, h, 0.02); dim >= n || dim == 0 {
		t.Errorf("a dim sky shows %d stars, a dark one %d; want fewer dim", dim, n)
	}
}

// A star's colour goes from blue-white to orange by its colour index, the brighter showing more.
func TestStarField_ColoursAndBrightness(t *testing.T) {
	blue, red := starColour(-0.3), starColour(1.8)
	if blue[2] <= blue[0] || red[0] <= red[2] {
		t.Errorf("a blue star is %v, a red one %v", blue, red)
	}
	if starLight(-1.46) <= starLight(2) || starLight(6) >= starLight(2) || starLight(2) != 1 {
		t.Errorf("Sirius shows %v, magnitude 2 %v, magnitude 6 %v", starLight(-1.46), starLight(2), starLight(6))
	}
}
