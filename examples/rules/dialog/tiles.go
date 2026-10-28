package main

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

type meadow struct{}

var _ board.Tile = meadow{}

func (meadow) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(MeadowCell, cell.Kind{Pace: 1, Allows: cell.Land})
}

func (meadow) Look(a *board.Atlas, _ *board.Plugin) { a.Add(MeadowCell, render.Solid(meadowColor)) }
