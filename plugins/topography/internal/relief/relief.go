package relief

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
)

// Corners is a cell's ground height at its corners: top-left, top-right, bottom-left,
// bottom-right. A hex cell is level, its four heights one.
type Corners [4]float32

// Level is the mean of the corners.
func (c Corners) Level() float64 {
	return (float64(c[0]) + float64(c[1]) + float64(c[2]) + float64(c[3])) / 4
}

// HeightsRun is how many of the ground's heights one of the topography's entities carries.
const HeightsRun = 1024

// Heights is a run of the ground's heights as the topography's entities carry them, saved with
// the game: Count of them from the First on, in the order the relief keeps them. Its size is
// fixed, so the ECS keeps it in its own memory.
type Heights struct {
	First, Count uint32
	Values       [HeightsRun]float32
}

// Relief is the ground's height over a board's grid: on a square grid a lattice of corners the
// neighbouring cells share, on any other a level per cell. Not safe for concurrent use.
type Relief struct {
	grid      grid.Grid
	board     *board.BoardRead // nil where the grid is no board's
	sq        grid.Shape
	square    bool
	cols      int // corners along x on a square grid
	rows      int // corners along y
	values    []float32
	version   uint64
	touched   func(c cell.ID)
	low, high float32
}

// New is level ground at 0 over g; every change to a cell's corners is counted by touched, if
// given: over a board, the board's (board.BoardWrite.Touch).
func New(g grid.Grid, touched func(c cell.ID)) *Relief {
	r := &Relief{grid: g, touched: touched}
	if b, ok := g.(*board.BoardRead); ok {
		r.board = b
		r.sq, r.square = b.Shape()
	} else {
		r.sq, r.square = grid.ShapeOf(g)
	}
	r.square = r.square && !r.sq.Hex
	if r.square {
		r.cols, r.rows = int(r.sq.Cols)+1, int(r.sq.Rows)+1
		if r.sq.WrapX {
			r.cols--
		}
		if r.sq.WrapY {
			r.rows--
		}
		r.values = make([]float32, r.cols*r.rows)
	} else {
		r.values = make([]float32, g.CellCount())
	}
	return r
}

// Lattice is the ground's heights over a square grid: cols by rows corners a cell apart, row by
// row, the relief's own values to read and not to keep. It is false off a square grid.
func (r *Relief) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	if !r.square {
		return 0, 0, 0, nil, false
	}
	return r.cols, r.rows, float32(r.sq.Cell), r.values, true
}

// Sloped reports whether the ground runs between a cell's corners: a square grid's.
func (r *Relief) Sloped() bool { return r.square }

// Version counts the changes to the heights; it only grows.
func (r *Relief) Version() uint64 { return r.version }

// Extent is the relief's lowest ground, sea level at most, and its highest.
func (r *Relief) Extent() (low, high float64) { return float64(r.low), float64(r.high) }

// changed counts a change to the heights and takes their extent anew
func (r *Relief) changed() {
	r.version++
	lo, hi := float32(0), float32(0)
	for _, v := range r.values {
		lo, hi = min(lo, v), max(hi, v)
	}
	r.low, r.high = lo, hi
}

// Highest is the relief's highest ground.
func (r *Relief) Highest() float64 {
	_, high := r.Extent()
	return high
}

// Runs is how many runs of HeightsRun the relief's heights take.
func (r *Relief) Runs() int { return (len(r.values) + HeightsRun - 1) / HeightsRun }

func (r *Relief) fill(h *Heights) {
	first := min(int(h.First), len(r.values))
	n := copy(h.Values[:], r.values[first:])
	h.Count = uint32(n)
	clear(h.Values[n:])
}

// only when together the runs cover the grid exactly
func (r *Relief) adopt(runs []Heights) bool {
	total := 0
	for _, h := range runs {
		if int(h.First)+int(h.Count) > len(r.values) || h.Count > HeightsRun {
			return false
		}
		total += int(h.Count)
	}
	if total != len(r.values) {
		return false
	}
	for _, h := range runs {
		copy(r.values[h.First:], h.Values[:h.Count])
	}
	r.changed()
	return true
}

// a corner of the lattice on a square grid, or a cell's ordinal in x on any other
type vertex struct{ x, y int }

func (r *Relief) index(v vertex) int {
	if !r.square {
		return v.x
	}
	return v.y*r.cols + v.x
}

func (r *Relief) corner(x, y int64) (vertex, bool) {
	fold := func(v, n int64, wraps bool) (int64, bool) {
		if wraps {
			return (v%n + n) % n, true
		}
		return v, v >= 0 && v < n
	}
	fx, okX := fold(x, int64(r.cols), r.sq.WrapX)
	fy, okY := fold(y, int64(r.rows), r.sq.WrapY)
	return vertex{int(fx), int(fy)}, okX && okY
}

func (r *Relief) corners(c cell.ID) (vs [4]vertex, n int, ok bool) {
	if !r.square {
		i, ok := r.grid.Ordinal(c)
		if !ok {
			return vs, 0, false
		}
		vs[0] = vertex{x: i}
		return vs, 1, true
	}
	x, y, ok := r.grid.Coords(c)
	if !ok {
		return vs, 0, false
	}
	for k, d := range [4][2]int64{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		vs[k], _ = r.corner(int64(x)+d[0], int64(y)+d[1])
	}
	return vs, 4, true
}

// Corners is the ground height at c's corners; zero off the board.
func (r *Relief) Corners(c cell.ID) Corners {
	vs, n, ok := r.corners(c)
	if !ok {
		return Corners{}
	}
	if n == 1 {
		h := r.values[r.index(vs[0])]
		return Corners{h, h, h, h}
	}
	var out Corners
	for k := range out {
		out[k] = r.values[r.index(vs[k])]
	}
	return out
}

// SetCorners puts c's corners at k's heights, the neighbours' corners meeting them going with
// them, or a hex cell level at k's first.
func (r *Relief) SetCorners(c cell.ID, k Corners) {
	vs, n, ok := r.corners(c)
	if !ok {
		return
	}
	changed := false
	for i := range n {
		changed = r.setValue(vs[i], k[i]) || changed
	}
	if changed {
		r.changed()
	}
}

func (r *Relief) setValue(v vertex, h float32) bool {
	i := r.index(v)
	if r.values[i] == h {
		return false
	}
	r.values[i] = h
	if r.touched != nil {
		r.touching(v, func(c cell.ID, _ int) { r.touched(c) })
	}
	return true
}

// Altitude is c's ground level, the mean of its corners.
func (r *Relief) Altitude(c cell.ID) float64 { return r.Corners(c).Level() }

// SetHeights sets the ground to heights, sampled at every corner on a square grid and at every
// cell's centre on any other.
func (r *Relief) SetHeights(heights func(p geom.Vec) float64) {
	changed := false
	if r.square {
		for y := range r.rows {
			for x := range r.cols {
				h := float32(heights(geom.NewVec(float64(x)*r.sq.Cell, float64(y)*r.sq.Cell)))
				changed = r.setValue(vertex{x, y}, h) || changed
			}
		}
	} else {
		r.grid.EachCell(func(c cell.ID) {
			if i, ok := r.grid.Ordinal(c); ok {
				changed = r.setValue(vertex{x: i}, float32(heights(r.grid.CellCenter(c)))) || changed
			}
		})
	}
	if changed {
		r.changed()
	}
}

// GroundAt is the ground height under p as it is drawn, 0 off the board.
func (r *Relief) GroundAt(p geom.Vec) float64 {
	if !r.square {
		c, ok := r.grid.CellAt(p)
		if !ok {
			return 0
		}
		return float64(r.Corners(c)[0])
	}
	size := r.sq.Cell
	if size == 0 {
		return 0
	}
	fx, fy := p.X/size, p.Y/size
	x0, y0 := math.Floor(fx), math.Floor(fy)
	cx, okX := foldAxis(int64(x0), int64(r.sq.Cols), r.sq.WrapX)
	cy, okY := foldAxis(int64(y0), int64(r.sq.Rows), r.sq.WrapY)
	if !okX || !okY {
		return 0
	}
	var hs Corners
	for k, d := range [4][2]int64{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		v, _ := r.corner(cx+d[0], cy+d[1])
		hs[k] = r.values[r.index(v)]
	}
	return drawnAt(float64(hs[0]), float64(hs[1]), float64(hs[2]), float64(hs[3]), fx-x0, fy-y0)
}

// two flat triangles split along the diagonal whose corners stand nearer in height, as
// render.Frame.Fold splits it; ties along 1–2, as a quad is drawn unfolded
func drawnAt(h0, h1, h2, h3, u, v float64) float64 {
	if math.Abs(h0-h3) < math.Abs(h1-h2) {
		if u >= v {
			return h0 + u*(h1-h0) + v*(h3-h1)
		}
		return h0 + v*(h2-h0) + u*(h3-h2)
	}
	if u+v <= 1 {
		return h0 + u*(h1-h0) + v*(h2-h0)
	}
	return h3 + (1-u)*(h2-h3) + (1-v)*(h1-h3)
}

func foldAxis(v, n int64, wraps bool) (int64, bool) {
	if wraps {
		return (v%n + n) % n, true
	}
	return v, v >= 0 && v < n
}

var _ ground.Heights = (*Relief)(nil)

// At is GroundAt, the ground.Heights contract.
func (r *Relief) At(p geom.Vec) float64 { return r.GroundAt(p) }

// Step is how far apart sight samples the ground: the shorter side of a cell.
func (r *Relief) Step() float64 {
	w, h := r.grid.CellBounds()
	return min(w, h)
}

// Top is c's corners with height standing on them, and its level.
func (r *Relief) Top(c cell.ID, height float64) (corners [4]float32, level float32) {
	k := r.Corners(c)
	level = float32(k.Level())
	for i := range k {
		k[i] += float32(height)
	}
	return k, level
}

// k is the index of v among the cell's corners, -1 for a hex cell
func (r *Relief) touching(v vertex, fn func(c cell.ID, k int)) {
	if !r.square {
		r.grid.EachCell(func(c cell.ID) {
			if i, ok := r.grid.Ordinal(c); ok && i == v.x {
				fn(c, -1)
			}
		})
		return
	}
	for k, d := range [4][2]int64{{0, 0}, {-1, 0}, {0, -1}, {-1, -1}} {
		cx, okX := foldAxis(int64(v.x)+d[0], int64(r.sq.Cols), r.sq.WrapX)
		cy, okY := foldAxis(int64(v.y)+d[1], int64(r.sq.Rows), r.sq.WrapY)
		if okX && okY {
			fn(r.grid.At(uint32(cx), uint32(cy)), 3-k)
		}
	}
}
