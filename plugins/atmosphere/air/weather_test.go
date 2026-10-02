package air_test

import (
	"math"
	"os"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// the white texel of these tests' sheet
type sheet struct{}

func (sheet) Atlas() *render.Image                        { return nil }
func (sheet) UV(render.SpriteID) (x0, y0, x1, y1 float32) { return 0, 0, 8, 8 }
func (sheet) White() (u, v float32)                       { return 40, 40 }

// source is a render.Source composing whatever its function says.
type source func(f *render.Frame)

func (source) Init(*goke.SysInit)                         {}
func (s source) Compose(f *render.Frame, _ camera.Camera) { s(f) }

// The clouds' noise is smooth, 0 to 1, and drifts with the wind: a point under a cloud is under the
// same cloud once both have moved on together.
func TestCloud_IsSmoothNoiseCarriedByTheDrift(t *testing.T) {
	var lo, hi float32 = 1, 0
	still, carried := air.Weather{}, air.Weather{Drift: [2]float32{300, -120}}
	for i := range 64 {
		for j := range 64 {
			x, y := float32(i)*50, float32(j)*50
			n := still.Cloud(x, y)
			lo, hi = min(lo, n), max(hi, n)
			if d := n - still.Cloud(x+1, y+1); d > 0.02 || d < -0.02 {
				t.Errorf("the clouds' noise jumps by %v over a unit at (%v, %v)", d, x, y)
			}
			if d := n - carried.Cloud(x+300, y-120); d > 1e-4 || d < -1e-4 {
				t.Errorf("carried 300, -120 the clouds at (%v, %v) changed by %v", x, y, d)
			}
		}
	}
	if lo < 0 || hi > 1 || hi-lo < 0.3 {
		t.Errorf("the clouds' noise runs %v to %v, want within 0 to 1 and varied", lo, hi)
	}
	half := air.Weather{Clouds: 0.5}
	if half.Shade(0) != 0 || half.Shade(1) != 1 || (air.Weather{}).Shade(1) != 0 {
		t.Error("Shade: no cloud where the noise is low, all of it where high, none under a clear sky")
	}
}

func TestOvercast_GreysTheSkyTheMoreItIsCovered(t *testing.T) {
	blue := render.Light{0.5, 0.72, 0.98}
	if air.Overcast(blue, 0) != blue {
		t.Errorf("a clear sky is %v, want it as it is", air.Overcast(blue, 0))
	}
	half, full := air.Overcast(blue, 0.5), air.Overcast(blue, 1)
	if !(full[2]-full[0] < half[2]-half[0] && half[2]-half[0] < blue[2]-blue[0]) {
		t.Errorf("the sky goes %v, %v, %v as clouds cover it; want it greyer each time", blue, half, full)
	}
}

func TestSway_LeansWithTheWindTheHarderTheFurtherAndNotAtAllInTheCalm(t *testing.T) {
	if x, y := (air.Weather{}).Sway(1, 5, 5, 1); x != 0 || y != 0 {
		t.Errorf("in the calm a tree leans %v, %v, want not at all", x, y)
	}
	gale := air.Weather{Wind: [2]float32{50, 0}}
	if x, y := gale.Sway(1, 5, 5, 0); x != 0 || y != 0 {
		t.Errorf("what does not sway leans %v, %v", x, y)
	}
	breeze, _ := (air.Weather{Wind: [2]float32{10, 0}}).Sway(1, 5, 5, 1)
	blown, across := gale.Sway(1, 5, 5, 1)
	if breeze <= 0 || blown <= breeze || across != 0 {
		t.Errorf("an east wind leans a tree %v in a breeze, %v across and %v in a gale; want east, further in the gale", breeze, across, blown)
	}
	wind := air.Weather{Wind: [2]float32{30, 0}}
	a, _ := wind.Sway(0, 0, 0, 1)
	b, _ := wind.Sway(0.7, 0, 0, 1)
	if a == b {
		t.Error("a tree in the wind stands still: want it rocking")
	}
}

// eyed is a camera with an eye at the origin, 10 up.
type eyed struct{ camera.Camera }

func (eyed) Eye() (float32, float32, float32, bool) { return 0, 0, 10, true }

// The air hides what lies far off as it thickens with distance, through a camera with an eye in
// air that does not go on without end.
func TestHaze_GrowsWithDistance(t *testing.T) {
	clear := air.Weather{}
	if h := clear.Haze(eyed{}, 100, 0, 10); h != 0 {
		t.Fatalf("in air without end the haze is %v, want 0", h)
	}
	w := air.Weather{Visibility: 100}
	near, far := w.Haze(eyed{}, 10, 0, 10), w.Haze(eyed{}, 300, 0, 10)
	if near <= 0 || near >= far || far >= 1 || math.Abs(float64(w.Haze(eyed{}, 100, 0, 10))-(1-math.Exp(-1))) > 1e-6 {
		t.Errorf("the haze 10, 100 and 300 off is %v, %v and %v; want growing, 1 − 1/e at the visibility", near, w.Haze(eyed{}, 100, 0, 10), far)
	}
	if h := w.Haze(icamera.NewFromSpace(64, 64, 0), 1000, 0, 0); h != 0 {
		t.Errorf("through a camera without an eye the haze is %v, want 0", h)
	}
}

// A kilometre a cell of 32 world units: the clear air lets one see 40 km, a downpour far less;
// without a scale the air is clear without end.
func TestVisibility_SeesAsFarAsTheScaleAndTheWeatherSay(t *testing.T) {
	s := world.Scale{Metres: 1000.0 / 32}
	clear := air.Visibility(s, air.Weather{})
	if math.Abs(clear*s.Metres-air.ClearAir) > 1e-6 {
		t.Errorf("clear air lets one see %v m, want %v", clear*s.Metres, air.ClearAir)
	}
	if rain := air.Visibility(s, air.Weather{Rain: 1, Clouds: 1}); rain >= clear/5 {
		t.Errorf("in a downpour one sees %v world units, want well under a fifth of %v", rain, clear)
	}
	if air.Visibility(world.Scale{}, air.Weather{}) != 0 {
		t.Error("without a scale the air is not clear without end")
	}
}

// The frame is handed the wind, the drift and the cover, and the fog is the sky under the clouds.
func TestWeather_HandsTheFrameTheAirAndTheFog(t *testing.T) {
	sun := sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.7, Sky: render.Light{0.5, 0.7, 1}}
	w := air.Weather{Wind: [2]float32{3, 4}, Drift: [2]float32{10, 20}, Clouds: 0.6}
	c := render.NewComposer(source(func(f *render.Frame) { w.Frame(f, sun) }))
	c.DrawWorld(nil, icamera.NewFromSpace(64, 64, 0))
	got := c.Uniforms()
	if got["Wind"][1] != 4 || got["Drift"][0] != 10 || got["Cover"][0] != 0.6 {
		t.Errorf("the shader is handed wind %v, drift %v, cover %v; want the weather's", got["Wind"], got["Drift"], got["Cover"])
	}
	if fog, want := got["Fog"], air.Overcast(sun.Sky, 0.6); fog[0] != want[0] || fog[2] != want[2] {
		t.Errorf("the fog is %v, want the sky under the clouds %v", fog, want)
	}
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
	if err := render.Compile(); err != nil {
		t.Fatal(err)
	}
}
