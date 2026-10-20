package pathfind

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// a breadth-first walk keeping its queue and what it has seen for the next one
type rings struct {
	queue []cell.ID
	seen  map[cell.ID]bool
}

// breadthFirst is the first cell from start, ring by ring, that match takes, crossing only the
// cells expand takes, within maxVisited cells.
func (r *rings) breadthFirst(start cell.ID, g grid.Grid, expand, match func(cell.ID) bool,
	maxVisited int) (cell.ID, bool) {
	if r.seen == nil {
		r.seen = map[cell.ID]bool{}
	}
	clear(r.seen)
	r.queue = append(r.queue[:0], start)
	r.seen[start] = true
	for head := 0; head < len(r.queue) && maxVisited > 0; head++ {
		c := r.queue[head]
		maxVisited--
		if match(c) {
			return c, true
		}
		for i := range g.Directions() {
			if n, ok := g.Toward(c, i); ok && !r.seen[n] && expand(n) {
				r.seen[n] = true
				r.queue = append(r.queue, n)
			}
		}
	}
	return 0, false
}
