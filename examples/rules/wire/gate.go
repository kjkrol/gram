package main

import (
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// gate is a wall keeping land off its cell until ajar.
type gate struct{}

var (
	_ board.Figure   = gate{}
	_ board.Stateful = gate{}
)

func (gate) Effects(b *board.Plugin) {
	b.World().Effects().Define(AjarEf, effect.Spec{effect.Alter(func(w *cell.Wall) { w.Keeps = 0 })})
}

func (gate) Define(b *board.Plugin) {
	board.NewEntities(b, board.Shape{Size: CellSize}, standingAt).Define(GateKind,
		comp.Const(cell.Wall{Keeps: cell.Land}))
}

func (gate) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[standingRow](w.Kinds(), GateKind), CellSize, render.Solid(gateColor)).
		Under(w.Effects().Named(AjarEf), render.Solid(gatewayColor))
}
