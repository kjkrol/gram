package main

import (
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/backdrop"
)

type earth struct{}

var _ atmosphere.Sky = earth{}

func (earth) Look(d *backdrop.Dome, _ *atmosphere.Plugin) {
	d.Vault = backdrop.EarthVault()
	d.Sun = backdrop.EarthSun()
	d.Moon = backdrop.EarthMoon()
	d.Stars = backdrop.EarthStars()
}
