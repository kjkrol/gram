package main

import (
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type lanternRow struct {
	At cell.ID `yaml:"at"`
}

// lantern is a lamp standing at a bridge's head: it shines warm as far as it reaches, casts no
// shadow and takes none.
type lantern struct{}

var _ board.Figure = lantern{}

func (lantern) Define(b *board.Plugin) {
	lanterns := board.NewEntities(b, board.Shape{Size: scale.Figure(1), Height: scale.Figure(4)},
		func(r lanternRow) cell.ID { return r.At })
	lanterns.Define(LanternKind,
		comp.Const(light.Source{Color: lanternColor, Strength: 1.5,
			Reach: float32(scale.Figure(10))}),
		comp.Const(light.Shadow{Cast: light.NoShadow, Take: render.Shines}))
}

func (lantern) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[lanternRow](w.Kinds(), LanternKind), 8, func(dst *render.Canvas, size int) {
		s := float32(size)
		dst.FillRect(s*0.4, s*0.5, s*0.2, s*0.5, pierColor)
		dst.FillCircle(s/2, s*0.3, s*0.3, lanternGlow)
	})
}
