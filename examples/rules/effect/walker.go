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

// walker is a man on the land: he drowns in the water, slides on the ice, freezes fallen in
type walker struct{}

var _ board.Figure = walker{}

func (walker) Define(b *board.Plugin) {
	units(b).Define(WalkerKind, unit.Mover{Domain: cell.Land}, profile(unitSpeed*4),
		ceremony.Plays(b.Read(0).World().Roles().Named(MortalRole)))
}

func (walker) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), WalkerKind), unitSize, render.Solid(walkerColor)).
		Under(w.Effects().Named(FrozenEf), render.Solid(walkerFrozenColor)).
		Under(w.Effects().Named(HasteEf), render.Solid(walkerHasteColor))
}
