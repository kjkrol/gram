package underground

import "github.com/kjkrol/gram/plugins/world"

// Figure is a kind of structure under the ground in one piece: its Spec defined on the plugin's
// Types in the Kinds section; the plugin draws it. One with states of its own is Stateful, one
// with roles and rules of its own Ruled.
type Figure interface {
	Define(p *Plugin)
}

// Stateful is a Figure with effects of its own, defined in the Effects section.
type Stateful interface{ Effects(p *Plugin) }

// Ruled is a Figure with roles and rules of its own, defined in the Rules section.
type Ruled interface{ Rules(p *Plugin) }

// Figures notes the kinds of structures among the world's figures, which the Stage leads through
// its sections. Called in the Stage's Figures link.
func (p *Plugin) Figures(figures ...Figure) {
	all := make([]world.Figure, len(figures))
	for i, f := range figures {
		all[i] = dug{f: f, p: p}
	}
	p.worldPlugin.Figures(all...)
}

// World is the world the structures lie under.
func (p *Plugin) World() *world.Plugin { return p.worldPlugin }

// a structure as the world's figure: defined on the plugin's Types, drawn by the plugin
type dug struct {
	f Figure
	p *Plugin
}

func (d dug) Define(*world.Plugin)             { d.f.Define(d.p) }
func (d dug) Look(*world.Atlas, *world.Plugin) {}

func (d dug) Effects(*world.Plugin) {
	if s, ok := d.f.(Stateful); ok {
		s.Effects(d.p)
	}
}

func (d dug) Rules(*world.Plugin) {
	if r, ok := d.f.(Ruled); ok {
		r.Rules(d.p)
	}
}
