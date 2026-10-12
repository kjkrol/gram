package grid

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Grid abstracts a board's topology (square, hex, ...) behind neighbor,
// coordinate, and distance queries.
type Grid interface {
	Neighbors(c cell.ID) []cell.ID
	// Toward is c's neighbour in the grid's i-th direction, across a seam where the grid wraps;
	// false off the grid or past its directions.
	Toward(c cell.ID, i int) (cell.ID, bool)
	// Directions is how many directions a cell has neighbours in, Toward's i below it.
	Directions() int
	// Opposite is the direction back from the neighbour in direction i.
	Opposite(i int) int
	// Offset is direction i in At's coordinates.
	Offset(i int) (da, db int)
	// Step is the way from a cell's middle to its neighbour's in direction i, in world units.
	Step(i int) geom.Vec
	// Corner reports whether direction i leaves the cell through a corner, not across an edge.
	Corner(i int) bool
	// Flanks are the edge directions either side of corner direction i; false for an edge one.
	Flanks(i int) (a, b int, ok bool)
	CellCenter(c cell.ID) geom.Vec
	CellAt(pos geom.Vec) (cell.ID, bool)
	// At is the cell at grid coordinates (col,row or axial q,r), wrapped where the grid
	// wraps; one off a closed axis panics by its coordinates.
	At(a, b uint32) cell.ID
	// NeighborCost is the geometric step cost from a to its neighbor b.
	NeighborCost(a, b cell.ID) float64
	// DiagonalNeighbors returns the two cells flanking the corner between a and its diagonal b.
	DiagonalNeighbors(a, b cell.ID) (c1, c2 cell.ID, ok bool)
	// Distance must never overestimate the true cost — it's the pathfinding heuristic.
	Distance(a, b cell.ID) float64
	// CellSpan is the world-space side length of one cell.
	CellSpan() float32
	// CellBounds is the width and height of the rectangle round one cell.
	CellBounds() (w, h float64)
	// CellOutline appends to dst the corners of c, in order round the cell.
	CellOutline(c cell.ID, dst []geom.Vec) []geom.Vec
	// CellBoxes appends to dst boxes that together cover c, exactly or from outside.
	CellBoxes(c cell.ID, dst []geom.AABB) []geom.AABB
	// CellsUnder calls fn for every cell the box touches, each once.
	CellsUnder(box geom.AABB, fn func(c cell.ID))
	// EachCell calls fn for every cell of the grid.
	EachCell(fn func(c cell.ID))
	// Ordinal is c's index in a table with one slot per cell, below CellCount; false for no cell.
	Ordinal(c cell.ID) (int, bool)
	// Coords inverts At: c's grid coordinates (col, row or axial q, r).
	Coords(c cell.ID) (a, b uint32, ok bool)
	// CellCount is how many cells the grid has.
	CellCount() int
}
