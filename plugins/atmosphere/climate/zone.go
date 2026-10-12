package climate

import (
	"math"

	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
)

// Zone is where in the world a climate lies: its Latitude in degrees from the equator, north or
// south alike, and the Factors that shape it on top.
type Zone struct {
	Latitude float64
	Factors  []Factor
}

// Factor is something besides the latitude that shapes a climate's Profile.
type Factor interface {
	Shape(p *Profile)
}

// Profile is a climate in numbers: the year's Mean temperature, how far above and below it the
// Year and the Day swing, all degrees Celsius, and how much likelier a weather that brings rain or
// snow comes in each calendar.Season (Wet).
type Profile struct {
	Mean, Year, Day float32
	Wet             [4]float32
}

// Profile is the zone's climate in numbers.
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

// SeaCurrent is a sea current along the coast, warming the climate by Warmth degrees (a cold
// one by a negative Warmth) and softening its seasons.
type SeaCurrent struct{ Warmth float32 }

func (c SeaCurrent) Shape(p *Profile) {
	p.Mean += c.Warmth
	p.Year *= 0.85
}

// DrySummer is the Mediterranean's summer drought, the wet of the year in winter.
type DrySummer struct{}

func (DrySummer) Shape(p *Profile) {
	p.Wet[calendar.Summer] *= 0.2
	p.Wet[calendar.Winter] *= 1.5
}

// The zones of the world, from the equator to the pole.
var (
	// Equatorial is hot and wet all year, without seasons to speak of.
	Equatorial = Zone{Latitude: 3}
	// Tropical is hot, its summer the rainy season and its winter dry.
	Tropical = Zone{Latitude: 20}
	// Mediterranean is warm, its summer dry and its winter mild and wet.
	Mediterranean = Zone{Latitude: 38, Factors: []Factor{DrySummer{}}}
	// Temperate has four seasons, a warm summer and a winter below freezing.
	Temperate = Zone{Latitude: 55}
	// Cold has a long, hard winter and a short, cool summer.
	Cold = Zone{Latitude: 66}
	// Polar is frozen most of the year, the sun up all summer and down all winter.
	Polar = Zone{Latitude: 78}
)
