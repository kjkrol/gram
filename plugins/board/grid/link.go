package grid

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Link is the bit of cell.Links a Way runs on from from to its neighbour to by; false when to is
// not its neighbour.
func Link(g Grid, from, to cell.ID) (cell.Links, bool) {
	if i, ok := Way(g, from, to); ok {
		return 1 << i, true
	}
	return 0, false
}

// Way is the direction from from to its neighbour to; false when to is not its neighbour.
func Way(g Grid, from, to cell.ID) (int, bool) {
	for i := range g.Directions() {
		if n, ok := g.Toward(from, i); ok && n == to {
			return i, true
		}
	}
	return 0, false
}

// Straight is the direction links run straight through a cell by, out of it and back: false for
// a bend, a fork or an end.
func Straight(g Grid, links cell.Links) (int, bool) {
	for i := range g.Directions() {
		if links == 1<<i|1<<g.Opposite(i) {
			return i, true
		}
	}
	return 0, false
}

// Direction is the direction whose Offset is da, db; false when none is.
func Direction(g Grid, da, db int) (int, bool) {
	for i := range g.Directions() {
		if a, b := g.Offset(i); a == da && b == db {
			return i, true
		}
	}
	return 0, false
}

// Nearest is the direction whose Step runs nearest the way v points.
func Nearest(g Grid, v geom.Vec) int {
	best, most := 0, f32.Inf(-1)
	for i := range g.Directions() {
		s := g.Step(i)
		if d := (s.X*v.X + s.Y*v.Y) / f32.Hypot(s.X, s.Y); d > most {
			best, most = i, d
		}
	}
	return best
}

// Exit is where direction i leaves c: the middle of the edge it crosses, or the corner it passes.
func Exit(g Grid, c cell.ID, i int) geom.Vec {
	m, s := g.CellCenter(c), g.Step(i)
	return geom.NewVec(m.X+s.X/2, m.Y+s.Y/2)
}

// Rim is which of c's CellOutline corners direction i leaves by: the two ends of the edge it
// crosses, or the one corner it passes (a == b).
func Rim(g Grid, c cell.ID, i int) (a, b int) {
	var buf [8]geom.Vec
	corners := g.CellOutline(c, buf[:0])
	e := Exit(g, c, i)
	far := func(k int) float32 { return f32.Hypot(corners[k].X-e.X, corners[k].Y-e.Y) }
	a, b = 0, -1
	for k := range corners {
		switch {
		case far(k) < far(a):
			a, b = k, a
		case k != a && (b < 0 || far(k) < far(b)):
			b = k
		}
	}
	if g.Corner(i) {
		return a, a
	}
	return a, b
}
