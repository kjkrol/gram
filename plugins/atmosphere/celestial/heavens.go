package celestial

// Heavens is where the bodies of the sky stand over a world, for whoever draws the sky: the ways
// to the sun and the moon (unit vectors, z up), how much of the moon's face is lit (Full, 0 new
// to 1 full), the ways the celestial sphere's equatorial axes point (Sphere, Pole) and the stars.
type Heavens struct {
	Sun, Moon [3]float32
	Full      float32
	MoonTint  [3]float32 // zero for none
	Sphere    [3][3]float32
	Pole      [3]float32
	Stars     StarSky
}

// HeavensAt is the sky over p at ofYear, the part of the year gone, and time of day t, with the
// moon moon round from new, showing stars.
func (p Place) HeavensAt(ofYear, t, moon float32, stars StarSky) Heavens {
	sphere := p.Sphere(SiderealTime(ofYear, t))
	return Heavens{
		Sun: p.SunPath(ofYear, t), Moon: p.MoonAt(ofYear, t, moon), Full: Phase(moon),
		Sphere: sphere, Pole: sphere[2], Stars: stars,
	}
}

// OnSky is the way towards the equatorial unit vector v, a Star's Dir, on the sky as h stands.
func (h Heavens) OnSky(v [3]float32) [3]float32 { return onSky(h.Sphere, v) }
