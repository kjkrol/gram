package climate

import (
	"time"

	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
)

// Weather is the weather now, held by its own entity and saved with it. Left is seconds of game
// time, Blow world units a second towards Heading (radians from +x), Billow 0 shreds to 1 heaps,
// Temperature degrees Celsius, Drift how far the wind has carried the clouds.
type Weather struct {
	State       int32
	Left        float32
	Heading     float32
	Blow        float32
	Target      float32
	Clouds      float32
	CloudsTo    float32
	Billow      float32
	BillowTo    float32
	Rain        float32
	Snow        float32
	Temperature float32
	Drift       [2]float32
	Dice        uint64
}

// Config is the climate: its Zone, the Weathers it goes through (weather.Default when none), the
// one a fresh Stage begins in (Start, by name; thrown when empty), the Seed of the dice and how
// long the air takes to settle into a new weather (Blend; 20 seconds when zero).
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

func (c Config) index(name string) int32 {
	for i, s := range c.Weathers {
		if s.Name == name {
			return int32(i)
		}
	}
	return -1
}
