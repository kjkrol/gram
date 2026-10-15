package sky

import (
	"slices"

	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/render"
)

// the full moon far brighter than the real one, so the night shows
const (
	sunStrength  = 0.71
	moonStrength = 0.5
)

var moonColor = render.Light{0.75, 0.82, 1}

// Daylight is a row of the light by the sun's height: at Height, the sine of the sun's height, the
// Sky's colour, the Sun's own and how strong the sky's Ambient light is, blended between two rows.
type Daylight struct {
	Height   float32
	Sky, Sun render.Light
	Ambient  float32
}

var earthDaylight = []Daylight{
	{-0.2, render.Light{0.03, 0.05, 0.12}, render.Light{1, 0.45, 0.2}, 1.2},   // night
	{-0.05, render.Light{0.25, 0.2, 0.35}, render.Light{1, 0.45, 0.2}, 0.8},   // twilight
	{0.05, render.Light{0.95, 0.55, 0.35}, render.Light{1, 0.55, 0.25}, 0.45}, // sunrise, sunset
	{0.35, render.Light{0.5, 0.72, 0.98}, render.Light{1, 0.97, 0.92}, 0.42},  // day
}

// EarthDaylight is the Earth's light: a dark blue night, a violet twilight, an orange sunrise and
// a blue day under a white sun.
func EarthDaylight() []Daylight { return slices.Clone(earthDaylight) }

func (c Config) daylightAt(height float32) (sky, sun render.Light, ambient float32) {
	rows := c.Daylight
	if height <= rows[0].Height {
		return rows[0].Sky, rows[0].Sun, rows[0].Ambient
	}
	for i := 1; i < len(rows); i++ {
		a, b := rows[i-1], rows[i]
		if height > b.Height {
			continue
		}
		f := (height - a.Height) / (b.Height - a.Height)
		for k := range sky {
			sky[k] = a.Sky[k] + (b.Sky[k]-a.Sky[k])*f
			sun[k] = a.Sun[k] + (b.Sun[k]-a.Sun[k])*f
		}
		return sky, sun, a.Ambient + (b.Ambient-a.Ambient)*f
	}
	last := rows[len(rows)-1]
	return last.Sky, last.Sun, last.Ambient
}

// SunAt is the sun at ofYear, the part of the year gone, and time of day t, both 0 to 1.
func (c Config) SunAt(ofYear, t float32) Sun {
	c = c.withDefaults()
	dir := c.place().SunPath(ofYear, t)
	sky, sun, ambient := c.daylightAt(dir[2])
	return Sun{
		Dir:      dir,
		Strength: sunStrength * celestial.Risen(dir[2]),
		Ambient:  ambient,
		Color:    sun,
		Sky:      sky,
	}
}

// LightAt is what lights the world at ofYear and time of day t: the sun, or once it is well
// below the horizon the moon m, moon round from new (0 to 1).
func (c Config) LightAt(ofYear, t, moon float32, m Moon) Sun {
	c = c.withDefaults()
	light := c.SunAt(ofYear, t)
	sunUp := light.Dir[2]
	if celestial.Up(sunUp) {
		return light
	}
	dir := c.place().MoonAt(ofYear, t, moon)
	light.Dir, light.Color = dir, m.Color
	light.Strength = m.Strength * celestial.Phase(moon) * celestial.Risen(dir[2]) *
		celestial.Gone(sunUp)
	return light
}
