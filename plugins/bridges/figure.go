package bridges

import "github.com/kjkrol/gram/plugins/world"

// Figure is a kind of bridge in one piece: its Spec defined on the plugin's Types in the Kinds
// section, the looks of its deck and pier declared on the world's atlas in a scene. One with
// states of its own is Stateful, one with roles and rules of its own Ruled.
type Figure interface {
	Define(p *Plugin)
	Look(a *world.Atlas, p *Plugin)
}

// Stateful is a Figure with effects of its own, defined in the Effects section.
type Stateful interface{ Effects(p *Plugin) }

// Ruled is a Figure with roles and rules of its own, defined in the Rules section.
type Ruled interface{ Rules(p *Plugin) }

// Figures notes the kinds of bridges among the world's figures, which the Stage leads through
// its sections. Called in the Stage's Figures link.
func (p *Plugin) Figures(figures ...Figure) {
	all := make([]world.Figure, len(figures))
	for i, f := range figures {
		all[i] = built{f: f, p: p}
	}
	p.worldPlugin.Figures(all...)
}

// World is the world the bridges stand in.
func (p *Plugin) World() *world.Plugin { return p.worldPlugin }

// a bridge as the world's figure: defined on the plugin's Types, looked on the world's atlas
type built struct {
	f Figure
	p *Plugin
}

func (b built) Define(*world.Plugin)                 { b.f.Define(b.p) }
func (b built) Look(a *world.Atlas, _ *world.Plugin) { b.f.Look(a, b.p) }

func (b built) Effects(*world.Plugin) {
	if s, ok := b.f.(Stateful); ok {
		s.Effects(b.p)
	}
}

func (b built) Rules(*world.Plugin) {
	if r, ok := b.f.(Ruled); ok {
		r.Rules(b.p)
	}
}
