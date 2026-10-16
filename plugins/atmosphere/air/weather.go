package air

import (
	"embed"

	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// the screen pixel (x, y) looks along LookDir + LookDX·x + LookDY·y; Visibility 0 is without end
var weatherUniforms = []render.Uniform{
	{Name: "Wind", Size: 2},
	{Name: "Drift", Size: 2},
	{Name: "Cover", Size: 1},
	{Name: "Billow", Size: 1},
	{Name: "EyeAt", Size: 3},
	{Name: "LookDir", Size: 3},
	{Name: "LookDX", Size: 3},
	{Name: "LookDY", Size: 3},
	{Name: "CloudHeight", Size: 1},
	{Name: "Visibility", Size: 1},
	{Name: "SkyOverhead", Size: 3},
}

// a level of the dome as the shaders read it (Dome.frame): AirLevelNA, AirLevelNC
var levelNames = func() (n [maxLevels][2]string) {
	for k := range maxLevels {
		for i, part := range []string{"A", "C"} {
			n[k][i] = "AirLevel" + string(rune('0'+k)) + part
		}
	}
	return n
}()

var weatherMaterials = render.RegisterMaterials(render.Files(shaders, "shaders/dome.wgsl",
	"shaders/cloud_noise.wgsl", "shaders/cloud_shadow.wgsl", "shaders/clouds.wgsl"),
	levelUniforms(weatherUniforms), "CloudShadow")

func levelUniforms(u []render.Uniform) []render.Uniform {
	for _, names := range levelNames {
		for _, name := range names {
			u = append(u, render.Uniform{Name: name, Size: 4})
		}
	}
	return u
}

var cloudShadow = weatherMaterials[0]

func init() {
	render.RegisterImage("CloudTile")
	render.RegisterImage("AirMasses")
}

// CloudShadow is the material of the clouds' shadows.
func CloudShadow() render.MaterialID { return cloudShadow }

// Weather is the air over the world; the zero Weather is a calm, clear day at 0°. Wind is in
// world units a second, Clouds, Rain and Snow 0 to 1, Billow 0 torn shreds to 1 big heaps,
// Temperature degrees Celsius, Drift how far the wind has carried the clouds, Sight how far one
// sees through clear air (Sight), world units, 0 without end. Dome is the air masses the wind
// carries over the world and its sky, nil the weather alike everywhere; Dry, nothing falls.
type Weather struct {
	Wind        [2]float32
	Clouds      float32
	Billow      float32
	Rain, Snow  float32
	Temperature float32
	Drift       [2]float32
	Sight       float64
	Dome        *Dome
	Dry         bool
}

// At is the air mass over the world point (x, y).
func (w Weather) At(x, y float32) Mass {
	if w.Dome == nil {
		return Mass{Cover: w.Clouds, Billow: w.Billow, Falls: w.Rain + w.Snow}
	}
	m, _ := w.Dome.At(x, y)
	if w.Dry {
		m.Falls = 0
	}
	return m
}

// Most is the most of each that any mass over the world holds.
func (w Weather) Most() Mass {
	if w.Dome == nil {
		return w.At(0, 0)
	}
	m := w.Dome.Most()
	if w.Dry {
		m.Falls = 0
	}
	return m
}

// Falling is what of falls comes down as rain and what as snow at a temperature in degrees
// Celsius.
func Falling(falls, temperature float32) (rain, snow float32) {
	if temperature < snowsBelow {
		return 0, falls
	}
	return falls, 0
}

// degrees Celsius
const snowsBelow = 1

// Falls is the rain and the snow coming down from the mass m.
func (w Weather) Falls(m Mass) (rain, snow float32) { return Falling(m.Falls, w.Temperature) }

// ClearAir is how far one sees through clear air, in metres.
const ClearAir = 40000.0

// Sight is how far one sees through clear air on a world of scale, in world units; 0, without
// end, on a world without a scale.
func Sight(scale world.Scale) float64 {
	if scale.Metres <= 0 {
		return 0
	}
	return scale.Units(ClearAir)
}

// Visibility is how far one sees through the air over the world point (x, y), its mass's clouds
// and what falls from them, in world units; 0 without end.
func (w Weather) Visibility(x, y float32) float64 {
	if w.Sight <= 0 {
		return 0
	}
	m := w.At(x, y)
	rain, snow := w.Falls(m)
	clouds, rain, snow := clamp01(m.Cover), clamp01(rain), clamp01(snow)
	return w.Sight * (1 - 0.35*float64(clouds)) / (1 + 8*float64(rain) + 20*float64(snow))
}

func clamp01(v float32) float32 { return min(max(v, 0), 1) }

// Frame hands f the air's uniforms (shaders/*.wgsl), where its Dome's levels lie and the clouds'
// Tile, and the sky's colours under sun as the mass over the world point (x, y) has them, the
// eye's: at the horizon, the Fog what lies far off turns to, and overhead (SkyOverhead). The
// dome's own picture is its DomeImage's.
func (w Weather) Frame(f *render.Frame, sun sky.Sun, x, y float32) {
	f.Image("CloudTile", Tile())
	f.Uniform("Wind", w.Wind[0], w.Wind[1])
	f.Uniform("Drift", float32(wrap(float64(w.Drift[0]), HeapTile)),
		float32(wrap(float64(w.Drift[1]), HeapTile)))
	most := w.Most()
	f.Uniform("Cover", most.Cover)
	f.Uniform("Billow", most.Billow)
	if w.Dome != nil {
		for k, names := range levelNames {
			a, c := w.Dome.frame(k)
			f.Uniform(names[0], a[:]...)
			f.Uniform(names[1], c[:]...)
		}
	}
	cover := w.At(x, y).Cover
	fog := Overcast(sun.SkyLight(), cover)
	f.Uniform("Fog", fog[0], fog[1], fog[2])
	overhead := Overhead(sun.SkyLight(), cover)
	f.Uniform("SkyOverhead", overhead[0], overhead[1], overhead[2])
}
