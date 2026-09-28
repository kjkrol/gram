package sky

import (
	"math"

	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// How bright the sun and the moon are high in the sky, the full moon's light a quarter of the
// sun's and bluer.
const (
	sunStrength  = 0.71
	moonStrength = 0.25
)

var moonColor = render.Light{0.75, 0.82, 1}

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

// axialTilt is how far the world's axis leans, which is how far north and south of the equator the
// sun goes through the year: the Earth's.
const axialTilt = 23.44 * math.Pi / 180

// path is the way towards what goes the sun's way across the sky, at ofYear and time of day t, at
// the config's latitude: at noon over NoonWay, 90° less the latitude up at the equinoxes — the
// sun's declination higher at midsummer and lower at midwinter — rising in the east at 6 and
// setting in the west at 18 at the equinoxes, the days longer in summer, shorter in winter; the
// way (x along the ground, y across it, z up) is worked out with noon in the south, then turned
// to NoonWay.
func (c Config) path(ofYear, t float32) [3]float32 {
	lat := c.latitude * math.Pi / 180
	decl := axialTilt * math.Sin(2*math.Pi*float64(ofYear)) // 0 at the equinoxes, highest at midsummer
	hour := (float64(t) - 0.5) * 2 * math.Pi                // 0 at noon
	up := math.Sin(lat)*math.Sin(decl) + math.Cos(lat)*math.Cos(decl)*math.Cos(hour)
	east := -math.Cos(decl) * math.Sin(hour)
	south := math.Sin(lat)*math.Cos(decl)*math.Cos(hour) - math.Cos(lat)*math.Sin(decl)
	turn := c.NoonWay.angle() - math.Pi/2 // the south's way (+y) turned to NoonWay
	sin, cos := math.Sincos(turn)
	return [3]float32{float32(east*cos - south*sin), float32(east*sin + south*cos), float32(up)}
}

// SunAt is the sun of the day c at ofYear, the part of the year gone, and time of day t, along its
// path, below the horizon at night; its strength rises and falls with it, and the colours of the
// sky and of its light go through the day: blue by day, orange at sunrise and sunset, deep blue at
// night (daylight).
func (c Config) SunAt(ofYear, t float32) world.Sun {
	c = c.withDefaults()
	dir := c.path(ofYear, t)
	sky, sun, ambient := daylightAt(dir[2])
	return world.Sun{Dir: dir, Strength: sunStrength * smoothstep(0, 0.2, dir[2]), Ambient: ambient, Color: sun, Sky: sky}
}

// LightAt is what lights the world at ofYear and time of day t with the moon moon round from new:
// the sun, and once it is well below the horizon the moon, going the sun's way as far behind it as
// it is round — rising at sunset when full — as bright as it is full and stands high, in its paler
// light, under the night sky.
func (c Config) LightAt(ofYear, t, moon float32) world.Sun {
	c = c.withDefaults()
	light := c.SunAt(ofYear, t)
	sunUp := light.Dir[2]
	if sunUp > -0.1 {
		return light // the sun, or its twilight
	}
	dir := c.path(ofYear+moon, t-moon)
	full := 0.5 * (1 - float32(math.Cos(2*math.Pi*float64(moon))))
	light.Dir, light.Color = dir, moonColor
	light.Strength = moonStrength * full * smoothstep(0, 0.2, dir[2]) * smoothstep(-0.1, -0.25, sunUp)
	return light
}

// smoothstep eases from 0 at a to 1 at b.
func smoothstep(a, b, x float32) float32 {
	t := min(max((x-a)/(b-a), 0), 1)
	return t * t * (3 - 2*t)
}
