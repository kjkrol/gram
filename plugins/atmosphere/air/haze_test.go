package air_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
)

// a camera with an eye or looking from above, all its lines of sight one way, down and to the
// east
type seer struct {
	camera.Camera
	eye bool
}

func (s seer) Eye() (x, y, z float32, ok bool) { return 100, 100, 500, s.eye }
func (seer) Viewport() (w, h float32)          { return 640, 480 }
func (seer) Unproject(x, y, z float32) (float32, float32) {
	return x, y
}
func (s seer) Rays() (camera.RayField, bool) {
	f := camera.RayField{Origin: [3]float32{0, 0, 9000}, Dir: [3]float32{3, 0, -4},
		DX: [3]float32{1, 0, 0}, DY: [3]float32{0, 1, 0}}
	return f, true
}

func TestWeather_SeenHazesFromAboveOnlyWhenAsked(t *testing.T) {
	w := air.Weather{Clouds: 0.5, Sight: 4000, Bases: [2]float32{1500, 6000}}
	for _, c := range []struct {
		name      string
		cam       seer
		fromAbove bool
		hazed, up bool
	}{
		{"an eye", seer{eye: true}, false, true, false},
		{"an eye, hazed from above", seer{eye: true}, true, true, false},
		{"from above", seer{}, false, false, false},
		{"from above, hazed from above", seer{}, true, true, true},
	} {
		seeing, up, way := w.Seen(c.cam, c.fromAbove)
		if (seeing > 0) != c.hazed || (up > 0) != c.up {
			t.Errorf("%s: one sees %v far, the air %v up; want hazed %v, through the air over the "+
				"ground %v", c.name, seeing, up, c.hazed, c.up)
		}
		if c.up && (up != w.Base(air.High) || way != [3]float32{0.6, 0, -0.8}) {
			t.Errorf("%s: the air %v up along %v; want the high clouds' base %v along the lines "+
				"of sight", c.name, up, way, w.Base(air.High))
		}
	}
}
