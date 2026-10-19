package main

import (
	"math"
	"time"

	"github.com/kjkrol/gram/plugins/light"
)

// a day of the game: the sun rises in the east, crosses the sky leaning south and sets in the
// west, the night as long as the day
const dayLength = 90 * time.Second

// sunAt is the sun of the moment t: strongest overhead, warm near the horizon, gone at night,
// the ambient light cooler and dimmer then.
func sunAt(t time.Duration) light.Distant {
	turn := float64(t%dayLength)/float64(dayLength)*2*math.Pi + math.Pi/4 // begun mid-morning
	height := math.Sin(turn)
	day := float32(min(max(height*2, 0), 1))
	warm := float32(min(max(height*3, 0), 1))
	return light.Distant{
		Dir:          [3]float32{float32(math.Cos(turn)), 0.4, float32(max(height, -0.2))},
		Strength:     0.7 * float32(max(height, 0)),
		Ambient:      0.18 + 0.17*day,
		Color:        render3(1, 0.75+0.25*warm, 0.5+0.5*warm),
		AmbientColor: render3(0.55+0.45*day, 0.65+0.35*day, 1),
	}
}
