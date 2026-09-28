package board

import "fmt"

// Crossing is what crosses a cell over its Way — a bridge over a river, a footbridge over a
// stream: a Way of its own, a band from the cell's middle out towards each neighbour its Links
// name. Its Kind lets whoever it admits over the cell too, at its cost, and leaves the way under it
// as it is: the water runs on under a bridge. The zero Crossing is none.
type Crossing struct{ Way }

// Over is under — the kind the ground and its Way make — as whoever crosses the cell meets it with
// the Crossing over it: it admits whoever either does, at the Crossing's cost for whoever it
// admits.
func (c Crossing) Over(under CellKind) CellKind {
	if !c.Runs() {
		return under
	}
	under.Allows |= c.Kind.Allows
	for i := range under.Costs {
		if d := Domain(1 << i); c.Kind.Allows&d != 0 {
			under.Costs[i] = c.Kind.CostFor(d)
		}
	}
	under.Graded = under.Graded || c.Kind.Graded
	return under
}

// Crossing is what crosses c over its Way; the zero Crossing off the board or where nothing does.
func (b *Board) Crossing(c CellID) Crossing {
	if b.cells == nil {
		return b.seed.Crossings[c]
	}
	i, ok := b.ordinal(c)
	if !ok {
		return Crossing{}
	}
	return *b.crossingOf(i)
}

// SetCrossing lays x across c over its Way, the zero Crossing taking what crossed there away; it
// takes effect immediately.
func (b *Board) SetCrossing(c CellID, x Crossing) {
	if b.cells == nil {
		before := b.seed.Version()
		b.seed.SetCrossing(c, x)
		if b.seed.Version() != before {
			b.version++
			b.touch(c)
		}
		return
	}
	i, ok := b.ordinal(c)
	if !ok {
		return
	}
	if p := b.crossingOf(i); *p != x {
		*p = x
		b.version++
		b.touch(c)
	}
}

// crossingOf is the i-th cell's Crossing, in place.
func (b *Board) crossingOf(i int) *Crossing {
	st := b.cells
	if !st.crossings.SeekH(st.ids[i]) && !st.crossings.Seek(st.ids[i]) {
		panic(fmt.Sprintf("board: cell entity %d is gone", st.ids[i]))
	}
	return st.crossing.At(st.crossings.Cursor())
}
