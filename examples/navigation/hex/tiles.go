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

var grounds = []ground{
	{name: GrassCell, kind: cell.Kind{Cost: 2, Allows: cell.Land}, color: grassColor},
	{name: WaterCell, kind: cell.Kind{Cost: 1, Allows: cell.Water}, color: waterColor},
	{name: RoadCell, kind: cell.Kind{Cost: 1, Allows: cell.Land}, color: roadColor},
}

func (g ground) Define(b *board.Plugin) { b.CellKinds().Define(g.name, g.kind) }

func (g ground) Look(a *board.Atlas, _ *board.Plugin) { a.Add(g.name, render.Hexagon(g.color)) }

func tiles() []board.Tile {
	all := make([]board.Tile, len(grounds))
	for i, g := range grounds {
		all[i] = g
	}
	return all
}
