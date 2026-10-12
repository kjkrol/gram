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

// radians from +x, east, with y running south
func (w Way) angle() float64 {
	return [...]float64{-3 * math.Pi / 4, -math.Pi / 2, -math.Pi / 4, 0, math.Pi / 4, math.Pi / 2, 3 * math.Pi / 4, math.Pi}[w%8]
}

// Place is where on a world its sky is watched from: the Latitude in degrees from the equator,
// north or south alike, and which Way the sun stands at noon.
type Place struct {
	Latitude float64
	NoonWay  Way
}

// AxialTilt is how far the world's axis leans, the Earth's, in radians.
const AxialTilt = 23.44 * math.Pi / 180

// SunPath is the way towards the sun at ofYear, the part of the year gone (0 the first of
// spring), and time of day t (0 to 1).
func (p Place) SunPath(ofYear, t float32) [3]float32 {
	decl := math.Asin(math.Sin(AxialTilt) * math.Sin(2*math.Pi*float64(ofYear)))
	hour := (float64(t) - 0.5) * 2 * math.Pi
	return p.horizon(decl, hour)
}

// hour in radians, 0 on the meridian at noon's side, growing westwards; worked out with noon in
// the south (x east, y south, z up), then turned to NoonWay
func (p Place) horizon(decl, hour float64) [3]float32 {
	lat := p.Latitude * math.Pi / 180
	up := math.Sin(lat)*math.Sin(decl) + math.Cos(lat)*math.Cos(decl)*math.Cos(hour)
	east := -math.Cos(decl) * math.Sin(hour)
	south := math.Sin(lat)*math.Cos(decl)*math.Cos(hour) - math.Cos(lat)*math.Sin(decl)
	turn := p.NoonWay.angle() - math.Pi/2
	sin, cos := math.Sincos(turn)
	return [3]float32{float32(east*cos - south*sin), float32(east*sin + south*cos), float32(up)}
}
