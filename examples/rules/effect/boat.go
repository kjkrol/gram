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

// boat sails the lake, slowly, and is held fast as it freezes over
type boat struct{}

var _ board.Figure = boat{}

func (boat) Define(b *board.Plugin) {
	units(b).Define(BoatKind, unit.Mover{Domain: cell.Water}, profile(unitSpeed/4), ordered(),
		ceremony.Plays(b.Read(0).World().Roles().Named(MortalRole)))
}

func (boat) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), BoatKind), unitSize, render.Solid(boatColor)).
		Under(w.Effects().Named(FrozenEf), iced).
		Under(w.Effects().Named(HasteEf), render.Solid(boatHasteColor))
}

func iced(dst *render.Canvas, size int) {
	render.Solid(boatColor)(dst, size)
	render.Border(boatRimColor)(dst, size)
}
