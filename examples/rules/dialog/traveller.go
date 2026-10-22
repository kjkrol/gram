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

// traveller is the player's unit, walked up to the hosts; the hosts talk to whoever plays its role
type traveller struct{}

var (
	_ board.Figure = traveller{}
	_ board.Ruled  = traveller{}
)

func (traveller) Rules(b *board.Plugin) { b.Read(0).World().Roles().Define(TravellerRole) }

func (traveller) Define(b *board.Plugin) {
	units(b).Define(TravellerKind, unit.Mover{Domain: cell.Land}, walks,
		ceremony.Plays(b.Read(0).World().Roles().Named(TravellerRole)))
}

func (traveller) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), TravellerKind), unitSize,
		render.Diamond(travellerColor))
}
