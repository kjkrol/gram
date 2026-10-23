package painter

import "github.com/kjkrol/gram/plugins/board/cell"

// base is the top c is drawn in: its own, or that of the first neighbour lying under the rest it
// runs into
func (l *Painter) base(c cell.ID) *cellTop {
	mine := l.topOf(c)
	if !l.square || mine.under || mine.raised || mine.spread <= 0 {
		return mine
	}
	for d := range l.board.Directions() {
		if n, ok := l.board.Toward(c, d); ok && l.topOf(n).under {
			return l.topOf(n)
		}
	}
	return mine
}
