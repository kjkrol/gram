package celestial

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

// Place is where on a world its sky is watched from: how far from the equator (Latitude,
// degrees; north or south alike) and which Way the sun stands at noon (NoonWay), so its whole path
// and the sphere of the stars turn with it over the map.
type Place struct {
	Latitude float64
	NoonWay  Way
}

// AxialTilt is how far the world's axis leans, which is how far north and south of the equator the
// sun goes through the year: the Earth's.
const AxialTilt = 23.44 * math.Pi / 180

// SunPath is the way towards the sun at ofYear, the part of the year gone (0 the first of spring),
// and time of day t: at noon over NoonWay, 90° less the latitude up at the equinoxes — its
// declination higher at midsummer and lower at midwinter — rising in the east at 6 and setting in
// the west at 18 at the equinoxes, the days longer in summer, shorter in winter; up all day or none
// at all past the polar circle.
func (p Place) SunPath(ofYear, t float32) [3]float32 {
	decl := math.Asin(math.Sin(AxialTilt) * math.Sin(2*math.Pi*float64(ofYear))) // 0 at the equinoxes, highest at midsummer
	hour := (float64(t) - 0.5) * 2 * math.Pi                                     // 0 at noon
	return p.horizon(decl, hour)
}

// horizon is the way towards what stands at declination decl and hour angle hour (radians, 0 on
// the meridian at noon's side, growing westwards) from p: worked out with noon in the south (x
// along the ground east, y south, z up), then turned to NoonWay.
func (p Place) horizon(decl, hour float64) [3]float32 {
	lat := p.Latitude * math.Pi / 180
	up := math.Sin(lat)*math.Sin(decl) + math.Cos(lat)*math.Cos(decl)*math.Cos(hour)
	east := -math.Cos(decl) * math.Sin(hour)
	south := math.Sin(lat)*math.Cos(decl)*math.Cos(hour) - math.Cos(lat)*math.Sin(decl)
	turn := p.NoonWay.angle() - math.Pi/2 // the south's way (+y) turned to NoonWay
	sin, cos := math.Sincos(turn)
	return [3]float32{float32(east*cos - south*sin), float32(east*sin + south*cos), float32(up)}
}
