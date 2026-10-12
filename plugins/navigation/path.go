package navigation

import "github.com/kjkrol/gram/plugins/board/cell"

// MaxPathLength bounds Path.Steps — a route longer than this is fetched in successive chunks.
const MaxPathLength = 64

// Path is the cached route toward MoveOrder.Target, consumed step by step by
// NavigationSystem — Length 0 means "not computed yet".
type Path struct {
	Steps  [MaxPathLength]cell.ID
	Length uint16
	Index  uint16
}

// through reports whether the route goes on past c, the step it is on: c met before the route's
// end, as a route to the ground under a deck may cross the deck over its goal first.
func (p *Path) through(c cell.ID) bool {
	return p.Index+1 < p.Length && p.Steps[p.Index] == c
}
