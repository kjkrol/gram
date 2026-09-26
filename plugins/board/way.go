package board

import "fmt"

// Way is what runs across a cell over its ground — a stream, a river, a road: a band Width wide
// from the cell's middle out towards each neighbour its Links name. Its Kind decides who may cross
// the cell and what it costs there, and how the band looks; the ground keeps the rest — whether it
// is solid, what it veils. Fade is how far it has faded out across the cell, 0 not at all to 1
// gone: a river running out into the sea. The zero Way is none.
type Way struct {
	Kind  CellKind
	Width float32
	Links Links
	Fade  float32
}

// Links is which ways a Way runs on out of its cell: bit i the grid's i-th direction — on a square
// grid north, south, west, east, north-west, north-east, south-west, south-east; on a hex grid its
// six in turn. Link finds the bit for a neighbour, Toward the neighbour for a bit.
type Links uint8

// Runs reports whether anything runs across the cell.
func (w Way) Runs() bool { return w.Width > 0 }

// Over is ground as whoever crosses the cell meets it: who may and what it costs from the Way's
// kind when one runs there, the rest from the ground.
func (w Way) Over(ground CellKind) CellKind {
	if !w.Runs() {
		return ground
	}
	ground.Allows, ground.Cost, ground.Costs = w.Kind.Allows, w.Kind.Cost, w.Kind.Costs
	return ground
}

// Link is the bit of Links a Way runs on from from to its neighbour to by; false when to is not
// its neighbour, or on a grid neither square nor hex.
func Link(g Grid, from, to CellID) (Links, bool) {
	for i := range 8 {
		if n, ok := Toward(g, from, i); ok && n == to {
			return 1 << i, true
		}
	}
	return 0, false
}

// Toward is c's neighbour in the grid's i-th direction, across a seam where the grid wraps; false
// off the grid, past its directions, or on a grid neither square nor hex.
func Toward(g Grid, c CellID, i int) (CellID, bool) {
	switch gr := g.(type) {
	case *Board:
		return Toward(gr.Grid, c, i)
	case *squareGrid:
		if i < 0 || i >= len(squareDirs) {
			return 0, false
		}
		x, y := gr.cellXY(c)
		nx, okX := foldAxis(int64(x)+squareDirs[i][0], int64(gr.Width), gr.WrapX)
		ny, okY := foldAxis(int64(y)+squareDirs[i][1], int64(gr.Height), gr.WrapY)
		if !okX || !okY {
			return 0, false
		}
		return gr.idAt(uint32(nx), uint32(ny)), true
	case *hexGrid:
		if i < 0 || i >= len(hexDirs) {
			return 0, false
		}
		q, r := unpackAxial(c)
		nq, nr, ok := gr.foldAxial(q+hexDirs[i][0], r+hexDirs[i][1])
		return packAxial(nq, nr), ok
	}
	return 0, false
}

// Way is what runs across c; the zero Way off the board or where nothing does.
func (b *Board) Way(c CellID) Way {
	if b.cells == nil {
		return b.seed.Ways[c]
	}
	i, ok := b.ordinal(c)
	if !ok {
		return Way{}
	}
	return *b.wayOf(i)
}

// SetWay lays w across c, the zero Way taking what ran there away; it takes effect immediately.
func (b *Board) SetWay(c CellID, w Way) {
	if b.cells == nil {
		before := b.seed.Version()
		b.seed.SetWay(c, w)
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
	if p := b.wayOf(i); *p != w {
		*p = w
		b.version++
		b.touch(c)
	}
}

// wayOf is the i-th cell's Way, in place.
func (b *Board) wayOf(i int) *Way {
	st := b.cells
	if !st.ways.SeekH(st.ids[i]) && !st.ways.Seek(st.ids[i]) {
		panic(fmt.Sprintf("board: cell entity %d is gone", st.ids[i]))
	}
	return st.way.At(st.ways.Cursor())
}
