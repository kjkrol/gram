package unit

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// Footprint is how many cells a side the box of size of a unit on g takes: one for a box no
// bigger than a cell; past it the fewest, two at least, whose side holds its longer side with
// room to spare, slack for collision. On a grid no direction of which leaves through a corner, a
// hex one, an even side has no window and is rounded up to an odd one.
func Footprint(g grid.Grid, size geom.Vec) int {
	side, span := max(size.X, size.Y), g.CellSpan()
	if span <= 0 || side <= span {
		return 1
	}
	cells := 2
	for side >= float32(cells)*span {
		cells++
	}
	if cells%2 == 0 && !cornered(g) {
		cells++
	}
	return cells
}

// cornered says whether a direction of g leaves its cells through a corner, as a square grid's do
func cornered(g grid.Grid) bool {
	for i := range g.Directions() {
		if g.Corner(i) {
			return true
		}
	}
	return false
}

// Offset is how far off the middle of the cell its footprint is laid from a unit of a footprint
// cells a side stands: half a cell right and down for an even side, on the corner its middle
// cells share; none for an odd one, whose footprint is laid round its cell.
func Offset(g grid.Grid, cells int) geom.Vec {
	if cells%2 == 1 {
		return geom.Vec{}
	}
	half := g.CellSpan() / 2
	return geom.NewVec(half, half)
}

// Place is where the middle of a unit of a box of size stands with its footprint laid from c:
// off c's middle by its Offset, a place to spawn it at.
func Place(g grid.Grid, size geom.Vec, c cell.ID) geom.Vec {
	m, off := g.CellCenter(c), Offset(g, Footprint(g, size))
	return geom.NewVec(m.X+off.X, m.Y+off.Y)
}

// LaidFrom is a point of the cell the footprint of a unit is laid from, its box of size and its
// middle at p: p back by its Offset.
func LaidFrom(g grid.Grid, size, p geom.Vec) geom.Vec {
	off := Offset(g, Footprint(g, size))
	return geom.NewVec(p.X-off.X, p.Y-off.Y)
}
