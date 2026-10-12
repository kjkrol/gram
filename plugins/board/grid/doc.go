// Package grid is a board's topology: a [Grid] answers which cells neighbour which and in which
// direction (Grid.Toward), where a cell lies and how far apart two are, which cells a box covers
// and every cell in turn. [DefaultGrids] makes a square or a hex one; the board wraps it per axis
// following its world's edges. A grid knows its directions — how many (Grid.Directions), each one's
// opposite, its offset in the grid's coordinates, its step in the world, whether it leaves a cell
// through a corner and which edges flank that corner — and nothing outside it spells them out:
// [Way] and [Link] name the direction to a neighbour (Link as the bit of cell.Links a Way runs
// on), [Direction] and [Nearest] the one of an offset or a vector, [Exit] and [Rim] where it
// leaves a cell. [ShapeOf] tells a default grid's [Shape]: square or hex, its columns and rows,
// its cell's size, whether each axis wraps.
package grid
