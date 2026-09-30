package sky

import (
	"time"

	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
)

// Latitude is where the sky stands unless the climate's zone says otherwise: 30° from the
// equator, the sun 60° up at noon at the equinoxes.
const Latitude = 30

// Config is the sun's path and the light: which way the sun stands at noon (NoonWay, the
// north-west by default), in how many Steps a day it moves — none, continuously, every tick — and
// whether the light begins Frozen at Hour on the clock's face (noon when zero; 18*time.Hour is six
// in the evening), and which Stars the night shows (the real ones by default). How high the sun
// goes, and where the stars turn, is the latitude's, the climate's zone's in an atmosphere.
type Config struct {
	NoonWay celestial.Way
	Steps   int
	Frozen  bool
	Hour    time.Duration
	Stars   celestial.StarSky

	latitude float64 // degrees from the equator; New sets it
}

func (c Config) withDefaults() Config {
	if c.Hour == 0 {
		c.Hour = 12 * time.Hour
	}
	return c
}

// place is where the config watches the sky from.
func (c Config) place() celestial.Place {
	return celestial.Place{Latitude: c.latitude, NoonWay: c.NoonWay}
}
