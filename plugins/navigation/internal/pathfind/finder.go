package pathfind

import (
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
	ways      ways // the terrain, when it knows its ways; nil, none run
	solver    *astar.Solver[cell.ID]
	least     float64            // the cheapest a step may be for the domain being planned, per unit of Distance
	blocked   func(cell.ID) bool // cells the route being planned goes round; nil, none
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

// New is a Finder over grid that respects terrain, its slopes and occupancy.
func New(grid grid.Grid, terrain cell.Terrain, slopes Slopes, occupancy cell.Occupancy) *Finder {
	p := &Finder{grid: grid, terrain: terrain, occupancy: occupancy, slopes: slopes, least: 1}
	p.ways, _ = terrain.(ways)
	p.solver = astar.New[cell.ID](func(a, b cell.ID) float64 { return p.least * grid.Distance(a, b) })
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

// Find computes a route from 'from' toward 'to' for entity moving in domain into dst, as many of
// its steps as dst holds, and how many it wrote — false if unreachable.
func (p *Finder) Find(entity uid.UID64, domain cell.Domain, from, to cell.ID, dst []cell.ID) (int, bool) {
	// A gentle descent is cheaper than the flat, so the heuristic counts every step at its quickest;
	// a kind costs at least 1 (full speed), or the estimate would overshoot.
	p.least = 1
	if p.slopes != nil {
		p.least = p.slopes.Least(domain)
	}
	full := p.solver.Solve(from, to, p.transitionsFor(entity, domain))
	if len(full) < 2 {
		return 0, false
	}
	return copy(dst, full[1:]), true
}

// FindAround is Find going round every cell blocked reports.
func (p *Finder) FindAround(entity uid.UID64, domain cell.Domain, from, to cell.ID, blocked func(cell.ID) bool, dst []cell.ID) (int, bool) {
	p.blocked = blocked
	defer func() { p.blocked = nil }()
	return p.Find(entity, domain, from, to, dst)
}

// transitionsFor adapts the grid, terrain and occupancy into astar's Transitions for entity.
func (p *Finder) transitionsFor(entity uid.UID64, domain cell.Domain) astar.Transitions[cell.ID] {
	return func(from, prev cell.ID, buf []astar.Transition[cell.ID]) []astar.Transition[cell.ID] {
		buf = buf[:0]
		for _, n := range p.grid.Neighbors(from) {
			if n == prev {
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

// enterable reports whether entity may hold c: terrain admitting its domain, the Occupancy letting it in.
func (p *Finder) enterable(c cell.ID, entity uid.UID64, domain cell.Domain) bool {
	return p.terrain.Kind(c).Admits(domain) && p.occupancy.CanEnter(c, entity, domain)
}

// maxVisitedCells bounds how many cells NearestFree inspects around its target.
const maxVisitedCells = 64

// NearestFree returns the free cell nearest target that entity can reach — not one taken says —
// and the route to it, written into dst as Find writes it.
func (p *Finder) NearestFree(entity uid.UID64, domain cell.Domain, from, target cell.ID, taken func(cell.ID) bool, dst []cell.ID) (cell.ID, int, bool) {
	n := 0
	passable := func(c cell.ID) bool { return p.terrain.Kind(c).Admits(domain) }
	reachableFree := func(c cell.ID) bool {
		if taken != nil && taken(c) || !p.enterable(c, entity, domain) {
			return false
		}
		if c == from {
			n = 0
			return true
		}
		var ok bool
		n, ok = p.Find(entity, domain, from, c, dst)
		return ok
	}
	dest, ok := breadthFirst(target, p.grid.Neighbors, passable, reachableFree, maxVisitedCells)
	return dest, n, ok
}
