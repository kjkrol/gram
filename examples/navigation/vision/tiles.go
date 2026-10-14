package main

import (
	"image/color"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

type ground struct {
	name  string
	kind  cell.Kind
	color color.RGBA
}

var _ board.Tile = ground{}

var open = cell.Land | cell.Air

var grounds = []ground{
	{name: GrassCell, kind: cell.Kind{Cost: 2, Allows: open}.Costing(cell.Air, 1), color: grassColor},
	{
		name:  WallCell,
		kind:  cell.Kind{Cost: 1, Solid: true, Allows: cell.Air, Veil: 1, Height: 10},
		color: wallColor,
	},
	{
		name:  ForestCell,
		kind:  cell.Kind{Cost: 3, Allows: open, Veil: 0.6, Height: 8}.Costing(cell.Air, 1),
		color: forestColor,
	},
	{name: RoadCell, kind: cell.Kind{Cost: 1, Allows: open}, color: roadColor},
	{name: HillCell, kind: cell.Kind{Cost: 2, Allows: open}.Costing(cell.Air, 1), color: hillColor},
}

func (g ground) Define(b *board.Plugin) { b.CellKinds().Define(g.name, g.kind) }

func (g ground) Look(a *board.Atlas, _ *board.Plugin) { a.Add(g.name, render.Solid(g.color)) }

func tiles() []board.Tile {
	all := make([]board.Tile, len(grounds))
	for i, g := range grounds {
		all[i] = g
	}
	return all
}
