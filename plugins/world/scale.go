package world

import "github.com/kjkrol/gram/internal/f32"

// Scale is how many Metres a world unit spans, across and up alike, and how many times bigger
// than life the Figures standing on it are drawn. The zero Scale leaves the world a board, flat
// as far as the eye goes.
type Scale struct {
	Metres  float32
	Figures float32
}

// EarthRadius is in metres; Refraction is the standard refraction, how much the air bends a line
// of sight back towards the ground.
const (
	EarthRadius = 6371000.0
	Refraction  = 0.13
)

// Units is m metres in world units; m itself without a scale.
func (s Scale) Units(m float32) float32 {
	if s.Metres <= 0 {
		return m
	}
	return m / s.Metres
}

// Figure is the size in world units of a figure m metres big in life, Units(m) times Figures.
func (s Scale) Figure(m float32) float32 {
	if s.Figures <= 0 {
		return s.Units(m)
	}
	return s.Units(m) * s.Figures
}

// Bend is how far below the level of an eye the ground d world units off lies, per d²; 0 for a
// flat world.
func (s Scale) Bend() float32 {
	if s.Metres <= 0 {
		return 0
	}
	return (1 - Refraction) / (2 * s.Units(EarthRadius))
}

// Drop is how far below the level of an eye the ground d world units off lies: Bend·d².
func (s Scale) Drop(d float32) float32 { return s.Bend() * d * d }

// Horizon is how far over level ground an eye h world units up sees it before it sinks away;
// +Inf for a flat world.
func (s Scale) Horizon(h float32) float32 {
	if b := s.Bend(); b > 0 && h > 0 {
		return f32.Sqrt(h / b)
	}
	return f32.Inf(1)
}
