package climate

import (
	"time"

	"github.com/kjkrol/gram/plugins/climate/weather"
)

// Weather is the weather now, the one fact of the plugin, held by its own entity and saved with
// it: which of the climate's weathers it is in and how many seconds of it are left, the wind it
// has come to — Blow world units a second towards Heading, radians from +x — and the Target it is
// blowing up or down to, the Clouds, the Rain and the Snow falling, the Temperature (degrees
// Celsius), how far the wind has carried the clouds (Drift), the dice the next weather is thrown
// with, and the sky's day it last saw (SeenDate, SeenTime), whose passing is its time.
type Weather struct {
	State       int32
	Left        float32
	Heading     float32
	Blow        float32
	Target      float32
	Clouds      float32
	Rain        float32
	Snow        float32
	Temperature float32
	Drift       [2]float32
	Dice        uint64
	SeenDate    int32
	SeenTime    float32
}

// Config is the climate: where in the world it lies (Zone; the equator when zero), the weathers it
// goes through (weather.Default when none), the one a fresh Stage begins in (Start, by name;
// thrown as the zone and the season have them when empty), the seed of the dice (1 when zero) and
// how long the wind, the clouds and what falls take to come most of the way to a new weather's
// (Blend; 20 seconds when zero).
type Config struct {
	Zone     Zone
	Weathers []weather.State
	Start    string
	Seed     uint64
	Blend    time.Duration
}

func (c Config) withDefaults() Config {
	if len(c.Weathers) == 0 {
		c.Weathers = weather.Default
	}
	if c.Seed == 0 {
		c.Seed = 1
	}
	if c.Blend <= 0 {
		c.Blend = 20 * time.Second
	}
	return c
}

// index is the weather named name, or -1.
func (c Config) index(name string) int32 {
	for i, s := range c.Weathers {
		if s.Name == name {
			return int32(i)
		}
	}
	return -1
}
