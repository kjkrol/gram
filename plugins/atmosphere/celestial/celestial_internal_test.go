package celestial

import (
	"math"
	"testing"
)

// angle is the angle between the unit ways a and b, degrees.
func angle(a, b [3]float32) float64 {
	d := float64(a[0]*b[0] + a[1]*b[1] + a[2]*b[2])
	return math.Acos(math.Max(-1, math.Min(1, d))) * 180 / math.Pi
}

// equatorial is the unit vector of right ascension ra (hours) and declination dec (degrees).
func equatorial(ra, dec float64) [3]float32 {
	a, d := ra/12*math.Pi, dec*math.Pi/180
	return [3]float32{float32(math.Cos(d) * math.Cos(a)), float32(math.Cos(d) * math.Sin(a)), float32(math.Sin(d))}
}

// The catalogue holds the real night sky, brightest first: Sirius, where it stands and as bright
// as it is, Polaris beside the pole.
func TestStars_AreTheBrightStarCatalogues(t *testing.T) {
	stars := Stars()
	if len(stars) < 5000 {
		t.Fatalf("%d stars, want all to magnitude 6", len(stars))
	}
	sirius := stars[0]
	if math.Abs(float64(sirius.Mag+1.46)) > 0.01 || angle(sirius.Dir, equatorial(6.7525, -16.716)) > 0.05 {
		t.Errorf("the brightest star is %+v, want Sirius at 6h45m −16.7°, magnitude −1.46", sirius)
	}
	near := 0
	for _, s := range stars {
		if s.Mag < 2.1 && angle(s.Dir, [3]float32{0, 0, 1}) < 1 {
			near++ // Polaris, within a degree of the pole
		}
	}
	if near != 1 {
		t.Errorf("%d bright stars within a degree of the pole, want Polaris", near)
	}
}

// The sky turns round the pole: Polaris stands as high over the north as the latitude is, away
// from the noon sun, the night through; a star on the celestial equator stands on the meridian,
// over the noon's side, when the sidereal time is its right ascension.
func TestHeavensAt_TurnTheStarsRoundThePole(t *testing.T) {
	c := Place{NoonWay: South, Latitude: 45}
	polaris := equatorial(2.5302, 89.264)
	for _, hour := range []float32{0, 0.25, 0.5, 0.75} {
		h := c.HeavensAt(0.3, hour, 0.2, RealStars)
		p := h.OnSky(polaris)
		if up := math.Asin(float64(p[2])) * 180 / math.Pi; math.Abs(up-45) > 1 || p[1] > 0 {
			t.Errorf("at %v of the day Polaris stands at %v, %.1f° up; want 45° up over the north (−y)", hour, p, up)
		}
	}
	h := c.HeavensAt(0.3, 0.1, 0, RealStars)
	lst := SiderealTime(0.3, 0.1)
	star := h.OnSky(equatorial(lst*12/math.Pi, 0))
	if math.Abs(float64(star[0])) > 1e-4 || star[1] <= 0 || math.Abs(math.Asin(float64(star[2]))*180/math.Pi-45) > 0.01 {
		t.Errorf("a star of the equator at its sidereal time stands at %v, want on the meridian 45° up over the south", star)
	}
	if angle(h.Pole, h.OnSky([3]float32{0, 0, 1})) > 0.05 {
		t.Errorf("the pole %v is not where the celestial sphere's pole stands", h.Pole)
	}
}

// The moon goes its own way: full, it stands across the sky from the sun; a day later it has
// fallen behind the stars by some 13°, eastwards.
func TestHeavensAt_TheMoonDriftsAmongTheStars(t *testing.T) {
	c := Place{NoonWay: South, Latitude: 45}
	if h := c.HeavensAt(0.3, 0.5, 0.5, RealStars); angle(h.Sun, h.Moon) < 170 {
		t.Errorf("the full moon stands %.1f° from the sun, want across the sky", angle(h.Sun, h.Moon))
	}
	a, b := c.HeavensAt(0.3, 0.9, 0.1, RealStars), c.HeavensAt(0.3+1/365.25, 0.9, 0.1+1/29.53, RealStars) // a day of the Earth's year on
	// the moon's place among the stars: the moon's way turned back into the celestial frame
	among := func(h Heavens) [3]float32 {
		m := h.Moon
		return [3]float32{dot(h.Sphere[0], m), dot(h.Sphere[1], m), dot(h.Sphere[2], m)}
	}
	if d := angle(among(a), among(b)); math.Abs(d-13) > 1.5 {
		t.Errorf("in a day the moon moved %.1f° among the stars, want about 13", d)
	}
}

func dot(a, b [3]float32) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

func TestSunPath_StandsAsHighAsTheLatitudeLetsAndLongerInSummer(t *testing.T) {
	temperate := Place{NoonWay: South, Latitude: 55}
	if up := temperate.SunPath(0, 0.5)[2]; !near(up, float32(math.Sin(35*math.Pi/180))) {
		t.Errorf("at 55° at noon at the equinox the sun stands %v up, want sin 35°", up)
	}
	if summer, winter := temperate.SunPath(0.25, 0.5)[2], temperate.SunPath(0.75, 0.5)[2]; !near(summer, float32(math.Sin((35+23.44)*math.Pi/180))) || winter >= summer {
		t.Errorf("at 55° the midsummer noon sun stands %v up and the midwinter %v, want 23.44° higher in summer", summer, winter)
	}
	// at 6 in the morning: up in summer, still down in winter, just rising at the equinox
	if summer, winter, equinox := temperate.SunPath(0.25, 0.25)[2], temperate.SunPath(0.75, 0.25)[2], temperate.SunPath(0, 0.25)[2]; summer <= 0 || winter >= 0 || !near(equinox, 0) {
		t.Errorf("at 55° at 6 the sun stands %v in summer, %v in winter, %v at the equinox; want up, down and rising", summer, winter, equinox)
	}
}

func TestSunPath_PastThePolarCircleTheSunStaysUpInSummerAndDownInWinter(t *testing.T) {
	polar := Place{Latitude: 78}
	for _, hour := range []float32{0, 0.25, 0.5, 0.75} {
		if up := polar.SunPath(0.25, hour)[2]; up <= 0 {
			t.Errorf("at 78° at midsummer at %v of the day the sun stands %v, want above the horizon all day", hour, up)
		}
		if up := polar.SunPath(0.75, hour)[2]; up >= 0 {
			t.Errorf("at 78° at midwinter at %v of the day the sun stands %v, want below the horizon all day", hour, up)
		}
	}
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }
