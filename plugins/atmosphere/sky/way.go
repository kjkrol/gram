package sky

import (
	"time"

	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
)

// Latitude is the default latitude, 30 degrees from the equator.
const Latitude = 30

// Config is the sun's path and the light: which way the sun stands at noon, in how many Steps a
// day the light moves (none: as it goes), whether it begins Frozen at Hour on the clock's face
// (noon when zero), the Daylight by the sun's height (none: the Earth's) and the Moon the night
// begins under (zero: DefaultMoon).
type Config struct {
	NoonWay  celestial.Way
	Steps    int
	Frozen   bool
	Hour     time.Duration
	Daylight []Daylight
	Moon     Moon

	latitude float64 // degrees from the equator
}

func (c Config) withDefaults() Config {
	if c.Hour == 0 {
		c.Hour = 12 * time.Hour
	}
	if len(c.Daylight) == 0 {
		c.Daylight = earthDaylight
	}
	if c.Moon == (Moon{}) {
		c.Moon = DefaultMoon()
	}
	return c
}

func (c Config) place() celestial.Place {
	return celestial.Place{Latitude: c.latitude, NoonWay: c.NoonWay}
}
