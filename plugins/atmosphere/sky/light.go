package sky

import (
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/render"
)

// the full moon far brighter than the real one, so the night shows
const (
	sunStrength  = 0.71
	moonStrength = 0.5
)

var moonColor = render.Light{0.75, 0.82, 1}

// height is the sine of the sun's height; between two rows the light is blended
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

// SunColorAt is the colour of the sun's own light at height, the sine of its height: white by
// day, orange low.
func SunColorAt(height float32) render.Light {
	_, sun, _ := daylightAt(height)
	return sun
}

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

// SunAt is the sun at ofYear, the part of the year gone, and time of day t, both 0 to 1.
func (c Config) SunAt(ofYear, t float32) Sun {
	c = c.withDefaults()
	dir := c.place().SunPath(ofYear, t)
	sky, sun, ambient := daylightAt(dir[2])
	return Sun{Dir: dir, Strength: sunStrength * smoothstep(0, 0.2, dir[2]), Ambient: ambient, Color: sun, Sky: sky}
}

// LightAt is what lights the world at ofYear and time of day t: the sun, or once it is well
// below the horizon the moon m, moon round from new (0 to 1).
func (c Config) LightAt(ofYear, t, moon float32, m Moon) Sun {
	c = c.withDefaults()
	light := c.SunAt(ofYear, t)
	sunUp := light.Dir[2]
	if sunUp > -0.1 {
		return light
	}
	dir := c.place().MoonAt(ofYear, t, moon)
	light.Dir, light.Color = dir, m.Color
	light.Strength = m.Strength * celestial.Phase(moon) * smoothstep(0, 0.2, dir[2]) * smoothstep(-0.1, -0.25, sunUp)
	return light
}

func smoothstep(a, b, x float32) float32 {
	t := min(max((x-a)/(b-a), 0), 1)
	return t * t * (3 - 2*t)
}
