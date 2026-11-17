package main

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

type grass struct{}

var _ board.Tile = grass{}

func (grass) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(GrassCell, cell.Kind{Pace: 1, Allows: cell.Land})
}

func (grass) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(GrassCell, func(dst *render.Canvas, size int) {
		dst.Fill(grassColor)
		for i := 0; i < size; i += 7 {
			dst.FillRect(float32(i), float32((i*5)%size), 2, 4, bladeColor)
		}
	})
}

type street struct{}

var _ board.Tile = street{}

func (street) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(StreetCell, cell.Kind{Pace: 1, Allows: cell.Land, Graded: true})
}

func (street) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(StreetCell, func(dst *render.Canvas, size int) {
		dst.Fill(cobbleColor)
		for i := 0; i < size; i += 8 {
			dst.FillRect(float32(i), float32((i*3)%size), 5, 1, jointColor)
		}
	})
}

// built is the ground a building stands on: nobody enters it
type built struct{}

var _ board.Tile = built{}

func (built) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(BuiltCell, cell.Kind{Solid: true})
}

func (built) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(BuiltCell, func(dst *render.Canvas, _ int) { dst.Fill(yardColor) })
}
