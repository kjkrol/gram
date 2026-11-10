package pathfind

import "github.com/kjkrol/gram/plugins/board/cell"

// Ended is the level of its goal the last route Find found ended on.
func (p *Finder) Ended() Level { return On(cell.Level(p.ended)) }

// Decks says which steps of the last route found lie on the first storey, bit k the k-th.
func (p *Finder) Decks() uint64 { return p.decks }
