package world

import "math"

// Scale is how the world measures against ours: Metres a world unit spans, across and up alike.
// The zero Scale leaves the world a board — flat as far as the eye goes, the air clear. With one
// the world is a stretch of the Earth's surface: a line of sight bends over it (Bend), the ground
// far off sinking under the horizon, and the air thickens with distance (Visibility). A game
// gives its sizes and heights in metres through Units.
type Scale struct{ Metres float64 }

// The Earth's radius in metres; how much the air bends a line of sight back towards the ground,
// the standard refraction; and how far one sees through clear air, in metres.
const (
	EarthRadius = 6371000.0
	Refraction  = 0.13
	ClearAir    = 40000.0
)

// Units is m metres in world units; m itself without a scale.
func (s Scale) Units(m float64) float64 {
	if s.Metres <= 0 {
		return m
	}
	return m / s.Metres
}

// Bend is how far below the level of an eye the ground d world units off lies, per d²: the
// Earth's curve as the air's refraction lessens it, (1 − Refraction) / (2·EarthRadius) in world
// units; 0 for a flat world.
func (s Scale) Bend() float64 {
	if s.Metres <= 0 {
		return 0
	}
	return (1 - Refraction) / (2 * s.Units(EarthRadius))
}

// Drop is how far below the level of an eye the ground d world units off lies: Bend·d².
func (s Scale) Drop(d float64) float64 { return s.Bend() * d * d }

// Horizon is how far over level ground an eye h world units up sees it before it sinks away:
// √(h / Bend); +Inf for a flat world.
func (s Scale) Horizon(h float64) float64 {
	if b := s.Bend(); b > 0 && h > 0 {
		return math.Sqrt(h / b)
	}
	return math.Inf(1)
}

// Visibility is how far one sees through the air of w, in world units: ClearAir under a clear sky,
// less under clouds, much less in rain and snow; 0 — no end to it — without a scale.
func (s Scale) Visibility(w Weather) float64 {
	if s.Metres <= 0 {
		return 0
	}
	clouds, rain, snow := clamp01(w.Clouds), clamp01(w.Rain), clamp01(w.Snow)
	return s.Units(ClearAir * (1 - 0.35*float64(clouds)) / (1 + 8*float64(rain) + 20*float64(snow)))
}

func clamp01(v float32) float32 { return min(max(v, 0), 1) }
