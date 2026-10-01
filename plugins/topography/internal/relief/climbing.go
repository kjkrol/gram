package relief

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	public "github.com/kjkrol/gram/plugins/topography/relief"
)

// over the way between their middles.
func (r *Relief) Climb(from, to cell.ID, d cell.Domain, climbing public.Climbing) float64 {
	if !climbing.Feels(d) {
		return 1
	}
	if r.square {
		fx, fy, _ := r.grid.Coords(from)
		tx, ty, _ := r.grid.Coords(to)
		dx, dy := stepAxis(fx, tx, r.sq.Cols), stepAxis(fy, ty, r.sq.Rows)
		w, h := r.grid.CellBounds()
		c := r.Corners(to)
		rise := (dx*float64(c[1]-c[0]+c[3]-c[2]) + dy*float64(c[2]-c[0]+c[3]-c[1])) / 2
		run := math.Hypot(dx*w, dy*h)
		if run <= 0 {
			return 1
		}
		return climbing.Factor(rise / run)
	}
	run := r.grid.NeighborCost(from, to) * r.Step()
	if run <= 0 {
		return 1
	}
	return climbing.Factor((r.Altitude(to) - r.Altitude(from)) / run)
}

// stepAxis is the step from a to its neighbour b along an axis size long, -1, 0 or 1, across the
// seam where the grid wraps.
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

// SlopeAt is the ground's slope at p going dir (of length 1): on a square grid the rise of the
// cell's ground under p that way, between its corners; on a hex grid the rise across a cell's step
// round p, over that step.
func (r *Relief) SlopeAt(p, dir geom.Vec) float64 {
	if r.square {
		c, ok := r.grid.CellAt(p)
		if !ok {
			return 0
		}
		w, h := r.grid.CellBounds()
		x, y, _ := r.grid.Coords(c)
		u, v := p.X/w-float64(x), p.Y/h-float64(y)
		u, v = u-math.Floor(u), v-math.Floor(v) // a wrapped p lies a whole board away
		k := r.Corners(c)
		gx := ((1-v)*float64(k[1]-k[0]) + v*float64(k[3]-k[2])) / w
		gy := ((1-u)*float64(k[2]-k[0]) + u*float64(k[3]-k[1])) / h
		return gx*dir.X + gy*dir.Y
	}
	half := r.Step() / 2
	if half <= 0 {
		return 0
	}
	ahead := r.GroundAt(geom.NewVec(p.X+dir.X*half, p.Y+dir.Y*half))
	behind := r.GroundAt(geom.NewVec(p.X-dir.X*half, p.Y-dir.Y*half))
	return (ahead - behind) / (2 * half)
}
