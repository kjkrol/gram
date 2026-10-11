package main

import (
	"container/heap"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/network"
	"github.com/kjkrol/gram/plugins/board/water"
)

// roads are the roads of a map: routed place to place over the cheapest ground, carried over
// water and down valleys by crossings — bridges, fords — and, in relief, laid on ground graded as
// Transport Tycoon lays a road (ground.RoadFits).
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
	thirds  map[cell.ID]int // the corner each diagonal holds level with its line
	skipped [][2]string
}

// road is one route's cells, place to place, which of them it crosses on a bridge or a ford, and
// the height it comes into each at, in metres.
type road struct {
	cells []cell.ID
	over  []bool
	at    []float64
}

// span is a crossing: the cells a road crosses by, all one way, and where the road meets it on
// either side.
type span struct {
	cells  []cell.ID
	way    int     // the link it runs along
	ford   bool    // waded, else bridged
	edges  [][]int // the corners the road meets it at
	deck   float64 // metres: where a bridge runs, both its ends
	merged bool
}

// The links' ways, as cell.Links numbers its bits.
var steps = [8][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}}

// compass is each link's place round the compass, in eighths from the north.
var compass = [8]int{0, 4, 6, 2, 7, 1, 5, 3}

var opposite = [8]int{1, 0, 3, 2, 7, 6, 5, 4}

// clearance is how high a bridge's deck stands over the beds under it, in metres.
const clearance = 3.0

// planSlack is how far off the height a road laid before planned a corner at another may meet it,
// in metres.
const planSlack = 0.5

// heightBand is how near two heights a road comes to a cell at are one to the search for it, in
// metres.
const heightBand = 20.0

// mark is what the routes laid so far made of a cell: a road through it, the ways it runs and
// the height it comes at; a bridge over it, its way and its deck.
type mark struct {
	paved   bool
	ways    cell.Links
	at      float64
	bridged bool
	axis    int
	deck    float64
}

// underpass is how far over a road a bridge's deck crossing it stands, in metres.
const underpass = 5.0

func newRoads(w *chart) *roads {
	r := &roads{w: w, cfg: w.r.Roads, net: network.New(w.grid), places: map[string]cell.ID{},
		spanOf: map[cell.ID]*span{}, wetK: make([]bool, len(w.corners)),
		planned: map[int]float64{}, thirds: map[cell.ID]int{}}
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

// roadable reports whether a road may run through c: land with land all round it, off the coast.
func (r *roads) roadable(c cell.ID) bool {
	for _, k := range r.w.cornersOf(c) {
		x, y := k%(r.w.cols+1), k/(r.w.cols+1)
		if !r.w.ashore(x, y) {
			return false
		}
	}
	return true
}

// crossing reports whether a road may only cross c, never run on it: running water, and in relief
// the cells sharing a corner with its bed too.
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

// nearest is the cell nearest to a place a road may begin on: dry, level enough to grade, off
// rock and steep ground where it can.
func (r *roads) nearest(at [2]float64) (cell.ID, bool) {
	a := r.w.r.Area
	x, y := metresOf(a.Centre, at[0], at[1])
	fx, fy := x/a.Cell+float64(r.w.cols)/2, y/a.Cell+float64(r.w.rows)/2
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

// find is the cheapest road from one cell to another. A road keeps its height through a bend,
// along a diagonal and over a bridge, and climbs only straight along a side, RoadSlope at most a
// cell: each way is priced by what grading the ground under the road it holds would move, the
// height it reaches a cell at the best way's. A bridge runs straight along a side over ground lower
// than its deck — water, a valley — Span cells at most, a cell over water with -flat.
func (r *roads) find(from, to cell.ID, h0 float64, laid map[cell.ID]mark,
	barred map[cell.ID]bool) (road, bool) {
	type state struct {
		c    cell.ID
		way  int8  // the link it came by; 8 none
		span int8  // how many cells of a bridge it has crossed; 0 on the road
		band int16 // the height it comes at, in bands
	}
	band := func(h float64) int16 { return int16(math.Round(h / heightBand)) }
	most := int8(r.cfg.Span)
	if r.w.flat {
		most = 1
	}
	start := state{from, 8, 0, band(h0)}
	dist := map[state]float64{start: 0}
	height := map[state]float64{start: h0}
	prev := map[state]state{}
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
		if s.c == to && s.span == 0 {
			var rd road
			for p := s; ; p = prev[p] {
				rd.cells, rd.over = append(rd.cells, p.c), append(rd.over, p.span > 0)
				rd.at = append(rd.at, height[p])
				if p.way == 8 {
					break
				}
			}
			slices.Reverse(rd.cells)
			slices.Reverse(rd.over)
			slices.Reverse(rd.at)
			return rd, true
		}
		h := height[s]
		under := s.span == 0 && s.way < 8 && laid[s.c].bridged
		for way := range int8(8) {
			if (s.span > 0 || under) && way != s.way {
				continue // a bridge runs straight, and so does a road under one
			}
			b, ok := r.w.grid.Toward(s.c, int(way))
			if !ok || barred[b] {
				continue
			}
			leave, next := 0.0, h
			if s.span == 0 {
				leave, next = r.earthwork(s.c, int(s.way), int(way), h)
				if math.IsInf(leave, 1) {
					continue
				}
			}
			try := func(n state, c, at float64) {
				if math.IsInf(c, 1) {
					return
				}
				c += e.at + leave
				n.band = band(at)
				if d, seen := dist[n]; !seen || c < d {
					dist[n], prev[n], height[n] = c, s, at
					push(n, c)
				}
			}
			if r.paveable(s.c, b, int(way), next, laid) {
				c, at := r.travel(s.c, b, int(s.way), int(way), h, next, laid), next
				if s.span > 0 {
					c += r.abutment(s.c, b, h)
				}
				if b == to {
					end, _ := r.earthwork(b, int(way), 8, at)
					c += end
				}
				try(state{b, way, 0, 0}, c, at)
			}
			if way < 4 && s.span < most {
				try(state{b, way, s.span + 1, 0}, r.bridge(b, int(way), next, laid), next)
			}
		}
	}
	return road{}, false
}

// once is find's road from one cell to another passing every cell once, but where it crosses
// itself on a bridge over itself running across, and giving no corner two heights: each cell it
// came back to otherwise, or held a corner of its own at another height, is barred and the road
// found anew.
func (r *roads) once(from, to cell.ID, h0 float64, laid map[cell.ID]mark) (road, bool) {
	barred := map[cell.ID]bool{}
	for range 32 {
		rd, ok := r.find(from, to, h0, laid, barred)
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
		r.aims(rd, func(_ int, c cell.ID, target map[int]float64, _ int) {
			for k, t := range target {
				if v, ok := aimed[k]; ok && math.Abs(v-t) > planSlack && c != from && c != to {
					barred[c], again = true, true
				}
				aimed[k] = t
			}
		})
		if !again {
			return rd, true
		}
	}
	return road{}, false
}

// overpass reports whether rd's two visits to one cell, at, are a bridge over itself: one over it,
// one under it running straight across the bridge, the deck underpass over it or higher.
func (r *roads) overpass(rd road, at []int) bool {
	if len(at) != 2 || rd.over[at[0]] == rd.over[at[1]] {
		return false
	}
	over, under := at[0], at[1]
	if rd.over[under] {
		over, under = under, over
	}
	if under == 0 || under == len(rd.cells)-1 {
		return false
	}
	in := r.wayOf(rd.cells[under-1], rd.cells[under])
	out := r.wayOf(rd.cells[under], rd.cells[under+1])
	axis := r.wayOf(rd.cells[over-1], rd.cells[over])
	return in == out && across(in, axis) &&
		rd.at[over] >= max(rd.at[under], rd.at[under+1])+underpass
}

// across reports whether the ways a and b run square to each other along the sides.
func across(a, b int) bool {
	turn := (compass[a] - compass[b] + 8) % 8
	return a < 4 && b < 4 && (turn == 2 || turn == 6)
}

// paveable reports whether a road may run on from a to b the way way at height h: onto dry
// ground off the coast, under a bridge only square across it and underpass under its deck,
// cutting no corner over ground it may not take.
func (r *roads) paveable(a, b cell.ID, way int, h float64, laid map[cell.ID]mark) bool {
	if !r.roadable(b) || r.crossing(b) {
		return false
	}
	if m := laid[b]; m.bridged {
		rise := r.w.r.Area.Cell * math.Tan(ground.RoadSlope)
		if !across(way, m.axis) || h+rise+underpass > m.deck {
			return false
		}
	}
	if way >= 4 {
		w := r.w
		ax, ay := w.xy(a)
		bx, by := w.xy(b)
		for _, s := range []cell.ID{w.cell(ax, by), w.cell(bx, ay)} {
			if !r.roadable(s) || r.crossing(s) {
				return false
			}
		}
	}
	return true
}

// travel is what a step of road from a to b costs, the way out after the way in, leaving a at
// height next come into it at h.
func (r *roads) travel(a, b cell.ID, in, out int, h, next float64, laid map[cell.ID]mark) float64 {
	w := r.w
	along := 1.0
	if out >= 4 {
		along = math.Sqrt2
	}
	cost := along * (1 + r.cfg.Climb*math.Abs(next-h)/(along*w.r.Area.Cell))
	if in < 8 {
		turn := (compass[out] - compass[in] + 8) % 8
		cost += r.cfg.Turn * float64(min(turn, 8-turn))
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

// abutment is what grading the corners a bridge's last cell, last, shares with the road after it,
// b, to its deck h costs.
func (r *roads) abutment(last, b cell.ID, h float64) float64 {
	if r.w.flat {
		return 0
	}
	earth := 0.0
	for _, k := range shared(r.w, last, b) {
		if p, ok := r.planned[k]; ok {
			if math.Abs(p-h) > planSlack {
				return math.Inf(1)
			}
			continue
		}
		d := math.Abs(r.w.corners[k] - h)
		if d > r.cfg.Deep {
			return math.Inf(1)
		}
		earth += d
	}
	return r.cfg.Grade * earth / r.w.r.Area.Cell
}

// bridge is what a cell of a bridge the way way over b at deck h costs: +Inf where b may not be
// bridged — on the coast, along a bridge at another deck or across one, over a road but one
// running square across it underpass under the deck, with -flat but over water, in relief High
// over the ground under it or higher — dearer by the earth cut where the ground stands over the
// deck, or the deck lower than clearance over a bed.
func (r *roads) bridge(b cell.ID, way int, h float64, laid map[cell.ID]mark) float64 {
	w := r.w
	m := laid[b]
	if !r.roadable(b) || w.flat && !w.wet(b) {
		return math.Inf(1)
	}
	if m.bridged && (math.Abs(m.deck-h) > heightBand/2 || way != m.axis &&
		way != opposite[m.axis]) {
		return math.Inf(1)
	}
	if m.paved {
		square := cell.Links(0)
		for _, x := range []int{0, 2} {
			if across(x, way) {
				square = 1<<x | 1<<opposite[x]
			}
		}
		if m.ways != square {
			return math.Inf(1)
		}
		for _, k := range w.cornersOf(b) {
			if r.height(k)+underpass > h {
				return math.Inf(1)
			}
		}
	}
	cost := 1 + r.cfg.Bridge
	if m.bridged {
		cost *= r.cfg.Reuse
	}
	if w.flat {
		return cost
	}
	for _, k := range w.cornersOf(b) {
		if m.paved {
			break
		}
		over := w.corners[k] - h
		if -over >= r.cfg.High || over > r.cfg.Deep {
			return math.Inf(1)
		}
		if r.wetK[k] {
			over += clearance
		}
		cost += r.cfg.Grade * max(over, 0) / w.r.Area.Cell
	}
	return cost
}

// earthwork is what grading the ground under a road through c costs, come in the way in (8: it
// begins there) at height h and going out the way out (8: it ends there) — its corners moved to
// where such a road lies, weighed by Grade; those on the way in counted already — and the height
// the road leaves c at: +Inf where a corner would move further than Deep, or off the height a road
// laid before gave it.
func (r *roads) earthwork(c cell.ID, in, out int, h float64) (float64, float64) {
	if r.w.flat {
		return 0, h
	}
	target, next, _ := r.shape(c, in, out, h)
	var paid []int
	if in < 8 {
		paid = sideOf(r.w.cornersOf(c), opposite[in])
	}
	earth := 0.0
	for k, t := range target {
		if slices.Contains(paid, k) {
			continue
		}
		if p, ok := r.planned[k]; ok {
			if math.Abs(p-t) > planSlack {
				return math.Inf(1), next
			}
			continue
		}
		d := math.Abs(r.w.corners[k] - t)
		if d > r.cfg.Deep {
			return math.Inf(1), next
		}
		earth += d
	}
	return r.cfg.Grade * earth / r.w.r.Area.Cell, next
}

// shape is where a road through c, come in the way in at height h and going out the way out,
// would have c's corners — every one it holds, by its height — the height it leaves c at, and the
// corner a diagonal holds level with its line (-1 for none).
func (r *roads) shape(c cell.ID, in, out int, h float64) (map[int]float64, float64, int) {
	ks := r.w.cornersOf(c)
	target := map[int]float64{}
	next, third := h, -1
	switch {
	case in < 8 && out == in && out < 4:
		exit := sideOf(ks, out)
		m := (r.height(exit[0]) + r.height(exit[1])) / 2
		rise := r.w.r.Area.Cell * math.Tan(ground.RoadSlope)
		next = min(max(m, h-rise), h+rise)
		for _, k := range sideOf(ks, opposite[in]) {
			target[k] = h
		}
		for _, k := range exit {
			target[k] = next
		}
	case in < 8 && out == in:
		line := []int{sideOf(ks, opposite[in])[0], sideOf(ks, out)[0]}
		var others []int
		for _, k := range ks {
			if !slices.Contains(line, k) {
				others = append(others, k)
			}
		}
		third = others[0]
		if math.Abs(r.height(others[1])-h) < math.Abs(r.height(third)-h) {
			third = others[1]
		}
		for _, k := range append(line, third) {
			target[k] = h
		}
	default:
		for _, k := range ks {
			target[k] = h
		}
	}
	return target, next, third
}

func (r *roads) height(k int) float64 {
	if p, ok := r.planned[k]; ok {
		return p
	}
	return r.w.corners[k]
}

// plan notes rd's cells as laid and the heights it gives their corners, a corner planned before
// kept as it was.
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
			if in < 8 {
				m.ways |= 1 << opposite[in]
			}
			if out < 8 {
				m.ways |= 1 << out
			}
		}
		laid[c] = m
	}
	r.aims(rd, func(_ int, c cell.ID, target map[int]float64, third int) {
		for k, t := range target {
			if _, ok := r.planned[k]; !ok {
				r.planned[k] = t
			}
		}
		if third >= 0 {
			r.thirds[c] = third
		}
	})
}

// aims hands each paved cell of rd, with its place on rd, to aim: where the road has its corners,
// by their heights, and the corner a diagonal holds level with its line (-1 for none).
func (r *roads) aims(rd road, aim func(i int, c cell.ID, target map[int]float64, third int)) {
	if r.w.flat {
		return
	}
	for i, c := range rd.cells {
		if rd.over[i] {
			continue
		}
		in, out := r.ways(rd, i)
		target, next, third := r.shape(c, in, out, rd.at[i])
		if out < 8 && rd.over[i+1] {
			for _, k := range sideOf(r.w.cornersOf(c), out) {
				target[k] = next
			}
		}
		aim(i, c, target, third)
	}
}

func (r *roads) ways(rd road, i int) (in, out int) {
	in, out = 8, 8
	if i > 0 {
		in = r.wayOf(rd.cells[i-1], rd.cells[i])
	}
	if i+1 < len(rd.cells) {
		out = r.wayOf(rd.cells[i], rd.cells[i+1])
	}
	return in, out
}

// sideOf is the corners of a cell, ks, on its side the way way goes: two for a side, one for a
// corner.
func sideOf(ks [4]int, way int) []int {
	tl, tr, bl, br := ks[0], ks[1], ks[2], ks[3]
	return [8][]int{{tl, tr}, {bl, br}, {tl, bl}, {tr, br}, {tl}, {tr}, {bl}, {br}}[way]
}

// crossings lays rd's paved cells on the network, linked as a level file links them, and its
// crossings as spans, those sharing a cell merged.
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
		r.add(s)
		i = j
	}
}

func (r *roads) wayOf(a, b cell.ID) int {
	for way := range 8 {
		if m, ok := r.w.grid.Toward(a, way); ok && m == b {
			return way
		}
	}
	return 0
}

// add takes s among the spans, merged into those it shares a cell with: a ford where it wades
// across fords alone.
func (r *roads) add(s *span) {
	for _, c := range s.cells {
		if o, ok := r.spanOf[c]; ok && o != s && !o.merged {
			for _, oc := range o.cells {
				if !slices.Contains(s.cells, oc) {
					s.cells = append(s.cells, oc)
				}
			}
			s.edges = append(s.edges, o.edges...)
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

// grade shapes the corners under the roads until every road cell lies as Transport Tycoon lays
// it: the corners a cell's road holds level are one height, the sides of a ramp no further apart
// than RoadSlope lets them, a bridge's ends all at its deck, over the beds under it; nothing else
// moves. Each height is the one its road came to it at, where roads meet the mean of theirs.
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
		tl, tr, bl, br := j.ks[0], j.ks[1], j.ks[2], j.ks[3]
		switch j.links {
		case linkN | linkS:
			u.join(tl, tr)
			u.join(bl, br)
			ramps = append(ramps, ramp{tl, bl})
		case linkW | linkE:
			u.join(tl, bl)
			u.join(tr, br)
			ramps = append(ramps, ramp{tl, tr})
		case linkNW | linkSE:
			u.join(tl, br)
			u.join(tl, r.third(c, tl, br, tr, bl))
		case linkNE | linkSW:
			u.join(tr, bl)
			u.join(tr, r.third(c, tr, bl, tl, br))
		default:
			u.join(tl, tr)
			u.join(tl, bl)
			u.join(tl, br)
		}
	}
	floor := map[int]float64{} // the least a class may stand at: a bridge's deck over its beds
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
		for _, e := range s.edges {
			ends = append(ends, e...)
		}
		for _, k := range ends[1:] {
			u.join(ends[0], k)
		}
		low := math.Inf(-1)
		for _, c := range s.cells {
			_, paved := r.net.Node(c)
			for _, k := range w.cornersOf(c) {
				switch {
				case paved:
					low = max(low, r.height(k)+underpass)
				case w.wet(c):
					low = max(low, w.corners[k]+clearance)
				}
			}
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
	for k := range w.corners {
		w.corners[k] = math.Round(value[u.find(k)]*100) / 100
	}
	for _, s := range r.live() {
		if !s.ford {
			s.deck = w.corners[s.edges[0][0]]
		}
	}
	var bad []string
	for _, j := range jobs {
		var units [4]float64
		for i, k := range j.ks {
			units[i] = w.corners[k] / w.perUnit
		}
		if !ground.RoadFits(units, j.links, w.size) {
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

// third is the corner of c a diagonal along a and b holds level with them: the one its road
// planned, else whichever of x and y stands nearer the middle of a and b.
func (r *roads) third(c cell.ID, a, b, x, y int) int {
	if k, ok := r.thirds[c]; ok {
		return k
	}
	h := r.w.corners
	m := (h[a] + h[b]) / 2
	if math.Abs(h[x]-m) <= math.Abs(h[y]-m) {
		return x
	}
	return y
}

// The links of a square cell, as cell.Links numbers them.
const (
	linkN cell.Links = 1 << iota
	linkS
	linkW
	linkE
	linkNW
	linkNE
	linkSW
	linkSE
)

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
