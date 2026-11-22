package relief

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// HexCorners is where the six corners of the cell in a column of a row of a hex grid cols cells
// wide lie among the relief's heights, clockwise from the top: one height a corner, the cells
// meeting at it sharing it.
func HexCorners(cols, col, row int) [6]int {
	top := hexTop(cols, col, row)
	// the cell under it and east, every other row half a cell east of the one over it
	east := hexTop(cols, col+row&1, row+1)
	west := east - 2
	return [6]int{top, top + 1, east, west + 1, west, top - 1}
}

// HexCount is how many heights a hex grid of cols by rows cells keeps: a slot for each of its
// corners, and some of no cell's.
func HexCount(cols, rows int) int { return 2 * (cols + 2) * (rows + 1) }

// every cell keeps its top and top-right corners, and so do a column of cells either side of the
// board and a row under it, which keep the corners of the board's cells beside them
func hexTop(cols, col, row int) int { return 2 * (row*(cols+2) + col + 1) }

// the column of the axial (q, row): a row's cells begin half the row's number west of q 0
func colOf(q, row int) int { return q + row>>1 }

// HexRim is the rectangle a hex board of cols by rows cells, size from middle to corner, keeps
// to: its cells and, where a row begins or ends half a cell in and between its first and last
// rows' points, the parts of the cells beside it that fill it (its hem).
func HexRim(cols, rows int, size float32) geom.AABB {
	s := float64(size)
	wide := math.Sqrt(3) * s * float64(cols)
	if rows > 1 {
		wide += math.Sqrt(3) / 2 * s
	}
	left := s - math.Sqrt(3)/2*s
	return geom.NewAABB(
		geom.NewVec(float32(left), 0),
		geom.NewVec(float32(left+wide), float32(s*(1.5*float64(rows-1)+2))),
	)
}

// Hem is the ground of a cell beside the board, in a column and a row past its own, which fills
// the board's rim: its six corners clockwise from the top, those a cell of the board has as it
// has them, the others and its level at their mean; false where it shares none.
func (r *Relief) Hem(col, row int) (corners [6]float32, level float32, ok bool) {
	if r.square {
		return corners, 0, false
	}
	q := col - row>>1
	mi, mj := 2*q+row, row-q
	var own [6]bool
	sum, n := 0.0, 0.0
	for k, d := range hexCorner {
		if i, ok := r.hexIndex(vertex{mi + d[0], mj + d[1]}); ok {
			corners[k], own[k] = r.values[i], true
			sum, n = sum+float64(r.values[i]), n+1
		}
	}
	if n == 0 {
		return corners, 0, false
	}
	level = float32(sum / n)
	for k := range corners {
		if !own[k] {
			corners[k] = level
		}
	}
	return corners, level, true
}

// the ground of a triangle of a cell of the hem, within the board's rim alone
func (r *Relief) hemFacet(f facet, at [3]vertex, col, row int, fi, fj float64) (facet, bool) {
	s := float64(r.sq.Cell)
	x, y := float32(s+fi*s*math.Sqrt(3)/2), float32(s+fi*s/2+fj*s)
	rim := HexRim(int(r.sq.Cols), int(r.sq.Rows), r.sq.Cell)
	if x < rim.TopLeft.X || x > rim.BottomRight.X || y < rim.TopLeft.Y || y > rim.BottomRight.Y {
		return f, false
	}
	_, level, ok := r.Hem(col, row)
	if !ok {
		return f, false
	}
	for k, v := range at {
		f.h[k] = float64(level)
		if i, ok := r.hexIndex(v); ok {
			f.h[k] = float64(r.values[i])
		}
	}
	return f, true
}

// the lattice offsets of a cell's corners from its middle, clockwise from the top
var hexCorner = [6][2]int{{0, -1}, {1, -1}, {1, 0}, {0, 1}, {-1, 1}, {-1, 0}}

// the first leads from a corner of class 1 to the middles round it and from one of class 2 to the
// corners beside it, the second the other way about
var hexSpokes = [2][3][2]int{{{-1, 0}, {0, 1}, {1, -1}}, {{1, 0}, {0, -1}, {-1, 1}}}

// 0 for a cell's middle, 1 for the top corner of the cell under it, 2 for the top-right corner of
// the cell under it and left
func class(i, j int) int { return ((i-j)%3 + 3) % 3 }

// the cell whose middle is the lattice point (i, j), of class 0
func (r *Relief) middle(i, j int) (col, row int, ok bool) {
	q := (i - j) / 3
	row = j + q
	col = colOf(q, row)
	return col, row, col >= 0 && row >= 0 && col < int(r.sq.Cols) && row < int(r.sq.Rows)
}

// false for a middle and a corner off the board
func (r *Relief) hexIndex(v vertex) (int, bool) {
	k := class(v.x, v.y)
	if k == 0 {
		return 0, false
	}
	q := (v.x - v.y - k) / 3
	row := v.y + 1 + q
	cols, col := int(r.sq.Cols), colOf(q, row)
	if col < -1 || col > cols || row < 0 || row > int(r.sq.Rows) {
		return 0, false
	}
	i := hexTop(cols, col, row) + k - 1
	return i, r.cornered[i]
}

// the corner whose height is the relief's i-th
func (r *Relief) hexVertex(i int) vertex {
	cols := int(r.sq.Cols)
	owner, k := i/2, i%2+1
	row := owner / (cols + 2)
	q := owner%(cols+2) - 1 - row>>1
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

// the mean of the corners of the cell in a column of a row
func (r *Relief) hexLevel(col, row int) float32 {
	at, v := HexCorners(int(r.sq.Cols), col, row), r.values
	return (v[at[0]] + v[at[1]] + v[at[2]] + v[at[3]] + v[at[4]] + v[at[5]]) / 6
}

// 0 off the board
func (r *Relief) levelOf(c cell.ID) float32 {
	col, row, ok := r.grid.Coords(c)
	if !ok {
		return 0
	}
	return r.hexLevel(int(col), int(row))
}

// Hex is the ground's height at the six corners of the hex cell c, clockwise from the top, and
// its level, their mean; false off a hex grid's board.
func (r *Relief) Hex(c cell.ID) (corners [6]float32, level float32, ok bool) {
	if r.square {
		return corners, 0, false
	}
	col, row, ok := r.grid.Coords(c)
	if !ok {
		return corners, 0, false
	}
	for k, i := range HexCorners(int(r.sq.Cols), int(col), int(row)) {
		corners[k] = r.values[i]
	}
	return corners, r.hexLevel(int(col), int(row)), true
}

// SetHex puts the six corners of the hex cell c at corners, clockwise from the top, the cells
// meeting at them going with them; nothing off a hex grid's board.
func (r *Relief) SetHex(c cell.ID, corners [6]float32) {
	if r.square {
		return
	}
	vs, n, ok := r.corners(c)
	if !ok {
		return
	}
	changed := false
	for k, v := range vs[:n] {
		changed = r.setValue(v, corners[k]) || changed
	}
	if changed {
		r.changed()
	}
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
			return r.hemFacet(f, at, q, row, fi, fj)
		}
		for k, v := range at {
			if i, ok := r.hexIndex(v); ok {
				f.h[k] = float64(r.values[i])
			} else {
				f.h[k] = float64(r.hexLevel(q, row))
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
	// the corners' i run from -1 to 2·cols + 1, their j from -cols - 1 to rows as a row's cells
	// begin as far west as the rows over it are many
	across := s * math.Sqrt(3) / 2
	i0 := int(max(math.Ceil((x0-s)/across), -1))
	i1 := int(min(math.Floor((x1-s)/across), 2*cols+1))
	for i := i0; i <= i1; i++ {
		lift := s + float64(i)*s/2
		j0 := int(max(math.Ceil((y0-lift)/s), -cols-1))
		j1 := int(min(math.Floor((y1-lift)/s), rows+rows/2+1))
		for j := j0; j <= j1; j++ {
			if v := (vertex{i, j}); class(i, j) != 0 {
				if _, ok := r.hexIndex(v); ok {
					fn(v)
				}
			}
		}
	}
}
