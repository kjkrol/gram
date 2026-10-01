package board

import (
	"fmt"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Along reports whether a way runs from from to its neighbour to, so a step between them goes
// along it rather than over the ground beside it: the way or the crossing across from links
// towards to, or to's back towards from.
func (b *Board) Along(from, to cell.ID) bool {
	out, ok := Link(b.Grid, from, to)
	if !ok {
		return false
	}
	back, _ := Link(b.Grid, to, from)
	return b.Way(from).Links&out != 0 || b.Way(to).Links&back != 0 ||
		b.Crossing(from).Links&out != 0 || b.Crossing(to).Links&back != 0
}

// Link is the bit of Links a Way runs on from from to its neighbour to by; false when to is not
// its neighbour, or on a grid neither square nor hex.
func Link(g Grid, from, to cell.ID) (cell.Links, bool) {
	for i := range 8 {
		if n, ok := Toward(g, from, i); ok && n == to {
			return 1 << i, true
		}
	}
	return 0, false
}

// Toward is c's neighbour in the grid's i-th direction, across a seam where the grid wraps; false
// off the grid, past its directions, or on a grid neither square nor hex.
func Toward(g Grid, c cell.ID, i int) (cell.ID, bool) {
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
func (b *Board) Way(c cell.ID) cell.Way {
	if b.cells == nil {
		return b.seed.Ways[c]
	}
	i, ok := b.ordinal(c)
	if !ok {
		return cell.Way{}
	}
	return *b.wayOf(i)
}

// SetWay lays w across c, the zero Way taking what ran there away; it takes effect immediately.
func (b *Board) SetWay(c cell.ID, w cell.Way) {
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
func (b *Board) wayOf(i int) *cell.Way {
	st := b.cells
	if !st.ways.SeekH(st.ids[i]) && !st.ways.Seek(st.ids[i]) {
		panic(fmt.Sprintf("board: cell entity %d is gone", st.ids[i]))
	}
	return st.way.At(st.ways.Cursor())
}
