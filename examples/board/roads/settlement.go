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

const settlementPx = 28

type settlementRow struct {
	At cell.ID `yaml:"at"`
}

// settlement is a village standing on its cell, lying flat on the ground: where the roads run
// from and to
type settlement struct{}

var _ board.Figure = settlement{}

func (settlement) Define(b *board.Plugin) {
	villages := board.NewEntities(b, board.Shape{Size: settlementPx},
		func(r settlementRow) cell.ID { return r.At })
	villages.Define(SettlementKind, comp.Const(light.Shadow{Cast: light.NoShadow}))
}

func (settlement) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[settlementRow](w.Kinds(), SettlementKind), settlementPx, village).Lying()
}

// the houses round the green, in fractions of the sprite: left, top, width, depth
var houses = [][4]float32{
	{0.06, 0.08, 0.32, 0.2},
	{0.56, 0.06, 0.34, 0.22},
	{0.74, 0.36, 0.2, 0.26},
	{0.06, 0.4, 0.2, 0.24},
	{0.08, 0.7, 0.3, 0.22},
	{0.6, 0.7, 0.32, 0.22},
}

func village(dst *render.Canvas, size int) {
	s := float32(size)
	dst.FillCircle(s/2, s/2, s*0.2, greenColor)
	dst.FillCircle(s/2, s/2, s*0.06, wellColor)
	for _, h := range houses {
		x, y, w, d := h[0]*s, h[1]*s, h[2]*s, h[3]*s
		if w >= d {
			dst.FillRect(x, y, w, d/2, roofColor)
			dst.FillRect(x, y+d/2, w, d/2, roofShade)
		} else {
			dst.FillRect(x, y, w/2, d, roofColor)
			dst.FillRect(x+w/2, y, w/2, d, roofShade)
		}
	}
}
