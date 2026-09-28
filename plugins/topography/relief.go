package topography

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
)

// Corners is a cell's ground height at its corners: top-left, top-right, bottom-left, bottom-right.
// A hex cell is level, its four heights one.
type Corners [4]float32

// Level is the mean of the corners.
func (c Corners) Level() float64 {
	return (float64(c[0]) + float64(c[1]) + float64(c[2]) + float64(c[3])) / 4
}

// Heights is the ground's heights as the topography's own entity carries them, saved with the
// game: on a square grid one per corner of the lattice, row by row — a column more than the grid
// has, a row more, save along an axis that wraps — on any other grid one per cell, by ordinal.
type Heights struct{ Values []float32 }

// MarshalBinary is the heights as a save writes them: their count, then each, little-endian.
func (h Heights) MarshalBinary() ([]byte, error) {
	out := make([]byte, 4+4*len(h.Values))
	binary.LittleEndian.PutUint32(out, uint32(len(h.Values)))
	for i, v := range h.Values {
		binary.LittleEndian.PutUint32(out[4+4*i:], math.Float32bits(v))
	}
	return out, nil
}

// UnmarshalBinary reads the heights a save wrote.
func (h *Heights) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("topography: heights of %d bytes", len(data))
	}
	n := int(binary.LittleEndian.Uint32(data))
	if len(data) != 4+4*n {
		return fmt.Errorf("topography: %d heights in %d bytes", n, len(data))
	}
	h.Values = make([]float32, n)
	for i := range h.Values {
		h.Values[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[4+4*i:]))
	}
	return nil
}

// Relief is the ground's height over a board's grid. On a square grid it is a lattice of corners
// the neighbouring cells share where they meet, so the ground runs on between the cells and has no
// vertical walls; on any other grid a cell is level. It is the world's Ground. Not safe for
// concurrent use.
type Relief struct {
	grid   board.Grid
	sq     board.SquareShape
	square bool
	cols   int // corners along x on a square grid
	rows   int // corners along y
	values []float32
	// version counts the changes; touched is told of every cell whose corners changed, nil none
	version uint64
	touched func(c board.CellID)
	// low and high are the relief's lowest ground, sea level at most, and its highest, as of the
	// version extentAt, one more than the version counted; 0 not read yet
	low, high float32
	extentAt  uint64
}

// NewRelief is level ground at 0 over grid; over a board, every change to a cell's corners is
// counted on the board too (board.Board.Touch), so whatever was worked out of the cell is read
// anew.
func NewRelief(grid board.Grid) *Relief {
	r := &Relief{grid: grid}
	if b, ok := grid.(*board.Board); ok {
		r.sq, r.square = b.Square()
		r.touched = b.Touch
	} else if sq, ok := grid.(interface {
		Square() (board.SquareShape, bool)
	}); ok {
		r.sq, r.square = sq.Square()
	}
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
		r.values = make([]float32, grid.CellCount())
	}
	return r
}

// Sloped reports whether the ground runs between a cell's corners: a square grid's.
func (r *Relief) Sloped() bool { return r.square }

// Version counts the changes to the heights; it only grows.
func (r *Relief) Version() uint64 { return r.version }

// Extent is the relief's lowest ground, sea level at most, and its highest: the heights a camera
// looks over; read anew only when the heights have changed.
func (r *Relief) Extent() (low, high float64) {
	if r.extentAt != r.version+1 {
		lo, hi := float32(0), float32(0)
		for _, v := range r.values {
			lo, hi = min(lo, v), max(hi, v)
		}
		r.low, r.high, r.extentAt = lo, hi, r.version+1
	}
	return float64(r.low), float64(r.high)
}

// Highest is the top of the relief: the highest ground, for an eye to fly over.
func (r *Relief) Highest() float64 {
	_, high := r.Extent()
	return high
}

// Heights is the ground as its entity carries it: the same values, not a copy.
func (r *Relief) Heights() Heights { return Heights{Values: r.values} }

// adopt takes h's values as the ground — a loaded game's — when they are as many as the grid has.
func (r *Relief) adopt(h Heights) bool {
	if len(h.Values) != len(r.values) {
		return false
	}
	r.values = h.Values
	r.version++
	return true
}

// vertex is a corner of the lattice on a square grid, or a cell's ordinal on any other.
type vertex struct{ x, y int }

// index is v's slot in the values.
func (r *Relief) index(v vertex) int {
	if !r.square {
		return v.x
	}
	return v.y*r.cols + v.x
}

// corner is the vertex at lattice (x, y), folded where the grid wraps; false off the lattice.
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

// corners is the vertices at c's four corners on a square grid, or its one on any other.
func (r *Relief) corners(c board.CellID) (vs [4]vertex, n int, ok bool) {
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
func (r *Relief) Corners(c board.CellID) Corners {
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

// SetCorners puts c's corners at k's heights — the neighbours' corners meeting them go with them,
// the ground has no vertical walls — or a hex cell level at k's first.
func (r *Relief) SetCorners(c board.CellID, k Corners) {
	vs, n, ok := r.corners(c)
	if !ok {
		return
	}
	changed := false
	for i := range n {
		changed = r.setValue(vs[i], k[i]) || changed
	}
	if changed {
		r.version++
	}
}

// setValue puts vertex v at h, telling of every cell meeting there; false when it stood there.
func (r *Relief) setValue(v vertex, h float32) bool {
	i := r.index(v)
	if r.values[i] == h {
		return false
	}
	r.values[i] = h
	if r.touched != nil {
		r.touching(v, func(c board.CellID, _ int) { r.touched(c) })
	}
	return true
}

// Altitude is c's ground level, the mean of its corners.
func (r *Relief) Altitude(c board.CellID) float64 { return r.Corners(c).Level() }

// SetHeights raises the ground to heights: sampled at every corner on a square grid, at every
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
		r.grid.EachCell(func(c board.CellID) {
			if i, ok := r.grid.Ordinal(c); ok {
				changed = r.setValue(vertex{x: i}, float32(heights(r.grid.CellCenter(c)))) || changed
			}
		})
	}
	if changed {
		r.version++
	}
}

// GroundAt is the ground height under p, 0 off the board: read between the cell's corners on a
// square grid, the cell's level elsewhere.
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
	u, v := fx-x0, fy-y0
	return (1-u)*(1-v)*float64(hs[0]) + u*(1-v)*float64(hs[1]) + (1-u)*v*float64(hs[2]) + u*v*float64(hs[3])
}

// foldAxis is a cell coordinate on an axis n cells long: wrapped where it wraps; false off it.
func foldAxis(v, n int64, wraps bool) (int64, bool) {
	if wraps {
		return (v%n + n) % n, true
	}
	return v, v >= 0 && v < n
}

var _ world.Ground = (*Relief)(nil)

// At is GroundAt — the world.Ground contract.
func (r *Relief) At(p geom.Vec) float64 { return r.GroundAt(p) }

// Step is how far apart sight samples the ground: the shorter side of a cell.
func (r *Relief) Step() float64 {
	w, h := r.grid.CellBounds()
	return min(w, h)
}

// Top is c's corners with its kind's height standing on them, and its level: the board.Map
// contract, for kind's Height.
func (r *Relief) Top(c board.CellID, height float64) (corners [4]float32, level float32) {
	k := r.Corners(c)
	for i := range k {
		k[i] += float32(height)
	}
	return k, float32(r.Altitude(c))
}

// touching calls fn with every cell meeting at v and the index of v among its corners, -1 for a
// hex cell, which is v itself.
func (r *Relief) touching(v vertex, fn func(c board.CellID, k int)) {
	if !r.square {
		r.grid.EachCell(func(c board.CellID) { // rare: the shaping of a hex cell
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
			if c, ok := r.grid.CellIndex(uint32(cx), uint32(cy)); ok {
				fn(c, 3-k)
			}
		}
	}
}

// MeanOfCells is a height function for SetHeights built from one height per cell: a point inside
// a cell is at its height, a point where cells meet at the mean of theirs.
func MeanOfCells(grid board.Grid, height func(c board.CellID) float64) func(p geom.Vec) float64 {
	const eps = 1e-6
	return func(p geom.Vec) float64 {
		var seen [4]board.CellID
		n, sum := 0, 0.0
	next:
		for _, d := range [4][2]float64{{-eps, -eps}, {eps, -eps}, {-eps, eps}, {eps, eps}} {
			c, ok := grid.CellAt(geom.NewVec(p.X+d[0], p.Y+d[1]))
			if !ok {
				continue
			}
			for _, s := range seen[:n] {
				if s == c {
					continue next
				}
			}
			seen[n], n, sum = c, n+1, sum+height(c)
		}
		if n == 0 {
			return 0
		}
		return sum / float64(n)
	}
}
