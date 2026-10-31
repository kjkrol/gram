package relief

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// HexCorners is where the six corners of the cell (q, r) of a hex grid cols cells wide lie among
// the relief's heights, clockwise from the top: one height a corner, the cells meeting at it
// sharing it.
func HexCorners(cols, q, r int) [6]int {
	top := hexTop(cols, q, r)
	row := 2 * (cols + 1)
	return [6]int{top, top + 1, top + row, top + row - 1, top + row - 2, top - 1}
}

// HexCount is how many corners, and so heights, a hex grid of cols by rows cells has.
func HexCount(cols, rows int) int { return 2*cols*rows + 2*cols + 2*rows }

// every cell keeps its top and top-right corners, and so do a column of cells left of the board
// and a row under it, less the two corners of no cell: the column's first one's top and the row's
// last one's top-right
func hexTop(cols, q, r int) int { return 2*(r*(cols+1)+q) + 1 }

// the lattice offsets of a cell's corners from its middle, clockwise from the top
var hexCorner = [6][2]int{{0, -1}, {1, -1}, {1, 0}, {0, 1}, {-1, 1}, {-1, 0}}

// the first leads from a corner of class 1 to the middles round it and from one of class 2 to the
// corners beside it, the second the other way about
var hexSpokes = [2][3][2]int{{{-1, 0}, {0, 1}, {1, -1}}, {{1, 0}, {0, -1}, {-1, 1}}}

// 0 for a cell's middle, 1 for the top corner of the cell under it, 2 for the top-right corner of
// the cell under it and left
func class(i, j int) int { return ((i-j)%3 + 3) % 3 }

// the cell whose middle is the lattice point (i, j), of class 0
func (r *Relief) middle(i, j int) (q, row int, ok bool) {
	q = (i - j) / 3
	row = j + q
	return q, row, q >= 0 && row >= 0 && q < int(r.sq.Cols) && row < int(r.sq.Rows)
}

// false for a middle and a corner off the board
func (r *Relief) hexIndex(v vertex) (int, bool) {
	k := class(v.x, v.y)
	if k == 0 {
		return 0, false
	}
	q := (v.x - v.y - k) / 3
	row := v.y + 1 + q
	cols := int(r.sq.Cols)
	i := hexTop(cols, q, row) + k - 1
	if q < -1 || q >= cols || row < 0 || row > int(r.sq.Rows) || i < 0 || i >= len(r.values) {
		return 0, false
	}
	return i, true
}

// the corner whose height is the relief's i-th
func (r *Relief) hexVertex(i int) vertex {
	cols := int(r.sq.Cols)
	owner, k := (i+1)/2, (i+1)%2+1
	row, q := owner/(cols+1), owner%(cols+1)-1
	v := vertex{2*q + row, row - q - 1}
	if k == 2 {
		v.x++
	}
	return v
}

func (r *Relief) hexPoint(v vertex) geom.Vec {
	s := float64(r.sq.Cell)
	i, j := float64(v.x), float64(v.y)
	return geom.NewVec(float32(s+i*s*math.Sqrt(3)/2), float32(s+i*s/2+j*s))
}

// onLattice is where p lies on the lattice of the triangles a hex grid's cells fall into, six a
// cell from its middle to its corners: the point (i, j) lies at (S + i·S·√3/2, S + i·S/2 + j·S),
// the cells S from middle to corner, a middle where i − j is a multiple of three, the cell
// (q, r)'s at (2q + r, r − q). In float64, as the ground reads where p lies, rounded once.
func (r *Relief) onLattice(p geom.Vec) (fi, fj float64) {
	s := float64(r.sq.Cell)
	fi = 2 * (float64(p.X) - s) / (math.Sqrt(3) * s)
	return fi, (float64(p.Y)-s)/s - fi/2
}

// the mean of the corners of the cell whose top corner is the relief's top-th height
func (r *Relief) hexLevel(top int) float32 {
	row, v := 2*(int(r.sq.Cols)+1), r.values
	return (v[top] + v[top+1] + v[top+row] + v[top+row-1] + v[top+row-2] + v[top-1]) / 6
}

// 0 off the board
func (r *Relief) levelOf(c cell.ID) float32 {
	q, row, ok := r.grid.Coords(c)
	if !ok {
		return 0
	}
	return r.hexLevel(hexTop(int(r.sq.Cols), int(q), int(row)))
}

// Hex is the ground's height at the six corners of the hex cell c, clockwise from the top, and
// its level, their mean; false off a hex grid's board.
func (r *Relief) Hex(c cell.ID) (corners [6]float32, level float32, ok bool) {
	if r.square {
		return corners, 0, false
	}
	q, row, ok := r.grid.Coords(c)
	if !ok {
		return corners, 0, false
	}
	cols := int(r.sq.Cols)
	for k, i := range HexCorners(cols, int(q), int(row)) {
		corners[k] = r.values[i]
	}
	return corners, r.hexLevel(hexTop(cols, int(q), int(row))), true
}

// a triangle of the lattice, a half of the rhombus from (i, j): the lower (i, j), (i+1, j),
// (i, j+1) or the upper (i+1, j+1), (i, j+1), (i+1, j), with the heights there in that order
type facet struct {
	i, j  float64
	upper bool
	h     [3]float64
}

// the halves meet along the rhombus's short diagonal, a side of the triangles round a middle
func triangle(fi, fj float64) (f facet, at [3]vertex) {
	f.i, f.j = math.Floor(fi), math.Floor(fj)
	f.upper = fi-f.i+fj-f.j > 1
	i, j := int(f.i), int(f.j)
	if f.upper {
		return f, [3]vertex{{i + 1, j + 1}, {i, j + 1}, {i + 1, j}}
	}
	return f, [3]vertex{{i, j}, {i + 1, j}, {i, j + 1}}
}

// false where the triangle is no cell's of the board
func (r *Relief) facet(fi, fj float64) (facet, bool) {
	f, at := triangle(fi, fj)
	for _, m := range at {
		if class(m.x, m.y) != 0 {
			continue
		}
		q, row, ok := r.middle(m.x, m.y)
		if !ok {
			return f, false
		}
		for k, v := range at {
			if i, ok := r.hexIndex(v); ok {
				f.h[k] = float64(r.values[i])
			} else {
				f.h[k] = float64(r.hexLevel(hexTop(int(r.sq.Cols), q, row)))
			}
		}
		return f, true
	}
	return f, false
}

func (f *facet) at(fi, fj float64) float64 {
	u, v := fi-f.i, fj-f.j
	if f.upper {
		return f.h[0] + (1-u)*(f.h[1]-f.h[0]) + (1-v)*(f.h[2]-f.h[0])
	}
	return f.h[0] + u*(f.h[1]-f.h[0]) + v*(f.h[2]-f.h[0])
}

func (r *Relief) hexGround(p geom.Vec) float32 {
	if r.sq.Cell == 0 {
		return 0
	}
	fi, fj := r.onLattice(p)
	f, ok := r.facet(fi, fj)
	if !ok {
		return 0
	}
	return float32(f.at(fi, fj))
}

// the nearer of the two corners of the triangle p lies in, which within a cell is the nearest of
// all; on the board or not
func (r *Relief) nearestCorner(p geom.Vec) vertex {
	fi, fj := r.onLattice(p)
	_, at := triangle(fi, fj)
	best, far := vertex{}, math.Inf(1)
	for _, v := range at {
		if class(v.x, v.y) == 0 {
			continue
		}
		// squared in the lattice's sides, which meet at 60°
		du, dv := float64(v.x)-fi, float64(v.y)-fj
		if d := du*du + dv*dv + du*dv; d < far {
			best, far = v, d
		}
	}
	return best
}

// fn has every corner of the board whose point lies within the box spanning a and b
func (r *Relief) hexCornersIn(a, b geom.Vec, fn func(v vertex)) {
	s := float64(r.sq.Cell)
	tol := 1e-3 * s
	x0, x1 := float64(min(a.X, b.X))-tol, float64(max(a.X, b.X))+tol
	y0, y1 := float64(min(a.Y, b.Y))-tol, float64(max(a.Y, b.Y))+tol
	cols, rows := float64(r.sq.Cols), float64(r.sq.Rows)
	// the corners' i run from -2 to 2·cols + rows - 1, their j from -cols to rows
	across := s * math.Sqrt(3) / 2
	i0 := int(max(math.Ceil((x0-s)/across), -2))
	i1 := int(min(math.Floor((x1-s)/across), 2*cols+rows-1))
	for i := i0; i <= i1; i++ {
		lift := s + float64(i)*s/2
		j0 := int(max(math.Ceil((y0-lift)/s), -cols))
		j1 := int(min(math.Floor((y1-lift)/s), rows))
		for j := j0; j <= j1; j++ {
			if v := (vertex{i, j}); class(i, j) != 0 {
				if _, ok := r.hexIndex(v); ok {
					fn(v)
				}
			}
		}
	}
}
