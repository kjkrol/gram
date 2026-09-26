package sky

import (
	"math"
	"time"

	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
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

// sunStrength is the sun's own strength high in the sky.
const sunStrength = 0.71

// daylight is the light of the day by how high the sun stands — the sine of its height, rising
// through the table — the colour of the sky, of the sun's light, and how much of the sky's light
// every surface gets; between two rows it is blended.
var daylight = []struct {
	height   float32
	sky, sun render.Light
	ambient  float32
}{
	{-0.2, render.Light{0.03, 0.05, 0.12}, render.Light{1, 0.45, 0.2}, 1.2},   // night
	{-0.05, render.Light{0.25, 0.2, 0.35}, render.Light{1, 0.45, 0.2}, 0.8},   // twilight
	{0.05, render.Light{0.95, 0.55, 0.35}, render.Light{1, 0.55, 0.25}, 0.45}, // sunrise, sunset
	{0.35, render.Light{0.5, 0.72, 0.98}, render.Light{1, 0.97, 0.92}, 0.42},  // day
}

// daylightAt is the row of daylight for a sun at height, blended between its neighbours.
func daylightAt(height float32) (sky, sun render.Light, ambient float32) {
	rows := daylight
	if height <= rows[0].height {
		return rows[0].sky, rows[0].sun, rows[0].ambient
	}
	for i := 1; i < len(rows); i++ {
		a, b := rows[i-1], rows[i]
		if height > b.height {
			continue
		}
		f := (height - a.height) / (b.height - a.height)
		for c := range sky {
			sky[c] = a.sky[c] + (b.sky[c]-a.sky[c])*f
			sun[c] = a.sun[c] + (b.sun[c]-a.sun[c])*f
		}
		return sky, sun, a.ambient + (b.ambient-a.ambient)*f
	}
	last := rows[len(rows)-1]
	return last.sky, last.sun, last.ambient
}

// SunAt is the sun of the day c at time of day t: over NoonWay at noon at the height Noon, a
// quarter turn round from it at 6 and at 18 — with noon in the south rising in the east (+x) and
// setting in the west — below the horizon at night; its strength rises and falls with it, and the
// colours of the sky and of its light go through the day: blue by day, orange at sunrise and
// sunset, deep blue at night (daylight).
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
	sky, sun, ambient := daylightAt(height)
	return world.Sun{
		Dir:      dir,
		Strength: sunStrength * smoothstep(0, 0.2, height),
		Ambient:  ambient,
		Color:    sun,
		Sky:      sky,
	}
}

// smoothstep eases from 0 at a to 1 at b.
func smoothstep(a, b, x float32) float32 {
	t := min(max((x-a)/(b-a), 0), 1)
	return t * t * (3 - 2*t)
}
