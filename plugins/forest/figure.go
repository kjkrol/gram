package forest

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
)

// Figure is a kind of forest in one piece: its kinds of trees defined on the plugin's Types and
// its own kind in the Kinds section, its look declared on the world's atlas in a scene. One with
// states of its own is Stateful, one with roles and rules of its own, its Stands among them,
// Ruled.
type Figure interface {
	Define(p *Plugin)
	Look(a *world.Atlas, p *Plugin)
}

// Stateful is a Figure with effects of its own, defined in the Effects section.
type Stateful interface{ Effects(p *Plugin) }

// Ruled is a Figure with roles and rules of its own, defined in the Rules section.
type Ruled interface{ Rules(p *Plugin) }

// Figures notes the kinds of forests among the world's figures, which the Stage leads through
// its sections. Called in the Stage's Figures link.
func (p *Plugin) Figures(figures ...Figure) {
	all := make([]world.Figure, len(figures))
	for i, f := range figures {
		all[i] = grown{f: f, p: p}
	}
	p.worldPlugin.Figures(all...)
}

// World is the world the forests stand in.
func (p *Plugin) World() *world.Plugin { return p.worldPlugin }

// Board is the board the forests grow on.
func (p *Plugin) Board() *board.Plugin { return p.board }

// a forest as the world's figure: defined over the plugin, looked on the world's atlas
type grown struct {
	f Figure
	p *Plugin
}

func (g grown) Define(*world.Plugin)                 { g.f.Define(g.p) }
func (g grown) Look(a *world.Atlas, _ *world.Plugin) { g.f.Look(a, g.p) }

func (g grown) Effects(*world.Plugin) {
	if s, ok := g.f.(Stateful); ok {
		s.Effects(g.p)
	}
}

func (g grown) Rules(*world.Plugin) {
	if r, ok := g.f.(Ruled); ok {
		r.Rules(g.p)
	}
}
