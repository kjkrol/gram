package sky

import (
	"math"
	"time"

	"github.com/kjkrol/gram/plugins/world"
)

// Day is the time of day, the one fact of the sky, held by the sky's own entity and saved with it:
// Time is the part of the day gone, 0 at midnight, 0.5 at noon; Pace how many times faster than
// Config.Length the day goes by; Stopped holds it where it is, the pace kept for when it goes on.
type Day struct {
	Time    float32
	Pace    float32
	Stopped bool
}

// Config is the day: how long a whole one takes at pace 1, the time a Stage starting fresh begins
// at, how high the sun stands at noon (radians above the horizon) and which way along the ground
// (x, y), and in how many steps a day the sun moves — the terrain's shadows are worked out anew at
// every step. Zero fields are 4 minutes, 8 in the morning, 60°, the south (0, 1) and 96 steps.
type Config struct {
	Length  time.Duration
	Start   float32
	Noon    float32
	NoonWay [2]float32
	Steps   int
}

func (c Config) withDefaults() Config {
	if c.Length <= 0 {
		c.Length = 4 * time.Minute
	}
	if c.Start == 0 {
		c.Start = 8.0 / 24
	}
	if c.Noon == 0 {
		c.Noon = math.Pi / 3
	}
	if c.NoonWay == [2]float32{} {
		c.NoonWay = [2]float32{0, 1}
	}
	if c.Steps <= 0 {
		c.Steps = 96
	}
	return c
}

// The light a day goes through: the sun's own strength, the ambient light by day and by night.
const (
	sunStrength  = 0.71
	dayAmbient   = 0.35
	nightAmbient = 0.12
)

// SunAt is the sun of the day c at time of day t: over NoonWay at noon at the height Noon, a
// quarter turn round from it at 6 and at 18 — with noon in the south rising in the east (+x) and
// setting in the west — below the horizon at night; its strength and the ambient light rise and
// fall with it.
func (c Config) SunAt(t float32) world.Sun {
	c = c.withDefaults()
	across := (float64(t) - 0.25) * 2 * math.Pi
	up := float64(c.Noon) * math.Sin(across)
	// the path over the south, turned round so its noon stands over NoonWay
	turn := math.Atan2(float64(c.NoonWay[1]), float64(c.NoonWay[0])) - math.Pi/2
	way := across + turn
	dir := [3]float32{
		float32(math.Cos(up) * math.Cos(way)),
		float32(math.Cos(up) * math.Sin(way)),
		float32(math.Sin(up)),
	}
	height := float32(math.Sin(up))
	return world.Sun{
		Dir:      dir,
		Strength: sunStrength * smoothstep(0, 0.2, height),
		Ambient:  nightAmbient + (dayAmbient-nightAmbient)*smoothstep(-0.2, 0.3, height),
	}
}

// smoothstep eases from 0 at a to 1 at b.
func smoothstep(a, b, x float32) float32 {
	t := min(max((x-a)/(b-a), 0), 1)
	return t * t * (3 - 2*t)
}
