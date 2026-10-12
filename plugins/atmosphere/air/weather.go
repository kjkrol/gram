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
var weatherMaterials = render.RegisterMaterials(render.Files(shaders, "shaders/cloud_noise.wgsl", "shaders/cloud_shadow.wgsl", "shaders/clouds.wgsl"), []render.Uniform{
	{Name: "Wind", Size: 2}, {Name: "Drift", Size: 2}, {Name: "Cover", Size: 1}, {Name: "Billow", Size: 1},
	{Name: "EyeAt", Size: 3}, {Name: "LookDir", Size: 3}, {Name: "LookDX", Size: 3}, {Name: "LookDY", Size: 3},
	{Name: "CloudHeight", Size: 1}, {Name: "Visibility", Size: 1},
}, "CloudShadow")

var cloudShadow = weatherMaterials[0]

// CloudShadow is the material of the clouds' shadows.
func CloudShadow() render.MaterialID { return cloudShadow }

// Weather is the air over the world; the zero Weather is a calm, clear day at 0°. Wind is in
// world units a second, Clouds, Rain and Snow 0 to 1, Billow 0 torn shreds to 1 big heaps,
// Temperature degrees Celsius, Drift how far the wind has carried the clouds, Visibility world
// units with 0 without end.
type Weather struct {
	Wind        [2]float32
	Clouds      float32
	Billow      float32
	Rain, Snow  float32
	Temperature float32
	Drift       [2]float32
	Visibility  float64
}

// ClearAir is how far one sees through clear air, in metres.
const ClearAir = 40000.0

// Visibility is how far one sees through the air of w on a world of scale, in world units; 0,
// without end, on a world without a scale.
func Visibility(scale world.Scale, w Weather) float64 {
	if scale.Metres <= 0 {
		return 0
	}
	clouds, rain, snow := clamp01(w.Clouds), clamp01(w.Rain), clamp01(w.Snow)
	return scale.Units(ClearAir * (1 - 0.35*float64(clouds)) / (1 + 8*float64(rain) + 20*float64(snow)))
}

func clamp01(v float32) float32 { return min(max(v, 0), 1) }

// Frame hands f the air's uniforms (shaders/*.wgsl) and the fog colour what lies far off turns
// to.
func (w Weather) Frame(f *render.Frame, sun sky.Sun) {
	f.Uniform("Wind", w.Wind[0], w.Wind[1])
	f.Uniform("Drift", float32(wrap(float64(w.Drift[0]), HeapTile)), float32(wrap(float64(w.Drift[1]), HeapTile)))
	f.Uniform("Cover", w.Clouds)
	f.Uniform("Billow", w.Billow)
	fog := Overcast(sun.SkyLight(), w.Clouds)
	f.Uniform("Fog", fog[0], fog[1], fog[2])
}
