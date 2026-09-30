package celestial

import "math"

// SiderealTime is the local sidereal time at ofYear, the part of the year gone (0 the first of
// spring, the sun at the vernal equinox), and time of day t, radians: the sun's hour angle and
// its right ascension along the ecliptic, the sky turning once a day and once more a year.
func SiderealTime(ofYear, t float32) float64 {
	sun := 2 * math.Pi * float64(ofYear) // the sun's ecliptic longitude
	return (float64(t)-0.5)*2*math.Pi + math.Atan2(math.Cos(AxialTilt)*math.Sin(sun), math.Cos(sun))
}

// Sphere is how the celestial sphere stands over p at sidereal time lst: the ways its equatorial
// frame's axes point — right ascension 0h and 6h on the celestial equator, and the north celestial
// pole — laid out as the sun's path is, so the sun, the moon and the stars stand together.
func (p Place) Sphere(lst float64) [3][3]float32 {
	return [3][3]float32{p.horizon(0, lst), p.horizon(0, lst-math.Pi/2), p.horizon(math.Pi/2, 0)}
}

// onSky is the way towards the equatorial unit vector v on a sphere standing as sphere says.
func onSky(sphere [3][3]float32, v [3]float32) [3]float32 {
	var w [3]float32
	for k := range w {
		w[k] = sphere[0][k]*v[0] + sphere[1][k]*v[1] + sphere[2][k]*v[2]
	}
	return w
}
