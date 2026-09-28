package air

import (
	_ "embed"

	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

//go:embed weather.kage
var weatherKage []byte

// cloudShadow is the material of the clouds' shadows over the ground (weather.kage), with the
// air's uniforms; a material of water calls its functions. Registered as the package is set up.
var cloudShadow = render.RegisterMaterials(weatherKage, "CloudShadow")[0]

// CloudShadow is the material of the clouds' shadows, for a test telling its overlays apart.
func CloudShadow() render.MaterialID { return cloudShadow }

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
