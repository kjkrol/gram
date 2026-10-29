package pathfind

import "github.com/kjkrol/gram/plugins/board/cell"

// what one search has read of a cell, paced for its domain; nothing changes the board while a
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
	hasKind
)

// a kind as a step onto it costs in the search's domain on the level: rate what a unit of
// length costs, the share's inverse; a kind refusing the step admits nothing
type step struct {
	rate   float64
	admits bool
	graded bool
}

func (p *Finder) stepOn(c cell.ID, k cell.Kind, level uint8) step {
	var share float64
	if p.pacing == nil {
		share = cell.PaceOf(nil, cell.Step{From: c, To: c, Level: cell.Level(level), Kind: k,
			Domain: p.domain})
	} else {
		share = p.pacing.Flat(c, k, cell.Level(level), p.domain)
	}
	s := step{admits: k.Admits(p.domain) && share > 0, graded: k.Graded}
	if share > 0 {
		s.rate = 1 / share
	}
	return s
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

// a cell's ground step as read for a domain, good while the board's passage stands: whatever
// changes what a search reads counts in it, as the kept answers rest on
type flatStep struct {
	passage uint64
	domain  cell.Domain
	read    bool
	step    step
}

// r's kind on the ground, under a deck where one is raised over it
func (p *Finder) groundOf(r *read) step {
	if r.has&hasGround == 0 {
		r.ground, r.has = p.groundStep(r.c), r.has|hasGround
	}
	return r.ground
}

// the ground step of c, kept across searches by the board's passage where the terrain has one
func (p *Finder) groundStep(c cell.ID) step {
	o, ok := p.grid.Ordinal(c)
	if !ok || p.passing == nil {
		return p.stepOn(c, p.kindAt(c, ground), ground)
	}
	if len(p.flats) == 0 {
		p.flats = make([]flatStep, p.grid.CellCount())
	}
	f, passage := &p.flats[o], p.passing.Passage()
	if !f.read || f.passage != passage || f.domain != p.domain {
		*f = flatStep{passage: passage, domain: p.domain, read: true,
			step: p.stepOn(c, p.kindAt(c, ground), ground)}
	}
	return f.step
}

func (p *Finder) stepAt(r *read, level uint8) step {
	if level == deck {
		return p.stepOn(r.c, p.kindAt(r.c, deck), deck)
	}
	return p.groundOf(r)
}

func (p *Finder) bareOf(r *read) step {
	if r.has&hasBare == 0 {
		r.bare, r.has = p.stepOn(r.c, p.ways.Bare(r.c), ground), r.has|hasBare
	}
	return r.bare
}

// the kind the strategy paces a step onto r's cell by: the ground's read once a search, kept
// by ordinal; a deck's and the bare ground's read anew, being rare
func (p *Finder) kindOn(r *read, level uint8, bare bool) cell.Kind {
	if bare {
		return p.ways.Bare(r.c)
	}
	if level == deck {
		return p.kindAt(r.c, deck)
	}
	o, ok := p.grid.Ordinal(r.c)
	if !ok {
		return p.kindAt(r.c, ground)
	}
	if p.kinds == nil {
		p.kinds = make([]cell.Kind, p.grid.CellCount())
	}
	if r.has&hasKind == 0 {
		p.kinds[o], r.has = p.kindAt(r.c, ground), r.has|hasKind
	}
	return p.kinds[o]
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
