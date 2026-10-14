package board

import "github.com/kjkrol/gram/plugins/world"

// Figure is a kind of entity standing on the board in one piece: what it is, defined over the
// board in the Kinds section (NewUnits, NewEntities), and how it looks, declared on the world's
// atlas in a scene as any figure's. One with states of its own is Stateful, one with roles and
// rules of its own Ruled.
type Figure interface {
	Define(b *Plugin)
	Look(a *world.Atlas, w *world.Plugin)
}

// Stateful is a Figure or a Tile with effects of its own, defined in the Effects section.
type Stateful interface{ Effects(b *Plugin) }

// Ruled is a Figure or a Tile with roles and rules of its own, defined in the Rules section.
type Ruled interface{ Rules(b *Plugin) }

// Tile is a kind of cell in one piece: what it is, defined in the Kinds section, and how it
// looks and what covers it under an effect, declared on the board's atlas in a scene. Stateful
// and Ruled as a Figure is.
type Tile interface {
	Define(b *Plugin)
	Look(a *Atlas, b *Plugin)
}

// Figures notes the figures standing on the board among the world's, which the Stage leads
// through its sections: defined over the board, looked on the world's atlas. Called in the
// Stage's Figures link.
func (p *Plugin) Figures(figures ...Figure) {
	all := make([]world.Figure, len(figures))
	for i, f := range figures {
		all[i] = standing{f: f, b: p}
	}
	p.worldPlugin.Figures(all...)
}

// Tiles notes the tiles among the world's figures, which the Stage leads through its sections,
// and keeps them for the board atlas's Looks. Called in the Stage's Figures link.
func (p *Plugin) Tiles(tiles ...Tile) {
	all := make([]world.Figure, len(tiles))
	for i, t := range tiles {
		all[i] = laid{t: t, b: p}
	}
	p.worldPlugin.Figures(all...)
	p.tiles = append(p.tiles, tiles...)
}

// Looks declares every tile's look on the atlas, in the order the tiles were noted.
func (a *Atlas) Looks() {
	for _, t := range a.p.tiles {
		t.Look(a, a.p)
	}
}

// a board's figure as the world's: defined over the board, looked as any
type standing struct {
	f Figure
	b *Plugin
}

func (s standing) Define(*world.Plugin)                 { s.f.Define(s.b) }
func (s standing) Look(a *world.Atlas, w *world.Plugin) { s.f.Look(a, w) }
func (s standing) Effects(*world.Plugin)                { effects(s.f, s.b) }
func (s standing) Rules(*world.Plugin)                  { rules(s.f, s.b) }

// a tile as the world's figure: defined over the board, drawn on the board's atlas alone
type laid struct {
	t Tile
	b *Plugin
}

func (l laid) Define(*world.Plugin)           { l.t.Define(l.b) }
func (laid) Look(*world.Atlas, *world.Plugin) {}
func (l laid) Effects(*world.Plugin)          { effects(l.t, l.b) }
func (l laid) Rules(*world.Plugin)            { rules(l.t, l.b) }

func effects(of any, b *Plugin) {
	if s, ok := of.(Stateful); ok {
		s.Effects(b)
	}
}

func rules(of any, b *Plugin) {
	if r, ok := of.(Ruled); ok {
		r.Rules(b)
	}
}
