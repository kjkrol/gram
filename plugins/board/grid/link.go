package grid

import "github.com/kjkrol/gram/plugins/board/cell"

// Link is the bit of cell.Links a Way runs on from from to its neighbour to by; false when to is
// not its neighbour.
func Link(g Grid, from, to cell.ID) (cell.Links, bool) {
	for i := range 8 {
		if n, ok := g.Toward(from, i); ok && n == to {
			return 1 << i, true
		}
	}
	return 0, false
}
