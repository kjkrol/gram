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

const (
	lanternPx     = 14
	lanternHeight = 24
	lanternReach  = 4 * CellSize
)

// lantern is a lamp: it lights what comes near and casts their shadows by night, casts none
// itself and takes no shade.
type lantern struct{}

var _ board.Figure = lantern{}

func (lantern) Define(b *board.Plugin) {
	lanterns := board.NewEntities(b, board.Shape{Size: lanternPx, Height: lanternHeight},
		func(r standingRow) cell.ID { return r.At })
	lanterns.Define(LanternKind,
		comp.Const(light.Source{Color: lanternColor, Strength: 1.5, Reach: lanternReach}),
		comp.Const(light.Shadow{Cast: light.NoShadow, Take: render.Shines}))
}

func (lantern) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[standingRow](w.Kinds(), LanternKind), lanternPx,
		func(dst *render.Canvas, size int) {
			s := float32(size)
			dst.FillRect(s*0.4, s*0.5, s*0.2, s*0.5, towerColor)
			dst.FillCircle(s/2, s*0.35, s*0.32, lanternGlow)
		})
}
