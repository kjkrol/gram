package board

import (
	"fmt"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Crossing is what crosses c over its Way; the zero Crossing off the board or where nothing does.
func (b *Board) Crossing(c cell.ID) cell.Crossing {
	if b.cells == nil {
		return b.seed.Crossings[c]
	}
	i, ok := b.ordinal(c)
	if !ok {
		return cell.Crossing{}
	}
	return *b.crossingOf(i)
}

// SetCrossing lays x across c over its Way, the zero Crossing taking what crossed there away; it
// takes effect immediately.
func (b *Board) SetCrossing(c cell.ID, x cell.Crossing) {
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
func (b *Board) crossingOf(i int) *cell.Crossing {
	st := b.cells
	if !st.crossings.SeekH(st.ids[i]) && !st.crossings.Seek(st.ids[i]) {
		panic(fmt.Sprintf("board: cell entity %d is gone", st.ids[i]))
	}
	return st.crossing.At(st.crossings.Cursor())
}
