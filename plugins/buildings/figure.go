package buildings

import "github.com/kjkrol/gram/plugins/world"

// Figure is a game's buildings in one piece: their kinds defined on the plugin's Types in the
// Kinds section. One with roles and rules of its own is Ruled.
type Figure interface{ Define(p *Plugin) }

// Ruled is a Figure with roles and rules of its own, defined in the Rules section.
type Ruled interface{ Rules(p *Plugin) }

// Figures notes the game's buildings among the world's figures, which the Stage leads through
// its sections. Called in the Stage's Figures link.
func (p *Plugin) Figures(figures ...Figure) {
	all := make([]world.Figure, len(figures))
	for i, f := range figures {
		all[i] = raised{f: f, p: p}
	}
	p.worldPlugin.Figures(all...)
}

// World is the world the buildings stand in.
func (p *Plugin) World() *world.Plugin { return p.worldPlugin }

type raised struct {
	f Figure
	p *Plugin
}

func (r raised) Define(*world.Plugin)           { r.f.Define(r.p) }
func (raised) Look(*world.Atlas, *world.Plugin) {}
func (r raised) Rules(*world.Plugin) {
	if ruled, ok := r.f.(Ruled); ok {
		ruled.Rules(r.p)
	}
}
