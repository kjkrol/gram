package main

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// soldier is the one the player drives and shoots with, facing east as it starts.
type soldier struct{}

var _ board.Figure = soldier{}

func (soldier) Define(b *board.Plugin) {
	units(b).Define(SoldierKind, unit.Mover{Domain: cell.Land}, walking(),
		ceremony.Plays(b.World().Roles().Named(MortalRole)),
		comp.Const(world.Velocity{Dir: geom.NewVec(1, 0)}),
		comp.Const(world.Eye{Height: 16}))
}

func (soldier) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), SoldierKind), unitSize, armed(soldierColor)).
		Under(w.Effects().Named(WoundedEf), armed(paleColor)).
		Turning()
}

// authored facing east: the barrel shows where Space fires
func armed(body color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		render.Diamond(body)(dst, size)
		render.Arrow(0, 3, barrelColor)(dst, size)
	}
}
