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
	ajar := effect.Alter(func(w *cell.Wall) { w.Keeps = 0 })
	b.Read(0).World().Effects().Define(AjarEf, effect.Spec{ajar})
}

func (gate) Define(b *board.Plugin) {
	board.NewEntities(b, board.Shape{Size: CellSize}, standingAt).Define(GateKind,
		comp.Const(cell.Wall{Keeps: cell.Land}),
		// its top spans the gateway over the hero's head: he passes under it
		comp.Const(world.Z{Altitude: heroHeight, Height: CellSize / 4}))
}

func (gate) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[standingRow](w.Kinds(), GateKind), CellSize, render.Solid(gateColor)).
		Under(w.Effects().Named(AjarEf), render.Solid(gatewayColor))
}
