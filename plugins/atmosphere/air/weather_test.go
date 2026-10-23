package air_test

import (
	"math"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type sheet struct{}

func (sheet) Atlas() *render.Image                        { return nil }
func (sheet) UV(render.SpriteID) (x0, y0, x1, y1 float32) { return 0, 0, 8, 8 }
func (sheet) White() (u, v float32)                       { return 40, 40 }

type source func(f *render.Frame)

func (source) Init(*goke.SysInit)                         {}
func (s source) Compose(f *render.Frame, _ camera.Camera) { s(f) }

func TestCloud_IsSmoothNoiseCarriedByTheDrift(t *testing.T) {
	for _, layer := range []air.Layer{air.Low, air.High} {
		var lo, hi float32 = 1, 0
		var low, high [2]float32
		still, carried := air.Weather{}, air.Weather{Drift: [2]float32{300, -120}}
		for i := range 64 {
			for j := range 64 {
				x, y := float32(i)*50, float32(j)*50
				n := still.Cloud(layer, x, y)
				if n < lo {
					lo, low = n, [2]float32{x, y}
				}
				if n > hi {
					hi, high = n, [2]float32{x, y}
				}
				// the heaps' cores rise steeply at their edges, a heap a few hundred units wide
				smooth := float32(0.02)
				if layer == air.Low {
					smooth = 0.08
				}
				if d := n - still.Cloud(layer, x+1, y+1); d > smooth || d < -smooth {
					t.Errorf("layer %d: the noise jumps by %v over a unit at (%v, %v)", layer, d,
						x, y)
				}
				if d := n - carried.Cloud(layer, x+300, y-120); d > 1e-4 || d < -1e-4 {
					t.Errorf("layer %d carried 300, -120: the clouds at (%v, %v) changed by %v",
						layer, x, y, d)
				}
			}
		}
		if lo < 0 || hi > 1 || hi-lo < 0.3 {
			t.Errorf("layer %d: the noise runs %v to %v, want within 0 to 1 and varied", layer, lo,
				hi)
		}
		if layer == air.High {
			continue
		}
		// the low clouds alone shading, half the sky's cover theirs, the sun straight up
		half := air.Weather{Clouds: 0.5, Billow: 1, Bases: [2]float32{10, 20}}
		up := [3]float32{0, 0, 1}
		if half.Shade(low[0], low[1], 0, up) != 0 || half.Shade(high[0], high[1], 0, up) != 1 ||
			(air.Weather{Bases: half.Bases}).Shade(high[0], high[1], 0, up) != 0 {
			t.Error("Shade leaves no cloud where the heaps' noise is low, all of it where high, " +
				"none under a clear sky")
		}
		if half.Shade(high[0], high[1], 15, up) != 0 {
			t.Error("ground over the low clouds lies in their shadow")
		}
	}
}

func TestMass_LayerHeapsTheLowCloudsAsTheSkyFillsAndBringsTheHighSoon(t *testing.T) {
	if low, high := (air.Mass{}).Layer(air.Low), (air.Mass{}).Layer(air.High); low.Cover != 0 ||
		high.Cover != 0 {
		t.Errorf("a clear sky holds %v low and %v high, want none", low.Cover, high.Cover)
	}
	fair, cloudy := air.Mass{Cover: 0.15, Billow: 0.9}, air.Mass{Cover: 0.6, Billow: 0.9}
	if f, c := fair.Layer(air.Low).Cover, cloudy.Layer(air.Low).Cover; f >= c || c < 0.4 {
		t.Errorf("heaps cover %v of a fair sky and %v of a cloudy one, want many more as it "+
			"fills", f, c)
	}
	if high := fair.Layer(air.High).Cover; high < 0.15 {
		t.Errorf("shreds cover %v of a fair sky, want them there with a few clouds already", high)
	}
	torn := air.Mass{Cover: 0.6}
	if h, c := torn.Layer(air.High).Cover, cloudy.Layer(air.High).Cover; h <= c {
		t.Errorf("shreds cover %v of a torn sky, %v of a heaped one, want fewer the more heaped",
			h, c)
	}
	if low, high := cloudy.Layer(air.Low), cloudy.Layer(air.High); low.Billow != 1 ||
		high.Billow != 0 {
		t.Errorf("the low layer heaped %v, the high %v; want heaps and shreds", low.Billow,
			high.Billow)
	}
}

func TestWeather_HighCloudsComeInPatchesEachSheetAtItsHeightIntoTheLowOnes(t *testing.T) {
	w := air.Weather{Bases: [2]float32{1500, 6000}}
	m := air.Mass{Cover: 0.6}.Layer(air.High)
	floor, top := float32(math.Inf(1)), float32(math.Inf(-1))
	var none, whole int
	for i := range 80 {
		for j := range 80 {
			x, y := float32(i)*163, float32(j)*157
			h := w.HighHeight(x, y)
			floor, top = min(floor, h), max(top, h)
			switch c := w.HighAt(m, x, y).Cover; {
			case c == 0:
				none++
			case c == m.Cover:
				whole++
			}
		}
	}
	// the heaps' lumps rise as high as 0.72 clouds' widths, 302 world units, over their base
	if floor < 1500 || top > 6000 || floor > 1500+302 || top-floor < 2000 {
		t.Errorf("the high clouds hang %v up to %v, want through their band down into the low "+
			"ones' bodies", floor, top)
	}
	if none < 400 || whole < 400 {
		t.Errorf("of 6400 points %d without high clouds and %d with all of them, want patches",
			none, whole)
	}
}

func TestShade_FallsAlongTheWayToTheSunFromTheCloudsHeight(t *testing.T) {
	bases := [2]float32{100, 400}
	sun := [3]float32{0.6, 0, 0.8}
	w := air.Weather{Clouds: 0.6, Billow: 1, Bases: bases}
	up := [3]float32{0, 0, 1}
	// a world unit up the shadow moves 0.75 away from the sun
	across := 0.75 * bases[air.Low]
	for i := range 200 {
		x, y := float32(i)*37, float32(i%13)*41
		under := w.Shade(x, y, 0, up)
		if a := w.Shade(x-across, y, 0, sun); math.Abs(float64(a-under)) > 1e-5 {
			t.Fatalf("in the sun %v the ground %v units off (%v, %v) is shaded %v, want %v as "+
				"straight under the cloud", sun, across, x, y, a, under)
		}
		nearer := 0.75 * (bases[air.Low] - 30)
		if a := w.Shade(x-nearer, y, 30, sun); math.Abs(float64(a-under)) > 1e-5 {
			t.Fatalf("ground 30 up is shaded %v, want %v: nearer the cloud, nearer under it", a,
				under)
		}
	}
}

func TestShade_OfTheHighCloudsOnlyWithHighShade(t *testing.T) {
	up := [3]float32{0, 0, 1}
	w := air.Weather{Clouds: 0.8, Bases: [2]float32{100, 400}}
	darker := 0
	for i := range 400 {
		x, y := float32(i)*37, float32(i%11)*43
		low := w.Shade(x, y, 0, up)
		w.HighShade = true
		both := w.Shade(x, y, 0, up)
		w.HighShade = false
		if both < low {
			t.Fatalf("at (%v, %v) the high clouds' shadow lightens the ground", x, y)
		}
		if both > low {
			darker++
		}
	}
	if darker == 0 {
		t.Error("with HighShade the high clouds shade nothing")
	}
}

func TestFallsAt_RainsFromTheLowCloudsAlone(t *testing.T) {
	w := air.Weather{Rain: 1, Clouds: 0.8, Billow: 0.5, Temperature: 10, Bases: [2]float32{1, 2}}
	up := [3]float32{0, 0, 1}
	wet := 0
	for i := range 400 {
		x, y := float32(i)*41, float32(i%9)*57
		rain, _ := w.FallsAt(x, y)
		// the ground under both layers, the high ones shading nothing: the low ones' cover
		low := w.Shade(x, y, -10, up)
		if rain > 0 && low == 0 {
			t.Fatalf("at (%v, %v) %v rain falls with no low cloud over it", x, y, rain)
		}
		if low >= 0.35 && rain != 1 {
			t.Fatalf("at (%v, %v) %v of the rain falls under a low cloud %v thick, want all", x,
				y, rain, low)
		}
		if rain > 0 {
			wet++
		}
	}
	if wet == 0 {
		t.Error("no rain falls from low clouds over most of the sky")
	}
}

func TestWeather_OneLayerHoldsTheCloudsHighMixedTheirShadowsStraightUnder(t *testing.T) {
	bases := [2]float32{100, 400}
	sun := [3]float32{0.6, 0, 0.8}
	up := [3]float32{0, 0, 1}
	for _, billow := range []float32{0, 0.5, 1} {
		w := air.Weather{Clouds: 0.6, Billow: billow, Bases: bases, OneLayer: true}
		m := w.At(0, 0)
		if w.Share(m, air.Low) != m || w.Share(m, air.High) != (air.Mass{}) {
			t.Errorf("billow %v: one layer holds %+v low and %+v high, want all of %+v low",
				billow, w.Share(m, air.Low), w.Share(m, air.High), m)
		}
		if w.Base(air.Low) != bases[air.High] || w.Base(air.High) != bases[air.High] {
			t.Errorf("billow %v: one layer hangs %v and %v up, want %v", billow,
				w.Base(air.Low), w.Base(air.High), bases[air.High])
		}
		two := air.Weather{Bases: bases}
		for i := range 100 {
			x, y := float32(i)*53, float32(i%7)*61
			if a, b := w.Shade(x, y, 0, sun), w.Shade(x, y, 0, up); a != b {
				t.Fatalf("billow %v: at (%v, %v) the sun's way shades %v, straight up %v; want "+
					"the shadow straight under", billow, x, y, a, b)
			}
			n := w.Cloud(air.Low, x, y)
			torn, heap := two.Cloud(air.High, x, y), two.Cloud(air.Low, x, y)
			if (billow == 0 && n != torn) || (billow == 1 && n != heap) || n < 0 || n > 1 {
				t.Fatalf("billow %v: one layer's noise %v, the shreds' %v, the heaps' %v; want "+
					"them mixed as heaped as the mass", billow, n, torn, heap)
			}
		}
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

type eyed struct{ camera.Camera }

func (eyed) Eye() (float32, float32, float32, bool) { return 0, 0, 10, true }

func TestHaze_GrowsWithDistance(t *testing.T) {
	clear := air.Weather{}
	if h := clear.Haze(eyed{}, 100, 0, 10); h != 0 {
		t.Fatalf("in air without end the haze is %v, want 0", h)
	}
	w := air.Weather{Sight: 100}
	near, far := w.Haze(eyed{}, 10, 0, 10), w.Haze(eyed{}, 300, 0, 10)
	if near <= 0 || near >= far || far >= 1 || math.Abs(float64(w.Haze(eyed{}, 100, 0, 10))-(1-math.Exp(-1))) > 1e-6 {
		t.Errorf("the haze 10, 100 and 300 off is %v, %v and %v; want growing, 1 − 1/e at the visibility", near, w.Haze(eyed{}, 100, 0, 10), far)
	}
	if h := w.Haze(cameratest.NewFromSpace(64, 64, 0), 1000, 0, 0); h != 0 {
		t.Errorf("through a camera without an eye the haze is %v, want 0", h)
	}
}

func TestVisibility_SeesAsFarAsTheScaleAndTheWeatherSay(t *testing.T) {
	s := world.Scale{Metres: 1000.0 / 32}
	sight := air.Sight(s)
	clear := air.Weather{Sight: sight}.Visibility(0, 0)
	if math.Abs(clear*s.Metres-air.ClearAir) > 1e-6 {
		t.Errorf("clear air lets one see %v m, want %v", clear*s.Metres, air.ClearAir)
	}
	pour := air.Weather{Sight: sight, Rain: 1, Clouds: 1, Temperature: 10}
	if rain := pour.Visibility(0, 0); rain >= clear/5 {
		t.Errorf("in a downpour one sees %v world units, want well under a fifth of %v", rain, clear)
	}
	if air.Sight(world.Scale{}) != 0 || (air.Weather{}).Visibility(0, 0) != 0 {
		t.Error("without a scale the air is not clear without end")
	}
}

func TestWeather_HandsTheFrameTheAirAndTheSkysColours(t *testing.T) {
	sun := light.Distant{Dir: [3]float32{0, 0, 1}, Strength: 0.7, AmbientColor: render.Light{0.5, 0.7, 1}}
	w := air.Weather{Wind: [2]float32{3, 4}, Drift: [2]float32{10, 20}, Clouds: 0.6}
	c := render.NewComposer(source(func(f *render.Frame) { w.Frame(f, sun, 0, 0) }))
	c.DrawWorld(nil, cameratest.NewFromSpace(64, 64, 0))
	got := c.Uniforms()
	if got["Wind"][1] != 4 || got["Drift"][0] != 10 || got["Cover"][0] != 0.6 {
		t.Errorf("the shader is handed wind %v, drift %v, cover %v; want the weather's", got["Wind"], got["Drift"], got["Cover"])
	}
	if fog, want := got["Fog"], air.Overcast(sun.AmbientColor, 0.6); fog[0] != want[0] || fog[2] != want[2] {
		t.Errorf("the fog is %v, want the sky under the clouds %v", fog, want)
	}
	overhead, fog := got["SkyOverhead"], got["Fog"]
	if want := air.Overhead(sun.AmbientColor, 0.6); overhead[0] != want[0] || overhead[2] != want[2] {
		t.Errorf("the sky overhead is %v, want %v", overhead, want)
	}
	if !(overhead[0] < fog[0] && overhead[1] < fog[1]) {
		t.Errorf("overhead %v, at the horizon %v: want the sky deeper overhead", overhead, fog)
	}
	gputest.Need(t)
	if err := render.Compile(); err != nil {
		t.Fatal(err)
	}
}
