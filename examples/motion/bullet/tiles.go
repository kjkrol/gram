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
	{name: GrassCell, kind: cell.Kind{Pace: 0.5, Allows: cell.Land}, color: grassColor},
	{name: RoadCell, kind: cell.Kind{Pace: 1, Allows: cell.Land}, color: roadColor},
	{name: WaterCell, kind: cell.Kind{Pace: 1, Allows: cell.Water}, color: waterColor},
	{name: WallCell, kind: cell.Kind{Pace: 1, Solid: true, Height: 30}, color: wallColor},
	{name: LowWallCell, kind: cell.Kind{Pace: 1, Solid: true, Height: 10}, color: lowWallColor},
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
