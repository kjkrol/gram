package grid

import "github.com/kjkrol/gram/plugins/board/internal/grids"

var (
	_ Grid = (*grids.Square)(nil)
	_ Grid = (*grids.Hex)(nil)
)

// DefaultGrids makes the grids gram ships; a board wraps them along the axes its world's edges
// wrap.
type DefaultGrids struct{}

// Square is a grid of width x height square cells cellSize wide, eight neighbours each.
func (DefaultGrids) Square(width, height, cellSize uint32) Grid {
	return grids.NewSquare(width, height, cellSize)
}

// Hex is a grid of width x height pointy-top hexagonal cells of the given size, addressed by axial
// coordinates.
func (DefaultGrids) Hex(width, height uint32, size float64) Grid {
	return grids.NewHex(width, height, size)
}
