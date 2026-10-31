package pathfind

import (
	"fmt"
	"math"

	"github.com/kjkrol/hpastar"

	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/hpastar/graph"
	"github.com/kjkrol/uid"
)

// Key is what a hierarchy is built for: the domain whose classes of bodies it serves.
type Key struct {
	Domain cell.Domain
}

// Hierarchy is a Finder's hierarchy over the cells for one Key: hpastar's HPA* over the leveled
// nodes some of its classes stand on, each class taking the steps its body takes, its clusters
// stale where the board's passage moved since they were built.
type Hierarchy struct {
	key     Key
	classes []Body // hpastar's classes by index, their feet left out
	all     uint8  // a bit a class
	side    int
	since   uint64 // the board's Passage the clusters were last marked stale against
	h       *hpastar.Solver[node]
	// the Finder whose steps h is built over: it alone builds and stales h
	builder    *Finder
	grid       grid.Grid
	cols, rows int
	across     int // the clusters of level 1 a row
	// by node slot, the classes standing on the node, a bit a class, h's graph holding no node
	// none stands on; written by the builder alone, read by every Search
	on   []uint8
	laid bool
	// by cluster of level 1, the Stale that counted it
	counted []uint32
	stale   uint32
}

// a cell no grid of grid.DefaultGrids has, the builder's goal: no step leads to a goal while a
// hierarchy is built
const offGrid = cell.ID(math.MaxUint64)

// NewHierarchy is a Hierarchy over p's grid for key's domain serving classes, at most MaxClasses
// bodies, its clusters side cells a side, nothing built yet; p alone builds and stales it. Over a
// Finder without levels, or a grid not of grid.DefaultGrids, it is never Ready.
func (p *Finder) NewHierarchy(key Key, side int, classes []Body) *Hierarchy {
	if len(classes) == 0 || len(classes) > MaxClasses {
		panic(fmt.Sprintf("pathfind: a hierarchy serves 1 to %d classes, not %d", MaxClasses,
			len(classes)))
	}
	h := &Hierarchy{key: key, side: max(side, 4), builder: p, grid: p.grid,
		classes: make([]Body, len(classes)), all: uint8(1<<len(classes) - 1)}
	for k, body := range classes {
		body.Feet = 0
		h.classes[k] = body
	}
	shape, ok := shapeOf(p.grid)
	if !ok || p.levels == nil || p.stepped == nil {
		return h
	}
	h.cols, h.rows = int(shape.Cols), int(shape.Rows)
	h.across = (h.cols + h.side - 1) / h.side
	h.on = make([]uint8, 2*p.grid.CellCount())
	forward, backward := h.steps(p)
	h.h = hpastar.NewSolver(graph.Implicit[node]{
		Forward:   forward,
		Heuristic: h.distance,
		Index:     p.nodeSlot,
		Size:      2*p.grid.CellCount() + 2,
		Layout: &graph.Layout[node]{
			Coords:   h.coords,
			At:       h.at,
			Width:    h.cols,
			Height:   h.rows,
			Backward: backward,
		},
		Classes: len(classes),
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

// Classes are the bodies h serves, by hpastar's index of their classes.
func (h *Hierarchy) Classes() []Body { return h.classes }

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
		if h.on[2*o+int(ground)] != 0 {
			buf = append(buf, node{c, ground})
		}
		if h.on[2*o+int(deck)] != 0 {
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

// classesOn are the classes standing on n, a bit a class; none on a goal or off the grid
func (h *Hierarchy) classesOn(n node) uint8 {
	o, ok := h.grid.Ordinal(n.c)
	if !ok || n.level == goal {
		return 0
	}
	return h.on[2*o+int(n.level)]
}

// standsAs says whether class k stands on n, a node of h's graph of class k
func (h *Hierarchy) standsAs(n node, k int) bool { return h.classesOn(n)&(1<<k) != 0 }

// classOf is the index of h's class body is, its feet left out; false where it is none
func (h *Hierarchy) classOf(body Body) (int, bool) {
	body.Feet = 0
	for k := range h.classes {
		if h.classes[k] == body {
			return k, true
		}
	}
	return 0, false
}

// standing reads which of h's classes stand on each of c's levels, o its ordinal, into h
func (p *Finder) standing(h *Hierarchy, c cell.ID, o int) {
	r := p.at(c)
	h.on[2*o+int(ground)] = p.standers(h, r, ground)
	h.on[2*o+int(deck)] = 0
	if len(p.levelsOf(r)) > 1 {
		h.on[2*o+int(deck)] = p.standers(h, r, deck)
	}
}

// standers are the classes of h standing on r's cell on level, a bit a class: the cell admits
// h's domain there and takes each body by what limits it
func (p *Finder) standers(h *Hierarchy, r *read, level uint8) uint8 {
	if !p.stepAt(r, level).admits {
		return 0
	}
	if !p.limited || !p.limiting(r, level) {
		return h.all
	}
	var on uint8
	for k := range h.classes {
		if p.borne(r, level, &h.classes[k]) {
			on |= 1 << k
		}
	}
	return on
}

// building has p read the board as h is built over it: for every class of its domain, blind to
// the others, no step leading to a goal
func (p *Finder) building(h *Hierarchy) {
	if h.builder != p {
		panic("pathfind: a hierarchy is built and staled by the Finder that made it")
	}
	p.body, p.entity, p.blocked = h.classes[0], 0, nil
	p.goal, p.end = offGrid, Either
	p.begin(h.key.Domain)
}

// lay reads which of h's classes stand on each node of h's cells, h then stale against the
// board's passage as it is
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

// staleAt reads anew which of h's classes stand on c's nodes and marks the clusters of c's nodes
// stale, of its deck too where some class stood on it or stands on it now; it reports 1 where
// this Stale had not yet counted c's cluster of level 1
func (p *Finder) staleAt(h *Hierarchy, c cell.ID) int {
	o, _ := p.grid.Ordinal(c)
	held := h.on[2*o+int(deck)] != 0
	p.standing(h, c, o)
	h.h.Changed(node{c, ground})
	if held || h.on[2*o+int(deck)] != 0 {
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
// into dst as Find writes one, through h as the class body is; false where h refuses it (a short
// route, a body of none of its classes, a start its class cannot stand on, no route, not ready),
// the caller searching then.
func (p *Finder) Climb(h *Hierarchy, entity uid.UID64, domain cell.Domain, body Body,
	from, to cell.ID, end Level, dst []cell.ID) (int, bool) {
	k, ok := p.answers(h, domain, body)
	if !ok || !h.h.Ready() {
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
	if !h.standsAs(start, k) {
		return 0, false
	}
	s := p.searchOf(h, k)
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

// Reaches says whether to can be reached from from for body, one of h's classes, moving in
// domain, read off h's level 1: false only where it cannot.
func (p *Finder) Reaches(h *Hierarchy, domain cell.Domain, body Body, from, to cell.ID) bool {
	k, ok := p.answers(h, domain, body)
	if !ok {
		return true
	}
	p.body, p.entity = body, 0
	p.begin(domain)
	start := node{from, p.levelOf(p.at(from), body)}
	if !h.standsAs(start, k) {
		return true
	}
	s := p.searchOf(h, k)
	p.goal, p.end = to, Either
	return s.Reaches(start, node{to, goal})
}

// answers is the index of the class of h body is, where h answers p for it moving in domain:
// built for the domain over p's levels, its clusters marked stale against the board's passage as
// it is
func (p *Finder) answers(h *Hierarchy, domain cell.Domain, body Body) (int, bool) {
	if h.h == nil || p.stepped == nil || p.levels == nil || h.key.Domain != domain || !h.laid ||
		p.passage() != h.since {
		return 0, false
	}
	return h.classOf(body)
}

// Climbs counts the routes p answered through a hierarchy.
func (p *Finder) Climbs() uint64 { return p.climbs }

// Builds counts the clusters p built.
func (p *Finder) Builds() uint64 { return p.builds }

// the most hierarchies a Finder keeps a Search of, the least lately asked given up for a new one
const climbers = 4

// what a Finder asks hierarchies through: a Search of each over its own steps, the class it asks
// as, and the route its last climb found
type climbing struct {
	searches [climbers]climber
	asks     uint64
	class    int
	route    []node
}

type climber struct {
	h     *Hierarchy
	s     *hpastar.Search[node]
	asked uint64
}

// searchOf is p's Search of h asking as class k, made the first time p asks h
func (p *Finder) searchOf(h *Hierarchy, k int) *hpastar.Search[node] {
	c := &p.climbing
	c.asks++
	c.class = k
	spare := &c.searches[0]
	for i := range c.searches {
		e := &c.searches[i]
		if e.h == h {
			e.asked = c.asks
			e.s.Class(k)
			return e.s
		}
		if e.asked < spare.asked {
			spare = e
		}
	}
	v := via{p, h}
	spare.h, spare.asked = h, c.asks
	spare.s = h.h.SearchVia(v.forward, v.backward)
	spare.s.Class(k)
	return spare.s
}

// via are the steps of the class p asks h as, as p reads the board: those leveledFrom and
// leveledInto take for its body between the nodes the class stands on, h's graph of the class
type via struct {
	p *Finder
	h *Hierarchy
}

// forward steps out of a node the class stands on alone, counted as leveledFrom counts
func (v via) forward(from, prev node, buf []graph.Transition[node]) []graph.Transition[node] {
	if !v.h.standsAs(from, v.p.climbing.class) {
		v.p.expansions++
		return buf[:0]
	}
	return v.p.leveledFrom(from, prev, buf)
}

// backward steps into to from the nodes the class stands on, counted as leveledFrom counts
func (v via) backward(to, next node, buf []graph.Transition[node]) []graph.Transition[node] {
	v.p.expansions++
	buf = v.p.leveledInto(to, next, buf)
	kept := buf[:0]
	for _, s := range buf {
		if v.h.standsAs(s.To, v.p.climbing.class) {
			kept = append(kept, s)
		}
	}
	return kept
}

// steps are the graph h is built over, as p reads the board: of one class, whose Refuses hpastar
// never reads, the steps leveledFrom takes for its body, which lead to nodes it stands on, and
// leveledInto's from those; of several, classedFrom's and classedInto's
func (h *Hierarchy) steps(p *Finder) (forward, backward graph.Transitions[node]) {
	if len(h.classes) == 1 {
		return p.stepped, alone{p, h}.backward
	}
	g := classed{p, h}
	return g.forward, g.backward
}

// alone is the graph of a hierarchy of one class into a node: the steps from the nodes the class
// stands on, the only ones a build expands
type alone struct {
	p *Finder
	h *Hierarchy
}

func (g alone) backward(to, next node, buf []graph.Transition[node]) []graph.Transition[node] {
	buf = g.p.leveledInto(to, next, buf)
	kept := buf[:0]
	for _, s := range buf {
		if g.h.classesOn(s.To) != 0 {
			kept = append(kept, s)
		}
	}
	return kept
}

// classed is the graph a hierarchy of several classes is built over, as its builder reads the
// board: the steps some class stands on the node they leave and takes, each with the classes
// refusing it
type classed struct {
	p *Finder
	h *Hierarchy
}

func (g classed) forward(from, prev node, buf []graph.Transition[node]) []graph.Transition[node] {
	return g.p.classedFrom(g.h, from, prev, buf)
}

func (g classed) backward(to, next node, buf []graph.Transition[node]) []graph.Transition[node] {
	return g.p.classedInto(g.h, to, next, buf)
}

// classedFrom is leveledFrom for every class of h at once: each step some class takes out of
// from, the classes refusing it in its Refuses, those not standing on from among them; their
// labels join the ends of a step either way, so a class steps out of no node it does not stand
// on
func (p *Finder) classedFrom(h *Hierarchy, from, prev node,
	buf []graph.Transition[node]) []graph.Transition[node] {
	buf = buf[:0]
	stood := h.classesOn(from)
	if stood == 0 {
		return buf
	}
	if from.c == p.goal && (p.end == Either || p.end == Deck == (from.level == deck)) {
		buf = append(buf, graph.Transition[node]{To: node{p.goal, goal}, Refuses: h.all &^ stood})
	}
	rf := p.at(from.c)
	for i := range p.grid.Directions() {
		n, ok := p.grid.Toward(from.c, i)
		if !ok || n == prev.c || p.blocked != nil && p.blocked(n) {
			continue
		}
		rn := p.at(n)
		c := p.cornerOf(from.c, n, i)
		for _, level := range p.levelsOf(rn) {
			if cost, refuses, ok := p.classedStep(h, from, rf, rn, level, c, stood); ok {
				buf = append(buf, graph.Transition[node]{To: node{n, level}, Cost: cost,
					Refuses: refuses})
			}
		}
	}
	return buf
}

// classedInto is classedFrom seen from where its steps end: the steps into to, each from the
// node it is taken from, priced and refused as classedFrom has it there
func (p *Finder) classedInto(h *Hierarchy, to, next node,
	buf []graph.Transition[node]) []graph.Transition[node] {
	buf = buf[:0]
	if to.level == goal {
		if to.c != p.goal {
			return buf
		}
		for _, level := range p.levelsOf(p.at(to.c)) {
			n := node{to.c, level}
			if stood := h.classesOn(n); stood != 0 &&
				(p.end == Either || p.end == Deck == (level == deck)) {
				buf = append(buf, graph.Transition[node]{To: n, Refuses: h.all &^ stood})
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
		rn := p.at(n)
		c := p.cornerOf(n, to.c, p.grid.Opposite(i))
		for _, level := range p.levelsOf(rn) {
			from := node{n, level}
			stood := h.classesOn(from)
			if stood == 0 {
				continue
			}
			if cost, refuses, ok := p.classedStep(h, from, rn, rt, to.level, c, stood); ok {
				buf = append(buf, graph.Transition[node]{To: from, Cost: cost, Refuses: refuses})
			}
		}
	}
	return buf
}

// classedStep is step for every class of h: what the step from from onto rn's cell on level
// costs and the classes refusing it, stood the classes standing on from; false where every class
// refuses it. What no body changes is read once, and a step meeting nothing a body is measured
// against is refused alike by every class, read once too.
func (p *Finder) classedStep(h *Hierarchy, from node, rf, rn *read, level uint8, c corner,
	stood uint8) (float64, uint8, bool) {
	if !p.occupancy.CanEnter(rn.c, level == deck, p.entity, p.domain) ||
		!p.crosses(from, rf, rn, level, c.diagonal) {
		return 0, 0, false
	}
	k := p.stepAt(rn, level)
	if !k.admits {
		return 0, 0, false
	}
	lane := p.laneOf(from.c, rn.c, level == deck)
	refuses := h.all &^ stood
	if p.bodily(from, rf, rn, level, c, lane) {
		for i := range h.classes {
			if refuses&(1<<i) == 0 && !p.takes(from, rf, rn, level, c, lane, h.classes[i]) {
				refuses |= 1 << i
			}
		}
	} else if !p.takes(from, rf, rn, level, c, lane, h.classes[0]) {
		return 0, 0, false
	}
	if refuses == h.all {
		return 0, 0, false
	}
	cost, ok := p.price(from.c, rn, k, level, c.diagonal)
	return cost, refuses, ok
}

// bodily says whether the step from from onto rn's cell on level meets what a body is measured
// against: a band narrowing it, a deck raised over a cell it crosses or cuts past, a cell
// limiting whom it takes
func (p *Finder) bodily(from node, rf, rn *read, level uint8, c corner, lane float32) bool {
	return !f32.IsInf(lane, 1) ||
		p.raised(rf) || p.raised(rn) || c.diagonal && (p.raised(c.r1) || p.raised(c.r2)) ||
		p.limited && (p.limiting(rf, from.level) || p.limiting(rn, level) ||
			c.diagonal && (p.limiting(c.r1, ground) || p.limiting(c.r2, ground)))
}

// takes says whether body takes the step from from onto rn's cell on level by what its body
// meets, as step has it: the corner it cuts, the decks, the band's lane, the limits
func (p *Finder) takes(from node, rf, rn *read, level uint8, c corner, lane float32,
	body Body) bool {
	p.body = body
	if c.diagonal && p.cutOff(c) && !p.along(from, rf, rn, level) {
		return false
	}
	return p.passes(from, rf, rn, level, body, c) && p.fitsLane(lane, from.c, rn.c, body) &&
		(!p.limited || p.within(from, rf, rn, level, body))
}
