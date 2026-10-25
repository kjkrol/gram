package climate

import (
	"time"

	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
)

// Weather is the weather of a world and its sky, held by its own entity and saved with it, the
// air masses beside it as air.Patch entities. Lanes are the weathers the wind brings in from the
// horizon, lane 0 the one upwind of the world's middle and, unless the climate is Patchy, the
// only one; Over is the weather over the world's middle now, its Rain and Snow 0 to 1 and its
// Temperature in degrees Celsius. Blow is world units a second towards Heading (radians from
// +x), Drift how far the wind has carried the clouds, Forced seconds of game time the world and
// the sky upwind of it still settle into a weather set now.
type Weather struct {
	Over        int32
	Heading     float32
	Blow        float32
	Target      float32
	Rain        float32
	Snow        float32
	Temperature float32
	Drift       [2]float32
	Forced      float32
	Dice        uint64
	Lanes       [MaxLanes]Lane
}

// MaxLanes is how many lanes a Patchy climate's horizon has at most.
const MaxLanes = 32

// Lane is a weather the wind brings in from the horizon along one lane: its State, the seconds
// of game time it has Left, and the clouds and what falls from them coming to be the State's;
// Billow 0 torn shreds to 1 big heaps.
type Lane struct {
	State    int32
	Left     float32
	Clouds   float32
	CloudsTo float32
	Billow   float32
	BillowTo float32
	Falls    float32
}

// Config is the climate: its Zone, the Weathers it goes through (weather.Default when none), the
// one a fresh Stage begins in (Start, by name; thrown when empty), the Seed of the dice and how
// long the air takes to settle into a new weather (Blend; 20 seconds when zero). Horizon is how
// many metres past the world the sky's air masses reach (three times air.ClearAir when zero);
// Patchy, the horizon brings weathers lane by lane, each its own; OneLayer, the clouds hang in one
// layer, for a flat map nothing stands up into (air.Weather.OneLayer).
type Config struct {
	Zone     Zone
	Weathers []weather.State
	Start    string
	Seed     uint64
	Blend    time.Duration
	Horizon  float32
	Patchy   bool
	OneLayer bool
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
	if c.Horizon <= 0 {
		c.Horizon = 3 * air.ClearAir
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
