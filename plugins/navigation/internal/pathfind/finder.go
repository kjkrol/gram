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
	ways      ways     // the terrain, when it knows its ways; nil, none run
	rails     railings // the terrain, when it knows what stands on it; nil, no railings
	levels    Levels   // the board's raised floors; nil, every cell one level
	solver    *astar.Solver[cell.ID]
	leveled   *astar.Solver[node] // with levels: a cell on its deck apart from under it
	least     float64             // the cheapest step for the domain planned, per unit of Distance
	blocked   func(cell.ID) bool  // cells the route being planned goes round; nil, none
	goal      cell.ID             // the cell the route being planned ends on, at either level
	body      Body                // whoever the route being planned is for
}

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

// railings is a terrain that knows what stands on its cells — the board: whether a railing stands
// anywhere, and what is built over a cell.
type railings interface {
	Railed() bool
	Built(c cell.ID) cell.Built
}

// Levels are a board's raised floors, which give a cell two levels: its deck and the ground under
// it.
type Levels interface {
	// Deck is c's raised floor: the height of its deck, of its underside, whom it carries; false
	// for a cell of one level.
	Deck(c cell.ID) (top, under float64, carries cell.Domain, ok bool)
	// Ground is the ground's height at p.
	Ground(p geom.Vec) float64
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
	p.rails, _ = terrain.(railings)
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

// Find computes a route from 'from' toward 'to' for entity moving in domain, its body as body
// says, into dst, as many of its steps as dst holds, and how many it wrote — false if unreachable.
func (p *Finder) Find(
	entity uid.UID64, domain cell.Domain, body Body, from, to cell.ID, dst []cell.ID,
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
	p.goal, p.body = to, body
	start := node{from, p.levelOf(from, domain, body)}
	full := p.leveled.Solve(start, node{to, goal}, p.leveledFor(entity, domain))
	if len(full) < 3 {
		return 0, false
	}
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

// FindAround is Find going round every cell blocked reports.
func (p *Finder) FindAround(
	entity uid.UID64, domain cell.Domain, body Body, from, to cell.ID, blocked func(cell.ID) bool,
	dst []cell.ID,
) (int, bool) {
	p.blocked = blocked
	defer func() { p.blocked = nil }()
	return p.Find(entity, domain, body, from, to, dst)
}

// Steps reports whether body, standing in from on the level its feet say, may step on to its
// neighbour to in domain: on a level whose kind admits domain, no further down than a step off a
// deck's edge, onto a deck no higher than a step, under one only where it fits.
func (p *Finder) Steps(domain cell.Domain, body Body, from, to cell.ID) bool {
	if p.railed(from, to, domain) {
		return false
	}
	if p.levels == nil {
		return p.terrain.Kind(to).Admits(domain)
	}
	at := node{from, p.levelOf(from, domain, body)}
	for _, level := range p.levelsOf(to, domain) {
		if p.passes(at, to, level, body) && p.kindAt(to, level).Admits(domain) {
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
			if n == prev || p.railed(from, n, domain) {
				continue
			}
			kind := p.terrain.Kind(n)
			if !kind.Admits(domain) || !p.occupancy.CanEnter(n, entity, domain) || p.blocked != nil && p.blocked(n) {
				continue
			}
			if c1, c2, ok := p.grid.DiagonalNeighbors(from, n); ok {
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
		if from.c == p.goal {
			buf = append(buf, astar.Transition[node]{To: node{p.goal, goal}})
		}
		for _, n := range p.grid.Neighbors(from.c) {
			if n == prev.c || !p.occupancy.CanEnter(n, entity, domain) ||
				p.blocked != nil && p.blocked(n) || p.railed(from.c, n, domain) {
				continue
			}
			c1, c2, diagonal := p.grid.DiagonalNeighbors(from.c, n)
			if diagonal && (!p.enterable(c1, entity, domain) || !p.enterable(c2, entity, domain) ||
				p.blocked != nil && (p.blocked(c1) || p.blocked(c2))) {
				continue
			}
			for _, level := range p.levelsOf(n, domain) {
				if diagonal && (from.level == deck || level == deck) {
					continue // a deck runs straight
				}
				if !p.passes(from, n, level, p.body) {
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

// railed reports whether a railing keeping domain off stands across the step from from to its
// neighbour n: on from's side towards n or n's side towards from, or, slantwise, either side of the
// corner the step cuts.
func (p *Finder) railed(from, n cell.ID, domain cell.Domain) bool {
	if p.rails == nil || !p.rails.Railed() {
		return false
	}
	way, ok := grid.Link(p.grid, from, n)
	if !ok {
		return false
	}
	out := crossing[bitOf(way)]
	a, b := p.rails.Built(from).Rail, p.rails.Built(n).Rail
	return a.Keeps&domain != 0 && a.Sides&out[0] != 0 || b.Keeps&domain != 0 && b.Sides&out[1] != 0
}

// crossing is, for each way out of a square cell, the sides of the cell a step that way crosses
// and the sides of the neighbour it crosses into.
var crossing = [8][2]cell.Links{
	{1, 2}, {2, 1}, {4, 8}, {8, 4},
	{1 | 4, 2 | 8}, {1 | 8, 2 | 4}, {2 | 4, 1 | 8}, {2 | 8, 1 | 4},
}

func bitOf(l cell.Links) int {
	for i := range 8 {
		if l == 1<<i {
			return i
		}
	}
	return 0
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
	at := 0.0 // how high body stands at the edge it crosses
	if from.level == deck {
		at, _, _, _ = p.levels.Deck(from.c)
	} else {
		at = p.levels.Ground(p.edge(from.c, n))
	}
	if level == deck {
		top, _, _, _ := p.levels.Deck(n)
		return math.Abs(top-at) <= body.Step+reachSlack
	}
	if from.level == deck {
		if edge := p.levels.Ground(p.edge(from.c, n)); math.Abs(edge-at) > body.Step+reachSlack {
			return false // off the deck's edge: a fall
		}
	}
	if _, under, _, ok := p.levels.Deck(n); ok {
		return under-p.levels.Ground(p.grid.CellCenter(n)) >= body.Height
	}
	return true
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
	entity uid.UID64, domain cell.Domain, body Body, from, target cell.ID,
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
		n, ok = p.Find(entity, domain, body, from, c, dst)
		return ok
	}
	dest, ok := breadthFirst(target, p.grid.Neighbors, passable, reachableFree, maxVisitedCells)
	return dest, n, ok
}
