package air

import (
	"embed"

	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// The air's materials (shaders/), with its uniforms, registered as the package is set up:
// cloudShadow the clouds' shadows over the ground, cloudsOverhead the clouds on the sky; a
// material of water calls their functions. Wind blows in world units a second, Drift is how far
// it has carried the clouds, Cover how much of the sky they cover (Weather.Frame sets them); the
// sky's lines of sight for the clouds drawn on it (Weather.CloudQuad): the eye, the way the screen
// pixel (x, y) looks, LookDir + LookDX·x + LookDY·y; CloudHeight how high the layer lies,
// Visibility how far one sees through the air, 0 without end.
var weatherMaterials = render.RegisterMaterials(render.Files(shaders, "shaders/cloud_noise.wgsl", "shaders/cloud_shadow.wgsl", "shaders/clouds.wgsl"), []render.Uniform{
	{Name: "Wind", Size: 2}, {Name: "Drift", Size: 2}, {Name: "Cover", Size: 1},
	{Name: "EyeAt", Size: 3}, {Name: "LookDir", Size: 3}, {Name: "LookDX", Size: 3}, {Name: "LookDY", Size: 3},
	{Name: "CloudHeight", Size: 1}, {Name: "Visibility", Size: 1},
}, "CloudShadow", "Clouds")

var cloudShadow, cloudsOverhead = weatherMaterials[0], weatherMaterials[1]

// CloudShadow is the material of the clouds' shadows, for a test telling its overlays apart.
func CloudShadow() render.MaterialID { return cloudShadow }

// CloudsOverhead is the material of the clouds on the sky, for a test telling its pieces apart.
func CloudsOverhead() render.MaterialID { return cloudsOverhead }

// Weather is the air over the world: Wind blowing, world units a second along x and y; Clouds
// covering the sky, 0 to 1; Rain and Snow falling now, 0 to 1 each; the Temperature, degrees
// Celsius; Drift, how far the wind has carried the clouds so far; Visibility, how far one sees
// through the air, world units, 0 without end. The zero Weather is a calm, clear day at 0°. The
// climate makes it; the renderers draw it — the clouds' shadows, what sways, the haze — and a
// game's behaviours read it.
type Weather struct {
	Wind        [2]float32
	Clouds      float32
	Rain, Snow  float32
	Temperature float32
	Drift       [2]float32
	Visibility  float64
}

// ClearAir is how far one sees through clear air, in metres.
const ClearAir = 40000.0

// Visibility is how far one sees through the air of w on a world of scale, in world units:
// ClearAir under a clear sky, less under clouds, far less in rain and snow; 0, without end, on a
// world without a scale.
func Visibility(scale world.Scale, w Weather) float64 {
	if scale.Metres <= 0 {
		return 0
	}
	clouds, rain, snow := clamp01(w.Clouds), clamp01(w.Rain), clamp01(w.Snow)
	return scale.Units(ClearAir * (1 - 0.35*float64(clouds)) / (1 + 8*float64(rain) + 20*float64(snow)))
}

func clamp01(v float32) float32 { return min(max(v, 0), 1) }

// Frame hands f the air as its shader reads it (weather.kage) — the wind, the clouds' drift and
// cover — and the colour what lies far off turns to: the sky of sun, greyed by the clouds.
func (w Weather) Frame(f *render.Frame, sun sky.Sun) {
	f.Uniform("Wind", w.Wind[0], w.Wind[1])
	f.Uniform("Drift", w.Drift[0], w.Drift[1])
	f.Uniform("Cover", w.Clouds)
	fog := Overcast(sun.SkyLight(), w.Clouds)
	f.Uniform("Fog", fog[0], fog[1], fog[2])
}
