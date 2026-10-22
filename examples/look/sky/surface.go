package main

import "github.com/kjkrol/gram/plugins/board"

type surface struct{}

var _ board.Surface = surface{}

func (surface) Look(a *board.Atlas, b *board.Plugin) {
	a.Grid(board.DefaultGridLine(b.Read(0).Board().Grid))
	a.Holds(board.DefaultHoldWidth)
}
