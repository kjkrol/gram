package main

import (
	_ "embed"

	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/backdrop"
	"github.com/kjkrol/gram/render"
)

//go:embed vault.wgsl
var vault []byte

type dusk struct{}

var _ atmosphere.Sky = dusk{}

func (dusk) Look(d *backdrop.Dome, _ *atmosphere.Plugin) {
	d.Vault = backdrop.Part{
		Source: vault,
		Reads:  []render.Uniform{{Name: VaultNightUniform, Size: 3}},
		Feed: func(v *backdrop.Values) {
			// the night's violet the brighter the fuller the moon
			full := v.Heavens().Full
			v.Set(VaultNightUniform, 0.1+0.06*full, 0.04+0.03*full, 0.22+0.1*full)
		},
	}
	d.Moon.Size = 3
	d.Moon.Color = render.Light{1, 0.62, 0.48}
	d.Moon.ByDay = 0.8
	d.Stars = backdrop.Stars{Dots: backdrop.ScatteredStars()}
}
