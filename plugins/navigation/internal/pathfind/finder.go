package pathfind

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/astar"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/uid"
)

// Finder computes routes over one grid, reusing its A* solver across calls — build once and share
// across systems.
type Finder struct {
	grid      grid.Grid
	terrain   cell.Terrain
	occupancy cell.Occupancy
	slopes    Slopes
	ways      ways   // the terrain, when it knows its ways; nil, none run
	built     built  // the terrain, when it knows what stands on it; nil, nothing
	levels    Levels // the board's raised floors; nil, every cell one level
	solver    *astar.Solver[cell.ID]
	leveled   *astar.Solver[node] // with levels: a cell on its deck apart from under it
	least     float64             // the cheapest step for the domain planned, per unit of Distance
	blocked   func(cell.ID) bool  // cells the route being planned goes round; nil, none
	goal      cell.ID             // the cell the route being planned ends on
	end       Level               // the level of it the route ends on
	ended     uint8               // the level the last route found ended on
	body      Body                // whoever the route being planned is for
}

// Level is which level of its goal cell a route ends on: Either, or the Ground under a raised
// floor, or its Deck — either where the goal has one level.
type Level uint8

const (
	Either Level = iota
	Ground
	Deck
)

// Slopes prices a step's climb: the board's Map; nil is level ground.
type Slopes interface {
	Climb(from, to cell.ID, d cell.Domain) float64
	Least(d cell.Domain) float64
}

// ways is a terrain that knows what runs across its cells — the board: which steps go along a
// way, and the ground bare of it.
type ways interface {
	Along(from, to cell.ID) bool
	Bare(c cell.ID) cell.Kind
}

// built is a terrain that knows what stands on its cells — the board: whether a Floor runs from a
// cell out towards its neighbour, and whether a railing stands across a step on the decks' level
// or on the ground's.
type built interface {
	Reaches(c, n cell.ID) bool
	Fenced(from, to cell.ID, d cell.Domain, deck bool) bool
}

// Levels are a board's raised floors, which give a cell two levels: its deck and the ground under
// it.
type Levels interface {
	// Deck is c's raised floor: the height of its deck, of its underside, whom it carries; false
	// for a cell of one level.
	Deck(c cell.ID) (top, under float64, carries cell.Domain, ok bool)
	// Ground is the ground's height at p.
	Ground(p geom.Vec) float64
	// Over is the underside of the lowest raised deck over p — a cell's whole, or a band p lies
	// on; false where none is.
	Over(p geom.Vec) (under float64, ok bool)
	// KindAt is c's kind on its deck or under it.
	KindAt(c cell.ID, deck bool) cell.Kind
}

// Body is whoever a route is planned for, in height: where its feet are, how high it steps up, how
// tall it is — what tells the deck it may step onto from the ground it stays on, and the deck it
// may pass under.
type Body struct{ Feet, Step, Height float64 }

// node is a cell on one of its levels; a route ends on the goal's.
type node struct {
	c     cell.ID
	level uint8
}

const (
	ground uint8 = iota
	deck
	goal
)

// New is a Finder over grid that respects terrain, its slopes and occupancy, and terrain's raised
// floors when it is a Levels.
func New(grid grid.Grid, terrain cell.Terrain, slopes Slopes, occupancy cell.Occupancy) *Finder {
	p := &Finder{grid: grid, terrain: terrain, occupancy: occupancy, slopes: slopes, least: 1}
	p.ways, _ = terrain.(ways)
	p.built, _ = terrain.(built)
	p.solver = astar.New[cell.ID](func(a, b cell.ID) float64 { return p.least * grid.Distance(a, b) })
	return p
}

// WithLevels has the Finder plan over the raised floors levels say: never off a deck's edge, onto
// one only within a step, under one only where the body fits.
func (p *Finder) WithLevels(levels Levels) *Finder {
	p.levels = levels
	p.leveled = astar.New[node](func(a, b node) float64 {
		return p.least * p.grid.Distance(a.c, b.c)
	})
	return p
}

// Price is what the step from from to its neighbour to costs an entity moving in d, to's kind
// being kind: the kind's cost over the step's length, times the slope unless the kind is Graded. A
// slantwise step not along a way cuts the corner beside it, over the ground bare of the way: it
// costs that ground, so a road is followed round its bend rather than cut across; false where
// that ground does not admit d.
func (p *Finder) Price(from, to cell.ID, kind cell.Kind, d cell.Domain) (float64, bool) {
	if _, _, diagonal := p.grid.DiagonalNeighbors(from, to); diagonal && p.ways != nil && !p.ways.Along(from, to) {
		if kind = p.ways.Bare(to); !kind.Admits(d) {
			return 0, false
		}
	}
	cost := kind.CostFor(d) * p.grid.NeighborCost(from, to)
	if p.slopes != nil && !kind.Graded {
		cost *= p.slopes.Climb(from, to, d)
	}
	return cost, true
}

// Climb is how many times as long as on the level the step from one cell to its neighbour takes
// in d, the slope's price: 1 on the level or with no slopes.
func (p *Finder) Climb(from, to cell.ID, d cell.Domain) float64 {
	if p.slopes == nil || from == to {
		return 1
	}
	return p.slopes.Climb(from, to, d)
}

// Find computes a route from 'from' toward 'to' — ending on its level end — for entity moving in
// domain, its body as body says, into dst, as many of its steps as dst holds, and how many it
// wrote — false if unreachable.
func (p *Finder) Find(
	entity uid.UID64, domain cell.Domain, body Body, from, to cell.ID, end Level, dst []cell.ID,
) (int, bool) {
	// A gentle descent is cheaper than the flat, so the heuristic counts every step at its quickest;
	// a kind costs at least 1 (full speed), or the estimate would overshoot.
	p.least = 1
	if p.slopes != nil {
		p.least = p.slopes.Least(domain)
	}
	if p.levels == nil {
		full := p.solver.Solve(from, to, p.transitionsFor(entity, domain))
		if len(full) < 2 {
			return 0, false
		}
		return copy(dst, full[1:]), true
	}
	p.goal, p.body, p.end = to, body, end
	if len(p.levelsOf(to, domain)) == 1 {
		p.end = Either
	}
	start := node{from, p.levelOf(from, domain, body)}
	full := p.leveled.Solve(start, node{to, goal}, p.leveledFor(entity, domain))
	if len(full) < 3 {
		return 0, false
	}
	p.ended = full[len(full)-2].level
	n := 0
	for _, s := range full[1 : len(full)-1] {
		if n == len(dst) {
			break
		}
		dst[n] = s.c
		n++
	}
	return n, true
}

// On reports whether body, standing in c, stands on c's level end — its deck, the ground under
// it, or either; every level where c has one.
func (p *Finder) On(domain cell.Domain, body Body, c cell.ID, end Level) bool {
	if end == Either || len(p.levelsOf(c, domain)) == 1 {
		return true
	}
	return (p.levelOf(c, domain, body) == deck) == (end == Deck)
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

// Steps reports whether body, standing in from on the level its feet say, may step on to its
// neighbour to in domain: on a level whose kind admits domain, no further down than a step off a
// deck's edge, onto a deck no higher than a step, under one only where it fits.
func (p *Finder) Steps(domain cell.Domain, body Body, from, to cell.ID) bool {
	if p.levels == nil {
		return !p.fenced(from, to, domain, false) && p.terrain.Kind(to).Admits(domain)
	}
	at := node{from, p.levelOf(from, domain, body)}
	_, _, diagonal := p.grid.DiagonalNeighbors(from, to)
	for _, level := range p.levelsOf(to, domain) {
		if p.crosses(at, to, level, domain, diagonal) && p.passes(at, to, level, body) &&
			p.kindAt(to, level).Admits(domain) {
			return true
		}
	}
	return false
}

// transitionsFor adapts the grid, terrain and occupancy into astar's Transitions for entity, every
// cell one level.
func (p *Finder) transitionsFor(entity uid.UID64, domain cell.Domain) astar.Transitions[cell.ID] {
	return func(from, prev cell.ID, buf []astar.Transition[cell.ID]) []astar.Transition[cell.ID] {
		buf = buf[:0]
		for _, n := range p.grid.Neighbors(from) {
			if n == prev || p.fenced(from, n, domain, false) {
				continue
			}
			kind := p.terrain.Kind(n)
			if !kind.Admits(domain) || !p.occupancy.CanEnter(n, entity, domain) || p.blocked != nil && p.blocked(n) {
				continue
			}
			if c1, c2, ok := p.grid.DiagonalNeighbors(from, n); ok && !p.decked(from, n) {
				if !p.enterable(c1, entity, domain) || !p.enterable(c2, entity, domain) || p.blocked != nil && (p.blocked(c1) || p.blocked(c2)) {
					continue
				}
			}
			cost, ok := p.Price(from, n, kind, domain)
			if !ok {
				continue
			}
			buf = append(buf, astar.Transition[cell.ID]{To: n, Cost: cost})
		}
		return buf
	}
}

// leveledFor is transitionsFor over the cells' levels: a raised floor's deck apart from the ground
// under it, a route ending on either of the goal's.
func (p *Finder) leveledFor(entity uid.UID64, domain cell.Domain) astar.Transitions[node] {
	return func(from, prev node, buf []astar.Transition[node]) []astar.Transition[node] {
		buf = buf[:0]
		if from.level == goal {
			return buf
		}
		if from.c == p.goal && (p.end == Either || p.end == Deck == (from.level == deck)) {
			buf = append(buf, astar.Transition[node]{To: node{p.goal, goal}})
		}
		for _, n := range p.grid.Neighbors(from.c) {
			if n == prev.c || !p.occupancy.CanEnter(n, entity, domain) ||
				p.blocked != nil && p.blocked(n) {
				continue
			}
			c1, c2, diagonal := p.grid.DiagonalNeighbors(from.c, n)
			cut := diagonal && (!p.enterable(c1, entity, domain) ||
				!p.enterable(c2, entity, domain) ||
				p.blocked != nil && (p.blocked(c1) || p.blocked(c2)))
			for _, level := range p.levelsOf(n, domain) {
				if !p.crosses(from, n, level, domain, diagonal) ||
					cut && !p.along(from, n, level) || !p.passes(from, n, level, p.body) {
					continue
				}
				kind := p.kindAt(n, level)
				if !kind.Admits(domain) {
					continue
				}
				cost, ok := p.Price(from.c, n, kind, domain)
				if !ok {
					continue
				}
				buf = append(buf, astar.Transition[node]{To: node{n, level}, Cost: cost})
			}
		}
		return buf
	}
}

// crosses reports whether the step from from on to n's level goes across no railing keeping
// domain off, on the decks' level or on the ground's as its ends lie, and, slantwise, on or off a
// deck only along it: a deck reaches through a corner only along its link.
func (p *Finder) crosses(from node, n cell.ID, level uint8, domain cell.Domain,
	diagonal bool) bool {
	if diagonal && (from.level == deck || level == deck) && !p.along(from, n, level) {
		return false
	}
	if p.fenced(from.c, n, domain, from.level == deck) {
		return false
	}
	return level == from.level || !p.fenced(from.c, n, domain, level == deck)
}

// along reports whether the step from from on to n's level runs along a deck at one end at least,
// out of from's towards n and into n's from from: such a step passes over the corner it cuts.
func (p *Finder) along(from node, n cell.ID, level uint8) bool {
	if p.built == nil || from.level != deck && level != deck {
		return false
	}
	return (from.level != deck || p.built.Reaches(from.c, n)) &&
		(level != deck || p.built.Reaches(n, from.c))
}

// decked reports whether a Floor runs between from and its neighbour n, out of either towards the
// other: a step along it passes over the corner it cuts.
func (p *Finder) decked(from, n cell.ID) bool {
	return p.built != nil && (p.built.Reaches(from, n) || p.built.Reaches(n, from))
}

// fenced reports whether a railing keeping domain off stands across the step from from to its
// neighbour n, on the decks' level or on the ground's.
func (p *Finder) fenced(from, n cell.ID, domain cell.Domain, deck bool) bool {
	return p.built != nil && p.built.Fenced(from, n, domain, deck)
}

// levelsOf is c's levels for whoever moves in domain: the ground, and a raised floor's deck that
// carries domain.
func (p *Finder) levelsOf(c cell.ID, domain cell.Domain) []uint8 {
	if p.levels != nil {
		if _, _, carries, ok := p.levels.Deck(c); ok && carries&domain != 0 {
			return bothLevels
		}
	}
	return groundLevel
}

var (
	groundLevel = []uint8{ground}
	bothLevels  = []uint8{ground, deck}
)

// levelOf is the level body stands on in c: the deck of a raised floor carrying domain within a
// step of its feet, else the ground.
func (p *Finder) levelOf(c cell.ID, domain cell.Domain, body Body) uint8 {
	if p.levels == nil {
		return ground
	}
	top, _, carries, ok := p.levels.Deck(c)
	if ok && carries&domain != 0 && body.Feet >= top-body.Step-reachSlack {
		return deck
	}
	return ground
}

// passes reports whether body on from steps on to n's level: onto a deck within a step of where
// it stands at their shared edge, off a deck onto ground within a step of it, along the ground
// under a deck only where it fits under it.
func (p *Finder) passes(from node, n cell.ID, level uint8, body Body) bool {
	if p.levels == nil {
		return true
	}
	if !p.underway(from, n, level, body) {
		return false
	}
	edge := p.levels.Ground(p.edge(from.c, n))
	at := edge // how high body stands at the edge it crosses
	if from.level == deck {
		at, _, _, _ = p.levels.Deck(from.c)
	}
	if level == deck {
		top, _, _, _ := p.levels.Deck(n)
		return math.Abs(top-at) <= body.Step+reachSlack
	}
	return from.level != deck || math.Abs(edge-at) <= body.Step+reachSlack // or off the edge: a fall
}

// underway reports whether body walking the ground of the step from from on to n's level fits
// under every raised deck over its way, read every underSteps of the step from middle to middle —
// the part it walks on the ground: before the edge it steps onto a deck at, past the edge it steps
// off one at.
func (p *Finder) underway(from node, n cell.ID, level uint8, body Body) bool {
	if !p.decksBy(from.c, n) {
		return true
	}
	a, b := p.grid.CellCenter(from.c), p.grid.CellCenter(n)
	for k := 1; k <= underSteps; k++ {
		t := float64(k) / underSteps
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

// underSteps is how many times along a step the ground under the decks over it is read.
const underSteps = 8

// decksBy reports whether a raised deck stands on from, on its neighbour n or, the step slantwise,
// on a cell whose corner it cuts: whether a deck may run over the step at all.
func (p *Finder) decksBy(from, n cell.ID) bool {
	raised := func(c cell.ID) bool {
		_, _, _, ok := p.levels.Deck(c)
		return ok
	}
	if raised(from) || raised(n) {
		return true
	}
	c1, c2, diagonal := p.grid.DiagonalNeighbors(from, n)
	return diagonal && (raised(c1) || raised(c2))
}

// reachSlack is how far over a step a deck may stand and still be stepped onto: the rounding of
// heights written to the centimetre.
const reachSlack = 1e-2

// edge is the middle of the side from and its neighbour n share — their shared corner, slantwise.
func (p *Finder) edge(from, n cell.ID) geom.Vec {
	a, b := p.grid.CellCenter(from), p.grid.CellCenter(n)
	return geom.NewVec((a.X+b.X)/2, (a.Y+b.Y)/2)
}

// kindAt is c's kind on a level.
func (p *Finder) kindAt(c cell.ID, level uint8) cell.Kind {
	if p.levels == nil {
		return p.terrain.Kind(c)
	}
	return p.levels.KindAt(c, level == deck)
}

// enterable reports whether entity may hold c on its ground: terrain admitting its domain, the
// Occupancy letting it in.
func (p *Finder) enterable(c cell.ID, entity uid.UID64, domain cell.Domain) bool {
	k := p.terrain.Kind(c)
	if p.levels != nil {
		k = p.levels.KindAt(c, false)
	}
	return k.Admits(domain) && p.occupancy.CanEnter(c, entity, domain)
}

// maxVisitedCells bounds how many cells NearestFree inspects around its target.
const maxVisitedCells = 64

// NearestFree returns the free cell nearest target that entity can reach — not one taken says —
// and the route to it, written into dst as Find writes it.
func (p *Finder) NearestFree(
	entity uid.UID64, domain cell.Domain, body Body, from, target cell.ID, end Level,
	taken func(cell.ID) bool, dst []cell.ID,
) (cell.ID, int, bool) {
	n := 0
	passable := func(c cell.ID) bool { return p.terrain.Kind(c).Admits(domain) }
	reachableFree := func(c cell.ID) bool {
		if taken != nil && taken(c) || !p.terrain.Kind(c).Admits(domain) ||
			!p.occupancy.CanEnter(c, entity, domain) {
			return false
		}
		if c == from {
			n = 0
			return true
		}
		var ok bool
		level := Either
		if c == target {
			level = end
		}
		n, ok = p.Find(entity, domain, body, from, c, level, dst)
		return ok
	}
	dest, ok := breadthFirst(target, p.grid.Neighbors, passable, reachableFree, maxVisitedCells)
	return dest, n, ok
}
