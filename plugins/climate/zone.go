package climate

import (
	"math"

	"github.com/kjkrol/gram/plugins/sky"
)

// Zone is where in the world a climate lies: how far from the equator (Latitude, degrees, north or
// south alike), which sets the sun's path and, above all, the climate's warmth and seasons; and
// the other Factors that shape it on top — a sea current, a dry summer — which a game adds to as
// its world needs.
type Zone struct {
	Latitude float64
	Factors  []Factor
}

// Factor is something besides the latitude that shapes a climate: it reshapes the Profile the
// latitude gave, after the factors before it.
type Factor interface {
	Shape(p *Profile)
}

// Profile is a climate in the numbers its weather needs: the year's mean temperature, how far
// above it midsummer and below it midwinter go (Year), how far the afternoon is warmer and the
// small hours colder (Day), all degrees Celsius; and how wet each season is, by sky.Season: how
// much likelier than elsewhere a weather that brings rain or snow comes then (Wet).
type Profile struct {
	Mean, Year, Day float32
	Wet             [4]float32
}

// Profile is the zone's climate: from its latitude, then shaped by each of its factors in turn.
func (z Zone) Profile() Profile {
	s := math.Sin(math.Abs(z.Latitude) * math.Pi / 180)
	s2 := s * s
	p := Profile{
		Mean: float32(27 - 20*s2 - 27*s2*s2*s2), // 27° at the equator down to −20° at the pole
		Year: float32(1 + 16*s2),
		Day:  4,
		Wet:  wetAt(math.Abs(z.Latitude)),
	}
	for _, f := range z.Factors {
		f.Shape(&p)
	}
	return p
}

// wetAt is how wet each season is at latitude: rain all year at the equator, a wet summer and a
// dry winter in the tropics, wet springs and autumns in the subtropics, all year and wettest in
// autumn in the temperate belt, less towards the cold, little in the polar desert.
func wetAt(latitude float64) [4]float32 {
	switch {
	case latitude < 10:
		return [4]float32{1.8, 1.8, 1.8, 1.8}
	case latitude < 25:
		return [4]float32{1, 2.2, 1.2, 0.3}
	case latitude < 45:
		return [4]float32{1.3, 0.7, 1.3, 1.2}
	case latitude < 60:
		return [4]float32{1, 1, 1.3, 1}
	case latitude < 72:
		return [4]float32{0.8, 0.9, 1, 0.8}
	}
	return [4]float32{0.4, 0.5, 0.5, 0.4}
}

// SeaCurrent is a sea current along the coast, warming the climate by Warmth degrees — a cold one
// by a negative Warmth — and softening its seasons a little.
type SeaCurrent struct{ Warmth float32 }

func (c SeaCurrent) Shape(p *Profile) {
	p.Mean += c.Warmth
	p.Year *= 0.85
}

// DrySummer is the Mediterranean's summer drought: summer all but dry, the wet of the year in
// winter.
type DrySummer struct{}

func (DrySummer) Shape(p *Profile) {
	p.Wet[sky.Summer] *= 0.2
	p.Wet[sky.Winter] *= 1.5
}

// The zones of the world, from the equator to the pole.
var (
	// Equatorial is hot and wet all year, without seasons to speak of.
	Equatorial = Zone{Latitude: 3}
	// Tropical is hot, its summer the rainy season and its winter dry.
	Tropical = Zone{Latitude: 20}
	// Mediterranean is warm, its summer dry and its winter mild and wet.
	Mediterranean = Zone{Latitude: 38, Factors: []Factor{DrySummer{}}}
	// Temperate has four seasons, a warm summer and a winter below freezing: central Europe and
	// southern Scandinavia before the warming.
	Temperate = Zone{Latitude: 55}
	// Cold has a long, hard winter and a short, cool summer.
	Cold = Zone{Latitude: 66}
	// Polar is frozen most of the year, the sun up all summer and down all winter.
	Polar = Zone{Latitude: 78}
)
