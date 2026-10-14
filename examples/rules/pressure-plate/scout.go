package main

import (
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// scout is the player's unit, sent about by a right click
type scout struct{}

var _ board.Figure = scout{}

func (scout) Define(b *board.Plugin) {
	units(b).Define(ScoutKind, unit.Mover{Domain: cell.Land}, profile,
		ceremony.Plays(b.World().Roles().Named(MortalRole)))
}

func (scout) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), ScoutKind), unitSize, render.Solid(scoutColor))
}
