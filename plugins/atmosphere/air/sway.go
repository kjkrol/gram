package air

import "math"

// Sway is how far something at the world point (x, y) swaying in w's wind leans at time, as a
// slope along x and y, a unit of height moving that far: a little in the steady wind, further as
// a gust goes over it, rocking as it leans. An amount of 0 sways not at all.
func (w Weather) Sway(time, x, y, amount float32) (float32, float32) {
	blow := float32(math.Hypot(float64(w.Wind[0]), float64(w.Wind[1])))
	if blow == 0 || amount <= 0 {
		return 0, 0
	}
	lean := amount * swayLean * min(blow/swayWind, 1.5) *
		(swayRest + (1-swayRest)*w.Gust(x, y)) *
		(1 + swayRock*float32(math.Sin(float64(time*swayRate+x*swayBeatX+y*swayBeatY))))
	return w.Wind[0] / blow * lean, w.Wind[1] / blow * lean
}

// swayWind is the wind what sways leans its whole lean in, world units a second; swayLean that
// lean under a gust, swayRest the share of it in the steady wind, swayRock how far about it a
// thing rocks; swayRate radians a second, swayBeat radians a world unit: neighbours rock apart.
const (
	swayWind  = 40
	swayLean  = 0.35
	swayRest  = 0.25
	swayRock  = 0.25
	swayRate  = 2.2
	swayBeatX = 0.071
	swayBeatY = 0.113
)
