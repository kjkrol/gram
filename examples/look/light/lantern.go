package main

import (
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
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
		comp.Const(light.Shadow{Cast: light.NoShadow}))
}

// a post on a foot under a globe of glass, which shines, and a cap over it
func (lantern) Look(a *world.Atlas, w *world.Plugin) {
	a.AddBody(kind.Named[standingRow](w.Kinds(), LanternKind), lanternPx, world.Body{
		Wide: lanternPx,
		Tall: lanternHeight,
		Parts: []world.Part{
			{Shape: world.Box, To: [3]float32{0, 0, 2}, Wide: 5, Tip: 5},
			{Shape: world.Box, To: [3]float32{0, 0, 14}, Wide: 2, Tip: 2},
			{Shape: world.Spheroid, From: [3]float32{0, 0, 13}, To: [3]float32{0, 0, 21},
				Wide: 13, Paint: 1},
			{Shape: world.Cone, From: [3]float32{0, 0, 20}, To: [3]float32{0, 0, 23}, Wide: 8,
				Tip: 1},
			{Shape: world.Spheroid, From: [3]float32{0, 0, 22.5}, To: [3]float32{0, 0, 24},
				Wide: 2},
		},
		Paints: []world.Paint{
			{Colour: towerColor},
			{Colour: lanternGlow, Shines: true},
		},
	})
}
