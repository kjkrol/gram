package bullet

import "github.com/kjkrol/gram/plugins/world"

// Figure is a kind of ammo in one piece: its Body defined on the plugin's Shots in the Kinds
// section, its look declared on the world's atlas in a scene. One with states of its own is
// Stateful, one with roles and rules of its own Ruled.
type Figure interface {
	Define(p *Plugin)
	Look(a *world.Atlas, p *Plugin)
}

// Stateful is a Figure with effects of its own, defined in the Effects section.
type Stateful interface{ Effects(p *Plugin) }

// Ruled is a Figure with roles and rules of its own, defined in the Rules section.
type Ruled interface{ Rules(p *Plugin) }

// Figures notes the kinds of ammo among the world's figures, which the Stage leads through its
// sections. Called in the Stage's Figures link.
func (p *Plugin) Figures(figures ...Figure) {
	all := make([]world.Figure, len(figures))
	for i, f := range figures {
		all[i] = loaded{f: f, p: p}
	}
	p.worldPlugin.Figures(all...)
}

// World is the world the shots fly in.
func (p *Plugin) World() *world.Plugin { return p.worldPlugin }

// an ammo as the world's figure: defined on the plugin's Shots, looked on the world's atlas
type loaded struct {
	f Figure
	p *Plugin
}

func (l loaded) Define(*world.Plugin)                 { l.f.Define(l.p) }
func (l loaded) Look(a *world.Atlas, _ *world.Plugin) { l.f.Look(a, l.p) }

func (l loaded) Effects(*world.Plugin) {
	if s, ok := l.f.(Stateful); ok {
		s.Effects(l.p)
	}
}

func (l loaded) Rules(*world.Plugin) {
	if r, ok := l.f.(Ruled); ok {
		r.Rules(l.p)
	}
}
