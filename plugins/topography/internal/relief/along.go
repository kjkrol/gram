package relief

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/ground"
)

var _ ground.Profile = (*Relief)(nil)

// Along calls visit with every point of the ray from origin along the unit dir, up to length,
// where the ground's slope changes, and with length itself: the ground.Profile contract.
func (r *Relief) Along(origin, dir geom.Vec, length float32, visit func(d, alt float32) bool) {
	if !r.square {
		r.hexAlong(origin, dir, length, visit)
		return
	}
	if r.sq.Cell == 0 {
		step := r.Step()
		for d := step; d < length; d += step {
			if !visit(d, r.GroundAt(geom.NewVec(origin.X+dir.X*d, origin.Y+dir.Y*d))) {
				return
			}
		}
		visit(length, r.GroundAt(geom.NewVec(origin.X+dir.X*length, origin.Y+dir.Y*length)))
		return
	}
	// the walk sums t cell after cell, so in float64
	size, end := float64(r.sq.Cell), float64(length)
	fx, fy := float64(origin.X)/size, float64(origin.Y)/size
	cx, cy := math.Floor(fx), math.Floor(fy)
	bx, by := float64(dir.X)/size, float64(dir.Y)/size // cells a unit along the ray
	stepX, nextX, deltaX := lattice(fx, bx)
	stepY, nextY, deltaY := lattice(fy, by)
	h := r.cellCorners(int64(cx), int64(cy))
	for t := 0.0; ; {
		ax, ay := fx-cx, fy-cy
		exit := min(nextX, nextY)
		if fold, alt, ok := folded(h, ax, ay, bx, by); ok && fold > t && fold < min(exit, end) {
			if !visit(float32(fold), float32(alt)) {
				return
			}
		}
		if exit >= end {
			visit(length, r.GroundAt(geom.NewVec(origin.X+dir.X*length, origin.Y+dir.Y*length)))
			return
		}
		u := min(max(ax+bx*exit, 0), 1)
		v := min(max(ay+by*exit, 0), 1)
		var alt float64
		switch {
		case nextX <= nextY && stepX > 0:
			alt = h[1] + v*(h[3]-h[1])
		case nextX <= nextY:
			alt = h[0] + v*(h[2]-h[0])
		case stepY > 0:
			alt = h[2] + u*(h[3]-h[2])
		default:
			alt = h[0] + u*(h[1]-h[0])
		}
		if !visit(float32(exit), float32(alt)) {
			return
		}
		acrossX := nextX <= nextY
		if acrossX {
			cx += stepX
			nextX += deltaX
		} else {
			cy += stepY
			nextY += deltaY
		}
		// off the board's edge the ground drops to 0: the step down, or up, is a second point
		h = r.cellCorners(int64(cx), int64(cy))
		if entry := entered(h, acrossX, stepX, stepY, u, v); entry != alt &&
			!visit(float32(exit), float32(entry)) {
			return
		}
		t = exit
	}
}

// the ray crosses from triangle to triangle where it crosses a line of the lattice (onLattice):
// i, j or i + j whole
func (r *Relief) hexAlong(origin, dir geom.Vec, length float32, visit func(d, alt float32) bool) {
	end := float64(length)
	if r.sq.Cell == 0 {
		visit(length, 0)
		return
	}
	s := float64(r.sq.Cell)
	fi, fj := r.onLattice(origin)
	bi := 2 * float64(dir.X) / (math.Sqrt(3) * s) // the lattice a unit along the ray
	bj := float64(dir.Y)/s - bi/2
	_, nextI, deltaI := lattice(fi, bi)
	_, nextJ, deltaJ := lattice(fj, bj)
	_, nextS, deltaS := lattice(fi+fj, bi+bj)
	last, walked := 0.0, false // the ground where the stretch before ended
	for t := 0.0; t < end; {
		next := min(nextI, nextJ, nextS, end)
		switch next {
		case nextI:
			nextI += deltaI
		case nextJ:
			nextJ += deltaJ
		case nextS:
			nextS += deltaS
		}
		if next <= t {
			continue // a corner, the lines through it crossed at once
		}
		// the stretch from t to next lies in one triangle, the one its middle lies in
		m := (t + next) / 2
		entry, exit := 0.0, 0.0
		if f, ok := r.facet(fi+bi*m, fj+bj*m); ok {
			entry, exit = f.at(fi+bi*t, fj+bj*t), f.at(fi+bi*next, fj+bj*next)
		}
		// off the board's edge the ground drops to 0: the step down, or up, is a second point
		if walked && math.Abs(entry-last) > 1e-9*max(1, math.Abs(last)) &&
			!visit(float32(t), float32(entry)) {
			return
		}
		if next < end && !visit(float32(next), float32(exit)) {
			return
		}
		last, walked, t = exit, true, next
	}
	visit(length, r.GroundAt(geom.NewVec(origin.X+dir.X*length, origin.Y+dir.Y*length)))
}

func entered(h [4]float64, acrossX bool, stepX, stepY, u, v float64) float64 {
	switch {
	case acrossX && stepX > 0:
		return h[0] + v*(h[2]-h[0])
	case acrossX:
		return h[1] + v*(h[3]-h[1])
	case stepY > 0:
		return h[0] + u*(h[1]-h[0])
	}
	return h[2] + u*(h[3]-h[2])
}

// f the start in cells, b cells a unit along the ray; never along a line
func lattice(f, b float64) (step, next, delta float64) {
	switch {
	case b > 0:
		return 1, (math.Floor(f) + 1 - f) / b, 1 / b
	case b < 0:
		return -1, (math.Floor(f) - f) / b, -1 / b
	}
	return 0, math.Inf(1), math.Inf(1)
}

// false where the ray runs along the diagonal
func folded(h [4]float64, ax, ay, bx, by float64) (t, alt float64, ok bool) {
	if math.Abs(h[0]-h[3]) < math.Abs(h[1]-h[2]) { // folded along 0–3: u = v
		if bx == by {
			return 0, 0, false
		}
		t = (ay - ax) / (bx - by)
		u := min(max(ax+bx*t, 0), 1)
		return t, h[0] + u*(h[3]-h[0]), true
	}
	if bx+by == 0 { // folded along 1–2: u + v = 1
		return 0, 0, false
	}
	t = (1 - ax - ay) / (bx + by)
	v := min(max(ay+by*t, 0), 1)
	return t, h[1] + v*(h[2]-h[1]), true
}

// top-left, top-right, bottom-left, bottom-right; 0 off the grid, as GroundAt has it
func (r *Relief) cellCorners(x, y int64) (h [4]float64) {
	cx, okX := foldAxis(x, int64(r.sq.Cols), r.sq.WrapX)
	cy, okY := foldAxis(y, int64(r.sq.Rows), r.sq.WrapY)
	if !okX || !okY {
		return h
	}
	h[0] = r.cornerAt(cx, cy)
	h[1] = r.cornerAt(cx+1, cy)
	h[2] = r.cornerAt(cx, cy+1)
	h[3] = r.cornerAt(cx+1, cy+1)
	return h
}

func (r *Relief) cornerAt(x, y int64) float64 {
	fx, _ := foldAxis(x, int64(r.cols), r.sq.WrapX)
	fy, _ := foldAxis(y, int64(r.rows), r.sq.WrapY)
	return float64(r.values[int(fy)*r.cols+int(fx)])
}
