package main

import (
	"image/color"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

type terrain struct {
	name  string
	kind  cell.Kind
	color color.RGBA
}

var _ board.Tile = terrain{}

var open = cell.Land | cell.Air

// the road is the kind of way the map and the tool lay over the others
var terrains = []terrain{
	{name: GrassCell, kind: cell.Kind{Pace: 0.5, Allows: open}, color: grassColor},
	{name: HillCell, kind: cell.Kind{Pace: 0.4, Allows: open}, color: hillColor},
	{
		name:  ForestCell,
		kind:  cell.Kind{Pace: 1.0 / 3, Allows: open, Veil: 0.6, Height: 8},
		color: forestColor,
	},
	{
		name:  WaterCell,
		kind:  cell.Kind{Pace: 1, Allows: cell.Water | cell.Air},
		color: waterColor,
	},
	{
		name:  FordCell,
		kind:  cell.Kind{Pace: 1.0 / 3, Allows: open | cell.Water},
		color: fordColor,
	},
	{
		name:  RoadCell,
		kind:  cell.Kind{Pace: 1, Allows: open, Graded: true},
		color: roadColor,
	},
}

func (t terrain) Define(b *board.Plugin) { b.Read(0).CellKinds().Define(t.name, t.kind) }

func (t terrain) Look(a *board.Atlas, _ *board.Plugin) { a.Add(t.name, render.Solid(t.color)) }

func tiles() []board.Tile {
	all := make([]board.Tile, len(terrains))
	for i, t := range terrains {
		all[i] = t
	}
	return all
}
