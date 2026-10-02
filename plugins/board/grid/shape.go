package grid

import "github.com/kjkrol/gram/plugins/board/internal/grids"

// Shape is the shape of a grid of DefaultGrids: square or hex, how many columns and rows of cells,
// how large — a square's side, a hex's size — and whether each axis wraps.
type Shape struct {
	Hex          bool
	Cols, Rows   uint32
	Cell         float64
	WrapX, WrapY bool
}

// ShapeOf is g's shape when it is one of DefaultGrids; false for any other.
func ShapeOf(g Grid) (Shape, bool) {
	switch gr := g.(type) {
	case *grids.Square:
		return Shape{Cols: gr.Width, Rows: gr.Height, Cell: float64(gr.CellSize), WrapX: gr.WrapX, WrapY: gr.WrapY}, true
	case *grids.Hex:
		return Shape{Hex: true, Cols: gr.Width, Rows: gr.Height, Cell: gr.Size, WrapX: gr.WrapX, WrapY: gr.WrapY}, true
	}
	return Shape{}, false
}
