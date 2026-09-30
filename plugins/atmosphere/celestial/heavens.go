package celestial

// Heavens is where the bodies of the sky stand over a world: the way to the sun and to the moon
// (unit vectors, z up), how much of the moon's face is lit (Full: 0 new, 1 full), how the celestial
// sphere stands (Sphere: the ways its equatorial frame's axes point — right ascension 0h and 6h on
// the celestial equator and the north celestial pole, Pole — so a star's equatorial unit vector v
// stands at v[0]·Sphere[0] + v[1]·Sphere[1] + v[2]·Sphere[2]) and which stars it shows (Stars) —
// for whoever draws the sky.
type Heavens struct {
	Sun, Moon [3]float32
	Full      float32
	Sphere    [3][3]float32
	Pole      [3]float32
	Stars     StarSky
}

// HeavensAt is the sky over p at ofYear, the part of the year gone, and time of day t, with the
// moon moon round from new, showing stars: the sun along its path, the moon along its own
// (MoonAt), the sphere turning once a day and once more a year round the pole, which stands as high
// over the north as the latitude is.
func (p Place) HeavensAt(ofYear, t, moon float32, stars StarSky) Heavens {
	sphere := p.Sphere(SiderealTime(ofYear, t))
	return Heavens{
		Sun: p.SunPath(ofYear, t), Moon: p.MoonAt(ofYear, t, moon), Full: Phase(moon),
		Sphere: sphere, Pole: sphere[2], Stars: stars,
	}
}

// OnSky is the way towards the equatorial unit vector v — a Star's Dir — on the sky as h stands.
func (h Heavens) OnSky(v [3]float32) [3]float32 { return onSky(h.Sphere, v) }
