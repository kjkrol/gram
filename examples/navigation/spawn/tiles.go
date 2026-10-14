package main

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

type grass struct{}

var _ board.Tile = grass{}

func (grass) Define(b *board.Plugin) {
	b.CellKinds().Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

func (grass) Look(a *board.Atlas, _ *board.Plugin) { a.Add(GrassCell, render.Solid(grassColor)) }

type wall struct{}

var _ board.Tile = wall{}

func (wall) Define(b *board.Plugin) {
	b.CellKinds().Define(WallCell, cell.Kind{Cost: 1, Solid: true})
}

func (wall) Look(a *board.Atlas, _ *board.Plugin) { a.Add(WallCell, render.Solid(wallColor)) }
