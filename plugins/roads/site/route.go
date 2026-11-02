package site

import (
	"math"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/hpastar"
	"github.com/kjkrol/hpastar/graph"
)

// Leap is a way over cells a road cannot run on, a bridge's: from a cell to the head To across
// the Spans cells between them, at Cost, never negative, on top of the road's length.
type Leap struct {
	To    cell.ID
	Spans int
	Cost  float32
}

// Leaps gives the planner ways over what a road cannot run on, a map maker's bridges: every leap
// from c in the grid's direction way, appended to dst. nil for a road over the ground alone.
type Leaps func(c cell.ID, way int, dst []Leap) []Leap

// Route is the cheapest way for a road, graded or a track, from the cell from to the cell to over
// g under r, written into dst from from on, or a Refusal where none runs, or, its cells kept,
// where it runs more cells than Rules.Longest. A step costs its length at Rules.Metre (without
// it, one a step of the grid's Distance) by its ground's Price, dearer by Rules.Climb a unit of
// the slope left once a graded road's levelling digs and fills within Rules.Cut and Fill, its
// length alone along a road standing and over a span, a leap of leaps its Cost on top; none is
// taken steeper than Rules.Slope, into water but over a deck or through a ford the rules allow,
// onto ground that takes no road, onto a road without Rules.Crossings, across a deck, past a
// corner of water nor, graded, through a corner or round a bend on a cell the levelling cannot
// lay flat, as ground.RoadFits lays a road.
func Route(g Ground, r Rules, from, to cell.ID, graded, levelling bool, leaps Leaps,
	dst []cell.ID) ([]cell.ID, error) {
	return NewPlanner(g, r, graded, levelling, leaps).Route(from, to, dst)
}

// Planner is Route kept for one road over one Ground: what it searches with is made once, so a
// route asked again allocates nothing; one goroutine asks it at a time.
type Planner struct {
	g         Ground
	grid      grid.Grid
	r         Rules
	graded    bool
	levelling bool
	leaps     Leaps
	search    *hpastar.Search[routeNode]
	// by direction: a step's length in world units, whether it cuts a corner, the way back, and
	// the way from one cell beside the corner it cuts to the other (-1 for none)
	runs   []float64
	corner []bool
	back   []int
	across []int
	ways   int // a cell's states: one for each way in, and one for any
	metre  float64
	climb  float64
	cut    float32
	fill   float32
	steep  float64 // the most a step may rise a unit of its length
	eps    float64
	least  float64 // what one of the grid's Distance costs at the least
	cells  []routeCell
	seen   uint32 // the search the cells read now are stamped with
	off    routeCell
	goal   cell.ID
	// what the Ground, the leaps and the search write into
	heights []float32
	jumps   []Leap
	path    []routeNode
}

// routeCell is what a search read of a cell, once
type routeCell struct {
	seen  uint32
	h     float32 // the mean of its corners
	price float32
	// lo and hi bound the height a graded road may level the cell flat at, its corners dug and
	// filled as far as the rules let them; levels says they leave one
	lo     float32
	hi     float32
	road   cell.Links
	deck   cell.Links
	takes  bool
	wet    bool
	decked bool
	ford   bool
	fixed  bool
	levels bool
}

// routeNode is a state of the search: a cell, and the way a graded road came into it where it
// cannot level the cell and so runs straight on; routeAny where it may leave any way, routeEnd
// for the end past the route's last cell
type routeNode struct {
	c  cell.ID
	in int8
}

const (
	routeAny int8 = -1
	routeEnd int8 = -2
)

// NewPlanner is the Planner of a graded road, or a track, over g under r, levelling the ground
// or not, over the leaps leaps makes.
func NewPlanner(g Ground, r Rules, graded, levelling bool, leaps Leaps) *Planner {
	gr := g.Grid()
	p := &Planner{
		g:         g,
		grid:      gr,
		r:         r,
		graded:    graded,
		levelling: graded && levelling,
		leaps:     leaps,
		climb:     max(float64(r.Climb), 0),
		cut:       max(r.Cut, 0),
		fill:      max(r.Fill, 0),
		eps:       1e-4 * float64(gr.CellSpan()),
	}
	n := gr.Directions()
	p.ways = n + 1
	p.runs = make([]float64, n)
	p.corner = make([]bool, n)
	p.back = make([]int, n)
	p.across = make([]int, n)
	for i := range n {
		s := gr.Step(i)
		p.runs[i] = math.Hypot(float64(s.X), float64(s.Y))
		p.corner[i] = gr.Corner(i)
		p.back[i] = gr.Opposite(i)
		p.across[i] = -1
		if a, b, ok := gr.Flanks(i); ok {
			da, db := gr.Offset(a)
			ea, eb := gr.Offset(b)
			if j, ok := grid.Direction(gr, ea-da, eb-db); ok {
				p.across[i] = j
			}
		}
	}
	slope := r.Slope
	if graded {
		slope = min(slope, ground.RoadSlope)
	}
	p.steep = math.Tan(min(max(slope, 0), math.Pi/2))
	unit := p.unit()
	p.metre = float64(r.Metre)
	if p.metre <= 0 {
		// a step of the grid's Distance at one: with no cost to its length a route would wander
		p.metre = 1
		if unit > 0 {
			p.metre = 1 / unit
		}
	}
	p.least = unit * p.metre
	p.cells = make([]routeCell, gr.CellCount())
	p.path = make([]routeNode, gr.CellCount()+1)
	p.search = hpastar.NewSolver(graph.Implicit[routeNode]{
		Forward:   p.forward,
		Heuristic: p.estimate,
		Index:     p.slot,
		Size:      gr.CellCount()*p.ways + 1,
	}).Search()
	return p
}

// Route is the cheapest way from from to to, written into dst, as the package's Route finds it.
func (p *Planner) Route(from, to cell.ID, dst []cell.ID) ([]cell.ID, error) {
	p.begin()
	if err := p.lies(from); err != nil {
		return dst[:0], err
	}
	if err := p.lies(to); err != nil {
		return dst[:0], err
	}
	p.goal = to
	start := routeNode{c: from, in: routeAny}
	end := routeNode{c: to, in: routeEnd}
	n, ok := p.search.Path(start, end, p.path)
	for ok && p.path[n-1] != end {
		// a way through a cell it cannot level twice, crossing itself: longer than the cells
		p.path = make([]routeNode, 2*len(p.path))
		n, ok = p.search.Path(start, end, p.path)
	}
	if !ok {
		x, y, _ := p.grid.Coords(to)
		return dst[:0], Refuse(p.grid, from, "finds no way to [%d, %d]", x, y)
	}
	dst = dst[:0]
	for _, s := range p.path[:n-1] {
		dst = append(dst, s.c)
	}
	if longest := p.r.Longest; longest > 0 && len(dst) > longest {
		return dst, Refuse(p.grid, dst[longest], "runs %d cells, %d at most", len(dst), longest)
	}
	return dst, nil
}

// unit is how long one of the grid's Distance is in world units at most: every step's length
// over its Distance, at a cell in the middle of the grid; 0 where no cell has a neighbour
func (p *Planner) unit() float64 {
	var c cell.ID
	found := false
	if s, ok := shapeOf(p.grid); ok && s.Cols > 0 && s.Rows > 0 {
		c, found = p.grid.At(s.Cols/2, s.Rows/2), true
	} else {
		p.grid.EachCell(func(e cell.ID) {
			if !found {
				c, found = e, true
			}
		})
	}
	unit := math.Inf(1)
	for i := range p.runs {
		if n, ok := p.grid.Toward(c, i); found && ok && n != c {
			if d := p.grid.Distance(c, n); d > 0 {
				unit = min(unit, p.runs[i]/d)
			}
		}
	}
	if math.IsInf(unit, 1) {
		return 0
	}
	return unit
}

// a board's read says the shape of the grid it embeds
func shapeOf(g grid.Grid) (grid.Shape, bool) {
	if s, ok := g.(interface{ Shape() (grid.Shape, bool) }); ok {
		return s.Shape()
	}
	return grid.ShapeOf(g)
}

// begin has the next search read every cell anew: the Ground may have changed since the last
func (p *Planner) begin() {
	if p.seen++; p.seen == 0 {
		clear(p.cells)
		p.seen = 1
	}
}

func (p *Planner) slot(n routeNode) int {
	o, ok := p.grid.Ordinal(n.c)
	if !ok || n.in == routeEnd {
		return p.grid.CellCount() * p.ways
	}
	return o*p.ways + int(n.in) + 1
}

// no step costs less than its length at the metre, nor a leap than its own: a Price is 1 at least
func (p *Planner) estimate(a, b routeNode) float64 { return p.grid.Distance(a.c, b.c) * p.least }

// at is c as this search reads it; off the grid, a cell no road may take
func (p *Planner) at(c cell.ID) *routeCell {
	o, ok := p.grid.Ordinal(c)
	if !ok {
		return &p.off
	}
	rc := &p.cells[o]
	if rc.seen == p.seen {
		return rc
	}
	*rc = routeCell{
		seen:   p.seen,
		price:  max(p.g.Price(c), 1),
		road:   p.g.Road(c),
		takes:  p.g.Takes(c),
		wet:    p.g.Wet(c),
		fixed:  p.levelling && p.g.Fixed(c),
		levels: true,
	}
	rc.deck, rc.decked = p.g.Decked(c)
	rc.ford = p.g.Ford(c)
	p.heights = p.g.Heights(c, p.heights[:0])
	if len(p.heights) == 0 {
		return rc
	}
	sum := 0.0
	top := p.heights[0]
	bottom := p.heights[0]
	for _, h := range p.heights {
		sum += float64(h)
		top = max(top, h)
		bottom = min(bottom, h)
	}
	rc.h = float32(sum / float64(len(p.heights)))
	if p.graded && !rc.span() {
		rc.lo = top
		rc.hi = bottom
		if p.levelling && !rc.fixed {
			rc.lo -= p.cut
			rc.hi += p.fill
		}
		rc.levels = float64(rc.lo) <= float64(rc.hi)+p.eps
	}
	return rc
}

// head says whether rc is a bridge's head: the deck runs one way out of it, its other side ground
func (rc *routeCell) head() bool { return rc.decked && rc.deck != 0 && rc.deck&(rc.deck-1) == 0 }

// span says whether rc lies under a bridge's span, whose deck is not at its ground's height
func (rc *routeCell) span() bool { return rc.decked && !rc.head() }

// lies is why a road may not lie on c, nil where it may
func (p *Planner) lies(c cell.ID) error {
	if _, ok := p.grid.Ordinal(c); !ok {
		return Refuse(p.grid, c, "lies off the board")
	}
	rc := p.at(c)
	if _, ok := p.enter(rc); ok {
		return nil
	}
	switch {
	case rc.decked && !p.r.Decks:
		return Refuse(p.grid, c, "may not run over a bridge's deck")
	case rc.road != 0:
		return Refuse(p.grid, c, "may not cross or join a road standing")
	case rc.ford:
		return Refuse(p.grid, c, "may not run through a ford")
	case rc.wet:
		return Refuse(p.grid, c, "may not run into water without a deck or a ford")
	}
	return Refuse(p.grid, c, "lies on ground that takes no road")
}
