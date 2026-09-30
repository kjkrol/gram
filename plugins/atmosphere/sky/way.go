package sky

import "math"

// Way is where along the ground the sun stands at noon.
type Way uint8

const (
	NorthWest Way = iota
	North
	NorthEast
	East
	SouthEast
	South
	SouthWest
	West
)

// angle is the way as radians from +x (east), y running south.
func (w Way) angle() float64 {
	return [...]float64{-3 * math.Pi / 4, -math.Pi / 2, -math.Pi / 4, 0, math.Pi / 4, math.Pi / 2, 3 * math.Pi / 4, math.Pi}[w%8]
}

// Latitude is where the sky stands unless the climate's zone says otherwise: 30° from the
// equator, the sun 60° up at noon at the equinoxes.
const Latitude = 30

// Config is the sun's path and the light: which Way the sun stands at noon (the north-west by
// default), in how many Steps a day it moves — none, continuously, every tick — and whether the
// light begins Frozen at Hour of the day (0 to 1; noon when zero). How high the sun goes is the latitude's, the climate's zone's in an atmosphere.
type Config struct {
	NoonWay Way
	Steps   int
	Frozen  bool
	Hour    float32

	latitude float64 // degrees from the equator; New sets it
}

func (c Config) withDefaults() Config {
	if c.Hour == 0 {
		c.Hour = 0.5
	}
	return c
}
