package main

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
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
	// square to the board
	towers.Define(TowerKind, comp.Const(world.Velocity{Dir: geom.NewVec(1, 0)}))
}

// a shaft of stone under a cornice with a merlon on each corner, a storey over it
func (tower) Look(a *world.Atlas, w *world.Plugin) {
	parts := []world.Part{
		{Shape: world.Box, To: [3]float32{0, 0, 50}, Wide: 22, Tip: 22},
		{Shape: world.Box, From: [3]float32{0, 0, 50}, To: [3]float32{0, 0, 54}, Wide: 27,
			Tip: 27},
		{Shape: world.Box, From: [3]float32{0, 0, 54}, To: [3]float32{0, 0, 64}, Wide: 15,
			Tip: 15, Paint: 1},
	}
	for _, corner := range [][2]float32{{-11, -11}, {11, -11}, {-11, 11}, {11, 11}} {
		parts = append(parts, world.Part{Shape: world.Box,
			From: [3]float32{corner[0], corner[1], 54}, To: [3]float32{corner[0], corner[1], 58},
			Wide: 5, Tip: 5})
	}
	a.AddBody(kind.Named[standingRow](w.Kinds(), TowerKind), towerPx, world.Body{
		Wide:  towerPx,
		Tall:  towerHeight,
		Parts: parts,
		Paints: []world.Paint{
			{Colour: towerColor, Dots: towerStoneColor},
			{Colour: towerTopColor},
		},
	})
}
