package pathfind

import "github.com/kjkrol/gram/plugins/board/cell"

// what one search has read of a cell, paced for its domain; nothing changes the board while a
// search runs, so a cell is read once a search however many steps ask about it
type read struct {
	c      cell.ID
	search uint32
	has    uint32
	ground step
	bare   step
	// the storeys making a level: bit s storey s, as the levels say and as they carry the
	// search's domain; their floors are kept apart (Finder.floors), a read being cleared a node
	storeys, sunk uint8
	levels        uint8
}

const (
	hasGround uint32 = 1 << iota
	hasBare
	hasStoreys
	hasKind
	// the limits read on a level, a bit a level
	hasLimits
	// some limit on a level, a bit a level
	limited = hasLimits << cell.Levels
	// a railing asked of on a level, a bit a level
	hasRailing = limited << cell.Levels
	// a railing along the cell's band on a level, a bit a level
	railing = hasRailing << cell.Levels
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
	p.limited = p.limits != nil && p.limits.Limited()
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
	if level != ground {
		return p.stepOn(r.c, p.kindAt(r.c, level), level)
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
	if level != ground {
		return p.kindAt(r.c, level)
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

// storeysOf reads r's cell's storeys once a search
func (p *Finder) storeysOf(r *read) {
	if r.has&hasStoreys != 0 {
		return
	}
	r.has |= hasStoreys
	// into the Finder's own: a local handed to the levels would be allocated a cell
	floors := &p.offFloors
	if r.storeys, r.sunk = p.levels.Storeys(r.c, floors); r.storeys == 0 {
		return
	}
	*p.floorsOf(r.c) = *floors
	for s := range uint8(cell.Storeys) {
		if r.storeys&(1<<s) != 0 && floors[s].Carries&p.domain != 0 {
			r.levels |= 1 << s
		}
	}
}

// floorsOf are the floors of c's storeys as last read, by ordinal; a cell off the grid shares
// one set, read and used at once
func (p *Finder) floorsOf(c cell.ID) *[cell.Storeys]Storey {
	o, ok := p.grid.Ordinal(c)
	if !ok {
		return &p.offFloors
	}
	if len(p.floors) == 0 {
		p.floors = make([][cell.Storeys]Storey, p.grid.CellCount())
	}
	return &p.floors[o]
}

// floorOf is the storey making r's cell's level, one of levelsOf's past the ground
func (p *Finder) floorOf(r *read, level uint8) *Storey { return &p.floorsOf(r.c)[level-1] }

// sunken says whether r's cell's level lies under the ground
func (p *Finder) sunken(r *read, level uint8) bool {
	return level != ground && r.sunk&(1<<(level-1)) != 0
}

// raised says whether a storey is raised over r's cell
func (p *Finder) raised(r *read) bool {
	if p.storeysOf(r); r.storeys == 0 {
		return false
	}
	return r.storeys&^r.sunk != 0
}

// lowest is the underside of the lowest storey raised over r's cell; false where none is
func (p *Finder) lowest(r *read) (under float32, ok bool) {
	p.storeysOf(r)
	floors := p.floorsOf(r.c)
	for s := range uint8(cell.Storeys) {
		if r.storeys&^r.sunk&(1<<s) != 0 && (!ok || floors[s].Bound < under) {
			under, ok = floors[s].Bound, true
		}
	}
	return under, ok
}
