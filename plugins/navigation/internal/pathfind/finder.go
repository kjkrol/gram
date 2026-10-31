package pathfind

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/hpastar"
	"github.com/kjkrol/hpastar/graph"
	"github.com/kjkrol/uid"
)

// Finder computes routes over one grid, reusing its A* solver across calls; build one and share
// it.
type Finder struct {
	grid       grid.Grid
	terrain    cell.Terrain
	occupancy  cell.Occupancy
	pacing     Pacing
	sloped     bool        // the pacing feels the slope: a step is paced by the strategy
	kinds      []cell.Kind // by ordinal, the ground kind read for a step over a relief
	passing    passing     // nil: the terrain counts no changes, a cell is read every search
	flats      []flatStep  // by ordinal, the ground steps kept across searches
	searches   uint64
	fills      uint64
	walks      uint64
	expansions uint64
	frontier   frontier
	seeded     []uint32 // by ordinal, seeding where a fill's seed has yet to be reached
	seeding    uint32
	ways       ways   // nil: no ways run
	built      built  // nil: nothing stands
	lanes      lanes  // nil: no band narrows a step
	limits     limits // nil: nothing limits a step beyond the kinds
	levels     Levels // nil: every cell one level
	plain      *hpastar.Search[cell.ID]
	leveled    *hpastar.Search[node]
	stepped    graph.Transitions[node]
	blocked    func(cell.ID) bool // nil: none
	entity     uid.UID64
	domain     cell.Domain
	limited    bool // something limits a step on the board as the search began
	goal       cell.ID
	end        Level
	ended      uint8
	decks      uint64
	body       Body
	reads      []read // by ordinal
	search     uint32
	rings      rings
	// what the searches last wrote, from the start, written over by the next search
	path  []cell.ID
	route []node
	// the hierarchies' last: ahead of the fields a fill reads, they made it 1.5% slower
	climbs, builds uint64
	climbing       climbing
	// the footprints of the bodies bigger than a cell searched for, after them: the body searched
	// for's first, nil for one no bigger than a cell
	print       *footprint
	spanned     *hpastar.Search[cell.ID] // plain's for a body bigger than a cell
	prints      []*footprint
	classPrints [MaxClasses]*footprint // by class of the hierarchy built
	asked       uint64
	railings    railings     // nil: no railing runs on the board
	cells       cellPassages // nil: the terrain says not which cells changed
	windows     [][]outward  // by side twice, plus whether turned
	ball, ring  []cell.ID
	ringOn      []bool
	scan        scanned
	laying      *footprint
	span        float32 // a cell's, past which a body has a footprint
	// the method values, bound once
	layCell, scanCell func(cell.ID)
}

// Level is which level of its goal cell a route ends on; a goal of one level takes any.
type Level uint8

const (
	Either Level = iota
	Ground
	Deck
)

// Pacing is the share of its top speed a unit keeps on a step, as the board's Read gives it;
// nil keeps every kind's own share and feels no slope.
type Pacing interface {
	// Pace is the share on the step from from onto to's kind k on level for d, 0 refusing it.
	Pace(from, to cell.ID, k cell.Kind, level cell.Level, d cell.Domain) float64
	// Flat is Pace with no slope.
	Flat(to cell.ID, k cell.Kind, level cell.Level, d cell.Domain) float64
	// Sloped reports whether a step's pace may depend on its slope.
	Sloped() bool
}

type ways interface {
	Along(from, to cell.ID) bool
	Bare(c cell.ID) cell.Kind
}

type built interface {
	Reaches(c, n cell.ID) bool
	Fenced(from, to cell.ID, d cell.Domain, deck bool) bool
}

// passing counts every change to what a search reads, as a board's Passage does
type passing interface {
	Passage() uint64
}

// lanes says how wide a body may be across a step along a deck's band, +Inf where none narrows it
type lanes interface {
	Lane(from, to cell.ID, d cell.Domain, deck bool) float32
}

// Levels are a board's raised floors, which give a cell two levels: its deck and the ground under
// it.
type Levels interface {
	// Deck is c's raised floor: the height of its deck, of its underside, whom it carries; false
	// for a cell of one level.
	Deck(c cell.ID) (top, under float32, carries cell.Domain, ok bool)
	// Ground is the ground's height at p.
	Ground(p geom.Vec) float32
	// Over is the underside of the lowest raised deck over p; false where none is.
	Over(p geom.Vec) (under float32, ok bool)
	// KindAt is c's kind on its deck or under it.
	KindAt(c cell.ID, deck bool) cell.Kind
}

// Body is whoever a route is planned for: where its feet are, how high it steps up, how tall it
// is, its box on the ground, across and down, and its weight in kg. A zero Size fits any lane and
// any gap, a zero Weight any floor.
type Body struct {
	Feet, Step, Height float32
	Size               geom.Vec
	Weight             float32
}

type node struct {
	c     cell.ID
	level uint8
}

const (
	ground uint8 = iota
	deck
	goal
)

// New is a Finder over grid that respects terrain, its pacing and occupancy. No step costs less
// than its length, so the grid's Distance never overestimates.
func New(grid grid.Grid, terrain cell.Terrain, pacing Pacing, occupancy cell.Occupancy) *Finder {
	p := &Finder{grid: grid, terrain: terrain, occupancy: occupancy, span: grid.CellSpan()}
	p.paces(pacing)
	p.ways, _ = terrain.(ways)
	p.built, _ = terrain.(built)
	p.lanes, _ = terrain.(lanes)
	p.limits, _ = terrain.(limits)
	p.passing, _ = terrain.(passing)
	p.railings, _ = terrain.(railings)
	p.cells, _ = terrain.(cellPassages)
	p.plain = hpastar.NewSolver(graph.Implicit[cell.ID]{
		Forward:   p.plainFrom,
		Heuristic: grid.Distance,
		Index:     p.slot,
		Size:      grid.CellCount() + 1,
	}).Search()
	return p
}

// Reads has p search through terrain, pacing and, made WithLevels, levels from now on: the same
// board read on another goroutine.
func (p *Finder) Reads(terrain cell.Terrain, pacing Pacing, levels Levels) {
	p.terrain, p.levels = terrain, levels
	p.paces(pacing)
	p.ways, _ = terrain.(ways)
	p.built, _ = terrain.(built)
	p.lanes, _ = terrain.(lanes)
	p.limits, _ = terrain.(limits)
	p.passing, _ = terrain.(passing)
	p.railings, _ = terrain.(railings)
	p.cells, _ = terrain.(cellPassages)
	clear(p.flats)
}

func (p *Finder) paces(pacing Pacing) {
	p.pacing, p.sloped = pacing, pacing != nil && pacing.Sloped()
}

// a cell's slot in a solver's dictionaries; a start off the grid, the one cell of a search
// without an ordinal, takes the slot past the last cell
func (p *Finder) slot(c cell.ID) int {
	if o, ok := p.grid.Ordinal(c); ok {
		return o
	}
	return p.grid.CellCount()
}

// a node's slot: the levels of a cell side by side, then the goal, then a start off the grid
func (p *Finder) nodeSlot(n node) int {
	if n.level == goal {
		return 2 * p.grid.CellCount()
	}
	if o, ok := p.grid.Ordinal(n.c); ok {
		return 2*o + int(n.level)
	}
	return 2*p.grid.CellCount() + 1
}

// WithLevels has the Finder plan over the raised floors levels say.
func (p *Finder) WithLevels(levels Levels) *Finder {
	p.levels = levels
	p.stepped = p.leveledFrom
	p.leveled = hpastar.NewSolver(graph.Implicit[node]{
		Forward:   p.stepped,
		Heuristic: func(a, b node) float64 { return p.grid.Distance(a.c, b.c) },
		Index:     p.nodeSlot,
		Size:      2*p.grid.CellCount() + 2,
	}).Search()
	return p
}

// Price is what the step from from to its neighbour to costs an entity moving in d, to's kind
// on level being kind: the step's length over the share of its top speed the unit keeps on it;
// false where the step is refused or the ground it cuts across does not admit d.
func (p *Finder) Price(from, to cell.ID, kind cell.Kind, level cell.Level, d cell.Domain) (float64,
	bool) {
	p.begin(d)
	_, _, diagonal := p.grid.DiagonalNeighbors(from, to)
	return p.price(from, p.at(to), p.stepOn(to, kind, uint8(level)), uint8(level), diagonal)
}

// off a way a slantwise step crosses the bare ground beside it; where that admits and paces it
// alike, whether it runs along one does not matter. On the level a step costs its length over
// the share the cell was read with; over a relief the strategy paces each step.
func (p *Finder) price(from cell.ID, to *read, k step, level uint8, diagonal bool) (float64,
	bool) {
	bare := false
	if diagonal && p.ways != nil {
		if b := p.bareOf(to); (b != k || !b.admits) && !p.ways.Along(from, to.c) {
			if k, bare = b, true; !k.admits {
				return 0, false
			}
		}
	}
	length := p.grid.NeighborCost(from, to.c)
	if !p.sloped || k.graded {
		return length * k.rate, true
	}
	on := cell.Level(level)
	if bare {
		on = cell.OnGround
	}
	share := p.pacing.Pace(from, to.c, p.kindOn(to, level, bare), on, p.domain)
	if share <= 0 {
		return 0, false
	}
	return length / share, true
}

// Pace is the share of its top speed a unit moving in d keeps on the step from from onto its
// neighbour to's kind on level, 0 refusing it; the kind's own share without a pacing or from
// to to itself.
func (p *Finder) Pace(from, to cell.ID, kind cell.Kind, level cell.Level, d cell.Domain) float64 {
	if p.pacing == nil || from == to {
		return cell.PaceOf(nil, cell.Step{From: from, To: to, Level: level, Kind: kind, Domain: d})
	}
	return p.pacing.Pace(from, to, kind, level, d)
}

// Flat is Pace with no slope: the share on to's kind on level.
func (p *Finder) Flat(to cell.ID, kind cell.Kind, level cell.Level, d cell.Domain) float64 {
	if p.pacing == nil {
		return cell.PaceOf(nil, cell.Step{From: to, To: to, Level: level, Kind: kind, Domain: d})
	}
	return p.pacing.Flat(to, kind, level, d)
}

// KindAt is c's kind on level: its deck's where it has one, else the ground's.
func (p *Finder) KindAt(c cell.ID, level cell.Level) cell.Kind { return p.kindAt(c, uint8(level)) }

// Sloped reports whether a step's pace may depend on its slope: the pacing's word, false with
// none.
func (p *Finder) Sloped() bool { return p.sloped }

// Searches counts the routes searched for since the Finder was made, an answer kept by the
// keeping searched for none.
func (p *Finder) Searches() uint64 { return p.searches }

// Expansions counts the nodes the Finder's searches expanded since it was made: the work a
// search does, a fill's left out.
func (p *Finder) Expansions() uint64 { return p.expansions }

// a step over a relief is refused where the strategy paces it at 0
func (p *Finder) paced(from cell.ID, r *read, level uint8) bool {
	return !p.sloped ||
		p.pacing.Pace(from, r.c, p.kindOn(r, level, false), cell.Level(level), p.domain) > 0
}

// Find writes the route from from to to, ending on its level end, for entity moving in domain
// into dst, as many steps as dst holds, and reports how many; false if unreachable.
func (p *Finder) Find(
	entity uid.UID64, domain cell.Domain, body Body, from, to cell.ID, end Level, dst []cell.ID,
) (int, bool) {
	p.searches++
	p.decks, p.entity = 0, entity
	p.begin(domain)
	p.as(body)
	if p.levels == nil {
		// the start, then as many steps as dst holds
		if cap(p.path) < len(dst)+1 {
			p.path = make([]cell.ID, len(dst)+1)
		}
		search := p.plain
		if p.print != nil {
			search = p.spanning()
		}
		n, ok := search.Path(from, to, p.path[:len(dst)+1])
		if !ok || n < 2 {
			return 0, false
		}
		return copy(dst, p.path[1:n]), true
	}
	return p.findFrom(node{from, p.levelOf(p.at(from), body)}, to, end, dst)
}

// spanning is the search by plain steps of a body bigger than a cell, made as one is first
// searched for
func (p *Finder) spanning() *hpastar.Search[cell.ID] {
	if p.spanned == nil {
		p.spanned = hpastar.NewSolver(graph.Implicit[cell.ID]{
			Forward:   p.plainSpanFrom,
			Heuristic: p.grid.Distance,
			Index:     p.slot,
			Size:      p.grid.CellCount() + 1,
		}).Search()
	}
	return p.spanned
}

// findFrom is Find by levels from start, its level known, the search begun for its body
func (p *Finder) findFrom(start node, to cell.ID, end Level, dst []cell.ID) (int, bool) {
	p.decks, p.goal, p.end = 0, to, end
	if len(p.levelsOf(p.at(to))) == 1 {
		p.end = Either
	}
	// the start, every step dst holds and the goal past them where the route ends there
	if cap(p.route) < len(dst)+2 {
		p.route = make([]node, len(dst)+2)
	}
	n, ok := p.leveled.Path(start, node{to, goal}, p.route[:len(dst)+2])
	if !ok {
		return 0, false
	}
	return p.stepsOf(p.route[:n], dst)
}

// stepsOf writes the steps of route, what a search wrote from its start, into dst as Find writes
// them, as many as dst holds, and notes the level the route ends on: its last step's where route
// holds its goal, else the one its end asks for, the ground for Either; false where it takes none
func (p *Finder) stepsOf(route []node, dst []cell.ID) (int, bool) {
	route = route[1:]
	whole := len(route) > 0 && route[len(route)-1].level == goal
	if whole {
		route = route[:len(route)-1]
	}
	if len(route) == 0 {
		return 0, false
	}
	switch {
	case whole:
		p.ended = route[len(route)-1].level
	case p.end == Deck:
		p.ended = deck
	default:
		p.ended = ground
	}
	route = route[:min(len(route), len(dst))]
	for k, s := range route {
		dst[k] = s.c
		if s.level == deck && k < 64 {
			p.decks |= 1 << k
		}
	}
	return len(route), true
}

// Decks says which steps of the last route found lie on a raised deck, bit k the k-th.
func (p *Finder) Decks() uint64 { return p.decks }

// On reports whether body, standing in c, stands on its level end.
func (p *Finder) On(domain cell.Domain, body Body, c cell.ID, end Level) bool {
	p.begin(domain)
	r := p.at(c)
	if end == Either || len(p.levelsOf(r)) == 1 {
		return true
	}
	return (p.levelOf(r, body) == deck) == (end == Deck)
}

// FindAround is Find going round every cell blocked reports.
func (p *Finder) FindAround(
	entity uid.UID64, domain cell.Domain, body Body, from, to cell.ID, end Level,
	blocked func(cell.ID) bool, dst []cell.ID,
) (int, bool) {
	p.blocked = blocked
	defer func() { p.blocked = nil }()
	return p.Find(entity, domain, body, from, to, end, dst)
}

// Steps reports whether body, standing in from, may step on to its neighbour to in domain, and
// whether onto to's raised deck.
func (p *Finder) Steps(domain cell.Domain, body Body, from, to cell.ID) (onto, ok bool) {
	p.begin(domain)
	fp := p.footprintOf(body)
	if p.levels == nil {
		return false, !p.fenced(from, to, false) && p.fits(from, to, false, body) &&
			p.terrain.Kind(to).Admits(domain) && p.paced(from, p.at(to), ground) &&
			(!p.limited || p.within(node{from, ground}, p.at(from), p.at(to), ground, body)) &&
			(fp == nil || p.spans(fp, p.at(to), ground, p.flanks(from, to)))
	}
	rf, rt := p.at(from), p.at(to)
	at := node{from, p.levelOf(rf, body)}
	c := p.flanks(from, to)
	for _, level := range p.levelsOf(rt) {
		if p.crosses(at, rf, rt, level, c.diagonal) && p.passes(at, rf, rt, level, body, c) &&
			p.fits(from, to, level == deck, body) && p.stepAt(rt, level).admits &&
			p.paced(from, rt, level) && (!p.limited || p.within(at, rf, rt, level, body)) &&
			(fp == nil || p.spans(fp, rt, level, c)) {
			return level == deck, true
		}
	}
	return false, false
}

// Level is the level of c body stands on, Either where c has one level.
func (p *Finder) Level(domain cell.Domain, body Body, c cell.ID) Level {
	p.begin(domain)
	r := p.at(c)
	if len(p.levelsOf(r)) == 1 {
		return Either
	}
	if p.levelOf(r, body) == deck {
		return Deck
	}
	return Ground
}

// the corner a step cuts, the cells either side of it; none for a step across an edge
type corner struct {
	r1, r2   *read
	diagonal bool
}

func (p *Finder) flanks(from, n cell.ID) corner {
	c1, c2, diagonal := p.grid.DiagonalNeighbors(from, n)
	if !diagonal {
		return corner{}
	}
	return corner{r1: p.at(c1), r2: p.at(c2), diagonal: true}
}

// a step in direction i cuts a corner only where i leaves through one: the grid's
// DiagonalNeighbors says no other step is diagonal
func (p *Finder) cornerOf(from, n cell.ID, i int) corner {
	if !p.grid.Corner(i) {
		return corner{}
	}
	return p.flanks(from, n)
}

func (p *Finder) cutOff(c corner) bool {
	return !p.enterable(c.r1) || !p.enterable(c.r2) ||
		p.blocked != nil && (p.blocked(c.r1.c) || p.blocked(c.r2.c))
}

func (p *Finder) plainFrom(from, prev cell.ID,
	buf []graph.Transition[cell.ID]) []graph.Transition[cell.ID] {
	p.expansions++
	buf = buf[:0]
	for i := range p.grid.Directions() {
		n, ok := p.grid.Toward(from, i)
		if !ok || n == prev || p.fenced(from, n, false) || !p.fits(from, n, false, p.body) {
			continue
		}
		rn := p.at(n)
		k := p.groundOf(rn)
		if !k.admits || !p.occupancy.CanEnter(n, false, p.entity, p.domain) ||
			p.blocked != nil && p.blocked(n) ||
			p.limited && !p.within(node{from, ground}, p.at(from), rn, ground, p.body) {
			continue
		}
		c := p.cornerOf(from, n, i)
		if c.diagonal && !p.decked(from, n) && p.cutOff(c) {
			continue
		}
		cost, ok := p.price(from, rn, k, ground, c.diagonal)
		if !ok {
			continue
		}
		buf = append(buf, graph.Transition[cell.ID]{To: n, Cost: cost})
	}
	return buf
}

// plainSpanFrom is plainFrom of a body bigger than a cell, stepping only where its footprint
// spans: a loop of its own, as the test in plainFrom cost bodies within a cell 2-3% of a search
func (p *Finder) plainSpanFrom(from, prev cell.ID,
	buf []graph.Transition[cell.ID]) []graph.Transition[cell.ID] {
	p.expansions++
	buf = buf[:0]
	for i := range p.grid.Directions() {
		n, ok := p.grid.Toward(from, i)
		if !ok || n == prev || p.fenced(from, n, false) || !p.fits(from, n, false, p.body) {
			continue
		}
		rn := p.at(n)
		k := p.groundOf(rn)
		if !k.admits || !p.occupancy.CanEnter(n, false, p.entity, p.domain) ||
			p.blocked != nil && p.blocked(n) ||
			p.limited && !p.within(node{from, ground}, p.at(from), rn, ground, p.body) {
			continue
		}
		c := p.cornerOf(from, n, i)
		if c.diagonal && !p.decked(from, n) && p.cutOff(c) ||
			!p.spans(p.print, rn, ground, c) {
			continue
		}
		cost, ok := p.price(from, rn, k, ground, c.diagonal)
		if !ok {
			continue
		}
		buf = append(buf, graph.Transition[cell.ID]{To: n, Cost: cost})
	}
	return buf
}

func (p *Finder) leveledFrom(from, prev node,
	buf []graph.Transition[node]) []graph.Transition[node] {
	p.expansions++
	buf = buf[:0]
	if from.level == goal {
		return buf
	}
	if from.c == p.goal && (p.end == Either || p.end == Deck == (from.level == deck)) {
		buf = append(buf, graph.Transition[node]{To: node{p.goal, goal}})
	}
	rf := p.at(from.c)
	for i := range p.grid.Directions() {
		n, ok := p.grid.Toward(from.c, i)
		if !ok || n == prev.c || p.blocked != nil && p.blocked(n) {
			continue
		}
		rn := p.at(n)
		c := p.cornerOf(from.c, n, i)
		cut := c.diagonal && p.cutOff(c)
		for _, level := range p.levelsOf(rn) {
			if cost, ok := p.step(from, rf, rn, level, c, cut); ok {
				buf = append(buf, graph.Transition[node]{To: node{n, level}, Cost: cost})
			}
		}
	}
	return buf
}

// step is what the step from from onto rn's cell on level costs, rf from's cell, c the corner
// it cuts and cut whether that is cut off; false where the step is refused
func (p *Finder) step(from node, rf, rn *read, level uint8, c corner, cut bool) (float64, bool) {
	if !p.occupancy.CanEnter(rn.c, level == deck, p.entity, p.domain) ||
		!p.crosses(from, rf, rn, level, c.diagonal) ||
		cut && !p.along(from, rf, rn, level) ||
		!p.passes(from, rf, rn, level, p.body, c) ||
		!p.fits(from.c, rn.c, level == deck, p.body) ||
		p.limited && !p.within(from, rf, rn, level, p.body) {
		return 0, false
	}
	k := p.stepAt(rn, level)
	if !k.admits || p.print != nil && !p.spans(p.print, rn, level, c) {
		return 0, false
	}
	return p.price(from.c, rn, k, level, c.diagonal)
}

// leveledInto is leveledFrom seen from where its steps end: the steps into to, each from the
// node it is taken from and priced as leveledFrom prices it there, next's cell passed over as
// leveledFrom passes over prev's; into the goal from its cell's levels a route may end on
func (p *Finder) leveledInto(to, next node,
	buf []graph.Transition[node]) []graph.Transition[node] {
	buf = buf[:0]
	if to.level == goal {
		if to.c != p.goal {
			return buf
		}
		for _, level := range p.levelsOf(p.at(to.c)) {
			if p.end == Either || p.end == Deck == (level == deck) {
				buf = append(buf, graph.Transition[node]{To: node{to.c, level}})
			}
		}
		return buf
	}
	if p.blocked != nil && p.blocked(to.c) {
		return buf
	}
	rt := p.at(to.c)
	for i := range p.grid.Directions() {
		n, ok := p.grid.Toward(to.c, i)
		if !ok || n == next.c {
			continue
		}
		in := p.stepsInto(to, rt, i, n)
		for k := range in.count {
			buf = append(buf, graph.Transition[node]{To: node{n, in.levels[k]},
				Cost: in.prices[k]})
		}
	}
	return buf
}

// the steps into a node from one of its neighbours, a level of the neighbour's each
type stepsIn struct {
	count  int
	levels [2]uint8
	prices [2]float64
}

// stepsInto is what the steps into t from its neighbour n, in direction i from t, cost, rt t's
// cell: each checked and priced as leveledFrom checks and prices it from n
func (p *Finder) stepsInto(t node, rt *read, i int, n cell.ID) stepsIn {
	var in stepsIn
	rn := p.at(n)
	c := p.cornerOf(n, t.c, p.grid.Opposite(i))
	cut := c.diagonal && p.cutOff(c)
	for _, level := range p.levelsOf(rn) {
		if price, ok := p.step(node{n, level}, rn, rt, t.level, c, cut); ok {
			in.levels[in.count], in.prices[in.count] = level, price
			in.count++
		}
	}
	return in
}

// a deck's band narrower than the body across a step along it, on the decks' level when deck,
// keeps it from stepping; the cells round a body bigger than a cell are its footprint's to take
func (p *Finder) fits(from, n cell.ID, deck bool, body Body) bool {
	if p.lanes == nil || body.Size == (geom.Vec{}) {
		return true
	}
	lane := p.lanes.Lane(from, n, p.domain, deck)
	return f32.IsInf(lane, 1) || p.fitsLane(lane, from, n, body)
}

// laneOf is how wide a body may be across the step from from to n, on the decks' level when
// deck: +Inf where no band narrows it
func (p *Finder) laneOf(from, n cell.ID, deck bool) float32 {
	if p.lanes == nil {
		return f32.Inf(1)
	}
	return p.lanes.Lane(from, n, p.domain, deck)
}

// fitsLane is fits with the step's lane read
func (p *Finder) fitsLane(lane float32, from, n cell.ID, body Body) bool {
	if body.Size == (geom.Vec{}) || f32.IsInf(lane, 1) {
		return true
	}
	way, ok := grid.Way(p.grid, from, n)
	if !ok {
		return true
	}
	return breadth(p.grid.Step(way), body.Size) <= lane
}

// breadth is how wide a box of size is across a step along s
func breadth(s, size geom.Vec) float32 {
	return (size.X*f32.Abs(s.Y) + size.Y*f32.Abs(s.X)) / f32.Hypot(s.X, s.Y)
}

// a deck reaches through a corner only along its link
func (p *Finder) crosses(from node, rf, rn *read, level uint8, diagonal bool) bool {
	if diagonal && (from.level == deck || level == deck) && !p.along(from, rf, rn, level) {
		return false
	}
	if p.fenced(from.c, rn.c, from.level == deck) {
		return false
	}
	return level == from.level || !p.fenced(from.c, rn.c, level == deck)
}

// such a step passes over the corner it cuts: along a raised deck's link, or on the ground along
// a deck lying on it
func (p *Finder) along(from node, rf, rn *read, level uint8) bool {
	if p.built == nil {
		return false
	}
	if from.level != deck && level != deck {
		return p.decked(from.c, rn.c) && !p.raised(rf) && !p.raised(rn)
	}
	return (from.level != deck || p.built.Reaches(from.c, rn.c)) &&
		(level != deck || p.built.Reaches(rn.c, from.c))
}

func (p *Finder) decked(from, n cell.ID) bool {
	return p.built != nil && (p.built.Reaches(from, n) || p.built.Reaches(n, from))
}

func (p *Finder) fenced(from, n cell.ID, deck bool) bool {
	return p.built != nil && p.built.Fenced(from, n, p.domain, deck)
}

func (p *Finder) levelsOf(r *read) []uint8 {
	if p.levels != nil {
		if _, _, carries, ok := p.deckOf(r); ok && carries&p.domain != 0 {
			return bothLevels
		}
	}
	return groundLevel
}

var (
	groundLevel = []uint8{ground}
	bothLevels  = []uint8{ground, deck}
	deckFirst   = []uint8{deck, ground}
)

func (p *Finder) levelOf(r *read, body Body) uint8 {
	if p.levels == nil {
		return ground
	}
	top, _, carries, ok := p.deckOf(r)
	if ok && carries&p.domain != 0 && body.Feet >= top-body.Step-reachSlack {
		return deck
	}
	return ground
}

// onto a deck within a step of where it stands at their shared edge, off one onto ground within
// a step, under one only where it fits
func (p *Finder) passes(from node, rf, rn *read, level uint8, body Body, c corner) bool {
	if p.levels == nil {
		return true
	}
	if !p.underway(from, rf, rn, level, body, c) {
		return false
	}
	if from.level != deck && level != deck {
		return true
	}
	var at float32
	if from.level == deck {
		at, _, _, _ = p.deckOf(rf)
	} else {
		at = p.levels.Ground(p.edge(from.c, rn.c))
	}
	if level == deck {
		top, _, _, _ := p.deckOf(rn)
		return f32.Abs(top-at) <= body.Step+reachSlack
	}
	return f32.Abs(p.levels.Ground(p.edge(from.c, rn.c))-at) <= body.Step+reachSlack
}

// the ground is read along the part of the step walked on it: before the edge it steps onto a
// deck at, past the one it steps off at
func (p *Finder) underway(from node, rf, rn *read, level uint8, body Body, c corner) bool {
	if !p.raised(rf) && !p.raised(rn) && !(c.diagonal && (p.raised(c.r1) || p.raised(c.r2))) {
		return true
	}
	a, b := p.grid.CellCenter(from.c), p.grid.CellCenter(rn.c)
	for k := 1; k <= underSteps; k++ {
		t := float32(k) / underSteps
		if from.level == deck && t <= 0.5 || level == deck && t >= 0.5 {
			continue
		}
		q := geom.NewVec(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
		if under, ok := p.levels.Over(q); ok && under-p.levels.Ground(q) < body.Height {
			return false
		}
	}
	return true
}

const underSteps = 8

// the rounding of heights written to the centimetre
const reachSlack = 1e-2

func (p *Finder) edge(from, n cell.ID) geom.Vec {
	a, b := p.grid.CellCenter(from), p.grid.CellCenter(n)
	return geom.NewVec((a.X+b.X)/2, (a.Y+b.Y)/2)
}

func (p *Finder) kindAt(c cell.ID, level uint8) cell.Kind {
	if p.levels == nil {
		return p.terrain.Kind(c)
	}
	return p.levels.KindAt(c, level == deck)
}

func (p *Finder) enterable(r *read) bool {
	return p.groundOf(r).admits && p.occupancy.CanEnter(r.c, false, p.entity, p.domain) &&
		p.bears(r, ground)
}

const maxVisitedCells = 64

// NearestFree is the free cell nearest target that entity can reach, on a level not taken says,
// and the route to it written into dst as Find writes it.
func (p *Finder) NearestFree(
	entity uid.UID64, domain cell.Domain, body Body, from, target cell.ID, end Level,
	taken func(c cell.ID, deck bool) bool, dst []cell.ID,
) (cell.ID, int, bool) {
	return p.NearestFreeThrough(nil, entity, domain, body, from, target, end, taken, dst)
}

// NearestFreeThrough is NearestFree searching, once a search for one cell has failed, no route
// to a cell h says from cannot reach; with a nil h, NearestFree.
func (p *Finder) NearestFreeThrough(
	h *Hierarchy, entity uid.UID64, domain cell.Domain, body Body, from, target cell.ID,
	end Level, taken func(c cell.ID, deck bool) bool, dst []cell.ID,
) (cell.ID, int, bool) {
	p.begin(domain)
	n, failed := 0, false
	passable := func(c cell.ID) bool { return p.terrain.Kind(c).Admits(domain) }
	own, fp := p.levelOf(p.at(from), body), p.footprintOf(body)
	reachableFree := func(c cell.ID) bool {
		if !p.terrain.Kind(c).Admits(domain) {
			return false
		}
		levels := p.levelsOf(p.at(c))
		if c == from && own == deck && len(levels) > 1 {
			levels = deckFirst
		}
		for _, l := range levels {
			on := Either
			if len(levels) > 1 {
				on = Ground
				if l == deck {
					on = Deck
				}
			}
			if c == target && end != Either && on != Either && on != end ||
				taken != nil && taken(c, l == deck) ||
				!p.occupancy.CanEnter(c, l == deck, entity, domain) {
				continue
			}
			if c == from && (on == Either || l == own) {
				n, p.decks = 0, 0
				return true
			}
			// a cell its footprint does not stand on, no route reaches
			if fp != nil && !p.standsOn(fp, c, l) {
				continue
			}
			if failed && h != nil && !p.Reaches(h, domain, body, from, c) {
				return false
			}
			var ok bool
			if n, ok = p.Find(entity, domain, body, from, c, on, dst); ok {
				return true
			}
			failed = true
		}
		return false
	}
	dest, ok := p.rings.breadthFirst(target, p.grid, passable, reachableFree, maxVisitedCells)
	return dest, n, ok
}
