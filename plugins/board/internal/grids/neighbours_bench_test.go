package grids

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
)

// as grid.Grid hands them out, through an interface
type stepping interface {
	Neighbors(c cell.ID) []cell.ID
	Toward(c cell.ID, i int) (cell.ID, bool)
	Directions() int
	EachCell(fn func(c cell.ID))
}

var sink cell.ID

// every cell's neighbours on a 200x200 grid, the list against a walk over the directions
func BenchmarkGrids_Neighbours(b *testing.B) {
	for _, g := range []struct {
		name string
		grid stepping
	}{
		{"square", NewSquare(200, 200, 16)},
		{"square,torus", &Square{Width: 200, Height: 200, CellSize: 16, WrapX: true, WrapY: true}},
		{"hex", NewHex(200, 200, 16)},
	} {
		var cells []cell.ID
		g.grid.EachCell(func(c cell.ID) { cells = append(cells, c) })
		b.Run(g.name+",list", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for _, c := range cells {
					for _, n := range g.grid.Neighbors(c) {
						sink += n
					}
				}
			}
		})
		b.Run(g.name+",toward", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for _, c := range cells {
					for i := range g.grid.Directions() {
						if n, ok := g.grid.Toward(c, i); ok {
							sink += n
						}
					}
				}
			}
		})
	}
}
