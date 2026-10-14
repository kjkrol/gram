package main

import (
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// witch is the ice witch: wherever she stands she freezes the cells round her, the lake into
// ice, the land under frost, and walks over both.
type witch struct{}

var (
	_ board.Figure = witch{}
	_ board.Ruled  = witch{}
)

func (witch) Rules(b *board.Plugin) {
	roles, fx := b.World().Roles(), b.World().Effects()
	roles.Define(WitchRole, act.Branch[unit.Standing]{
		Name: "freeze",
		When: cond.Self.If(cond.Always),
		Then: act.OneOf(
			act.Branch[any]{
				When: cond.Around(1).If(roles.Named(LakeRole)),
				Then: act.Around(1).Put(fx.Named(IcedEf)),
			},
			act.Around(1).Put(fx.Named(FrostEf)),
		),
	})
}

func (witch) Define(b *board.Plugin) {
	roles := b.World().Roles()
	units(b).Define(WitchKind,
		unit.Mover{Domain: cell.Land | cell.Water | Frost},
		profile(unitSpeed*4),
		ordered(),
		ceremony.Plays(roles.Named(WitchRole), roles.Named(MortalRole)))
}

func (witch) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), WitchKind), unitSize, render.Diamond(witchColor)).
		Under(w.Effects().Named(FrozenEf), render.Diamond(witchFrozenColor))
}
