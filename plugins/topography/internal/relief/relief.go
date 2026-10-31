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
// bottom-right. A hex cell's are its level four times, its six corners Relief.Hex's.
type Corners [4]float32

// Level is the mean of the corners.
func (c Corners) Level() float32 {
	return (c[0] + c[1] + c[2] + c[3]) / 4
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

// Relief is the ground's height over a board's grid at its cells' corners, which the
// neighbouring cells share: a square grid's lattice, a hex grid's corners with every cell's
// middle at their mean. Not safe for concurrent use.
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
// given: over a board, the board's (board.BoardWrite.Touch). It panics for a grid neither square
// nor a hex grid wrapping along no axis.
func New(g grid.Grid, touched func(c cell.ID)) *Relief {
	r := &Relief{grid: g, touched: touched}
	known := false
	if b, ok := g.(*board.BoardRead); ok {
		r.board = b
		r.sq, known = b.Shape()
	} else {
		r.sq, known = grid.ShapeOf(g)
	}
	if !known || r.sq.Hex && (r.sq.WrapX || r.sq.WrapY) {
		panic("relief: a relief lies over a square grid or a hex one wrapping along no axis")
	}
	r.square = !r.sq.Hex
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
		r.values = make([]float32, HexCount(int(r.sq.Cols), int(r.sq.Rows)))
	}
	return r
}

// Lattice is the ground's heights over a square grid: cols by rows corners a cell apart, row by
// row, the relief's own values to read and not to keep. It is false off a square grid.
func (r *Relief) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	if !r.square {
		return 0, 0, 0, nil, false
	}
	return r.cols, r.rows, r.sq.Cell, r.values, true
}

// CopyInto has dst hold the heights r holds now, copied where its version differs, keeping its
// memory: a relief to read alone, changed by nothing.
func (r *Relief) CopyInto(dst *Relief) {
	if dst.grid == nil {
		*dst = Relief{grid: r.grid, sq: r.sq, square: r.square, cols: r.cols, rows: r.rows}
	}
	if dst.version == r.version && len(dst.values) == len(r.values) {
		return
	}
	dst.values = append(dst.values[:0], r.values...)
	dst.version, dst.low, dst.high = r.version, r.low, r.high
}

// Version counts the changes to the heights; it only grows.
func (r *Relief) Version() uint64 { return r.version }

// Extent is the relief's lowest ground, sea level at most, and its highest.
func (r *Relief) Extent() (low, high float32) { return r.low, r.high }

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
func (r *Relief) Highest() float32 {
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

// only when together the runs cover the grid exactly: its corners, or on a hex grid its cells,
// a level a cell as a hex relief was saved before it held the corners
func (r *Relief) adopt(runs []Heights) bool {
	n := 0
	for _, h := range runs {
		n += int(h.Count)
	}
	cells := !r.square && n == r.grid.CellCount()
	if n != len(r.values) && !cells {
		return false
	}
	for _, h := range runs {
		if int(h.First)+int(h.Count) > n || h.Count > HeightsRun {
			return false
		}
	}
	values := r.values
	if cells {
		values = make([]float32, n)
	}
	for _, h := range runs {
		copy(values[h.First:], h.Values[:h.Count])
	}
	if cells {
		r.levelled(values)
	}
	r.changed()
	return true
}

// every corner at the mean of the levels of the cells meeting at it, the levels by the cells'
// ordinals; in float64, as ground.MeanOfCells means them
func (r *Relief) levelled(levels []float32) {
	for i := range r.values {
		sum, n := 0.0, 0.0
		r.touching(r.hexVertex(i), func(c cell.ID) {
			o, _ := r.grid.Ordinal(c)
			sum, n = sum+float64(levels[o]), n+1
		})
		r.values[i] = float32(sum / n)
	}
}

// a corner of the lattice on a square grid, on a hex one a corner on the lattice of its
// triangles (onLattice)
type vertex struct{ x, y int }

func (r *Relief) index(v vertex) int {
	if !r.square {
		i, _ := r.hexIndex(v)
		return i
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

// a square cell's four corners, a hex cell's six clockwise from the top
func (r *Relief) corners(c cell.ID) (vs [6]vertex, n int, ok bool) {
	x, y, ok := r.grid.Coords(c)
	if !ok {
		return vs, 0, false
	}
	if !r.square {
		mi, mj := 2*int(x)+int(y), int(y)-int(x)
		for k, d := range hexCorner {
			vs[k] = vertex{mi + d[0], mj + d[1]}
		}
		return vs, 6, true
	}
	for k, d := range [4][2]int64{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		vs[k], _ = r.corner(int64(x)+d[0], int64(y)+d[1])
	}
	return vs, 4, true
}

// Corners is the ground height at c's corners; zero off the board.
func (r *Relief) Corners(c cell.ID) Corners {
	if !r.square {
		level := r.levelOf(c)
		return Corners{level, level, level, level}
	}
	vs, _, ok := r.corners(c)
	if !ok {
		return Corners{}
	}
	var out Corners
	for k := range out {
		out[k] = r.values[r.index(vs[k])]
	}
	return out
}

// SetCorners puts c's corners at k's heights, or a hex cell's six at k's first, the neighbours'
// corners meeting them going with them.
func (r *Relief) SetCorners(c cell.ID, k Corners) {
	vs, n, ok := r.corners(c)
	if !ok {
		return
	}
	changed := false
	for i := range n {
		h := k[0]
		if r.square {
			h = k[i]
		}
		changed = r.setValue(vs[i], h) || changed
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
		r.touching(v, r.touched)
	}
	return true
}

// Altitude is c's ground level, the mean of its corners.
func (r *Relief) Altitude(c cell.ID) float32 {
	if !r.square {
		return r.levelOf(c)
	}
	return r.Corners(c).Level()
}

// SetHeights sets the ground to heights, sampled at every corner.
func (r *Relief) SetHeights(heights func(p geom.Vec) float32) {
	changed := false
	if r.square {
		for y := range r.rows {
			for x := range r.cols {
				h := heights(geom.NewVec(float32(x)*r.sq.Cell, float32(y)*r.sq.Cell))
				changed = r.setValue(vertex{x, y}, h) || changed
			}
		}
	} else {
		for i := range r.values {
			v := r.hexVertex(i)
			changed = r.setValue(v, heights(r.hexPoint(v))) || changed
		}
	}
	if changed {
		r.changed()
	}
}

// GroundAt is the ground height under p as it is drawn, 0 off the board.
func (r *Relief) GroundAt(p geom.Vec) float32 {
	if !r.square {
		return r.hexGround(p)
	}
	// in float64, the ground's own reading of where p lies in its cell, rounded once
	size := float64(r.sq.Cell)
	if size == 0 {
		return 0
	}
	fx, fy := float64(p.X)/size, float64(p.Y)/size
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
	return float32(drawnAt(float64(hs[0]), float64(hs[1]), float64(hs[2]), float64(hs[3]), fx-x0,
		fy-y0))
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
func (r *Relief) At(p geom.Vec) float32 { return r.GroundAt(p) }

// Step is how far apart sight samples the ground: the shorter side of a cell.
func (r *Relief) Step() float32 {
	w, h := r.grid.CellBounds()
	return min(w, h)
}

// Top is c's corners with height standing on them, and its level; a hex cell's level four times.
func (r *Relief) Top(c cell.ID, height float32) (corners [4]float32, level float32) {
	if !r.square {
		level = r.Altitude(c)
		h := level + height
		return [4]float32{h, h, h, h}, level
	}
	k := r.Corners(c)
	level = k.Level()
	for i := range k {
		k[i] += height
	}
	return k, level
}

// the cells meeting at the corner v
func (r *Relief) touching(v vertex, fn func(c cell.ID)) {
	if !r.square {
		for _, d := range hexSpokes[class(v.x, v.y)-1] {
			if q, row, ok := r.middle(v.x+d[0], v.y+d[1]); ok {
				fn(r.grid.At(uint32(q), uint32(row)))
			}
		}
		return
	}
	for _, d := range [4][2]int64{{0, 0}, {-1, 0}, {0, -1}, {-1, -1}} {
		cx, okX := foldAxis(int64(v.x)+d[0], int64(r.sq.Cols), r.sq.WrapX)
		cy, okY := foldAxis(int64(v.y)+d[1], int64(r.sq.Rows), r.sq.WrapY)
		if okX && okY {
			fn(r.grid.At(uint32(cx), uint32(cy)))
		}
	}
}
