package air

import "math"

// Sway is how far something at the world point (x, y) swaying in w's wind leans at time, as a
// slope along x and y — a unit of height moves that far: with the wind, the harder it blows the
// further, rocking about that as gusts roll through downwind; amount 0 or no wind, not at all.
func (w Weather) Sway(time, x, y, amount float32) (float32, float32) {
	wind := w.Wind
	blow := float32(math.Hypot(float64(wind[0]), float64(wind[1])))
	if blow == 0 || amount <= 0 {
		return 0, 0
	}
	wx, wy := wind[0]/blow, wind[1]/blow
	strength := min(blow/swayWind, 1.5)
	gust := (x*wx + y*wy) / swayGust // a gust reaches what lies further downwind later
	rock := float32(math.Sin(float64(time*swayRate*(0.7+0.3*strength) - gust)))
	lean := amount * swayLean * strength * (0.5 + 0.5*rock)
	return wx * lean, wy * lean
}

// How swaying goes: the wind, in world units a second, that bends it swayLean a unit of height,
// how fast it rocks, radians a second, and how far apart, in world units, gusts roll through.
const (
	swayWind = 40
	swayLean = 0.35
	swayRate = 2.2
	swayGust = 60
)
