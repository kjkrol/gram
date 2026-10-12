package main

import (
	"container/heap"
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
	"github.com/kjkrol/gram/plugins/bridges/site"
)

type roads struct {
	w       *chart
	cfg     roadsCfg
	net     *network.Network
	laid    []road
	places  map[string]cell.ID
	spans   []*span
	spanOf  map[cell.ID]*span
	wetK    []bool    // the corners of running water: its beds, never graded
	ground  []float64 // the corners' heights before grading
	planned map[int]float64
	under   map[int]float64 // the most a road rises to under a bridge, its heads' corners aside
	skipped [][2]string
}

type road struct {
	cells []cell.ID
	over  []bool
	at    []float64 // metres
}

type span struct {
	cells  []cell.ID
	way    int
	ford   bool
	edges  [][]int // the corners the road meets a ford at
	heads  []head
	deck   float64 // metres
	merged bool
}

type head struct {
	at  cell.ID
	way int // from it into the bridge
}

// none is the way into the cell a road begins on, and out of the one it ends on
const none = -1

// metres
const planSlack = 0.5

// metres: heights this near are one to the search
const heightBand = 20.0

type mark struct {
	paved   bool
	ways    cell.Links
	at      float64
	bridged bool
	axis    int
	deck    float64
	head    bool
	headWay int
}

// metres: the deck's thickness and a metre to spare
func (r *roads) clearance() float64 { return r.cfg.Thickness + 1 }

// metres: the figures the map is made for, 2 m where it says none, and the clearance over them
func (r *roads) underpass() float64 { return max(r.cfg.Figure, 2) + r.clearance() }

func newRoads(w *chart) *roads {
	r := &roads{w: w, cfg: w.r.Roads, net: network.New(w.grid), places: map[string]cell.ID{},
		spanOf: map[cell.ID]*span{}, wetK: make([]bool, len(w.corners)),
		planned: map[int]float64{}, under: map[int]float64{}}
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
		c, ok := r.nearest(r.cfg.Places[name])
		if !ok {
			return fmt.Errorf("no ground a road may take near %s", name)
		}
		r.places[name] = c
	}
	laid := map[cell.ID]mark{}
	keep := func(rd road) {
		if len(rd.cells) > 0 {
			r.laid = append(r.laid, rd)
		}
	}
	for _, route := range r.cfg.Routes {
		var rd road
		for i := 1; i < len(route); i++ {
			from := r.places[route[i-1]]
			h := r.w.level(from)
			if _, ok := laid[from]; ok && !r.w.flat {
				h = r.height(r.w.cornersOf(from)[0])
			}
			leg, ok := r.once(from, r.places[route[i]], h, laid)
			if !ok {
				r.skipped = append(r.skipped, [2]string{route[i-1], route[i]})
				keep(rd)
				rd = road{}
				continue
			}
			r.plan(leg, laid)
			if len(rd.cells) > 0 {
				leg.cells, leg.over, leg.at = leg.cells[1:], leg.over[1:], leg.at[1:]
			}
			rd.cells, rd.over = append(rd.cells, leg.cells...), append(rd.over, leg.over...)
			rd.at = append(rd.at, leg.at...)
		}
		keep(rd)
	}
	for _, rd := range r.laid {
		r.crossings(rd)
	}
	if r.w.flat {
		return nil
	}
	return r.grade()
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
		if !r.w.flat && (hi-lo)/2 > r.cfg.Deep {
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

func (r *roads) find(from, to cell.ID, h0 float64, laid map[cell.ID]mark,
	barred map[cell.ID]bool, banned map[[2]cell.ID]bool) (road, bool) {
	type state struct {
		c    cell.ID
		way  int8
		band int16 // the height it comes at, in heightBands
		head bool  // landed on from a bridge
	}
	band := func(h float64) int16 { return int16(math.Round(h / heightBand)) }
	start := state{from, none, band(h0), false}
	dist := map[state]float64{start: 0}
	height := map[state]float64{start: h0}
	prev := map[state]state{}
	spans := map[state]int{}
	type leapKey struct {
		c      cell.ID
		way, n int
		centi  int64
	}
	type leapCost struct {
		c  float64
		ok bool
	}
	leaps := map[leapKey]leapCost{}
	var q queue
	at := map[int]state{}
	pushed := 0
	push := func(s state, d float64) {
		at[pushed] = s
		heap.Push(&q, entry{id: pushed, at: d})
		pushed++
	}
	push(start, 0)
	for q.Len() > 0 {
		e := heap.Pop(&q).(entry)
		s := at[e.id]
		delete(at, e.id)
		if e.at > dist[s] {
			continue
		}
		if s.c == to {
			var rd road
			for p := s; ; p = prev[p] {
				rd.cells, rd.over = append(rd.cells, p.c), append(rd.over, false)
				rd.at = append(rd.at, height[p])
				if p.way == none {
					break
				}
				back := r.w.grid.Opposite(int(p.way))
				c := p.c
				for range spans[p] {
					c, _ = r.w.grid.Toward(c, back)
					rd.cells, rd.over = append(rd.cells, c), append(rd.over, true)
					rd.at = append(rd.at, height[p])
				}
			}
			slices.Reverse(rd.cells)
			slices.Reverse(rd.over)
			slices.Reverse(rd.at)
			return rd, true
		}
		h := height[s]
		under := s.way != none && laid[s.c].bridged
		try := func(n state, c, at, leave float64, over int) {
			if math.IsInf(c, 1) || math.IsInf(leave, 1) {
				return
			}
			c += e.at + leave
			n.band = band(at)
			if d, seen := dist[n]; !seen || c < d {
				dist[n], prev[n], height[n], spans[n] = c, s, at, over
				push(n, c)
			}
		}
		for way := range int8(r.w.grid.Directions()) {
			if under && way != s.way {
				continue // a road under a bridge runs straight across it
			}
			b, ok := r.w.grid.Toward(s.c, int(way))
			if !ok || barred[b] {
				continue
			}
			leave, next := 0.0, h
			if !s.head {
				leave, next = r.earthwork(s.c, int(s.way), int(way), h)
			}
			if r.paveable(s.c, b, int(way), next, laid) {
				c := r.travel(s.c, b, int(s.way), int(way), h, next, laid)
				if b == to {
					end, _ := r.earthwork(b, int(way), none, next)
					c += end
				}
				try(state{b, way, 0, false}, c, next, leave, 0)
			}
			if s.head || r.w.flat && r.w.grid.Corner(int(way)) {
				continue // no bridge begins where one ends; a flat map's run along a side
			}
			for n := 1; n <= r.longest(); n++ {
				head, ok := r.leap(s.c, int(way), n+1)
				if !ok || barred[head] {
					break
				}
				if banned[[2]cell.ID{s.c, head}] {
					continue
				}
				key := leapKey{s.c, int(way), n, int64(math.Round(h * 100))}
				lc, seen := leaps[key]
				if !seen {
					lc.c, lc.ok = r.bridge(s.c, head, int(way), n, h, laid)
					leaps[key] = lc
				}
				if !lc.ok {
					continue
				}
				c := lc.c
				c += r.flatten(head, none, h)
				try(state{head, way, 0, true}, c, h, r.flatten(s.c, int(s.way), h), n)
			}
		}
	}
	return road{}, false
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

// world units
func (r *roads) rules() site.Rules {
	per := r.w.perUnit
	return site.Rules{
		Longest:   r.longest(),
		Under:     r.underpass() / per,
		Clearance: r.clearance() / per,
		Reshape:   r.cfg.Deep / per,
		High:      r.cfg.High / per,
		Heights:   !r.w.flat,
	}
}

func (r *roads) once(from, to cell.ID, h0 float64, laid map[cell.ID]mark) (road, bool) {
	barred, banned := map[cell.ID]bool{}, map[[2]cell.ID]bool{}
	for range 32 {
		rd, ok := r.find(from, to, h0, laid, barred, banned)
		if !ok {
			return rd, false
		}
		visits, again := map[cell.ID][]int{}, false
		for i, c := range rd.cells {
			visits[c] = append(visits[c], i)
		}
		for c, at := range visits {
			if len(at) > 1 && !r.overpass(rd, at) {
				barred[c], again = true, true
			}
		}
		aimed := map[int]float64{}
		r.aims(rd, func(_ int, c cell.ID, target map[int]float64) {
			for k, t := range target {
				if v, ok := aimed[k]; ok && math.Abs(v-t) > planSlack && c != from && c != to {
					barred[c], again = true, true
				}
				aimed[k] = t
			}
		})
		for _, b := range leaps(rd) {
			if r.underRoad(rd, b, aimed) {
				banned[[2]cell.ID{rd.cells[b[0]], rd.cells[b[1]]}], again = true, true
			}
		}
		if !again {
			return rd, true
		}
	}
	return road{}, false
}

func leaps(rd road) [][2]int {
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

func (r *roads) underRoad(rd road, b [2]int, aimed map[int]float64) bool {
	ends := r.w.cornersOf(rd.cells[b[0]])
	far := r.w.cornersOf(rd.cells[b[1]])
	deck := rd.at[b[0]]
	for _, c := range rd.cells[b[0]+1 : b[1]] {
		for _, k := range r.w.cornersOf(c) {
			if slices.Contains(ends[:], k) || slices.Contains(far[:], k) {
				continue
			}
			if t, ok := aimed[k]; ok && t > deck-r.clearance()+planSlack {
				return true
			}
		}
	}
	return false
}

func (r *roads) overpass(rd road, at []int) bool {
	if len(at) != 2 || rd.over[at[0]] == rd.over[at[1]] {
		return false
	}
	over, under := at[0], at[1]
	if rd.over[under] {
		over, under = under, over
	}
	if under == 0 || under == len(rd.cells)-1 || rd.over[under-1] || rd.over[under+1] {
		return false // an end, or a bridge's head
	}
	in := r.wayOf(rd.cells[under-1], rd.cells[under])
	out := r.wayOf(rd.cells[under], rd.cells[under+1])
	axis := r.wayOf(rd.cells[over-1], rd.cells[over])
	return in == out && r.across(in, axis) &&
		rd.at[over] >= max(rd.at[under], rd.at[under+1])+r.underpass()
}

func (r *roads) across(a, b int) bool { return 4*r.turns(a, b) == r.w.grid.Directions() }

func (r *roads) turns(a, b int) int {
	g := r.w.grid
	sa, sb := g.Step(a), g.Step(b)
	angle := math.Abs(math.Remainder(math.Atan2(sb.Y, sb.X)-math.Atan2(sa.Y, sa.X), 2*math.Pi))
	return int(math.Round(angle / (2 * math.Pi / float64(g.Directions()))))
}

func (r *roads) paveable(a, b cell.ID, way int, h float64, laid map[cell.ID]mark) bool {
	if !r.roadable(b) || r.crossing(b) {
		return false
	}
	if m := laid[b]; m.bridged {
		rise := r.w.r.Area.Cell * math.Tan(ground.RoadSlope)
		if !r.across(way, m.axis) || h+rise+r.underpass() > m.deck {
			return false
		}
	}
	if fa, fb, corner := r.w.grid.Flanks(way); corner {
		for _, f := range [2]int{fa, fb} {
			if s, ok := r.w.grid.Toward(a, f); !ok || !r.roadable(s) || r.crossing(s) {
				return false
			}
		}
	}
	return true
}

func (r *roads) travel(a, b cell.ID, in, out int, h, next float64, laid map[cell.ID]mark) float64 {
	w := r.w
	along := w.grid.NeighborCost(a, b)
	cost := along * (1 + r.cfg.Climb*math.Abs(next-h)/(along*w.r.Area.Cell))
	if in != none {
		cost += r.cfg.Turn * float64(r.turns(in, out))
	}
	bx, by := w.xy(b)
	if w.soils[by*w.cols+bx] == w.r.Kinds.Rock {
		cost *= r.cfg.Rock
	}
	if laid[b].paved {
		cost *= r.cfg.Reuse
	}
	return cost
}

func (r *roads) headable(c cell.ID, way int, laid map[cell.ID]mark) bool {
	m := laid[c]
	return !m.bridged && (!m.head || m.headWay == way)
}

func (r *roads) flatten(c cell.ID, in int, h float64) float64 {
	if r.w.flat {
		return 0
	}
	target := map[int]float64{}
	for _, k := range r.w.cornersOf(c) {
		target[k] = h
	}
	return r.price(c, in, target)
}

func (r *roads) bridge(a, b cell.ID, way, n int, h float64, laid map[cell.ID]mark) (float64,
	bool) {
	along := r.w.grid.NeighborCost(a, r.must(r.leap(a, way, 1)))
	cost := float64(n+1) * along * (1 + r.cfg.Bridge)
	if ma, mb := laid[a], laid[b]; ma.head && mb.head && ma.headWay == way &&
		mb.headWay == r.w.grid.Opposite(way) {
		return cost * r.cfg.Reuse, true
	}
	if !r.crosses(a, way, n, h) {
		return 0, false
	}
	if _, err := site.Plan(chartGround{r, laid}, r.rules(), a, b, h/r.w.perUnit); err != nil {
		return 0, false
	}
	return cost, true
}

// crosses asks cheaply what site.Plan asks first of all, so most leaps are refused before it.
func (r *roads) crosses(a cell.ID, way, n int, h float64) bool {
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
				r.height(k) <= h-r.underpass() {
				return true
			}
		}
	}
	return false
}

func (r *roads) must(c cell.ID, ok bool) cell.ID {
	if !ok {
		panic("gramap: a leap off the board")
	}
	return c
}

// world units, as site asks
type chartGround struct {
	r    *roads
	laid map[cell.ID]mark
}

func (g chartGround) Grid() grid.Grid { return g.r.w.grid }

func (g chartGround) Heights(c cell.ID, dst []float64) []float64 {
	var buf [4]geom.Vec
	for _, p := range g.r.w.grid.CellOutline(c, buf[:0]) {
		dst = append(dst, g.r.height(g.r.w.corner(p))/g.r.w.perUnit)
	}
	return dst
}

func (g chartGround) Takes(c cell.ID) bool { return g.r.roadable(c) && !g.r.crossing(c) }

func (g chartGround) Wet(c cell.ID) bool { return g.r.w.wet(c) }

func (g chartGround) Bridged(c cell.ID) (site.Part, bool) {
	m := g.laid[c]
	switch {
	case m.bridged:
		return site.Part{Way: m.axis}, true
	case m.head:
		return site.Part{Way: m.headWay, Head: true}, true
	}
	return site.Part{}, false
}

func (g chartGround) Road(c cell.ID) cell.Links {
	if m := g.laid[c]; m.paved {
		return m.ways
	}
	return 0
}

func (r *roads) earthwork(c cell.ID, in, out int, h float64) (float64, float64) {
	if r.w.flat {
		return 0, h
	}
	target, next := r.shape(c, in, out, h)
	return r.price(c, in, target), next
}

// the corners on the way in are paid already
func (r *roads) price(c cell.ID, in int, target map[int]float64) float64 {
	var paid []int
	if in != none {
		paid = r.w.rim(c, r.w.grid.Opposite(in))
	}
	earth := 0.0
	for k, t := range target {
		if slices.Contains(paid, k) {
			continue
		}
		if u, ok := r.under[k]; ok && t > u+planSlack {
			return math.Inf(1) // over a bridge's deck
		}
		if p, ok := r.planned[k]; ok {
			if math.Abs(p-t) > planSlack {
				return math.Inf(1)
			}
			continue
		}
		d := math.Abs(r.w.corners[k] - t)
		if d > r.cfg.Deep {
			return math.Inf(1)
		}
		earth += d
	}
	return r.cfg.Grade * earth / r.w.r.Area.Cell
}

// a ramp straight along a side, else the whole cell level, a diagonal too
func (r *roads) shape(c cell.ID, in, out int, h float64) (map[int]float64, float64) {
	ks := r.w.cornersOf(c)
	target := map[int]float64{}
	next := h
	g := r.w.grid
	switch {
	case in != none && out == in && !g.Corner(out):
		exit := r.w.rim(c, out)
		m := (r.height(exit[0]) + r.height(exit[1])) / 2
		rise := r.w.r.Area.Cell * math.Tan(ground.RoadSlope)
		next = min(max(m, h-rise), h+rise)
		for _, k := range r.w.rim(c, g.Opposite(in)) {
			target[k] = h
		}
		for _, k := range exit {
			target[k] = next
		}
	default:
		for _, k := range ks {
			target[k] = h
		}
	}
	return target, next
}

func (r *roads) height(k int) float64 {
	if p, ok := r.planned[k]; ok {
		return p
	}
	return r.w.corners[k]
}

func (r *roads) plan(rd road, laid map[cell.ID]mark) {
	for i, c := range rd.cells {
		in, out := r.ways(rd, i)
		m := laid[c]
		if rd.over[i] {
			if !m.bridged {
				m.bridged, m.axis, m.deck = true, in, rd.at[i]
			}
		} else {
			if !m.paved {
				m.paved, m.at = true, rd.at[i]
			}
			if in != none {
				m.ways |= 1 << r.w.grid.Opposite(in)
			}
			if out != none {
				m.ways |= 1 << out
			}
			switch {
			case out != none && rd.over[i+1]:
				m.head, m.headWay = true, out
			case in != none && rd.over[i-1]:
				m.head, m.headWay = true, r.w.grid.Opposite(in)
			}
		}
		laid[c] = m
	}
	r.aims(rd, func(_ int, c cell.ID, target map[int]float64) {
		for k, t := range target {
			if _, ok := r.planned[k]; !ok {
				r.planned[k] = t
			}
		}
	})
	if r.w.flat {
		return
	}
	for _, b := range leaps(rd) {
		ends := r.w.cornersOf(rd.cells[b[0]])
		far := r.w.cornersOf(rd.cells[b[1]])
		ceiling := rd.at[b[0]] - r.clearance()
		for _, c := range rd.cells[b[0]+1 : b[1]] {
			for _, k := range r.w.cornersOf(c) {
				if slices.Contains(ends[:], k) || slices.Contains(far[:], k) {
					continue
				}
				if u, ok := r.under[k]; !ok || ceiling < u {
					r.under[k] = ceiling
				}
			}
		}
	}
}

func (r *roads) aims(rd road, aim func(i int, c cell.ID, target map[int]float64)) {
	if r.w.flat {
		return
	}
	for i, c := range rd.cells {
		if rd.over[i] {
			continue
		}
		in, out := r.ways(rd, i)
		target, _ := r.shape(c, in, out, rd.at[i])
		if out != none && rd.over[i+1] || in != none && rd.over[i-1] {
			for _, k := range r.w.cornersOf(c) {
				target[k] = rd.at[i] // a bridge's head: flat at its deck
			}
		}
		aim(i, c, target)
	}
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

func (r *roads) crossings(rd road) {
	node := network.Node{Kind: r.w.r.Kinds.Road, Width: r.cfg.Width}
	for i, c := range rd.cells {
		if !rd.over[i] {
			r.net.Set(c, node)
		}
		if i > 0 && !(rd.over[i-1] && rd.over[i]) {
			r.net.Link(rd.cells[i-1], c)
		}
	}
	for i := 0; i < len(rd.cells); {
		if !rd.over[i] {
			i++
			continue
		}
		j := i
		for j < len(rd.cells) && rd.over[j] {
			j++
		}
		s := &span{cells: slices.Clone(rd.cells[i:j]), way: r.wayOf(rd.cells[i-1], rd.cells[i])}
		s.edges = append(s.edges, shared(r.w, rd.cells[i-1], rd.cells[i]),
			shared(r.w, rd.cells[j-1], rd.cells[j]))
		s.heads = append(s.heads, head{rd.cells[i-1], s.way},
			head{rd.cells[j], r.wayOf(rd.cells[j], rd.cells[j-1])})
		r.add(s)
		i = j
	}
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
			s.edges = append(s.edges, o.edges...)
			for _, h := range o.heads {
				if !slices.ContainsFunc(s.heads, func(g head) bool { return g.at == h.at }) {
					s.heads = append(s.heads, h)
				}
			}
			o.merged = true
		}
	}
	for _, c := range s.cells {
		r.spanOf[c] = s
	}
	fords, others := 0, 0
	for _, c := range s.cells {
		switch {
		case r.w.rivers.Courses[c] == water.Ford:
			fords++
		case r.w.wet(c) || !r.crossing(c):
			others++
		}
	}
	s.ford = fords > 0 && others == 0
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

func shared(w *chart, a, b cell.ID) []int {
	var out []int
	bk := w.cornersOf(b)
	for _, k := range w.cornersOf(a) {
		if slices.Contains(bk[:], k) {
			out = append(out, k)
		}
	}
	return out
}

// grade joins the corners each cell's road holds level into classes and lets only ramps apart;
// nothing off a road moves.
func (r *roads) grade() error {
	w := r.w
	r.ground = slices.Clone(w.corners)
	u := newUnion(len(w.corners))
	type job struct {
		c     cell.ID
		ks    [4]int
		links cell.Links
	}
	var jobs []job
	type ramp struct{ a, b int }
	var ramps []ramp
	for _, c := range r.net.Cells() {
		j := job{c, w.cornersOf(c), r.net.Links(c)}
		jobs = append(jobs, j)
		i, straight := grid.Straight(w.grid, j.links)
		switch {
		case straight && !w.grid.Corner(i):
			a, b := w.rim(c, i), w.rim(c, w.grid.Opposite(i))
			u.join(a[0], a[1])
			u.join(b[0], b[1])
			ramps = append(ramps, ramp{a[0], b[0]})
		default:
			for _, k := range j.ks[1:] {
				u.join(j.ks[0], k)
			}
		}
	}
	floor := map[int]float64{} // the least a class may stand at
	for _, s := range r.live() {
		if s.ford {
			for _, e := range s.edges {
				for _, k := range e[1:] {
					u.join(e[0], k)
				}
			}
			continue
		}
		var ends []int
		for _, h := range s.heads {
			ks := w.cornersOf(h.at)
			ends = append(ends, ks[:]...)
		}
		for _, k := range ends[1:] {
			u.join(ends[0], k)
		}
		low, bottom, wet := math.Inf(-1), math.Inf(1), false
		for _, c := range s.cells {
			_, paved := r.net.Node(c)
			wet = wet || w.wet(c)
			for _, k := range w.cornersOf(c) {
				if !slices.Contains(ends, k) {
					bottom = min(bottom, r.height(k))
				}
				switch {
				case paved:
					low = max(low, r.height(k)+r.underpass())
				case w.wet(c):
					low = max(low, w.corners[k]+r.clearance())
				}
			}
		}
		if !wet && !math.IsInf(bottom, 1) {
			low = max(low, bottom+r.underpass())
		}
		root := u.find(ends[0])
		floor[root] = max(floor[root], low)
	}
	value, size := map[int]float64{}, map[int]float64{}
	aimed, aims := map[int]float64{}, map[int]float64{}
	for k := range w.corners {
		root := u.find(k)
		value[root] += w.corners[k]
		size[root]++
		if p, ok := r.planned[k]; ok {
			aimed[root] += p
			aims[root]++
		}
	}
	for root := range value {
		value[root] /= size[root]
		if aims[root] > 0 {
			value[root] = aimed[root] / aims[root]
		}
		if f, ok := floor[root]; ok {
			value[root] = max(value[root], f)
		}
	}
	rise := 0.999 * w.r.Area.Cell * math.Tan(ground.RoadSlope)
	for range 1000000 {
		worst := 0.0
		for _, rp := range ramps {
			a, b := u.find(rp.a), u.find(rp.b)
			over := math.Abs(value[b]-value[a]) - rise
			if over <= 0 {
				continue
			}
			worst = max(worst, over)
			toward := math.Copysign(1, value[b]-value[a])
			value[a] += toward * over * size[b] / (size[a] + size[b])
			value[b] -= toward * over * size[a] / (size[a] + size[b])
		}
		for root, f := range floor {
			if value[root] < f {
				worst = max(worst, f-value[root])
				value[root] = f
			}
		}
		if worst < 1e-7 {
			break
		}
	}
	for _, s := range r.live() {
		if s.ford {
			continue
		}
		deck := value[u.find(s.edges[0][0])]
		for _, c := range s.cells {
			for _, k := range w.cornersOf(c) {
				if root := u.find(k); size[root] == 1 {
					value[root] = min(value[root], deck-r.clearance()) // cut under the deck
				}
			}
		}
	}
	for k := range w.corners {
		w.corners[k] = math.Round(value[u.find(k)]*100) / 100
	}
	for _, s := range r.live() {
		if !s.ford {
			s.deck = w.corners[w.cornersOf(s.heads[0].at)[0]]
		}
	}
	var bad []string
	for _, j := range jobs {
		if !ground.RoadFits(w.grid, j.c, w.outline(j.c), j.links, w.size) {
			x, y := w.xy(j.c)
			bad = append(bad, fmt.Sprintf("[%d, %d]", x, y))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("the ground under the road at %s cannot be graded",
			strings.Join(bad, ", "))
	}
	return nil
}

type union struct{ parent []int }

func newUnion(n int) *union {
	u := &union{parent: make([]int, n)}
	for i := range u.parent {
		u.parent[i] = i
	}
	return u
}

func (u *union) find(k int) int {
	for u.parent[k] != k {
		u.parent[k] = u.parent[u.parent[k]]
		k = u.parent[k]
	}
	return k
}

func (u *union) join(a, b int) {
	a, b = u.find(a), u.find(b)
	if a == b {
		return
	}
	u.parent[b] = a
}

type entry struct {
	id int
	at float64
}

type queue []entry

func (q queue) Len() int           { return len(q) }
func (q queue) Less(i, j int) bool { return q[i].at < q[j].at }
func (q queue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x any)        { *q = append(*q, x.(entry)) }
func (q *queue) Pop() any {
	old := *q
	e := old[len(old)-1]
	*q = old[:len(old)-1]
	return e
}
