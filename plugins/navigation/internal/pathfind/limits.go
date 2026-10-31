package pathfind

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// limits says what limits a body on a cell beyond its kind, as a board's read does: how much its
// floor bears, the gap in its wall
type limits interface {
	Limited() bool
	Limits(c cell.ID, d cell.Domain, deck bool) (load float32, gap cell.Gap)
}

// limitsOf are what r's cell limits a body on level by, asked once a search where nothing does
func (p *Finder) limitsOf(r *read, level uint8) (load float32, gap cell.Gap) {
	read, some := hasLimits<<level, limited<<level
	if r.has&read != 0 && r.has&some == 0 {
		return 0, cell.Gap{}
	}
	load, gap = p.limits.Limits(r.c, p.domain, level == deck)
	if r.has |= read; load > 0 || gap.Width > 0 {
		r.has |= some
	}
	return load, gap
}

// within says whether body may take the step from from, rf its cell, onto rn's cell on level by
// what the cells limit beyond their kinds: the floor it steps onto bears it, and the gaps in the
// walls it leaves and enters let it through across the step, either way alike
func (p *Finder) within(from node, rf, rn *read, level uint8, body Body) bool {
	load, gap := p.limitsOf(rn, level)
	if load > 0 && body.Weight > load || !p.through(gap, from.c, rn.c, body) {
		return false
	}
	_, gap = p.limitsOf(rf, from.level)
	return p.through(gap, from.c, rn.c, body)
}

// bears says whether the body searched for may stand on r's cell on level by what limits it
// there: the floor bears it, and the wall's gap lets it through whichever way it turns. Small
// enough to inline, it costs a board limiting nothing a branch.
func (p *Finder) bears(r *read, level uint8) bool { return !p.limited || p.borne(r, level) }

func (p *Finder) borne(r *read, level uint8) bool {
	load, gap := p.limitsOf(r, level)
	return (load <= 0 || p.body.Weight <= load) && p.through(gap, r.c, r.c, p.body)
}

// through says whether gap lets body through, as tall as it is and as wide across the step from
// from to its neighbour to, its narrower side where from is to; a zero gap is none to pass
func (p *Finder) through(gap cell.Gap, from, to cell.ID, body Body) bool {
	switch {
	case gap.Width <= 0:
		return true
	case gap.Height > 0 && body.Height > gap.Height:
		return false
	}
	if way, ok := grid.Way(p.grid, from, to); ok && from != to {
		return breadth(p.grid.Step(way), body.Size) <= gap.Width
	}
	return min(body.Size.X, body.Size.Y) <= gap.Width
}
