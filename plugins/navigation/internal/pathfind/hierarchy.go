package pathfind

import (
	"math"

	"github.com/kjkrol/hpastar"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/hpastar/graph"
	"github.com/kjkrol/uid"
)

// Key is what a hierarchy is built for: the domain and the body's class, its feet left out.
type Key struct {
	Domain cell.Domain
	Body   Body
}

// Hierarchy is a Finder's hierarchy over the cells for one Key: hpastar's HPA* over the leveled
// nodes its class stands on, its clusters stale where the board's passage moved since they were
// built.
type Hierarchy struct {
	key   Key
	side  int
	since uint64 // the board's Passage the clusters were last marked stale against
	h     *hpastar.Solver[node]
	// the Finder whose steps h is built over: it alone builds and stales h
	builder    *Finder
	grid       grid.Grid
	cols, rows int
	across     int // the clusters of level 1 a row
	// by node slot, a bit a level of a cell: the key's class stands on it, h's graph holding no
	// other node; written by the builder alone, read by every Search
	nodes []uint64
	laid  bool
	// by cluster of level 1, the Stale that counted it
	counted []uint32
	stale   uint32
}

// a cell no grid of grid.DefaultGrids has, the builder's goal: no step leads to a goal while a
// hierarchy is built
const offGrid = cell.ID(math.MaxUint64)

// NewHierarchy is a Hierarchy over p's grid for key, its clusters side cells a side, nothing
// built yet; p alone builds and stales it. Over a Finder without levels, or a grid not of
// grid.DefaultGrids, it is never Ready.
func (p *Finder) NewHierarchy(key Key, side int) *Hierarchy {
	h := &Hierarchy{key: key, side: max(side, 4), builder: p, grid: p.grid}
	shape, ok := shapeOf(p.grid)
	if !ok || p.levels == nil || p.stepped == nil {
		return h
	}
	h.cols, h.rows = int(shape.Cols), int(shape.Rows)
	h.across = (h.cols + h.side - 1) / h.side
	h.nodes = make([]uint64, (2*p.grid.CellCount()+63)/64)
	// a step's destination is a node its class stands on, so a step out of one leads to another;
	// only a start may stand on none, and steps out of it as a search does
	h.h = hpastar.NewSolver(graph.Implicit[node]{
		Forward:   p.stepped,
		Heuristic: h.distance,
		Index:     p.nodeSlot,
		Size:      2*p.grid.CellCount() + 2,
		Layout: &graph.Layout[node]{
			Coords:   h.coords,
			At:       h.at,
			Width:    h.cols,
			Height:   h.rows,
			Backward: backward{p, h}.steps,
		},
	}, hpastar.WithClusterSide(h.side), hpastar.WithNear(h.near()))
	h.counted = make([]uint32, h.across*((h.rows+h.side-1)/h.side))
	return h
}

// a grid's shape: a board's read says it of the grid it embeds
func shapeOf(g grid.Grid) (grid.Shape, bool) {
	if s, ok := g.(interface{ Shape() (grid.Shape, bool) }); ok {
		return s.Shape()
	}
	return grid.ShapeOf(g)
}

// Key is what h was built for.
func (h *Hierarchy) Key() Key { return h.key }

// Side is the side of h's level-1 clusters, in cells.
func (h *Hierarchy) Side() int { return h.side }

// Ready says no cluster of h is stale.
func (h *Hierarchy) Ready() bool { return h.h != nil && h.h.Ready() }

// the Layout reads the grid and the nodes alone: every Search asks it at once
func (h *Hierarchy) coords(n node) (int, int, bool) {
	x, y, ok := h.grid.Coords(n.c)
	return int(x), int(y), ok
}

func (h *Hierarchy) at(x, y int, buf []node) []node {
	c := h.grid.At(uint32(x), uint32(y))
	if o, ok := h.grid.Ordinal(c); ok {
		if h.stood(2*o + int(ground)) {
			buf = append(buf, node{c, ground})
		}
		if h.stood(2*o + int(deck)) {
			buf = append(buf, node{c, deck})
		}
	}
	return buf
}

func (h *Hierarchy) distance(a, b node) float64 { return h.grid.Distance(a.c, b.c) }

// near is how far apart a route's ends lie at least for h to climb it, twice a cluster's side:
// hpastar would search a nearer one itself, in a dictionary every Search would keep besides the
// Finder's, which searches it instead
func (h *Hierarchy) near() float64 { return 2 * float64(h.side) }

// clusterOf is n's cluster of level 1, as hpastar lays them out from the Layout's coordinates
func (h *Hierarchy) clusterOf(n node) (int, bool) {
	x, y, ok := h.coords(n)
	if !ok {
		return 0, false
	}
	return y/h.side*h.across + x/h.side, true
}

// stands says whether n is a node of h's graph, one its class stands on
func (h *Hierarchy) stands(n node) bool {
	o, ok := h.grid.Ordinal(n.c)
	return ok && n.level != goal && h.stood(2*o+int(n.level))
}

func (h *Hierarchy) stood(slot int) bool { return h.nodes[slot>>6]&(1<<(slot&63)) != 0 }

func (h *Hierarchy) setStood(slot int, on bool) {
	if on {
		h.nodes[slot>>6] |= 1 << (slot & 63)
	} else {
		h.nodes[slot>>6] &^= 1 << (slot & 63)
	}
}

// standing reads which of c's levels, o its ordinal, h's class stands on into h: the cell has
// the level and admits the class's domain there
func (p *Finder) standing(h *Hierarchy, c cell.ID, o int) {
	r := p.at(c)
	h.setStood(2*o+int(ground), p.stepAt(r, ground).admits)
	h.setStood(2*o+int(deck), len(p.levelsOf(r)) > 1 && p.stepAt(r, deck).admits)
}

// building has p read the board as h is built over it: for its key, blind to the others, no
// step leading to a goal
func (p *Finder) building(h *Hierarchy) {
	if h.builder != p {
		panic("pathfind: a hierarchy is built and staled by the Finder that made it")
	}
	p.body, p.entity, p.blocked = h.key.Body, 0, nil
	p.goal, p.end = offGrid, Either
	p.begin(h.key.Domain)
}

// lay reads which nodes of h's cells its class stands on, h then stale against the board's
// passage as it is
func (p *Finder) lay(h *Hierarchy) {
	for y := range h.rows {
		for x := range h.cols {
			c := p.grid.At(uint32(x), uint32(y))
			o, _ := p.grid.Ordinal(c)
			p.standing(h, c, o)
		}
	}
	h.since, h.laid = p.passage(), true
}

// passage is the board's passage, 0 where the terrain counts no changes
func (p *Finder) passage() uint64 {
	if p.passing == nil {
		return 0
	}
	return p.passing.Passage()
}

// Stale marks the clusters of the cells whose passage moved since the board's passage since,
// their ring of one cell and what lies across their borders, and reports how many it marked.
func (p *Finder) Stale(h *Hierarchy, since uint64) int {
	if h.h == nil {
		return 0
	}
	expanded := p.expansions
	defer p.keepExpansions(expanded)
	p.building(h)
	if !h.laid {
		p.lay(h)
		return 0
	}
	since = min(since, h.since)
	now := p.passage()
	if p.passing != nil && now <= since {
		return 0
	}
	// a terrain counting no cell's passage has every cell changed
	cells, _ := p.terrain.(interface{ CellPassage(c cell.ID) uint64 })
	if h.stale++; h.stale == 0 {
		clear(h.counted)
		h.stale = 1
	}
	marked := 0
	for y := range h.rows {
		for x := range h.cols {
			c := p.grid.At(uint32(x), uint32(y))
			if cells != nil && cells.CellPassage(c) <= since {
				continue
			}
			marked += p.staleAt(h, c)
			if cells == nil {
				continue
			}
			// a step reads its neighbours: the corners it cuts, the bands over it, the slope
			for i := range p.grid.Directions() {
				if n, ok := p.grid.Toward(c, i); ok {
					marked += p.staleAt(h, n)
				}
			}
		}
	}
	h.since = now
	return marked
}

// staleAt reads anew which of c's nodes h's class stands on and marks the clusters of c's nodes
// stale, of its deck too where h held it or holds it now; it reports 1 where this Stale had not
// yet counted c's cluster of level 1
func (p *Finder) staleAt(h *Hierarchy, c cell.ID) int {
	o, _ := p.grid.Ordinal(c)
	held := h.stood(2*o + int(deck))
	p.standing(h, c, o)
	h.h.Changed(node{c, ground})
	if held || h.stood(2*o+int(deck)) {
		h.h.Changed(node{c, deck})
	}
	k, _ := h.clusterOf(node{c, ground})
	if h.counted[k] == h.stale {
		return 0
	}
	h.counted[k] = h.stale
	return 1
}

// the steps a hierarchy's build or staling takes are no search's
func (p *Finder) keepExpansions(n uint64) { p.expansions = n }

// Build builds what is stale of h, budget steps of hpastar's work and the step that spends them,
// the lowest level first, and reports how many clusters it finished; a budget of none builds
// nothing.
func (p *Finder) Build(h *Hierarchy, budget int) (built int) {
	if h.h == nil || budget <= 0 {
		return 0
	}
	expanded := p.expansions
	defer p.keepExpansions(expanded)
	p.building(h)
	if !h.laid {
		p.lay(h)
	}
	built, _ = h.h.Update(budget)
	p.builds += uint64(built)
	return built
}

// Climb writes the route from from to to, ending on its level end, for entity moving in domain
// into dst as Find writes one, through h; false where h refuses it (a short route, a start its
// class cannot stand on, no route, not ready), the caller searching then.
func (p *Finder) Climb(h *Hierarchy, entity uid.UID64, domain cell.Domain, body Body,
	from, to cell.ID, end Level, dst []cell.ID) (int, bool) {
	if !p.answers(h, domain, body) || !h.h.Ready() {
		return 0, false
	}
	p.decks, p.body, p.entity = 0, body, entity
	if p.grid.Distance(from, to) < h.near() {
		return 0, false
	}
	p.begin(domain)
	start := node{from, p.levelOf(p.at(from), body)}
	// hpastar sets a start into its own cluster alone: from one h holds no node of, its steps
	// across the border go unseen
	if !h.stands(start) {
		return 0, false
	}
	s := p.searchOf(h)
	p.goal, p.end = to, end
	if len(p.levelsOf(p.at(to))) == 1 {
		p.end = Either
	}
	c := &p.climbing
	// the start, every step dst holds and the goal past them where the route ends there
	if cap(c.route) < len(dst)+2 {
		c.route = make([]node, len(dst)+2)
	}
	climbs := s.Stats().Climbs
	n, ok := s.Path(start, node{to, goal}, c.route[:len(dst)+2])
	if !ok {
		return 0, false
	}
	p.climbs += s.Stats().Climbs - climbs
	return p.stepsOf(c.route[:n], dst)
}

// Reaches says whether to can be reached from from for a body of body's class moving in
// domain, read off h's level 1: false only where it cannot.
func (p *Finder) Reaches(h *Hierarchy, domain cell.Domain, body Body, from, to cell.ID) bool {
	if !p.answers(h, domain, body) {
		return true
	}
	p.body, p.entity = body, 0
	p.begin(domain)
	start := node{from, p.levelOf(p.at(from), body)}
	if !h.stands(start) {
		return true
	}
	s := p.searchOf(h)
	p.goal, p.end = to, Either
	return s.Reaches(start, node{to, goal})
}

// answers says whether h answers p for a body of body's class moving in domain: built for them
// over p's levels, its clusters marked stale against the board's passage as it is
func (p *Finder) answers(h *Hierarchy, domain cell.Domain, body Body) bool {
	body.Feet = 0
	return h.h != nil && p.stepped != nil && p.levels != nil &&
		h.key == (Key{Domain: domain, Body: body}) && h.laid && p.passage() == h.since
}

// Climbs counts the routes p answered through a hierarchy.
func (p *Finder) Climbs() uint64 { return p.climbs }

// Builds counts the clusters p built.
func (p *Finder) Builds() uint64 { return p.builds }

// the most hierarchies a Finder keeps a Search of, the least lately asked given up for a new one
const climbers = 4

// what a Finder asks hierarchies through: a Search of each over its own steps, and the route
// its last climb found
type climbing struct {
	searches [climbers]climber
	asks     uint64
	route    []node
}

type climber struct {
	h     *Hierarchy
	s     *hpastar.Search[node]
	asked uint64
}

// searchOf is p's Search of h, made the first time p asks h
func (p *Finder) searchOf(h *Hierarchy) *hpastar.Search[node] {
	c := &p.climbing
	c.asks++
	spare := &c.searches[0]
	for i := range c.searches {
		e := &c.searches[i]
		if e.h == h {
			e.asked = c.asks
			return e.s
		}
		if e.asked < spare.asked {
			spare = e
		}
	}
	spare.h, spare.asked = h, c.asks
	spare.s = h.h.SearchVia(p.stepped, backward{p, h}.steps)
	return spare.s
}

// backward is the steps into a node of h's graph as p reads them
type backward struct {
	p *Finder
	h *Hierarchy
}

// steps are leveledInto's from the nodes h's class stands on, counted as leveledFrom counts
func (b backward) steps(to, next node, buf []graph.Transition[node]) []graph.Transition[node] {
	b.p.expansions++
	buf = b.p.leveledInto(to, next, buf)
	kept := buf[:0]
	for _, s := range buf {
		if b.h.stands(s.To) {
			kept = append(kept, s)
		}
	}
	return kept
}
