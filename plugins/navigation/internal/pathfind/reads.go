package pathfind

import "github.com/kjkrol/gram/plugins/board/cell"

// what one search has read of a cell, priced for its domain; nothing changes the board while a
// search runs, so a cell is read once a search however many steps ask about it
type read struct {
	c       cell.ID
	search  uint32
	has     uint8
	carries cell.Domain
	ground  step
	bare    step
	top     float32
	under   float32
}

const (
	hasGround uint8 = 1 << iota
	hasBare
	hasDeck
	raisedDeck
)

// a kind as a step onto it costs in the search's domain
type step struct {
	cost   float64
	admits bool
	graded bool
}

func stepOn(k cell.Kind, d cell.Domain) step {
	return step{cost: k.CostFor(d), admits: k.Admits(d), graded: k.Graded}
}

// begin starts a search in domain: what the last one read holds no more.
func (p *Finder) begin(domain cell.Domain) {
	p.domain = domain
	if p.search++; p.search == 0 {
		clear(p.reads)
		p.search = 1
	}
}

// at is what this search has read of c; a cell off the grid is read anew every time.
func (p *Finder) at(c cell.ID) *read {
	o, ok := p.grid.Ordinal(c)
	if ok && len(p.reads) == 0 {
		p.reads = make([]read, p.grid.CellCount())
	}
	if !ok || o >= len(p.reads) {
		return &read{c: c, search: p.search}
	}
	r := &p.reads[o]
	if r.search != p.search {
		*r = read{c: c, search: p.search}
	}
	return r
}

// r's kind on the ground, under a deck where one is raised over it
func (p *Finder) groundOf(r *read) step {
	if r.has&hasGround == 0 {
		r.ground, r.has = stepOn(p.kindAt(r.c, ground), p.domain), r.has|hasGround
	}
	return r.ground
}

func (p *Finder) stepAt(r *read, level uint8) step {
	if level == deck {
		return stepOn(p.kindAt(r.c, deck), p.domain)
	}
	return p.groundOf(r)
}

func (p *Finder) bareOf(r *read) step {
	if r.has&hasBare == 0 {
		r.bare, r.has = stepOn(p.ways.Bare(r.c), p.domain), r.has|hasBare
	}
	return r.bare
}

// Levels' Deck of r's cell
func (p *Finder) deckOf(r *read) (top, under float32, carries cell.Domain, ok bool) {
	if r.has&hasDeck == 0 {
		r.has |= hasDeck
		if r.top, r.under, r.carries, ok = p.levels.Deck(r.c); ok {
			r.has |= raisedDeck
		}
	}
	return r.top, r.under, r.carries, r.has&raisedDeck != 0
}

func (p *Finder) raised(r *read) bool {
	_, _, _, ok := p.deckOf(r)
	return ok
}
