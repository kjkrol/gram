// Package grid is a board's topology: a [Grid] answers which cells neighbour which and in which
// direction (Grid.Toward), where a cell lies and how far apart two are, which cells a box covers
// and every cell in turn. [DefaultGrids] makes a square or a hex one; the board wraps it per axis
// following its world's edges. [Link] names the direction from a cell to its neighbour as the bit
// of cell.Links a Way runs on, and [ShapeOf] tells a default grid's [Shape]: square or hex, its
// columns and rows, its cell's size, whether each axis wraps.
package grid
