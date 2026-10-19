package main

import (
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	towerPx     = 28
	towerHeight = 64
)

type standingRow struct {
	At cell.ID `yaml:"at"`
}

// tower stands tall on its cell: its shadow is Baked into the ground's shade, as everything
// standing on the board casts by default, and swings round with the sun.
type tower struct{}

var _ board.Figure = tower{}

func (tower) Define(b *board.Plugin) {
	towers := board.NewEntities(b, board.Shape{Size: towerPx, Height: towerHeight},
		func(r standingRow) cell.ID { return r.At })
	towers.Define(TowerKind)
}

func (tower) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[standingRow](w.Kinds(), TowerKind), towerPx,
		func(dst *render.Canvas, size int) {
			s := float32(size)
			dst.Fill(towerColor)
			dst.FillRect(s*0.2, s*0.2, s*0.6, s*0.6, towerTopColor)
		})
}
