package atmosphere

import (
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/atmosphere/backdrop"
)

// Sky is how a game's sky looks: its Look sets the parts of the dome it draws its own way, the
// Earth's standing in the rest.
type Sky interface {
	Look(d *backdrop.Dome, p *Plugin)
}

// Skies notes the skies the atmosphere draws, each Look in the order noted, a later part taking
// the place of an earlier; none is the Earth's. Called in the Stage's Figures link.
func (p *Plugin) Skies(skies ...Sky) {
	if err := p.worldPlugin.InSection("skies noted", section.Plugins); err != nil {
		panic("atmosphere: " + err.Error())
	}
	p.skies = append(p.skies, skies...)
}

func (p *Plugin) dress() {
	if len(p.skies) == 0 {
		return
	}
	d := backdrop.Earth()
	for _, s := range p.skies {
		s.Look(&d, p)
	}
	p.vault.Dress(d)
}
