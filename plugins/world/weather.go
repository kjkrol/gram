package world

import "github.com/kjkrol/gram/render"

// Weather is the air over the world: Wind blowing, world units a second along x and y; Clouds
// covering the sky, 0 to 1; Rain and Snow falling now, 0 to 1 each; the Temperature, degrees
// Celsius; Drift, how far the wind has carried the clouds so far; Visibility, how far one sees
// through the air, world units, 0 without end — the world works it out of its Scale when whoever
// sets the weather leaves it 0. The zero Weather is a calm, clear day at 0°. Whoever makes the weather — plugins/climate — sets it; the renderers draw it
// and a game's behaviours read it, casting their effects — snow, ice, trees swaying — as it says.
type Weather struct {
	Wind        [2]float32
	Clouds      float32
	Rain, Snow  float32
	Temperature float32
	Drift       [2]float32
	Visibility  float64
}

// Frame is the weather as a render.Frame needs it.
func (w Weather) Frame() render.Weather {
	return render.Weather{Wind: w.Wind, Drift: w.Drift, Clouds: w.Clouds, Visibility: float32(w.Visibility)}
}
