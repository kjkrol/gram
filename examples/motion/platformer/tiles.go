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
	{name: SkyCell, kind: cell.Kind{Pace: 1, Allows: cell.Land}, color: skyColor},
	{name: RockCell, kind: cell.Kind{Solid: true}, color: rockColor},
	{name: LedgeCell, kind: cell.Kind{Solid: true}, color: ledgeColor},
}

func (g ground) Define(b *board.Plugin) { b.Read(0).CellKinds().Define(g.name, g.kind) }

func (g ground) Look(a *board.Atlas, _ *board.Plugin) { a.Add(g.name, render.Solid(g.color)) }

func tiles() []board.Tile {
	all := make([]board.Tile, len(grounds))
	for i, g := range grounds {
		all[i] = g
	}
	return all
}
