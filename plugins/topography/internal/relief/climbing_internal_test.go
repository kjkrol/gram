package relief

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	public "github.com/kjkrol/gram/plugins/topography/relief"
)

// reliefOf is a level relief over a cols x rows square grid of cells size wide.
func reliefOf(cols, rows, size uint32) (*Relief, grid.Grid) {
	grid := grid.DefaultGrids{}.Square(cols, rows, size)
	return New(board.NewBoard(grid)), grid
}

// On a ramp rising 1 in 2 eastward a step is priced by the slope of the cell it enters, the way
// it goes: up, down, across and slantwise.
func TestClimb_ReadsTheSlopeOffTheCorners(t *testing.T) {
	r, grid := reliefOf(3, 3, 10)
	r.SetHeights(func(p geom.Vec) float64 { return 0.5 * p.X })
	at := func(x, y uint32) cell.ID { c := grid.CellIndex(x, y); return c }
	mid := at(1, 1)
	for _, c := range []struct {
		name string
		to   cell.ID
		d    cell.Domain
		want float64
	}{
		{"up", at(2, 1), cell.Land, 1 + 10*0.5},
		{"down", at(0, 1), cell.Land, 1 - 0.3 + 5*(0.5-0.1)},
		{"across", at(1, 2), cell.Land, 1},
		{"slantwise up", at(2, 2), cell.Land, 1 + 10*5/math.Sqrt(200)},
		{"flying up", at(2, 1), cell.Air, 1},
		{"up on foot or wing", at(2, 1), cell.Land | cell.Air, 1},
	} {
		if got := r.Climb(mid, c.to, c.d, public.DefaultClimbing); math.Abs(got-c.want) > 1e-6 {
			t.Errorf("%s: climb %v, want %v", c.name, got, c.want)
		}
	}
}

// The corners are a lattice: raising one cell's raises its neighbours' where they meet it.
func TestSetCorners_LeavesNoVerticalWall(t *testing.T) {
	r, grid := reliefOf(3, 3, 10)
	at := func(x, y uint32) cell.ID { c := grid.CellIndex(x, y); return c }
	r.SetCorners(at(1, 1), Corners{1, 2, 3, 4})
	for _, c := range []struct {
		cell cell.ID
		want Corners
	}{
		{at(0, 0), Corners{0, 0, 0, 1}},
		{at(1, 0), Corners{0, 0, 1, 2}},
		{at(2, 1), Corners{2, 0, 4, 0}},
		{at(2, 2), Corners{4, 0, 0, 0}},
	} {
		if got := r.Corners(c.cell); got != c.want {
			t.Errorf("cell %v's corners %v, want %v", c.cell, got, c.want)
		}
	}
}
