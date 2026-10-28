package relief

import (
	"math"

	"github.com/kjkrol/gram/plugins/board/cell"
)

// Rise is how steep the step from one cell to its neighbour is: the ground's rise between their
// middles over the run, negative downhill; 0 where the run is none.
func (r *Relief) Rise(from, to cell.ID) float64 {
	if r.square {
		fx, fy, _ := r.grid.Coords(from)
		tx, ty, _ := r.grid.Coords(to)
		dx, dy := stepAxis(fx, tx, r.sq.Cols), stepAxis(fy, ty, r.sq.Rows)
		w, h := r.grid.CellBounds()
		c := r.Corners(to)
		rise := (dx*float64(c[1]-c[0]+c[3]-c[2]) + dy*float64(c[2]-c[0]+c[3]-c[1])) / 2
		run := math.Sqrt(dx*dx*float64(w)*float64(w) + dy*dy*float64(h)*float64(h))
		if run <= 0 {
			return 0
		}
		return rise / run
	}
	run := r.grid.NeighborCost(from, to) * float64(r.Step())
	if run <= 0 {
		return 0
	}
	return float64(r.Altitude(to)-r.Altitude(from)) / run
}

// -1, 0 or 1, across the seam where the grid wraps
func stepAxis(a, b, size uint32) float64 {
	switch {
	case b == a:
		return 0
	case b == a+1 || a == size-1 && b == 0:
		return 1
	default:
		return -1
	}
}
