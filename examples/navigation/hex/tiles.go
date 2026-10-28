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
	{
		name:  GrassCell,
		kind:  cell.Kind{Pace: 0.5, Allows: open},
		color: grassColor,
	},
	{
		name:  WallCell,
		kind:  cell.Kind{Pace: 1, Solid: true, Allows: cell.Air, Veil: 1, Height: 10},
		color: wallColor,
	},
	{
		name:  ForestCell,
		kind:  cell.Kind{Pace: 1.0 / 3, Allows: open, Veil: 0.6, Height: 8},
		color: forestColor,
	},
	{name: RoadCell, kind: cell.Kind{Pace: 1, Allows: open}, color: roadColor},
	{
		name:  WaterCell,
		kind:  cell.Kind{Pace: 1, Allows: cell.Water | cell.Air},
		color: waterColor,
	},
	{name: HillCell, kind: cell.Kind{Pace: 0.5, Allows: open}, color: hillColor},
	{
		name:  FordCell,
		kind:  cell.Kind{Pace: 1.0 / 3, Allows: open | cell.Water},
		color: fordColor,
	},
}

func (g ground) Define(b *board.Plugin) { b.Read(0).CellKinds().Define(g.name, g.kind) }

func (g ground) Look(a *board.Atlas, _ *board.Plugin) { a.Add(g.name, render.Hexagon(g.color)) }

func tiles() []board.Tile {
	all := make([]board.Tile, len(grounds))
	for i, g := range grounds {
		all[i] = g
	}
	return all
}
