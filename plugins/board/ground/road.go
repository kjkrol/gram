package ground

import (
	"math"

	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// RoadSlope is the steepest a road climbs: an even ramp rises at most this much, in radians.
var RoadSlope = 10 * math.Pi / 180

// RoadFits reports whether a road running out of cell c of g as links say may lie on corners, the
// heights at c's CellOutline in its order, run the cell's side, as Transport Tycoon lays it: level,
// or an even ramp of RoadSlope at most run straight up. A road through a corner, a bend, a fork or
// an end lies level, else a diagonal would lie lower on one side than on the other.
func RoadFits(g grid.Grid, c cell.ID, corners []float32, links cell.Links, run float32) bool {
	if links == 0 {
		return true
	}
	eps := 1e-4 * run
	same := func(a, b float32) bool { return f32.Abs(a-b) <= eps }
	level := true
	for _, h := range corners[1:] {
		level = level && same(h, corners[0])
	}
	if level {
		return true
	}
	i, ok := grid.Straight(g, links)
	if !ok {
		return false
	}
	if g.Corner(i) {
		return false
	}
	a, b := grid.Rim(g, c, i)
	o, p := grid.Rim(g, c, g.Opposite(i))
	climbs := f32.Abs(corners[a]-corners[o]) <= run*float32(math.Tan(RoadSlope))+eps
	return same(corners[a], corners[b]) && same(corners[o], corners[p]) && climbs
}

// RoadJoins reports whether a road joins two cells of a grid of level cells, a hex grid, standing
// at levels a and b: only level with each other.
func RoadJoins(a, b float32) bool {
	return f32.Abs(a-b) <= 1e-6*max(1, f32.Abs(a), f32.Abs(b))
}
