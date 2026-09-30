package relief

import (
	"math"
	"time"

	"github.com/kjkrol/goke/v3"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
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

// Shaper carries out the shaping commands — Raise, Lower, Level — on a relief as its System runs,
// moving the ground as its Shaping says; its Queues take them.
type Shaper struct {
	relief *Relief
	cfg    Shaping
	raise  control.Queue[Raise]
	lower  control.Queue[Lower]
	level  control.Queue[Level]
}

// NewShaper is the shaping of r as cfg says.
func NewShaper(r *Relief, cfg Shaping) *Shaper { return &Shaper{relief: r, cfg: cfg} }

// Queues are the shaping commands' queues: Raise, Lower, Level.
func (s *Shaper) Queues() []control.CommandQueue {
	return []control.CommandQueue{&s.raise, &s.lower, &s.level}
}

// System is the shaper as a goke.System, run once a tick in the interface part of a plan.
func (s *Shaper) System() goke.System { return shapingSystem{s} }

var _ goke.System = shapingSystem{}

// shapingSystem carries out the shaping commands as they come.
type shapingSystem struct{ s *Shaper }

func (shapingSystem) Init(*goke.SysInit) {}

func (y shapingSystem) Update(*goke.CmdBuf, time.Duration) {
	s, r := y.s, y.s.relief
	s.raise.Drain(func(i control.Issued[Raise]) { r.Lift(i.Command.At, s.cfg.Step, s.cfg.MaxStep) })
	s.lower.Drain(func(i control.Issued[Lower]) { r.Lift(i.Command.At, -s.cfg.Step, s.cfg.MaxStep) })
	s.level.Drain(func(i control.Issued[Level]) { r.Flatten(i.Command.From, i.Command.To, s.cfg.MaxStep) })
}

// shapeTolerance keeps float32 heights from chasing a slope limit they cannot store exactly.
const shapeTolerance = 1e-3

// Lift moves the ground at p by `by`, then the ground round it until no two neighbouring corners
// differ by more than maxStep (zero: any).
func (r *Relief) Lift(p geom.Vec, by, maxStep float64) {
	v, ok := r.vertexAt(p)
	if !ok || by == 0 {
		return
	}
	if r.setValue(v, float32(r.heightAt(v)+by)) {
		r.settle([]vertex{v}, maxStep)
		r.version++
	}
}

// Flatten brings the ground between from and to to the height at from, then the ground round it
// within maxStep, as Lift.
func (r *Relief) Flatten(from, to geom.Vec, maxStep float64) {
	v0, ok := r.vertexAt(from)
	if !ok {
		return
	}
	h := float32(r.heightAt(v0))
	var set []vertex
	if !r.square {
		lo, hi := geom.NewVec(min(from.X, to.X), min(from.Y, to.Y)), geom.NewVec(max(from.X, to.X), max(from.Y, to.Y))
		r.grid.CellsUnder(geom.NewAABB(lo, hi), func(c board.CellID) {
			if i, ok := r.grid.Ordinal(c); ok {
				if v := (vertex{x: i}); r.setValue(v, h) {
					set = append(set, v)
				}
			}
		})
	} else {
		x0, y0 := r.lattice(from)
		x1, y1 := r.lattice(to)
		for y := min(y0, y1); y <= max(y0, y1); y++ {
			for x := min(x0, x1); x <= max(x0, x1); x++ {
				if v, ok := r.corner(x, y); ok && r.setValue(v, h) {
					set = append(set, v)
				}
			}
		}
	}
	if len(set) > 0 {
		r.settle(set, maxStep)
		r.version++
	}
}

// settle walks out from the vertices changed, pulling each neighbour to within maxStep.
func (r *Relief) settle(queue []vertex, maxStep float64) {
	if maxStep <= 0 {
		return
	}
	var ns []vertex
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		h := r.heightAt(v)
		ns = r.vertexNeighbours(v, ns[:0])
		for _, n := range ns {
			hn, want := r.heightAt(n), 0.0
			switch {
			case hn < h-maxStep-shapeTolerance:
				want = h - maxStep
			case hn > h+maxStep+shapeTolerance:
				want = h + maxStep
			default:
				continue
			}
			if r.setValue(n, float32(want)) {
				queue = append(queue, n)
			}
		}
	}
}

// lattice is the square grid's corner nearest p, unfolded.
func (r *Relief) lattice(p geom.Vec) (x, y int64) {
	w, h := r.grid.CellBounds()
	return int64(math.Round(p.X / w)), int64(math.Round(p.Y / h))
}

// vertexAt is the vertex shaped for p: the nearest corner, or the cell under p on a hex grid.
func (r *Relief) vertexAt(p geom.Vec) (vertex, bool) {
	if !r.square {
		c, ok := r.grid.CellAt(p)
		if !ok {
			return vertex{}, false
		}
		i, ok := r.grid.Ordinal(c)
		return vertex{x: i}, ok
	}
	return r.corner(r.lattice(p))
}

func (r *Relief) heightAt(v vertex) float64 { return float64(r.values[r.index(v)]) }

// vertexNeighbours lists the vertices one edge away from v.
func (r *Relief) vertexNeighbours(v vertex, dst []vertex) []vertex {
	if !r.square {
		var cell board.CellID
		found := false
		r.grid.EachCell(func(c board.CellID) {
			if i, ok := r.grid.Ordinal(c); ok && i == v.x {
				cell, found = c, true
			}
		})
		if !found {
			return dst
		}
		for _, n := range r.grid.Neighbors(cell) {
			if i, ok := r.grid.Ordinal(n); ok {
				dst = append(dst, vertex{x: i})
			}
		}
		return dst
	}
	for _, d := range [4][2]int64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		if n, ok := r.corner(int64(v.x)+d[0], int64(v.y)+d[1]); ok {
			dst = append(dst, n)
		}
	}
	return dst
}
