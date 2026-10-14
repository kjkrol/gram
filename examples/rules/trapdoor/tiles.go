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

// pit holds nobody: a hole, unless a trapdoor lies over it
type pit struct{}

var _ board.Tile = pit{}

func (pit) Define(b *board.Plugin) { b.CellKinds().Define(PitCell, cell.Kind{Cost: 1}) }

func (pit) Look(a *board.Atlas, _ *board.Plugin) { a.Add(PitCell, render.Solid(pitColor)) }
