package main

import (
	"bytes"
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/network"
	"github.com/kjkrol/gram/plugins/board/water"
	usite "github.com/kjkrol/gram/plugins/underground/site"
)

// streets are OpenStreetMap's ways laid as they run: a cell wide on the grid, one road a cell,
// each cell's axis where the ways pass it, the ground under them cut and filled to a profile
// along the axis and level across it.
type streets struct {
	w      *chart
	lots   map[cell.ID]*lot
	legs   []street
	rounds map[cell.ID]float64 // a roundabout's radius, metres, by its middle's cell
	spans  []*viaduct
	shaped map[int]bool // the corners a street's ground was laid at
	// the cells of water streets end in, covered
	covered int
	// the buildings drawn back off a road or a bed, and those left out for standing in one
	narrowed, gone int
	// the trees moved off a road, and those no room was left
	shifted, felled int
	// the tunnels a street's cut left bare, left out
	bared int
	// the cells a tunnel's head drew back from and those between a tunnel and its twin: streets
	freed [][2]int
	// the bridges a head of which would stand far off its ground, left out
	aloft int
	// what the laying came to: the steepest climb left, the deepest cut and the highest fill,
	// metres, and the cells cut or filled further than the recipe lets a corner be
	steepest, cut, fill float64
	over                int
}

// lot is a cell a street runs over: where its axis passes, in cells, the axis's height, metres,
// its lanes, and the way it is driven where it runs one way
type lot struct {
	at     [2]float64
	h      float64
	set    bool
	pin    bool // a tunnel's head: its height is the tunnel's
	lanes  int
	oneway bool
	dir    [2]float64
	// the ways through the cell as they are gathered: how many, the lanes of those run both
	// ways, of those run one way and the way the first of them runs
	n        int
	sum      [2]float64
	two, one int
	opposed  bool
}

// street is a stretch from a junction to the next: its cells in the order driven
type street struct {
	cells  []cell.ID
	lanes  int
	oneway bool
	// it leads onto a bridge's head at its first cell, at its last
	first, last bool
}

// how often the profile is eased, the share of its neighbours' mean a cell's height takes each
// time, the ground under it the rest, and how far off its cell's middle an axis may pass, of a
// cell
const (
	easings = 400
	easing  = 0.8
	leeway  = 0.3
)

func (w *chart) newStreets(ways []osmWay) *streets {
	s := &streets{w: w, lots: map[cell.ID]*lot{}, rounds: map[cell.ID]float64{}}
	ground, decks := s.structures(ways)
	open := s.freed
	var keep [][2]int
	for _, t := range w.roads.tunnels {
		for _, h := range [2]cell.ID{t.from, t.to} {
			x, y := w.xy(h)
			keep = append(keep, [2]int{x, y})
		}
	}
	for _, ring := range roundabouts(ways) {
		x, y := w.place(ring.at[0], ring.at[1])
		cx, cy := int(math.Floor(x)), int(math.Floor(y))
		if cx < 0 || cy < 0 || cx >= w.cols || cy >= w.rows {
			continue
		}
		c := w.cell(cx, cy)
		s.rounds[c] = ring.radius
		s.lots[c] = &lot{at: [2]float64{x, y}, lanes: ring.lanes, n: -1}
		keep = append(keep, [2]int{cx, cy})
		reach := ring.radius / w.r.Area.Cell
		for dy := -int(reach) - 1; dy <= int(reach)+1; dy++ {
			for dx := -int(reach) - 1; dx <= int(reach)+1; dx++ {
				px, py := cx+dx, cy+dy
				if px < 0 || py < 0 || px >= w.cols || py >= w.rows {
					continue
				}
				if math.Hypot(float64(px)+0.5-x, float64(py)+0.5-y) <= reach {
					open = append(open, [2]int{px, py})
				}
			}
		}
	}
	for _, v := range decks {
		s.spans = append(s.spans, v)
		for _, h := range [2]cell.ID{v.cells[0], v.cells[len(v.cells)-1]} {
			x, y := w.xy(h)
			keep = append(keep, [2]int{x, y})
		}
	}
	for _, way := range ground {
		s.gather(way)
	}
	g := w.newGuide(ground, open, keep)
	for c, l := range s.lots {
		l.settle(w, c)
	}
	// a deck begins where its street's axis passes its head, the road running on unbroken
	for _, v := range s.spans {
		for _, h := range [2]int{0, len(v.cells) - 1} {
			if l, ok := s.lots[v.cells[h]]; ok {
				v.at[h] = l.at
			}
		}
	}
	// a tunnel's head lies where its tunnel laid it, the street running in through its middle
	for _, t := range w.roads.tunnels {
		for _, h := range [2]cell.ID{t.from, t.to} {
			x, y := w.xy(h)
			l := s.lot(h)
			l.at = [2]float64{float64(x) + 0.5, float64(y) + 0.5}
			l.h, l.set, l.pin = w.level(h), true, true
		}
	}
	for _, leg := range g.legs {
		s.legs = append(s.legs, s.forded(s.street(leg))...)
	}
	// the beds of the water still running are never moved, those covered are a street's ground
	clear(w.roads.wetK)
	w.grid.EachCell(func(c cell.ID) {
		if w.wet(c) {
			for _, k := range w.cornersOf(c) {
				w.roads.wetK[k] = true
			}
		}
	})
	// a bridge a head of which the profile leaves high over its ground or deep under it is no
	// bridge the ground shows: the streets are laid again without it
	for {
		s.profile()
		n := len(s.spans)
		s.spans = slices.DeleteFunc(s.spans, func(v *viaduct) bool {
			return slices.ContainsFunc([]cell.ID{v.cells[0], v.cells[len(v.cells)-1]},
				func(h cell.ID) bool {
					l := s.lot(h)
					return math.Abs(l.h-w.groundAt(l.at[0], l.at[1])) > aloft
				})
		})
		if len(s.spans) == n {
			break
		}
		s.aloft += n - len(s.spans)
		for _, l := range s.lots {
			l.set = l.pin
		}
	}
	heads := map[cell.ID]bool{}
	for _, v := range s.spans {
		heads[v.cells[0]], heads[v.cells[len(v.cells)-1]] = true, true
	}
	for i := range s.legs {
		st := &s.legs[i]
		st.first = st.first || heads[st.cells[0]]
		st.last = st.last || heads[st.cells[len(st.cells)-1]]
	}
	s.shape()
	s.clear()
	s.open()
	return s
}

// viaduct is a bridge the streets cross by: every cell from head to head, where its axis
// passes each, in cells, its deck's height over each, metres, and its lanes
type viaduct struct {
	cells []cell.ID
	at    [][2]float64
	deck  []float64
	lanes int
	// the most air under its deck, metres, and whether it crosses water
	void float64
	wet  bool
}

// metres: a bridge as long or as high over the ground is a viaduct, a shorter one over water
// an arch
const (
	viaductLong = 60
	viaductHigh = 12
	archLong    = 50
)

// kind is the kind of bridge v is written as
func (s *streets) kind(v *viaduct) string {
	k := s.w.r.Kinds
	long := float64(len(v.cells)-1) * s.w.r.Area.Cell
	switch {
	case k.Viaduct != "" && (long >= viaductLong || v.void >= viaductHigh):
		return k.Viaduct
	case k.Arch != "" && v.wet && long <= archLong:
		return k.Arch
	}
	return k.Bridge
}

// metres of air under a deck that make it a bridge: one OpenStreetMap draws over ground the
// heights show no hollow in lies on the ground
const hollow = 3

// the steepest a bridge's deck climbs, rise over run
const deckRise = 0.06

// the most lanes a deck carries: as wide as its cell
const deckLanes = 3

// metres a bridge's head may stand over its ground or under it
const aloft = 20

// how far off its cell's middle a deck's axis may pass, of a cell: whoever crosses walks the
// middles
const deckLeeway = 0.25

// between are the cells on the straight line from a to b
func (w *chart) between(a, b cell.ID) [][2]int {
	ax, ay := w.xy(a)
	bx, by := w.xy(b)
	n := max(abs(bx-ax), abs(by-ay))
	var out [][2]int
	for k := 0; k <= n; k++ {
		t := float64(k) / float64(max(n, 1))
		out = append(out, [2]int{
			ax + int(math.Round(float64(bx-ax)*t)),
			ay + int(math.Round(float64(by-ay)*t)),
		})
	}
	return out
}

// forded cuts st where it runs through water: a bridge from the dry cell before to the dry
// cell after, the street leading onto it from either side; water a stretch ends in is covered,
// the streets meeting over it
func (s *streets) forded(st street) []street {
	w := s.w
	var out []street
	start := 0
	first := false
	for i := 0; i < len(st.cells); i++ {
		if !w.wet(st.cells[i]) {
			continue
		}
		j := i
		for j < len(st.cells) && w.wet(st.cells[j]) {
			j++
		}
		if i == 0 || j == len(st.cells) {
			// streets meet over the water, or one runs along it: the water runs under them
			for _, c := range st.cells[i:j] {
				w.rivers.Courses[c] = water.Dry
			}
			s.covered += j - i
			i = j
			continue
		}
		v := &viaduct{cells: slices.Clone(st.cells[i-1 : j+1]), lanes: st.lanes}
		for _, c := range v.cells {
			v.at = append(v.at, s.lot(c).at)
		}
		s.spans = append(s.spans, v)
		if i-1 > start {
			out = append(out, street{cells: st.cells[start:i], lanes: st.lanes,
				oneway: st.oneway, first: first, last: true})
		}
		start, first = j, true
		i = j
	}
	if len(st.cells)-start >= 2 {
		out = append(out, street{cells: st.cells[start:], lanes: st.lanes, oneway: st.oneway,
			first: first})
	}
	return out
}

// clear sets every bridge's deck even from head to head and lowers the ground that would
// stand in it
func (s *streets) clear() {
	w := s.w
	room := w.r.Roads.Thickness + 0.5
	for _, v := range s.spans {
		last := len(v.cells) - 1
		run := make([]float64, len(v.cells))
		for i := 1; i <= last; i++ {
			run[i] = run[i-1] + math.Hypot(v.at[i][0]-v.at[i-1][0], v.at[i][1]-v.at[i-1][1])
		}
		for _, h := range [2]int{0, last} {
			if l := s.lot(v.cells[h]); !l.set {
				l.h, l.set = w.groundAt(v.at[h][0], v.at[h][1]), true
			}
		}
		from, to := s.lot(v.cells[0]).h, s.lot(v.cells[last]).h
		v.deck = make([]float64, len(v.cells))
		for i, c := range v.cells {
			v.deck[i] = from + (to-from)*run[i]/run[last]
			if i == 0 || i == last {
				continue
			}
			v.void = max(v.void, v.deck[i]-w.groundAt(v.at[i][0], v.at[i][1]))
			v.wet = v.wet || w.wet(c)
			if _, road := s.lots[c]; road && s.lots[c].set {
				continue // a street under the bridge keeps its ground
			}
			for _, k := range w.cornersOf(c) {
				if !w.roads.wetK[k] && !s.shaped[k] && !w.roads.pinned[k] {
					w.corners[k] = min(w.corners[k], max(v.deck[i]-room, 0))
				}
			}
		}
	}
}

// a roundabout as OpenStreetMap draws it: its middle, longitude and latitude, its radius, metres
type circus struct {
	at     [2]float64
	radius float64
	lanes  int
}

// roundabouts are the rings of ways, each made of the ways sharing an end
func roundabouts(ways []osmWay) []circus {
	var parts []osmWay
	for _, w := range ways {
		if w.roundabout {
			parts = append(parts, w)
		}
	}
	group := make([]int, len(parts))
	for i := range group {
		group[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if group[i] != i {
			group[i] = find(group[i])
		}
		return group[i]
	}
	ends := map[[2]float64]int{}
	for i, p := range parts {
		for _, e := range [2][2]float64{p.line[0], p.line[len(p.line)-1]} {
			if j, ok := ends[e]; ok {
				group[find(i)] = find(j)
			} else {
				ends[e] = i
			}
		}
	}
	var out []circus
	for root := range parts {
		if find(root) != root {
			continue
		}
		var pts [][2]float64
		lanes := 1
		for i, p := range parts {
			if find(i) == root {
				pts = append(pts, p.line...)
				lanes = max(lanes, p.lanes)
			}
		}
		var mid [2]float64
		for _, p := range pts {
			mid[0], mid[1] = mid[0]+p[0]/float64(len(pts)), mid[1]+p[1]/float64(len(pts))
		}
		r := 0.0
		for _, p := range pts {
			mx, my := metresOf(mid, p[0], p[1])
			r += math.Hypot(mx, my) / float64(len(pts))
		}
		out = append(out, circus{at: mid, radius: r, lanes: lanes})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].at, out[j].at
		return a[0] < b[0] || a[0] == b[0] && a[1] < b[1]
	})
	return out
}

// a way's pass through a cell: the point of it nearest the cell's middle, in cells, and the
// way it runs there
type pass struct {
	at  [2]float64
	d   float64
	dir [2]float64
}

// passes are the cells way runs over, in order, and its pass through each
func (s *streets) passes(way osmWay) ([]cell.ID, map[cell.ID]*pass) {
	w := s.w
	passes := map[cell.ID]*pass{}
	var order []cell.ID
	for i := 1; i < len(way.line); i++ {
		ax, ay := w.place(way.line[i-1][0], way.line[i-1][1])
		bx, by := w.place(way.line[i][0], way.line[i][1])
		run := math.Hypot(bx-ax, by-ay)
		if run == 0 {
			continue
		}
		dir := [2]float64{(bx - ax) / run, (by - ay) / run}
		n := max(1, int(math.Ceil(8*run)))
		for k := 0; k <= n; k++ {
			t := float64(k) / float64(n)
			x, y := ax+(bx-ax)*t, ay+(by-ay)*t
			cx, cy := int(math.Floor(x)), int(math.Floor(y))
			if cx < 0 || cy < 0 || cx >= w.cols || cy >= w.rows {
				continue
			}
			c := w.cell(cx, cy)
			d := math.Hypot(x-float64(cx)-0.5, y-float64(cy)-0.5)
			p := passes[c]
			if p == nil {
				p = &pass{d: math.Inf(1)}
				passes[c] = p
				order = append(order, c)
			}
			if d < p.d {
				p.at, p.d, p.dir = [2]float64{x, y}, d, dir
			}
		}
	}
	return order, passes
}

// gather notes where way passes each cell it runs over, its lanes and the way it is driven
func (s *streets) gather(way osmWay) {
	order, passes := s.passes(way)
	for _, c := range order {
		p, l := passes[c], s.lots[c]
		if l == nil {
			l = &lot{}
			s.lots[c] = l
		}
		if l.n < 0 { // a roundabout's middle
			continue
		}
		l.n++
		l.sum[0], l.sum[1] = l.sum[0]+p.at[0], l.sum[1]+p.at[1]
		switch {
		case !way.oneway:
			l.two = max(l.two, way.lanes)
		case l.one == 0:
			l.one, l.dir = way.lanes, p.dir
		case l.dir[0]*p.dir[0]+l.dir[1]*p.dir[1] < -0.5 && !l.opposed:
			l.one, l.opposed = l.one+way.lanes, true
		default:
			l.one = max(l.one, way.lanes)
		}
	}
}

// settle makes of what was gathered the cell's axis, lanes and way
func (l *lot) settle(w *chart, c cell.ID) {
	x, y := w.xy(c)
	if l.n > 0 {
		l.at = [2]float64{l.sum[0] / float64(l.n), l.sum[1] / float64(l.n)}
	}
	if l.n == 0 {
		l.at = [2]float64{float64(x) + 0.5, float64(y) + 0.5}
	}
	l.at[0] = min(max(l.at[0], float64(x)+0.5-leeway), float64(x)+0.5+leeway)
	l.at[1] = min(max(l.at[1], float64(y)+0.5-leeway), float64(y)+0.5+leeway)
	if l.n < 0 {
		return
	}
	l.lanes = max(l.two, l.one, 1)
	l.oneway = l.two == 0 && l.one > 0 && !l.opposed
}

// street is leg as it is driven: its lanes those of most of its cells, one way where they all
// are, turned to run the way they are driven
func (s *streets) street(leg leg) street {
	st := street{cells: slices.Clone(leg.trace)}
	count := map[int]int{}
	oneway, along := 0, 0.0
	for i, c := range st.cells {
		l := s.lot(c)
		if l.n < 0 {
			continue
		}
		count[l.lanes]++
		if !l.oneway {
			continue
		}
		oneway++
		a, b := s.lot(st.cells[max(i-1, 0)]), s.lot(st.cells[min(i+1, len(st.cells)-1)])
		along += l.dir[0]*(b.at[0]-a.at[0]) + l.dir[1]*(b.at[1]-a.at[1])
	}
	best := 0
	for lanes, n := range count {
		if n > count[best] || n == count[best] && lanes < best {
			best = lanes
		}
	}
	st.lanes = max(best, 1)
	st.oneway = oneway > 0 && 5*oneway >= 4*(len(st.cells)-2)
	if st.oneway && along < 0 {
		slices.Reverse(st.cells)
	}
	return st
}

// lot is the cell c's, one whose axis passes its middle where no way was gathered over it
func (s *streets) lot(c cell.ID) *lot {
	l := s.lots[c]
	if l == nil || l.n == 0 && l.lanes == 0 {
		l = &lot{lanes: 1}
		l.settle(s.w, c)
		s.lots[c] = l
	}
	return l
}

// ground is the ground's height at a point given in cells, metres
func (w *chart) groundAt(x, y float64) float64 {
	x, y = min(max(x, 0), float64(w.cols)), min(max(y, 0), float64(w.rows))
	x0, y0 := min(int(x), w.cols-1), min(int(y), w.rows-1)
	u, v := x-float64(x0), y-float64(y0)
	k := y0*(w.cols+1) + x0
	top := w.corners[k]*(1-u) + w.corners[k+1]*u
	bottom := w.corners[k+w.cols+1]*(1-u) + w.corners[k+w.cols+2]*u
	return top*(1-v) + bottom*v
}

// profile sets every cell's height along its street: the ground's under its axis, eased along
// the street and held to the steepest climb the recipe lets a road take
func (s *streets) profile() {
	w := s.w
	s.steepest, s.cut, s.fill, s.over = 0, 0, 0, 0
	type link struct {
		a, b *lot
		run  float64 // metres
		rise float64 // the most it climbs, rise over run
	}
	var links []link
	rise := math.Tan(w.r.Roads.Slope * math.Pi / 180)
	seen := map[[2]cell.ID]bool{}
	near := map[*lot][]*lot{}
	for _, st := range s.legs {
		for i := 1; i < len(st.cells); i++ {
			a, b := st.cells[i-1], st.cells[i]
			if seen[[2]cell.ID{a, b}] || seen[[2]cell.ID{b, a}] {
				continue
			}
			seen[[2]cell.ID{a, b}] = true
			la, lb := s.lot(a), s.lot(b)
			run := math.Hypot(lb.at[0]-la.at[0], lb.at[1]-la.at[1]) * w.r.Area.Cell
			links = append(links, link{la, lb, run, rise})
			near[la], near[lb] = append(near[la], lb), append(near[lb], la)
		}
	}
	var lots []*lot
	touch := func(c cell.ID) *lot {
		l := s.lot(c)
		if !l.set {
			l.h, l.set = w.groundAt(l.at[0], l.at[1]), true
			lots = append(lots, l)
		}
		return l
	}
	for _, st := range s.legs {
		for _, c := range st.cells {
			touch(c)
		}
	}
	for _, v := range s.spans {
		la, lb := touch(v.cells[0]), touch(v.cells[len(v.cells)-1])
		links = append(links, link{la, lb, float64(len(v.cells)-1) * w.r.Area.Cell,
			min(rise, deckRise)})
	}
	lies := make([]float64, len(lots))
	for i, l := range lots {
		lies[i] = l.h
	}
	next := make([]float64, len(lots))
	hold := func() {
		for _, k := range links {
			over := math.Abs(k.b.h-k.a.h) - k.rise*k.run
			if over <= 0 || k.a.pin && k.b.pin {
				continue
			}
			shift := math.Copysign(over/2, k.b.h-k.a.h)
			switch {
			case k.a.pin:
				k.b.h -= 2 * shift
			case k.b.pin:
				k.a.h += 2 * shift
			default:
				k.a.h, k.b.h = k.a.h+shift, k.b.h-shift
			}
		}
	}
	for range easings {
		for i, l := range lots {
			sum := 0.0
			for _, m := range near[l] {
				sum += m.h
			}
			next[i] = l.h
			if n := len(near[l]); n > 0 {
				next[i] = lies[i]*(1-easing) + sum/float64(n)*easing
			}
		}
		for i, l := range lots {
			l.h = next[i]
		}
		hold()
	}
	for range easings {
		hold()
	}
	for _, k := range links {
		s.steepest = max(s.steepest, math.Abs(k.b.h-k.a.h)/k.run)
	}
	for _, l := range lots {
		g := w.groundAt(l.at[0], l.at[1])
		s.cut, s.fill = max(s.cut, g-l.h), max(s.fill, l.h-g)
		if g-l.h > w.r.Roads.Cut || l.h-g > w.r.Roads.Fill {
			s.over++
		}
	}
}

// shape lays the ground under the streets: each corner of a street's cell at the height its
// profile has abreast of it, the mean where streets share it, so the road lies level across
func (s *streets) shape() {
	w := s.w
	sum := map[int]float64{}
	count := map[int]int{}
	lay := func(c cell.ID, at [2]float64, h float64, way [2]float64, grade float64) {
		x, y := w.xy(c)
		for i, k := range w.cornersOf(c) {
			kx, ky := float64(x+i%2), float64(y+i/2)
			along := ((kx-at[0])*way[0] + (ky-at[1])*way[1]) * w.r.Area.Cell
			sum[k] += h + grade*along
			count[k]++
		}
	}
	ends := map[cell.ID]int{}
	for _, st := range s.legs {
		for i, c := range st.cells {
			if i == 0 || i == len(st.cells)-1 {
				ends[c]++
			}
		}
	}
	laid := map[cell.ID]bool{}
	for _, st := range s.legs {
		for i, c := range st.cells {
			l := s.lot(c)
			inner := i > 0 && i < len(st.cells)-1
			if !inner && ends[c] != 1 || laid[c] {
				continue // a junction lies level
			}
			laid[c] = true
			a, b := s.lot(st.cells[max(i-1, 0)]), s.lot(st.cells[min(i+1, len(st.cells)-1)])
			run := math.Hypot(b.at[0]-a.at[0], b.at[1]-a.at[1])
			if run == 0 {
				continue
			}
			way := [2]float64{(b.at[0] - a.at[0]) / run, (b.at[1] - a.at[1]) / run}
			lay(c, l.at, l.h, way, (b.h-a.h)/(run*w.r.Area.Cell))
		}
	}
	for c, n := range ends {
		if n != 1 && !laid[c] {
			l := s.lot(c)
			lay(c, l.at, l.h, [2]float64{}, 0)
		}
	}
	s.shaped = map[int]bool{}
	for k, n := range count {
		if !w.roads.wetK[k] && !w.roads.pinned[k] {
			w.corners[k] = max(sum[k]/float64(n), 0)
			s.shaped[k] = true
		}
	}
	// a bridge's head lies level at its street's height, the deck beginning even with all of it
	for _, v := range s.spans {
		for _, h := range [2]cell.ID{v.cells[0], v.cells[len(v.cells)-1]} {
			for _, k := range w.cornersOf(h) {
				if !w.roads.wetK[k] && !w.roads.pinned[k] {
					w.corners[k] = max(s.lot(h).h, 0)
					s.shaped[k] = true
				}
			}
		}
	}
	// a street over a tunnel leaves the ground its roof wants: a rise in the street, no hole
	// in the tunnel
	cfg := w.r.Tunnels
	for _, t := range w.roads.tunnels {
		for i, c := range t.plan.Cells {
			roof := float64(t.plan.Floors[i])*w.perUnit + cfg.Clear*w.figures() + cfg.Cover + spare
			// its cell's corners and those of the cells beside it, where it turns
			x, y := w.xy(c)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if x+dx < 0 || y+dy < 0 || x+dx >= w.cols || y+dy >= w.rows {
						continue
					}
					for _, k := range w.cornersOf(w.cell(x+dx, y+dy)) {
						if s.shaped[k] && w.corners[k] < roof {
							w.corners[k] = roof
						}
					}
				}
			}
		}
	}
}

// open has no street run through a building's cell, which nothing walks: the cell is what the
// land cover draws there, bare earth where it says nothing
func (s *streets) open() {
	w := s.w
	if w.r.Buildings.Cells == "" {
		return
	}
	for _, st := range s.legs {
		for _, c := range st.cells {
			i := w.at(c)
			if w.soils[i] != w.r.Buildings.Cells {
				continue
			}
			w.soils[i] = w.r.Kinds.Earth
			if w.covered != nil && w.covered[i] != "" {
				w.soils[i] = w.covered[i]
			}
		}
	}
	for _, v := range s.spans {
		for _, h := range [2]cell.ID{v.cells[0], v.cells[len(v.cells)-1]} {
			if i := w.at(h); w.soils[i] == w.r.Buildings.Cells {
				w.soils[i] = w.r.Kinds.Earth
			}
		}
	}
}

// dug keeps the tunnels a street runs to at either head whose ground still closes over them
// once the streets are laid, and reports how many it refused for good
func (s *streets) dug() int {
	r := s.w.roads
	r.reached()
	g := boreGround{chartGround: chartGround{r: r}}
	r.tunnels = slices.DeleteFunc(r.tunnels, func(t tunnel) bool {
		g.but, g.under = t.plan.Storey, t.plan.Cells
		_, err := usite.Plan(g, r.tunnelRules(0), r.clear(), t.from, t.to, t.via...)
		if err == nil {
			return false
		}
		s.bared++
		if s.w.refused == nil {
			s.w.refused = map[[2][2]int]bool{}
		}
		s.w.refused[t.ends] = true
		for _, c := range t.plan.Cells {
			b := r.bored[c]
			*b.Storey(t.plan.Storey) = cell.Floor{}
			r.bored[c] = b
		}
		return true
	})
	return s.bared
}

// net is the streets as a network of the chart's cells
func (s *streets) net() *network.Network {
	n := network.New(s.w.grid)
	node := network.Node{Kind: s.w.r.Kinds.Road, Width: float32(s.w.r.Roads.Width)}
	for _, st := range s.legs {
		for i, c := range st.cells {
			if _, ok := n.Node(c); !ok {
				n.Set(c, node)
			}
			if i > 0 {
				n.Link(st.cells[i-1], c)
			}
		}
	}
	return n
}

// world units: a street's lanes and a verge either side
// rim is how far off its middle a roundabout's roadway reaches, metres: its radius is the
// roadway's middle
func (s *streets) rim(c cell.ID) float64 {
	return s.rounds[c] + float64(max(s.lot(c).lanes, 1))*s.w.r.Roads.Lane/2
}

func (s *streets) width(lanes int) float64 {
	return (float64(lanes)*s.w.r.Roads.Lane + 2*s.w.r.Roads.Verge) / s.w.perUnit
}

func (s *streets) write(b *bytes.Buffer) {
	w := s.w
	fmt.Fprintf(b, "  roads:\n    %s:\n      width: %s\n      paths:\n", w.r.Kinds.Road,
		number(s.width(2)))
	for _, st := range s.legs {
		var via []string
		for _, at := range w.turns(st.cells) {
			xy := w.point(st.cells[at])
			if at == 0 && st.first || at == len(st.cells)-1 && st.last {
				xy = "{onto: " + xy + "}"
			}
			via = append(via, xy)
		}
		line := fmt.Sprintf("        - {lanes: %d, ", st.lanes)
		if st.lanes != 2 {
			line += fmt.Sprintf("width: %s, ", number(s.width(st.lanes)))
		}
		if st.oneway {
			line += "oneway: true, "
		}
		writeList(b, line+"via: ", via)
		b.Truncate(b.Len() - 1)
		b.WriteString("}\n")
	}
	// a street runs on into each tunnel from its head
	for _, t := range w.roads.tunnels {
		under := t.plan.Cells
		for _, e := range [2][2]cell.ID{{t.from, under[0]}, {t.to, under[len(under)-1]}} {
			fmt.Fprintf(b, "        - {lanes: %d, via: [%s, {onto: %s}]}\n", max(t.lanes, 1),
				w.point(e[0]), w.point(e[1]))
		}
	}
	var axes, rounds []string
	cells := make([]cell.ID, 0, len(s.lots))
	for c := range s.lots {
		cells = append(cells, c)
	}
	slices.Sort(cells)
	laid := s.w.roads.net
	for _, c := range cells {
		if _, ok := laid.Node(c); !ok {
			continue
		}
		l := s.lots[c]
		x, y := w.xy(c)
		dx := math.Round((l.at[0]-float64(x)-0.5)*w.size*10) / 10
		dy := math.Round((l.at[1]-float64(y)-0.5)*w.size*10) / 10
		if dx != 0 || dy != 0 {
			axes = append(axes, fmt.Sprintf("[%d, %d, %s, %s]", x, y, number(dx), number(dy)))
		}
		if _, ok := s.rounds[c]; ok {
			rounds = append(rounds, fmt.Sprintf("{at: [%d, %d], roundabout: %s, lanes: %d}", x, y,
				number(math.Round(s.rim(c)/w.perUnit*10)/10), max(l.lanes, 1)))
		}
	}
	if len(axes) > 0 {
		writeList(b, "      axes: ", axes)
	}
	if len(rounds) > 0 {
		writeList(b, "      junctions: ", rounds)
	}
}

// rows are the bridges as a level's entities
func (s *streets) rows(b *bytes.Buffer) {
	w := s.w
	for _, v := range s.spans {
		var path []string
		for i, c := range v.cells {
			x, y := w.xy(c)
			path = append(path, fmt.Sprintf("[%d, %d, %s, %s]", x, y,
				number(math.Round((v.at[i][0]-float64(x)-0.5)*w.size*10)/10),
				number(math.Round((v.at[i][1]-float64(y)-0.5)*w.size*10)/10)))
		}
		writeList(b, fmt.Sprintf("    - {kind: %s, lanes: %d, width: %s, path: ", s.kind(v),
			v.lanes, number(s.width(v.lanes))), path)
		b.Truncate(b.Len() - 1)
		b.WriteString("}\n")
	}
}

func (s *streets) summary() string {
	cells := len(s.w.roads.net.Cells())
	r := s.w.roads
	return fmt.Sprintf("streets: %d cells in %d stretches, %d roundabouts, %d bridges (%d a "+
		"head of which would stand far off its ground left out), %d "+
		"tunnels (%d the ground lets none lie in, %d no street reaches, %d a street bares), %d "+
		"cells of water covered where streets meet over it; the steepest climbs %.1f°, the "+
		"deepest cut %.1f m, the "+
		"highest fill %.1f m, %d cells past the recipe's; %d buildings drawn back off a road "+
		"or a bed, %d standing in one left out; %d trees moved off a road, %d left out", cells, len(s.legs), len(s.rounds),
		len(s.spans), s.aloft, len(r.tunnels), r.undug, r.unreached, s.bared, s.covered,
		math.Atan(s.steepest)*180/math.Pi, s.cut, s.fill, s.over, s.narrowed, s.gone,
		s.shifted, s.felled)
}
