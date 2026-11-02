package site

import "github.com/kjkrol/gram/plugins/board/cell"

// Steps calls each with every step and leap a road may take from c, come into by way in (-1 at
// the start or into a cell it may level) after the cell prev: the cell it leads to, the way it
// comes into it by (-1 likewise) and what it costs, as the Planner's routes take them.
func (p *Planner) Steps(c cell.ID, in int, prev cell.ID,
	each func(to cell.ID, in int, cost float64)) {
	p.begin()
	p.goal = ^cell.ID(0)
	from := routeNode{c: c, in: int8(in)}
	for _, s := range p.forward(from, routeNode{c: prev, in: routeAny}, nil) {
		each(s.To.c, int(s.To.in), s.Cost)
	}
}
