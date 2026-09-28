package navigation

import (
	"github.com/kjkrol/astar"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/uid"
)

// pathFinder computes routes over one grid, reusing its A* solver across
// calls — build once and share across systems.
type pathFinder struct {
	grid      board.Grid
	terrain   board.Terrain
	occupancy board.Occupancy
	slopes    slopes
	solver    *astar.Solver[board.CellID]
	least     float64                 // the cheapest a step may be for the domain being planned, per unit of Distance
	blocked   func(board.CellID) bool // cells the route being planned goes round; nil, none
}

// slopes prices a step's climb: the board's Map; nil is level ground.
type slopes interface {
	Climb(from, to board.CellID, d board.Domain) float64
	Least(d board.Domain) float64
}

// newPathFinder builds a pathFinder over grid that respects terrain, its slopes and occupancy.
func newPathFinder(grid board.Grid, terrain board.Terrain, slopes slopes, occupancy board.Occupancy) *pathFinder {
	p := &pathFinder{grid: grid, terrain: terrain, occupancy: occupancy, slopes: slopes, least: 1}
	p.solver = astar.New[board.CellID](func(a, b board.CellID) float64 { return p.least * grid.Distance(a, b) })
	return p
}

// findPath computes a route from 'from' toward 'to' for entity moving in domain — ok=false if
// unreachable.
func (p *pathFinder) findPath(entity uid.UID64, domain board.Domain, from, to board.CellID) (Path, bool) {
	// A gentle descent is cheaper than the flat, so the heuristic counts every step at its quickest;
	// a kind costs at least 1 (full speed), or the estimate would overshoot.
	p.least = 1
	if p.slopes != nil {
		p.least = p.slopes.Least(domain)
	}
	full := p.solver.Solve(from, to, p.transitionsFor(entity, domain))
	if len(full) < 2 {
		return Path{}, false
	}
	steps := full[1:]
	n := min(len(steps), MaxPathLength)
	var path Path
	copy(path.Steps[:], steps[:n])
	path.Length = uint16(n)
	return path, true
}

// findPathAround is findPath going round every cell blocked reports.
func (p *pathFinder) findPathAround(entity uid.UID64, domain board.Domain, from, to board.CellID, blocked func(board.CellID) bool) (Path, bool) {
	p.blocked = blocked
	defer func() { p.blocked = nil }()
	return p.findPath(entity, domain, from, to)
}

// transitionsFor adapts the grid, terrain and occupancy into astar's Transitions for entity.
func (p *pathFinder) transitionsFor(entity uid.UID64, domain board.Domain) astar.Transitions[board.CellID] {
	return func(from, prev board.CellID, buf []astar.Transition[board.CellID]) []astar.Transition[board.CellID] {
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
			cost := kind.CostFor(domain) * p.grid.NeighborCost(from, n)
			if p.slopes != nil {
				cost *= p.slopes.Climb(from, n, domain)
			}
			buf = append(buf, astar.Transition[board.CellID]{To: n, Cost: cost})
		}
		return buf
	}
}

// enterable reports whether entity may hold c: terrain admitting its domain, the Occupancy letting it in.
func (p *pathFinder) enterable(c board.CellID, entity uid.UID64, domain board.Domain) bool {
	return p.terrain.Kind(c).Admits(domain) && p.occupancy.CanEnter(c, entity, domain)
}

// maxVisitedCells bounds how many cells nearestFree inspects around its target.
const maxVisitedCells = 64

// nearestFree returns the free cell nearest target that entity can reach, and the route to it.
func (p *pathFinder) nearestFree(entity uid.UID64, domain board.Domain, from, target board.CellID, taken map[board.CellID]bool) (board.CellID, Path, bool) {
	var path Path
	passable := func(c board.CellID) bool { return p.terrain.Kind(c).Admits(domain) }
	reachableFree := func(c board.CellID) bool {
		if taken[c] || !p.enterable(c, entity, domain) {
			return false
		}
		if c == from {
			path = Path{}
			return true
		}
		found, ok := p.findPath(entity, domain, from, c)
		path = found
		return ok
	}
	dest, ok := breadthFirst(target, p.grid.Neighbors, passable, reachableFree, maxVisitedCells)
	return dest, path, ok
}
