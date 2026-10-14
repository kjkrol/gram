package main

import (
	"time"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// trapdoor is boards over a pit: a floor carrying land until opened, shut again a second later
type trapdoor struct{}

var (
	_ board.Figure   = trapdoor{}
	_ board.Stateful = trapdoor{}
)

func (trapdoor) Effects(b *board.Plugin) {
	b.World().Effects().Define(OpenEf, effect.Spec{
		effect.Lasts(time.Second),
		effect.Alter(func(f *cell.Floor) { f.Allows = 0 }),
	})
}

func (trapdoor) Define(b *board.Plugin) {
	board.NewEntities(b, board.Shape{Size: CellSize}, standingAt).Define(TrapdoorKind,
		comp.Const(cell.Floor{Allows: cell.Land}))
}

func (trapdoor) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[standingRow](w.Kinds(), TrapdoorKind), CellSize, render.Solid(boardsColor)).
		Under(w.Effects().Named(OpenEf), render.Solid(pitColor))
}
