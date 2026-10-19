package main

import "github.com/kjkrol/gram/plugins/board"

type surface struct{}

var _ board.Surface = surface{}

func (surface) Look(a *board.Atlas, _ *board.Plugin) { a.Holds(3) }
