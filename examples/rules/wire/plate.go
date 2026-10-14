package main

import (
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const plateSize = 20

// plate is a pressure plate: a sensor stood on, never pushed, playing the role that triggers.
type plate struct{}

var (
	_ board.Figure = plate{}
	_ board.Ruled  = plate{}
)

// stood on, a plate triggers: the command naming it says what that opens
func (plate) Rules(b *board.Plugin) {
	b.World().Roles().Define(PlateRole, act.Branch[collision.Meeting]{
		Name: "press",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(ceremony.Trigger{}),
	})
}

func (plate) Define(b *board.Plugin) {
	board.NewEntities(b, board.Shape{Size: plateSize}, standingAt).Define(PlateKind,
		comp.Const(collision.Collider{}),
		ceremony.Plays(b.World().Roles().Named(PlateRole)))
}

func (plate) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[standingRow](w.Kinds(), PlateKind), plateSize, render.Solid(plateColor))
}
