package main

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

type ground struct {
	name string
	kind cell.Kind
}

var _ board.Tile = ground{}

var dry = cell.Land | cell.Air

var grounds = []ground{
	{name: WaterCell, kind: cell.Kind{Cost: 1, Allows: cell.Water | cell.Air}},
	{name: SandCell, kind: cell.Kind{Cost: 3, Allows: dry}},
	{name: EarthCell, kind: cell.Kind{Cost: 2, Allows: dry}},
	{name: ForestCell, kind: cell.Kind{Cost: 6, Allows: dry, Veil: 0.6, Height: scale.Units(20)}},
	{name: RockCell, kind: cell.Kind{Cost: 4, Allows: dry}},
}

func (g ground) Define(b *board.Plugin) { b.CellKinds().Define(g.name, g.kind) }

func (g ground) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(g.name, render.Solid(cellColors[g.name]))
}

func tiles() []board.Tile {
	all := make([]board.Tile, len(grounds))
	for i, g := range grounds {
		all[i] = g
	}
	return all
}
