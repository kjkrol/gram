package board

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
)

// Raise lifts the ground at At by one Shaping.Step: the nearest corner on a square grid, the cell
// on a hex one.
type Raise struct{ At geom.Vec }

// Lower sinks the ground at At by one Shaping.Step, as Raise lifts it.
type Lower struct{ At geom.Vec }

// Level brings the ground between From and To to the height at From.
type Level struct{ From, To geom.Vec }

// Shaping is how Raise, Lower and Level move the ground: Step per Raise or Lower, and at most
// MaxStep between two corners along a cell's edge (two neighbouring cells on a hex grid), the
// ground round about following as in Transport Tycoon. MaxStep zero lets any slope stand.
type Shaping struct{ Step, MaxStep float64 }

// shaping holds the shaping commands until the cell system carries them out.
type shaping struct {
	cfg   Shaping
	raise control.Queue[Raise]
	lower control.Queue[Lower]
	level control.Queue[Level]
}

func (s *shaping) run(b *Board) {
	s.raise.Drain(func(i control.Issued[Raise]) { b.Lift(i.Command.At, s.cfg.Step, s.cfg.MaxStep) })
	s.lower.Drain(func(i control.Issued[Lower]) { b.Lift(i.Command.At, -s.cfg.Step, s.cfg.MaxStep) })
	s.level.Drain(func(i control.Issued[Level]) { b.Flatten(i.Command.From, i.Command.To, s.cfg.MaxStep) })
}

// vertex is a point the ground is shaped at: a lattice corner (x, y) of a square grid, or a hex
// cell's id in x.
type vertex struct{ x, y int64 }

// shapeTolerance keeps float32 heights from chasing a slope limit they cannot store exactly.
const shapeTolerance = 1e-3

// Lift moves the ground at p by `by`, then the ground round it until no two neighbouring corners
// differ by more than maxStep (zero: any).
func (b *Board) Lift(p geom.Vec, by, maxStep float64) {
	v, ok := b.vertexAt(p)
	if !ok || by == 0 {
		return
	}
	if b.setHeightAt(v, b.heightAt(v)+by) {
		b.settle([]vertex{v}, maxStep)
		b.version++
	}
}

// Flatten brings the ground between from and to to the height at from, then the ground round it
// within maxStep, as Lift.
func (b *Board) Flatten(from, to geom.Vec, maxStep float64) {
	v0, ok := b.vertexAt(from)
	if !ok {
		return
	}
	h := b.heightAt(v0)
	var set []vertex
	if !b.sloped() {
		lo, hi := geom.NewVec(min(from.X, to.X), min(from.Y, to.Y)), geom.NewVec(max(from.X, to.X), max(from.Y, to.Y))
		b.CellsUnder(geom.NewAABB(lo, hi), func(c CellID) {
			if v := (vertex{x: int64(c)}); b.setHeightAt(v, h) {
				set = append(set, v)
			}
		})
	} else {
		x0, y0 := b.lattice(from)
		x1, y1 := b.lattice(to)
		for y := min(y0, y1); y <= max(y0, y1); y++ {
			for x := min(x0, x1); x <= max(x0, x1); x++ {
				if v, ok := b.foldVertex(x, y); ok && b.setHeightAt(v, h) {
					set = append(set, v)
				}
			}
		}
	}
	if len(set) > 0 {
		b.settle(set, maxStep)
		b.version++
	}
}

// settle walks out from the vertices changed, pulling each neighbour to within maxStep.
func (b *Board) settle(queue []vertex, maxStep float64) {
	if maxStep <= 0 {
		return
	}
	var ns []vertex
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		h := b.heightAt(v)
		ns = b.vertexNeighbours(v, ns[:0])
		for _, n := range ns {
			hn, want := b.heightAt(n), 0.0
			switch {
			case hn < h-maxStep-shapeTolerance:
				want = h - maxStep
			case hn > h+maxStep+shapeTolerance:
				want = h + maxStep
			default:
				continue
			}
			if b.setHeightAt(n, want) {
				queue = append(queue, n)
			}
		}
	}
}

// lattice is the square grid's corner nearest p, unfolded.
func (b *Board) lattice(p geom.Vec) (x, y int64) {
	w, h := b.CellBounds()
	return int64(math.Round(p.X / w)), int64(math.Round(p.Y / h))
}

// vertexAt is the vertex shaped for p: the nearest corner, or the cell under p on a hex grid.
func (b *Board) vertexAt(p geom.Vec) (vertex, bool) {
	if !b.sloped() {
		c, ok := b.CellAt(p)
		return vertex{x: int64(c)}, ok
	}
	return b.foldVertex(b.lattice(p))
}

// foldVertex puts a square grid's corner (x, y) on the board: wrapped where the grid wraps.
func (b *Board) foldVertex(x, y int64) (vertex, bool) {
	sq := b.square
	fold := func(v, size int64, wraps bool) (int64, bool) {
		if wraps {
			return wrapModI64(v, size), true
		}
		return v, v >= 0 && v <= size
	}
	x, okX := fold(x, int64(sq.Width), sq.WrapX)
	y, okY := fold(y, int64(sq.Height), sq.WrapY)
	return vertex{x, y}, okX && okY
}

// touching calls fn with every cell meeting at v and the index of v among its corners, -1 for a
// hex cell, which is v itself.
func (b *Board) touching(v vertex, fn func(c CellID, k int)) {
	if !b.sloped() {
		fn(CellID(v.x), -1)
		return
	}
	sq := b.square
	for k, d := range [4][2]int64{{0, 0}, {-1, 0}, {0, -1}, {-1, -1}} {
		cx, okX := foldAxis(v.x+d[0], int64(sq.Width), sq.WrapX)
		cy, okY := foldAxis(v.y+d[1], int64(sq.Height), sq.WrapY)
		if okX && okY {
			fn(sq.idAt(uint32(cx), uint32(cy)), k)
		}
	}
}

func (b *Board) heightAt(v vertex) float64 {
	h, found := 0.0, false
	b.touching(v, func(c CellID, k int) {
		if !found {
			h, found = float64(b.Relief(c).Corners[max(k, 0)]), true
		}
	})
	return h
}

func (b *Board) setHeightAt(v vertex, h float64) bool {
	changed := false
	b.touching(v, func(c CellID, k int) {
		r := b.Relief(c)
		if k < 0 {
			r.Corners = [4]float32{float32(h), float32(h), float32(h), float32(h)}
		} else {
			r.Corners[k] = float32(h)
		}
		changed = b.setRelief(c, r) || changed
	})
	return changed
}

// vertexNeighbours lists the vertices one edge away from v.
func (b *Board) vertexNeighbours(v vertex, dst []vertex) []vertex {
	if !b.sloped() {
		for _, n := range b.Neighbors(CellID(v.x)) {
			dst = append(dst, vertex{x: int64(n)})
		}
		return dst
	}
	for _, d := range [4][2]int64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		if n, ok := b.foldVertex(v.x+d[0], v.y+d[1]); ok {
			dst = append(dst, n)
		}
	}
	return dst
}
