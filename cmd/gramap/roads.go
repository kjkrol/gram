package main

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/network"
	"github.com/kjkrol/gram/plugins/board/water"
	bridgesite "github.com/kjkrol/gram/plugins/bridges/site"
	roadsite "github.com/kjkrol/gram/plugins/roads/site"
	usite "github.com/kjkrol/gram/plugins/underground/site"
)

type roads struct {
	w        *chart
	cfg      roadsCfg
	rules    roadsite.Rules
	planners []*roadsite.Planner // a leg's, tightest first
	net      *network.Network
	laid     []road
	places   map[string]cell.ID
	spans    []*span
	spanOf   map[cell.ID]*span
	wetK     []bool // the corners of running water: its beds, never moved
	marks    map[cell.ID]mark
	barred   map[cell.ID]bool
	banned   map[[2]cell.ID]bool
	dear     map[cell.ID]float32     // how many times dearer to the planner, where levelling failed
	offered  map[[2]cell.ID]bridging // to an attempt's searches, weighed anew once a cell is barred
	// the leg under search and the corridor round it a planner keeps to, a way from its start
	// through a cell to its end reach long at most (0: none): a failed try searched all 205k cells
	leg     [2]cell.ID
	reach   float64
	graded  roadsite.Estimate // the levelling of every road laid
	skipped []skip
	route   []cell.ID
	outline []geom.Vec
	// a guide's: the cells a bridge may span, the corridor round the leg under search and its
	// trace, and the roads planned and marked, not yet levelled
	spannable map[cell.ID]bool
	corridor  map[cell.ID]bool
	traced    map[cell.ID]bool
	waiting   []road
	moved     map[cell.ID]cell.ID // the cell a leg ends on, by its junction's
	guide     *guide
	legs      int
	lost      []leg
	// the tunnels dug, those of the guide's that could not be, the floors under the cells they
	// run under, their heads and the cells by their mouths
	tunnels   []tunnel
	undug     int
	unreached int // tunnels dug no road was laid to
	bored     map[cell.ID]cell.Built
	heads     map[cell.ID]bool
	mouths    map[cell.ID]bool
	pinned    map[int]bool // the corners of the heads and of the cells by the mouths
}

// a leg of a route, from one place to the next
type road struct {
	cells []cell.ID
	over  []bool    // a bridge's span, or a ford led onto
	at    []float64 // metres: the ground's middle, a bridge's deck
}

type span struct {
	cells  []cell.ID
	way    int
	ford   bool
	heads  []cell.ID
	deck   float64 // metres
	merged bool
}

type mark struct {
	paved   bool
	ways    cell.Links
	bridged bool // a span, or a ford
	ford    bool
	axis    int
	deck    float64
	head    bool
	headWay int
}

// a bridge offered to the search: its way and spans, its deck in metres and its price on top of
// the road's length over it
type bridging struct {
	way, n int
	deck   float64
	cost   float32
	ok     bool
}

type skip struct {
	from, to string
	why      error
}

// a corner and its height, metres
type height struct {
	k int
	h float64
}

// none is the way into the cell a road begins on, and out of the one it ends on
const none = -1

// how often a leg is planned anew under one route allowance, a cell made dearer, barred or a
// bridge banned each time
const tries = 24

// the shares of the route's cut and fill a leg is planned with in turn: none of it, so a road
// climbs as it may lie, then more where none leads on
var rungs = [...]float64{0, 0.25, 0.5, 1}

// how many times dearer to the planner a cell gets where the levelling cannot lay the road
const dearer = 3

// metres a deck keeps further off the ground than the rules ask, so the map's centimetres keep it
const margin = 0.05

// metres: the deck's thickness and a metre to spare
func (r *roads) clearance() float64 { return r.cfg.Thickness*r.w.figures() + 1 }

// metres: the figures the map is made for, 2 m where it says none, and the clearance over them
func (r *roads) underpass() float64 {
	return max(r.cfg.Figure*r.w.figures(), 2) + r.clearance()
}

func newRoads(w *chart) *roads {
	r := &roads{w: w, cfg: w.r.Roads, net: network.New(w.grid), places: map[string]cell.ID{},
		spanOf: map[cell.ID]*span{}, wetK: make([]bool, len(w.corners)),
		marks: map[cell.ID]mark{}, barred: map[cell.ID]bool{}, banned: map[[2]cell.ID]bool{},
		dear: map[cell.ID]float32{}, offered: map[[2]cell.ID]bridging{},
		bored: map[cell.ID]cell.Built{}, heads: map[cell.ID]bool{}, mouths: map[cell.ID]bool{},
		pinned: map[int]bool{}}
	r.rules = r.cfg.rules(w.perUnit, w.grid.CellCount())
	for _, share := range rungs {
		rules := r.cfg.routing(w.perUnit, w.grid.CellCount(), share)
		r.planners = append(r.planners, roadsite.NewPlanner(plannerGround{chartGround{r: r}}, rules,
			true, !w.flat, r.leaps))
	}
	w.grid.EachCell(func(c cell.ID) {
		if w.wet(c) {
			for _, k := range w.cornersOf(c) {
				r.wetK[k] = true
			}
		}
	})
	return r
}

func (r *roads) lay() error {
	names := make([]string, 0, len(r.cfg.Places))
	for name := range r.cfg.Places {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		c, ok := r.nearest(r.cfg.Places[name].At)
		if !ok {
			return fmt.Errorf("no ground a road may take near %s", name)
		}
		r.places[name] = c
	}
	if r.guide != nil {
		r.follow(r.guide)
	} else {
		for _, route := range r.cfg.Routes {
			for i := 1; i < len(route); i++ {
				rd, err := r.once(r.places[route[i-1]], r.places[route[i]])
				if err != nil {
					r.skipped = append(r.skipped, skip{route[i-1], route[i], err})
					continue
				}
				r.laid = append(r.laid, rd)
			}
		}
	}
	r.net = r.layout(r.laid).net
	r.reached()
	for _, rd := range r.laid {
		for _, b := range crossed(rd) {
			spans := rd.cells[b[0]+1 : b[1]]
			r.add(&span{cells: slices.Clone(spans), way: r.wayOf(rd.cells[b[0]], spans[0]),
				heads: []cell.ID{rd.cells[b[0]], rd.cells[b[1]]}})
		}
	}
	if r.w.flat {
		return nil
	}
	return r.settle()
}

func (r *roads) roadable(c cell.ID) bool {
	for _, k := range r.w.cornersOf(c) {
		x, y := k%(r.w.cols+1), k/(r.w.cols+1)
		if !r.w.ashore(x, y) {
			return false
		}
	}
	return true
}

func (r *roads) crossing(c cell.ID) bool {
	if r.w.wet(c) {
		return true
	}
	if r.w.flat {
		return false
	}
	for _, k := range r.w.cornersOf(c) {
		if r.wetK[k] {
			return true
		}
	}
	return false
}

// ford reports whether a road may cross c through its ford, leading onto it
func (r *roads) ford(c cell.ID) bool {
	return r.cfg.Fords && r.w.rivers.Courses[c] == water.Ford
}

// within reports whether c lies in the corridor round the leg under search
func (r *roads) within(c cell.ID) bool {
	if r.corridor != nil {
		return r.corridor[c]
	}
	g := r.w.grid
	return r.reach == 0 || g.Distance(r.leg[0], c)+g.Distance(c, r.leg[1]) <= r.reach
}

func (r *roads) nearest(at [2]float64) (cell.ID, bool) {
	a := r.w.r.Area
	x, y := metresOf(a.Centre, at[0], at[1])
	fx := x/a.Cell + float64(a.Cells[0])/2 + float64(r.w.origin[0])
	fy := y/a.Cell + float64(a.Cells[1])/2 + float64(r.w.origin[1])
	best, found, bestD := cell.ID(0), false, math.Inf(1)
	r.w.grid.EachCell(func(c cell.ID) {
		if !r.roadable(c) || r.crossing(c) {
			return
		}
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, k := range r.w.cornersOf(c) {
			lo, hi = min(lo, r.w.corners[k]), max(hi, r.w.corners[k])
		}
		if !r.w.flat && (hi-lo)/2 > min(r.cfg.Cut, r.cfg.Fill) {
			return
		}
		cx, cy := r.w.xy(c)
		d := math.Hypot(float64(cx)+0.5-fx, float64(cy)+0.5-fy)
		if r.w.soils[cy*r.w.cols+cx] == r.w.r.Kinds.Rock {
			d += 3
		}
		if !r.w.flat {
			d += 4 * (hi - lo) / r.w.r.Area.Cell
		}
		if d < bestD {
			best, found, bestD = c, true, d
		}
	})
	return best, found
}

// once lays the leg from one place to the next, planned under each route allowance in turn
func (r *roads) once(from, to cell.ID) (road, error) {
	r.leg, r.reach = [2]cell.ID{from, to}, 1.5*r.w.grid.Distance(from, to)+16
	defer func() { r.reach = 0 }()
	var why error
	for _, p := range r.planners {
		rd, err := r.attempt(p, from, to)
		if err == nil {
			return rd, nil
		}
		why = err
	}
	return road{}, why
}

// attempt lays the leg as p finds it, planned anew with a cell made dearer, barred or a bridge
// banned while it crosses itself or a bridge, or while the roads laid cannot all be levelled with
// it
func (r *roads) attempt(p *roadsite.Planner, from, to cell.ID) (road, error) {
	clear(r.barred)
	clear(r.banned)
	clear(r.dear)
	clear(r.offered)
	why := errors.New("no way but across itself or a bridge")
	for range tries {
		cells, err := p.Route(from, to, r.route[:0])
		if err != nil {
			return road{}, err
		}
		r.route = cells
		rd := r.unfold(cells)
		if r.clash(rd, from, to) {
			clear(r.offered)
			continue
		}
		if why = r.trial(rd); why == nil {
			return rd, nil
		}
		if !r.away(rd, why, from, to) {
			break
		}
	}
	return road{}, why
}

// unfold is the road the planner's cells lay: a bridge's spans put back between its heads, a ford
// led onto
func (r *roads) unfold(cells []cell.ID) road {
	var rd road
	for i, c := range cells {
		if i > 0 {
			if _, near := grid.Way(r.w.grid, cells[i-1], c); !near {
				var b bridging
				way, n, ok := r.spanning(cells[i-1], c)
				if ok {
					b = r.bridge(cells[i-1], c, way, n)
				}
				if !b.ok {
					x, y := r.w.xy(cells[i-1])
					panic(fmt.Sprintf("gramap: the road leaps from [%d, %d] by no bridge", x, y))
				}
				s := cells[i-1]
				for range n {
					s = r.must(r.leap(s, way, 1))
					rd.cells = append(rd.cells, s)
					rd.over = append(rd.over, true)
					rd.at = append(rd.at, b.deck)
				}
			}
		}
		rd.cells = append(rd.cells, c)
		rd.over = append(rd.over, r.ford(c))
		rd.at = append(rd.at, r.w.level(c))
	}
	return rd
}

// spanning is the way a bridge runs from the head a to the head b and the cells it spans; false
// where b lies on no line from a a bridge may span
func (r *roads) spanning(a, b cell.ID) (way, n int, ok bool) {
	for way := range r.w.grid.Directions() {
		for n := 1; n <= r.longest(); n++ {
			if head, on := r.leap(a, way, n+1); on && head == b {
				return way, n, true
			}
		}
	}
	return 0, 0, false
}

// crossed are the stretches rd crosses over, a bridge's spans or a ford, by the heads either side
func crossed(rd road) [][2]int {
	var out [][2]int
	for i := 1; i < len(rd.cells); i++ {
		if rd.over[i] && !rd.over[i-1] {
			j := i
			for j < len(rd.cells) && rd.over[j] {
				j++
			}
			out = append(out, [2]int{i - 1, j})
		}
	}
	return out
}

// clash bars a cell or bans a bridge where rd crosses a bridge or itself as no road may: under a
// bridge only straight through, square across it and low enough, never on its heads; through a
// ford only straight; no bridge beginning where one ends. It reports whether it did.
func (r *roads) clash(rd road, from, to cell.ID) bool {
	again := false
	bar := func(c cell.ID) {
		if c != from && c != to && !r.barred[c] {
			r.barred[c], again = true, true
		}
	}
	ban := func(b [2]int) {
		key := [2]cell.ID{rd.cells[b[0]], rd.cells[b[1]]}
		if !r.banned[key] {
			r.banned[key], again = true, true
		}
	}
	runs := crossed(rd)
	spanned := map[cell.ID][2]int{}
	for i, b := range runs {
		if i > 0 && runs[i-1][1] == b[0] {
			ban(b)
		}
		in := r.wayOf(rd.cells[b[0]], rd.cells[b[0]+1])
		out := r.wayOf(rd.cells[b[1]-1], rd.cells[b[1]])
		for _, s := range rd.cells[b[0]+1 : b[1]] {
			if in != out {
				bar(s)
			}
			if _, twice := spanned[s]; twice {
				ban(b)
			}
			spanned[s] = b
		}
	}
	for _, b := range runs {
		if r.forded(rd.cells[b[0]+1 : b[1]]) {
			continue
		}
		for _, h := range [2][2]int{{b[0], b[0] + 1}, {b[1], b[1] - 1}} {
			for _, f := range r.flanks(rd.cells[h[0]], rd.cells[h[1]]) {
				if i := slices.Index(rd.cells, f); i >= 0 && !rd.over[i] {
					bar(f)
					if f == from || f == to {
						ban(b)
					}
				}
			}
		}
	}
	for i, c := range rd.cells {
		if rd.over[i] {
			continue
		}
		if b, ok := spanned[c]; ok && !r.under(rd, i, r.wayOf(rd.cells[b[0]], rd.cells[b[0]+1]),
			rd.at[b[0]+1]) {
			bar(c)
			if c == from || c == to {
				ban(b)
			}
		}
		if m := r.marks[c]; m.bridged && !m.ford && !r.under(rd, i, m.axis, m.deck) {
			bar(c)
		}
		if r.flanksLaid(c) {
			bar(c)
		}
	}
	return again
}

// flanks are the cells beside a bridge's head h that its span s, laid across a corner, touches
// too: a road on one lies on the head's corners at the deck and the span's on the ground
func (r *roads) flanks(h, s cell.ID) []cell.ID {
	g := r.w.grid
	fa, fb, ok := g.Flanks(r.wayOf(h, s))
	if !ok {
		return nil
	}
	var out []cell.ID
	for _, f := range [2]int{fa, fb} {
		if c, ok := g.Toward(h, f); ok {
			out = append(out, c)
		}
	}
	return out
}

// flanksLaid reports whether c flanks the head of a bridge laid across a corner
func (r *roads) flanksLaid(c cell.ID) bool {
	g := r.w.grid
	for i := range g.Directions() {
		h, ok := g.Toward(c, i)
		m := r.marks[h]
		if !ok || !m.head || !g.Corner(m.headWay) {
			continue
		}
		if s, ok := g.Toward(h, m.headWay); ok && r.marks[s].bridged && !r.marks[s].ford &&
			slices.Contains(r.flanks(h, s), c) {
			return true
		}
	}
	return false
}

// under reports whether the road may pass at rd.cells[i] under a bridge running way, its deck at
// deck: straight through, square across the bridge and low enough, no bridge's head itself
func (r *roads) under(rd road, i, way int, deck float64) bool {
	if r.w.flat || i == 0 || i == len(rd.cells)-1 || rd.over[i-1] || rd.over[i+1] {
		return false
	}
	in := r.wayOf(rd.cells[i-1], rd.cells[i])
	if in != r.wayOf(rd.cells[i], rd.cells[i+1]) || !r.across(in, way) {
		return false
	}
	for _, k := range r.w.cornersOf(rd.cells[i]) {
		if r.w.corners[k]+r.underpass() > deck {
			return false
		}
	}
	return true
}

// away bans rd's bridge err refuses, else makes the cell it refuses and those round it dearer to
// the planner, so the next way keeps off them where another runs and through them where none
// does; false where err names no cell
func (r *roads) away(rd road, err error, from, to cell.ID) bool {
	at, ok := refusedAt(err)
	if !ok {
		return false
	}
	for _, b := range crossed(rd) {
		key := [2]cell.ID{rd.cells[b[0]], rd.cells[b[1]]}
		if !r.forded(rd.cells[b[0]+1:b[1]]) && !r.banned[key] &&
			slices.Contains(rd.cells[b[0]:b[1]+1], at) {
			r.banned[key] = true
			return true
		}
	}
	// a road standing is no dearer to the planner: the new road's cells round it are
	reach := 1
	if r.marks[at].paved {
		reach = 2
	}
	ax, ay := r.w.xy(at)
	for y := max(ay-reach, 0); y <= min(ay+reach, r.w.rows-1); y++ {
		for x := max(ax-reach, 0); x <= min(ax+reach, r.w.cols-1); x++ {
			if c := r.w.cell(x, y); c != from && c != to {
				r.dear[c] = max(r.dear[c], 1) * dearer
			}
		}
	}
	return true
}

func refusedAt(err error) (cell.ID, bool) {
	if ref, ok := errors.AsType[*roadsite.Refusal](err); ok {
		return ref.At, true
	}
	if ref, ok := errors.AsType[*bridgesite.Refusal](err); ok {
		return ref.At, true
	}
	if ref, ok := errors.AsType[*usite.Refusal](err); ok {
		return ref.At, true
	}
	return 0, false
}

// trial lays rd beside the roads laid if they can all be levelled with it and every bridge still
// stands: rd's bridges' heads flat at their decks, the levelling kept for the end
func (r *roads) trial(rd road) error {
	l := r.layout(append(r.laid[:len(r.laid):len(r.laid)], rd))
	flat := r.flatten(rd)
	est, err := r.levels(&l)
	if err != nil {
		r.undo(flat)
		return err
	}
	r.graded = est
	r.mark(rd)
	return nil
}

// levels is how the roads of l are levelled, or why they cannot be, or why a bridge would not
// stand on the ground they leave; the ground stays as it was
func (r *roads) levels(l *layout) (roadsite.Estimate, error) {
	var est roadsite.Estimate
	if !r.w.flat {
		var err error
		est, err = roadsite.Plan(chartGround{r: r, l: l}, r.rules, l.cells, r.w.r.Kinds.Road,
			float32(r.cfg.Width), true, true, float32(r.cfg.Step/r.w.perUnit), nil)
		if err != nil {
			return est, err
		}
	}
	moved := r.grade(est.Corners)
	err := r.hold(l)
	if err == nil {
		err = r.held(l)
	}
	r.undo(moved)
	return est, err
}

// hold is why a board would not hold the roads and bridges of l on the ground as it lies, as it
// holds a map's: a bridge's heads off its deck, a road's cell as Transport Tycoon would not lay it
func (r *roads) hold(l *layout) error {
	rules := r.bridgeRules()
	rules.Reshape = 1e-3 * r.w.grid.CellSpan()
	for i, b := range l.bridges {
		g := bridgeGround{chartGround{r: r, l: l}, l.bridges[:i]}
		if _, err := bridgesite.Plan(g, rules, b.From, b.To, g.level(b.From)); err != nil {
			return err
		}
	}
	if r.w.flat {
		return nil
	}
	for _, c := range l.cells {
		if !ground.RoadFits(r.w.grid, c, r.w.outline(c), l.net.Links(c)) {
			return roadsite.Refuse(r.w.grid, c, "lies on ground no road may")
		}
	}
	return nil
}

// layout is roads as their levelling sees them: their network, their cells each once in the order
// laid, the bridges they cross, and the cells whose corners stay: the bridges' heads, flat at
// their decks, and the ground no road runs over under their spans
type layout struct {
	net     *network.Network
	cells   []cell.ID
	bridges []bridgesite.Bridge
	fixed   map[cell.ID]bool
}

func (r *roads) layout(rds []road) layout {
	w := r.w
	l := layout{net: network.New(w.grid), fixed: map[cell.ID]bool{}}
	node := network.Node{Kind: w.r.Kinds.Road, Width: float32(r.cfg.Width)}
	listed := map[cell.ID]bool{}
	stood := map[[2]cell.ID]bool{}
	for _, rd := range rds {
		for i, c := range rd.cells {
			if !rd.over[i] {
				l.net.Set(c, node)
				if !listed[c] {
					listed[c] = true
					l.cells = append(l.cells, c)
				}
			}
			if i > 0 && !(rd.over[i-1] && rd.over[i]) {
				l.net.Link(rd.cells[i-1], c)
			}
		}
		for _, b := range crossed(rd) {
			from, to, spans := rd.cells[b[0]], rd.cells[b[1]], rd.cells[b[0]+1:b[1]]
			if r.forded(spans) || stood[[2]cell.ID{from, to}] || stood[[2]cell.ID{to, from}] {
				continue
			}
			stood[[2]cell.ID{from, to}] = true
			l.fixed[from], l.fixed[to] = true, true
			l.bridges = append(l.bridges, bridgesite.Bridge{From: from, To: to,
				Way: r.wayOf(from, spans[0]), Spans: spans})
		}
	}
	for _, b := range l.bridges {
		for _, s := range b.Spans {
			if _, paved := l.net.Node(s); !paved {
				l.fixed[s] = true
			}
		}
	}
	return l
}

// flatten lays the heads of rd's bridges flat at their decks, giving back where their corners
// stood
func (r *roads) flatten(rd road) []height {
	if r.w.flat {
		return nil
	}
	var out []height
	for _, b := range crossed(rd) {
		if r.forded(rd.cells[b[0]+1 : b[1]]) {
			continue
		}
		for _, c := range [2]cell.ID{rd.cells[b[0]], rd.cells[b[1]]} {
			for _, k := range r.w.cornersOf(c) {
				out = append(out, height{k, r.w.corners[k]})
				r.w.corners[k] = rd.at[b[0]+1]
			}
		}
	}
	return out
}

// grade moves the corners as the levelling moves them, giving back where they stood
func (r *roads) grade(moves []roadsite.CornerMove) []height {
	out := make([]height, 0, len(moves))
	for _, m := range moves {
		if m.To == m.From {
			continue
		}
		k := r.w.corner(m.At)
		out = append(out, height{k, r.w.corners[k]})
		r.w.corners[k] = float64(m.To) * r.w.perUnit
	}
	return out
}

func (r *roads) undo(hs []height) {
	for _, h := range slices.Backward(hs) {
		r.w.corners[h.k] = h.h
	}
}

// settle lays the levelling of every road on the ground, to the centimetre as the map has it
func (r *roads) settle() error {
	w := r.w
	r.grade(r.graded.Corners)
	for k, h := range w.corners {
		w.corners[k] = math.Round(h*100) / 100
	}
	for _, s := range r.live() {
		if !s.ford {
			s.deck = w.corners[w.cornersOf(s.heads[0])[0]]
		}
	}
	var bad []string
	for _, c := range r.net.Cells() {
		if !ground.RoadFits(w.grid, c, w.outline(c), r.net.Links(c)) {
			x, y := w.xy(c)
			bad = append(bad, fmt.Sprintf("[%d, %d]", x, y))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("the ground under the road at %s cannot be graded",
			strings.Join(bad, ", "))
	}
	return nil
}

// mark notes rd laid: its cells paved with their ways, its bridges' and fords' heads and spans
func (r *roads) mark(rd road) {
	g := r.w.grid
	for i, c := range rd.cells {
		if rd.over[i] {
			continue
		}
		in, out := r.ways(rd, i)
		m := r.marks[c]
		m.paved = true
		if in != none {
			m.ways |= 1 << g.Opposite(in)
		}
		if out != none {
			m.ways |= 1 << out
		}
		switch {
		case out != none && rd.over[i+1]:
			m.head, m.headWay = true, out
		case in != none && rd.over[i-1]:
			m.head, m.headWay = true, g.Opposite(in)
		}
		r.marks[c] = m
	}
	for _, b := range crossed(rd) {
		spans := rd.cells[b[0]+1 : b[1]]
		ford, way := r.forded(spans), r.wayOf(rd.cells[b[0]], spans[0])
		for k, s := range spans {
			if m := r.marks[s]; !m.bridged {
				m.bridged, m.ford, m.axis, m.deck = true, ford, way, rd.at[b[0]+1+k]
				r.marks[s] = m
			}
		}
	}
}

// forded reports whether a road crosses spans through a ford, no bridge: a ford and its banks
func (r *roads) forded(spans []cell.ID) bool {
	fords := 0
	for _, c := range spans {
		switch {
		case r.ford(c):
			fords++
		case r.w.wet(c) || !r.crossing(c):
			return false
		}
	}
	return fords > 0
}

// leaps are the bridges a road may take from c along way, a map maker's: over water, or over a
// valley deep enough to pass under
func (r *roads) leaps(c cell.ID, way int, dst []roadsite.Leap) []roadsite.Leap {
	if r.w.flat && r.w.grid.Corner(way) {
		return dst // a flat map's bridge runs across a side
	}
	if r.spannable != nil {
		// a guide's roads bridge where its own do, and water
		if s, ok := r.leap(c, way, 1); !ok || !r.spannable[s] && !r.crossing(s) {
			return dst
		}
	}
	for n := 1; n <= r.longest(); n++ {
		head, ok := r.leap(c, way, n+1)
		if !ok || r.barred[head] {
			break
		}
		if r.banned[[2]cell.ID{c, head}] || !r.within(head) {
			continue
		}
		if b := r.bridge(c, head, way, n); b.ok {
			dst = append(dst, roadsite.Leap{To: head, Spans: n, Cost: b.cost})
		}
	}
	return dst
}

func (r *roads) bridge(a, b cell.ID, way, n int) bridging {
	key := [2]cell.ID{a, b}
	if t, ok := r.offered[key]; ok {
		return t
	}
	t := r.weigh(a, b, way, n)
	r.offered[key] = t
	return t
}

// weigh is the bridge from a to b, n cells on along way: one laid already at no price; else its
// deck as deckFor sets it, at the price of its length Bridge times a metre's and of the earth its
// heads move
func (r *roads) weigh(a, b cell.ID, way, n int) bridging {
	t := bridging{way: way, n: n}
	if ma, mb := r.marks[a], r.marks[b]; ma.head && mb.head && ma.headWay == way &&
		mb.headWay == r.w.grid.Opposite(way) {
		t.deck, t.ok = r.marks[r.must(r.leap(a, way, 1))].deck, true
		return t
	}
	deck, ok := r.deckFor(a, b, way, n)
	if !ok || !r.crosses(a, way, n, deck) {
		return t
	}
	g := bridgeGround{chartGround: chartGround{r: r}}
	if _, err := bridgesite.Plan(g, r.bridgeRules(), a, b, float32(deck/r.w.perUnit)); err != nil {
		return t
	}
	step := r.w.grid.Step(way)
	along := math.Hypot(float64(step.X), float64(step.Y)) * r.w.perUnit
	earth := 0.0
	for _, h := range [2]cell.ID{a, b} {
		for _, k := range r.w.cornersOf(h) {
			if !r.w.flat {
				earth += math.Abs(r.w.corners[k] - deck)
			}
		}
	}
	side := r.w.r.Area.Cell
	earth *= side * side / 4 // a corner's share of its cell
	t.deck, t.ok = deck, true
	t.cost = float32(float64(n+1)*along*r.cfg.Metre*r.cfg.Bridge + earth*r.cfg.Cubic)
	return t
}

// deckFor is the deck of a bridge from a to b, n cells on along way, metres: as near the middle of
// its heads' corners as cutting and filling them lets it, the ground it spans the clearance under
// it, and the middle of a span's top too, which a square folds along a diagonal from a head's
// corner; false where no height does
func (r *roads) deckFor(a, b cell.ID, way, n int) (float64, bool) {
	w := r.w
	if w.flat {
		return 0, true
	}
	ka, kb := w.cornersOf(a), w.cornersOf(b)
	ends := append(ka[:], kb[:]...)
	heads := make([]float64, len(ends))
	for i, k := range ends {
		heads[i] = w.corners[k]
	}
	slices.Sort(heads)
	lo, hi := heads[len(heads)-1]-r.cfg.Cut, heads[0]+r.cfg.Fill
	bottom := math.Inf(1)
	c := a
	for range n {
		c = r.must(r.leap(c, way, 1))
		ks := w.cornersOf(c)
		for i, k := range ks {
			end, across := slices.Contains(ends, k), slices.Contains(ends, ks[3-i])
			switch {
			case end && across:
				return 0, false
			case end:
				continue
			case across:
				lo = max(lo, w.corners[k]+2*r.clearance()+margin)
			default:
				lo = max(lo, w.corners[k]+r.clearance()+margin)
			}
			bottom = min(bottom, w.corners[k])
		}
	}
	if r.cfg.High > 0 {
		hi = min(hi, bottom+r.cfg.High-margin)
	}
	if lo > hi {
		return 0, false
	}
	mid := (heads[len(heads)/2-1] + heads[len(heads)/2]) / 2
	return min(max(mid, lo), hi), true
}

// crosses asks cheaply what bridgesite.Plan asks first of all, so most leaps are refused before it
func (r *roads) crosses(a cell.ID, way, n int, deck float64) bool {
	ends := r.w.cornersOf(a)
	far := r.w.cornersOf(r.must(r.leap(a, way, n+1)))
	c := a
	for range n {
		c = r.must(r.leap(c, way, 1))
		if r.w.wet(c) || r.w.flat && (!r.roadable(c) || r.crossing(c)) {
			return true
		}
		if r.w.flat {
			continue
		}
		for _, k := range r.w.cornersOf(c) {
			if !slices.Contains(ends[:], k) && !slices.Contains(far[:], k) &&
				r.w.corners[k] <= deck-r.underpass() {
				return true
			}
		}
	}
	return false
}

func (r *roads) leap(c cell.ID, way, steps int) (cell.ID, bool) {
	for range steps {
		n, ok := r.w.grid.Toward(c, way)
		if !ok {
			return 0, false
		}
		c = n
	}
	return c, true
}

func (r *roads) longest() int {
	if r.cfg.Longest > 0 {
		return r.cfg.Longest
	}
	return 3
}

func (r *roads) must(c cell.ID, ok bool) cell.ID {
	if !ok {
		panic("gramap: a leap off the board")
	}
	return c
}

// world units; a head's ground cut and filled as a road's corners are
func (r *roads) bridgeRules() bridgesite.Rules {
	per := r.w.perUnit
	return bridgesite.Rules{
		Longest:   r.longest(),
		Under:     float32(r.underpass() / per),
		Clearance: float32(r.clearance() / per),
		Reshape:   float32(max(r.cfg.Cut, r.cfg.Fill) / per),
		High:      float32(r.cfg.High / per),
		Heights:   !r.w.flat,
	}
}

// summary is what the roads came to: their cells, bridges and fords, the earth levelled
func (r *roads) summary() string {
	bridges, fords := 0, 0
	for _, s := range r.live() {
		if s.ford {
			fords++
		} else {
			bridges++
		}
	}
	cubed := r.w.perUnit * r.w.perUnit * r.w.perUnit
	cut, fill := float64(r.graded.Cut)*cubed, float64(r.graded.Fill)*cubed
	line := fmt.Sprintf("roads: %d cells, %d bridges, %d fords; earth: %.0f m³ cut, %.0f filled",
		len(r.net.Cells()), bridges, fords, cut, fill)
	if r.guide == nil {
		return line
	}
	left := 0
	for _, l := range r.lost {
		left += len(l.trace)
	}
	return line + fmt.Sprintf("\nOpenStreetMap: %d legs of road, %d stretches left out "+
		"(%d cells); %d tunnels dug, %d left as roads, %d no road reaches left out", r.legs,
		len(r.lost), left, len(r.tunnels), r.undug, r.unreached)
}

// the ground in world units, as the roads' site asks: with l, the roads a levelling plans; else
// the roads laid, a cell barred taking no road nor showing one, a ford or a deck
type chartGround struct {
	r *roads
	l *layout
}

// the ground as the planner of a route sees it: a cell the levelling may lay flat lies flat at the
// middle of its corners, so a road bends where the levelling may flatten it and climbs as the
// route's own cut and fill let it between cells' middles; a cell the levelling failed at dearer;
// none off the corridor round the leg under search taking a road
type plannerGround struct{ chartGround }

func (g plannerGround) Takes(c cell.ID) bool {
	return g.chartGround.Takes(c) && g.r.within(c) && !g.r.mouths[c]
}

func (g plannerGround) Heights(c cell.ID, dst []float32) []float32 {
	n := len(dst)
	dst = g.chartGround.Heights(c, dst)
	if len(dst) == n {
		return dst
	}
	lo, hi, sum := dst[n], dst[n], float32(0)
	for _, h := range dst[n:] {
		lo, hi, sum = min(lo, h), max(hi, h), sum+h
	}
	if hi-lo > g.r.rules.Cut+g.r.rules.Fill {
		return dst
	}
	mid := sum / float32(len(dst)-n)
	for i := n; i < len(dst); i++ {
		dst[i] = mid
	}
	return dst
}

func (g plannerGround) Price(c cell.ID) float32 {
	price := g.chartGround.Price(c)
	if g.r.traced != nil && !g.r.traced[c] && !g.r.marks[c].paved {
		price *= astray
	}
	if d, ok := g.r.dear[c]; ok {
		return price * d
	}
	return price
}

// the ground as the bridges' site asks; stood, with l, are the bridges standing
type bridgeGround struct {
	chartGround
	stood []bridgesite.Bridge
}

var (
	_ roadsite.Ground   = chartGround{}
	_ roadsite.Ground   = plannerGround{}
	_ bridgesite.Ground = bridgeGround{}
)

func (g chartGround) Grid() grid.Grid { return g.r.w.grid }

func (g chartGround) Heights(c cell.ID, dst []float32) []float32 {
	w := g.r.w
	g.r.outline = w.grid.CellOutline(c, g.r.outline[:0])
	for _, p := range g.r.outline {
		h := float32(0)
		if !w.flat {
			h = float32(w.corners[w.corner(p)] / w.perUnit)
		}
		dst = append(dst, h)
	}
	return dst
}

func (g chartGround) level(c cell.ID) float32 {
	var buf [4]float32
	hs := g.Heights(c, buf[:0])
	sum := float32(0)
	for _, h := range hs {
		sum += h
	}
	return sum / float32(len(hs))
}

func (g chartGround) barred(c cell.ID) bool { return g.l == nil && g.r.barred[c] }

func (g chartGround) Takes(c cell.ID) bool {
	return !g.barred(c) && g.r.roadable(c) && !g.r.crossing(c)
}

func (g chartGround) Wet(c cell.ID) bool { return g.r.w.wet(c) }

func (g chartGround) Road(c cell.ID) cell.Links {
	if g.l != nil {
		return g.l.net.Links(c)
	}
	if m := g.r.marks[c]; m.paved && !g.barred(c) {
		return m.ways
	}
	return 0
}

// Decked is a bridge laid over c, no road under it: a map maker's road takes one from its head,
// as a leap; a levelling's roads run on none
func (g chartGround) Decked(c cell.ID) (cell.Links, bool) {
	if m := g.r.marks[c]; g.l == nil && m.bridged && !m.ford && !m.paved {
		return 1<<m.axis | 1<<g.r.w.grid.Opposite(m.axis), true
	}
	return 0, false
}

func (g chartGround) Ford(c cell.ID) bool {
	return !g.barred(c) && g.r.w.rivers.Courses[c] == water.Ford
}

func (g chartGround) Fixed(c cell.ID) bool {
	if g.r.w.wet(c) {
		return true
	}
	if g.r.heads[c] || g.r.mouths[c] {
		return true
	}
	if g.l != nil {
		return g.l.fixed[c]
	}
	m := g.r.marks[c]
	return m.head || m.bridged && !m.paved
}

// Held is never: a map has no units yet
func (g chartGround) Held(cell.ID) bool { return false }

func (g chartGround) Price(c cell.ID) float32 {
	w := g.r.w
	x, y := w.xy(c)
	if w.soils[y*w.cols+x] == w.r.Kinds.Rock {
		return float32(max(1, g.r.cfg.Rock))
	}
	return 1
}

func (g bridgeGround) Bridged(c cell.ID) (bridgesite.Part, bool) {
	if g.l != nil {
		for _, b := range g.stood {
			switch {
			case c == b.From:
				return bridgesite.Part{Way: b.Way, Head: true}, true
			case c == b.To:
				return bridgesite.Part{Way: g.r.w.grid.Opposite(b.Way), Head: true}, true
			case slices.Contains(b.Spans, c):
				return bridgesite.Part{Way: b.Way}, true
			}
		}
		return bridgesite.Part{}, false
	}
	m := g.r.marks[c]
	switch {
	case m.bridged:
		return bridgesite.Part{Way: m.axis}, true
	case m.head:
		return bridgesite.Part{Way: m.headWay, Head: true}, true
	}
	return bridgesite.Part{}, false
}

func (r *roads) across(a, b int) bool { return 4*r.turns(a, b) == r.w.grid.Directions() }

func (r *roads) turns(a, b int) int {
	g := r.w.grid
	sa, sb := g.Step(a), g.Step(b)
	turn := math.Atan2(float64(sb.Y), float64(sb.X)) - math.Atan2(float64(sa.Y), float64(sa.X))
	angle := math.Abs(math.Remainder(turn, 2*math.Pi))
	return int(math.Round(angle / (2 * math.Pi / float64(g.Directions()))))
}

func (r *roads) ways(rd road, i int) (in, out int) {
	in, out = none, none
	if i > 0 {
		in = r.wayOf(rd.cells[i-1], rd.cells[i])
	}
	if i+1 < len(rd.cells) {
		out = r.wayOf(rd.cells[i], rd.cells[i+1])
	}
	return in, out
}

func (r *roads) wayOf(a, b cell.ID) int {
	way, _ := grid.Way(r.w.grid, a, b)
	return way
}

func (r *roads) add(s *span) {
	for _, c := range s.cells {
		if o, ok := r.spanOf[c]; ok && o != s && !o.merged {
			for _, oc := range o.cells {
				if !slices.Contains(s.cells, oc) {
					s.cells = append(s.cells, oc)
				}
			}
			for _, h := range o.heads {
				if !slices.Contains(s.heads, h) {
					s.heads = append(s.heads, h)
				}
			}
			o.merged = true
		}
	}
	for _, c := range s.cells {
		r.spanOf[c] = s
	}
	s.ford = r.forded(s.cells)
	r.spans = append(r.spans, s)
}

func (r *roads) live() []*span {
	var out []*span
	for _, s := range r.spans {
		if !s.merged {
			out = append(out, s)
		}
	}
	return out
}
