package main

import (
	"math"
	"slices"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// course is a run of OpenStreetMap's tunnels and bridges joined end to end, as one road drives
// them: every cell it passes, where its axis passes each, in cells, how far along each lies,
// metres, and which a tunnel's way passes and which a bridge's
type course struct {
	cells  []cell.ID
	at     [][2]float64
	run    []float64
	bored  []bool
	decked []bool
	way    osmWay // its class, its lanes and the way it is driven
}

// cells between the end of a run of structures and the beginning of the next that make them
// one course
const gap = 4.0

// what a cell of a course is once its grade line is laid over the ground: a street on the
// ground, a deck over it, a tunnel under it
const (
	onGround = iota
	onDeck
	inTunnel
)

// courses joins the tunnels and bridges of ways end to end where two alone meet, whatever
// ground way runs off there: a slip road leaves a viaduct, the road runs on
func (s *streets) courses(ways []osmWay) []course {
	w := s.w
	type end = [2]float64
	at := map[end][]int{}
	var built []osmWay
	for _, way := range ways {
		if !way.tunnel && !way.bridge || way.roundabout {
			continue
		}
		for _, p := range [2]end{way.line[0], way.line[len(way.line)-1]} {
			at[p] = append(at[p], len(built))
		}
		built = append(built, way)
	}
	used := make([]bool, len(built))
	type joined struct {
		line    [][2]float64
		members []osmWay
		whole   osmWay
	}
	var runs []joined
	for i := range built {
		if used[i] {
			continue
		}
		used[i] = true
		line := slices.Clone(built[i].line)
		members := []osmWay{built[i]}
		whole := built[i]
		for _, front := range [2]bool{false, true} {
			for {
				p := line[len(line)-1]
				if front {
					p = line[0]
				}
				m := at[p]
				if len(m) != 2 {
					break
				}
				j := m[0]
				if used[j] {
					j = m[1]
				}
				if used[j] {
					break
				}
				used[j] = true
				more := slices.Clone(built[j].line)
				turned := front && more[len(more)-1] != p || !front && more[0] != p
				if turned {
					slices.Reverse(more)
				}
				if front {
					line = append(more[:len(more)-1], line...)
				} else {
					line = append(line, more[1:]...)
				}
				members = append(members, built[j])
				whole.lanes = max(whole.lanes, built[j].lanes)
				whole.class = min(whole.class, built[j].class)
				whole.oneway = whole.oneway && built[j].oneway && !turned
			}
		}
		runs = append(runs, joined{line: line, members: members, whole: whole})
	}
	// a bridge ends, a few metres of plain road, a tunnel begins: runs whose ends lie within
	// gap cells of each other, each running on the other's way, are one course
	place := func(p [2]float64) [2]float64 {
		x, y := w.place(p[0], p[1])
		return [2]float64{x, y}
	}
	// the way a line runs out of its end: its last point off the one a cell before it
	out := func(line [][2]float64, front bool) (at, dir [2]float64) {
		pts := slices.Clone(line)
		if front {
			slices.Reverse(pts)
		}
		at = place(pts[len(pts)-1])
		for k := len(pts) - 2; k >= 0; k-- {
			p := place(pts[k])
			if d := math.Hypot(at[0]-p[0], at[1]-p[1]); d >= 1 || k == 0 {
				if d > 0 {
					dir = [2]float64{(at[0] - p[0]) / d, (at[1] - p[1]) / d}
				}
				break
			}
		}
		return at, dir
	}
	for merged := true; merged; {
		merged = false
		best, bi, bj := gap, -1, -1
		var bf, bg bool
		for i := range runs {
			for j := i + 1; j < len(runs); j++ {
				for _, f := range [2]bool{false, true} {
					for _, g := range [2]bool{false, true} {
						a, da := out(runs[i].line, f)
						b, db := out(runs[j].line, g)
						d := math.Hypot(b[0]-a[0], b[1]-a[1])
						on := d < 0.5 || ((b[0]-a[0])*da[0]+(b[1]-a[1])*da[1])/d > 0.5
						if d < best && da[0]*db[0]+da[1]*db[1] < -0.5 && on {
							best, bi, bj, bf, bg = d, i, j, f, g
						}
					}
				}
			}
		}
		if bi < 0 {
			break
		}
		a, b := runs[bi], runs[bj]
		if bf {
			slices.Reverse(a.line)
		}
		if !bg {
			slices.Reverse(b.line)
		}
		a.line = append(a.line, b.line...)
		a.members = append(a.members, b.members...)
		a.whole.lanes = max(a.whole.lanes, b.whole.lanes)
		a.whole.class = min(a.whole.class, b.whole.class)
		a.whole.oneway = false
		runs[bi] = a
		runs = slices.Delete(runs, bj, bj+1)
		merged = true
	}
	var courses []course
	for _, run := range runs {
		line, members, whole := run.line, run.members, run.whole
		whole.line, whole.tunnel, whole.bridge = line, false, false
		order, passes := s.passes(whole)
		// a corner clipped on the way from a cell to one beside it is no cell of the course
		for k := 1; k+1 < len(order); {
			if _, ok := grid.Link(w.grid, order[k-1], order[k+1]); ok {
				order = slices.Delete(order, k, k+1)
				continue
			}
			k++
		}
		linked := len(order) >= 3
		for k := 1; k < len(order) && linked; k++ {
			_, linked = grid.Link(w.grid, order[k-1], order[k])
		}
		if !linked {
			continue
		}
		bored, decked := map[cell.ID]bool{}, map[cell.ID]bool{}
		for _, m := range members {
			cells, _ := s.passes(m)
			for _, c := range cells {
				if m.bridge {
					decked[c] = true
				} else {
					bored[c] = true
				}
			}
		}
		co := course{cells: order, way: whole}
		for k, c := range order {
			co.at = append(co.at, passes[c].at)
			co.bored = append(co.bored, bored[c] && !decked[c])
			co.decked = append(co.decked, decked[c])
			run := 0.0
			if k > 0 {
				a, b := co.at[k-1], co.at[k]
				run = co.run[k-1] + math.Hypot(b[0]-a[0], b[1]-a[1])*w.r.Area.Cell
			}
			co.run = append(co.run, run)
		}
		courses = append(courses, co)
	}
	return courses
}

// grade is the course's line: the height at each of its cells, metres. It lies on the ground
// where the road does: at its ends, on its plain cells and where a tunnel gives way to a
// bridge, straight between those. The heights are coarser than a town, a ravine's wall a cell
// or two off in them: a spot the line could reach only steeper than a deck or a tunnel's floor
// climbs is passed over.
func (s *streets) grade(co course) []float64 {
	w := s.w
	last := len(co.cells) - 1
	type knot struct {
		k int
		h float64
	}
	var knots []knot
	for k := range co.cells {
		plain := !co.bored[k] && !co.decked[k]
		turn := k > 0 && co.decked[k] && co.bored[k-1] || k < last && co.decked[k] && co.bored[k+1]
		if k == 0 || k == last || plain || turn {
			knots = append(knots, knot{k, w.groundAt(co.at[k][0], co.at[k][1])})
		}
	}
	// how much further apart two knots lie than the stretch between them may climb
	over := func(a, b knot) float64 {
		run := co.run[b.k] - co.run[a.k]
		if slices.Contains(co.decked[a.k:b.k+1], true) {
			return max(math.Abs(b.h-a.h)-min(w.r.Tunnels.Grade, deckRise)*run, 0)
		}
		// a tunnel counts its run in cells, a slanting step as long as a straight one
		run = min(run, float64(b.k-a.k)*w.r.Area.Cell)
		return max(math.Abs(b.h-a.h)-0.98*w.r.Tunnels.Grade*run, 0)
	}
	for len(knots) > 2 {
		// of the knots a stretch too steep ends at, the one furthest off the line between
		// its neighbours
		worst, at := 0.0, -1
		for i := 1; i < len(knots)-1; i++ {
			a, k, b := knots[i-1], knots[i], knots[i+1]
			if over(a, k)+over(k, b) == 0 {
				continue
			}
			chord := a.h + (b.h-a.h)*(co.run[k.k]-co.run[a.k])/(co.run[b.k]-co.run[a.k])
			if off := math.Abs(k.h - chord); off > worst {
				worst, at = off, i
			}
		}
		if at < 0 {
			break
		}
		knots = slices.Delete(knots, at, at+1)
	}
	// its ends alone left too far apart come half the way each
	for i := 1; i < len(knots); i++ {
		a, b := &knots[i-1], &knots[i]
		if o := over(*a, *b); o > 0 {
			shift := math.Copysign(o/2, b.h-a.h)
			a.h, b.h = a.h+shift, b.h-shift
		}
	}
	line := make([]float64, len(co.cells))
	for i := 1; i < len(knots); i++ {
		a, b := knots[i-1], knots[i]
		for k := a.k; k <= b.k; k++ {
			line[k] = a.h + (b.h-a.h)*(co.run[k]-co.run[a.k])/max(co.run[b.k]-co.run[a.k], 1e-9)
		}
	}
	for i := 0; i <= last; i++ {
		if !co.bored[i] {
			continue
		}
		j := i
		for j < last && co.bored[j+1] {
			j++
		}
		s.sag(co, line, max(i-1, 0), min(j+1, last))
		i = j
	}
	return line
}

// metres a tunnel's way is sunk under its line at most where the ground does not close over
// it, how many cells of tunnel a way has that needs no sinking and a sunk one has at least,
// and the metres of ground asked over a roof beyond its cover
const (
	sagMost  = 3.5
	sagStep  = 0.5
	sagLeast = 6
	spare    = 0.5
)

// sag sinks the line between from and to, a tunnel's way on ground too shallow to close over
// it: a ramp down from either end as steep as a tunnel's floor climbs, level with the line
// between them, as deep as gives the longest tunnel
func (s *streets) sag(co course, line []float64, from, to int) {
	w := s.w
	cfg := w.r.Tunnels
	steep := 0.0
	for k := from + 1; k <= to; k++ {
		steep = max(steep, math.Abs(line[k]-line[k-1])/(co.run[k]-co.run[k-1]))
	}
	ramp := 0.98*cfg.Grade - steep
	if ramp <= 0 {
		return
	}
	// how far under the line a cell lies sunk by depth: all of it between the ramps
	drop := func(k int, depth float64) float64 {
		return min(depth, (co.run[k]-co.run[from])*ramp, (co.run[to]-co.run[k])*ramp)
	}
	// the longest run of cells the ground closes over, each between two cells as deep
	longest := func(depth float64) int {
		best, n := 0, 0
		for k := from + 1; k < to; k++ {
			level := drop(k-1, depth) == depth && drop(k+1, depth) == depth
			cover := w.groundAt(co.at[k][0], co.at[k][1]) - line[k] + depth
			if level && cover >= cfg.Clear+cfg.Cover+spare {
				n++
				best = max(best, n)
				continue
			}
			n = 0
		}
		return best
	}
	if longest(0) >= sagLeast {
		return
	}
	most, depth := sagLeast-1, 0.0
	for d := sagStep; d <= sagMost; d += sagStep {
		if n := longest(d); n > most {
			most, depth = n, d
		}
	}
	for k := from; k <= to; k++ {
		line[k] -= drop(k, depth)
	}
}

// lie is what each cell of the course is under its grade line: a tunnel where the ground
// closes over its roof, a deck where the ground lies a hollow under the
// line or runs wet, a street on the ground between; its ends lie on the ground
func (s *streets) lie(co course, line []float64) []int {
	w := s.w
	cfg := w.r.Tunnels
	last := len(co.cells) - 1
	kinds := make([]int, len(co.cells))
	for k := 1; k < last; k++ {
		d := line[k] - w.groundAt(co.at[k][0], co.at[k][1])
		switch {
		case d <= -(cfg.Clear + cfg.Cover):
			kinds[k] = inTunnel
		case d >= hollow || co.decked[k] && w.wet(co.cells[k]):
			kinds[k] = onDeck
		}
	}
	each := func(kind int, do func(i, j int)) {
		for i := 1; i < last; {
			if kinds[i] != kind {
				i++
				continue
			}
			j := i
			for j+1 < last && kinds[j+1] == kind {
				j++
			}
			do(i, j)
			i = j + 1
		}
	}
	// ground closing over a bridge's way is a tunnel only where a tunnel's way runs on into it
	each(inTunnel, func(i, j int) {
		if !slices.Contains(co.bored[i:j+1], true) {
			for k := i; k <= j; k++ {
				kinds[k] = onGround
			}
		}
	})
	// a deck over a tunnel's way stands only where a bridge's deck runs on into it
	each(onDeck, func(i, j int) {
		whole := true
		for k := i; k <= j; k++ {
			whole = whole && co.bored[k]
		}
		if whole && !(kinds[i-1] == onDeck || kinds[j+1] == onDeck) {
			for k := i; k <= j; k++ {
				kinds[k] = onGround
			}
		}
	})
	// a tunnel leaves two cells of ground before a deck: its head and the deck's
	each(inTunnel, func(i, j int) {
		if kinds[i-1] == onDeck {
			kinds[i], kinds[min(i+1, j)] = onGround, onGround
		}
		if kinds[j+1] == onDeck {
			kinds[j], kinds[max(j-1, i)] = onGround, onGround
		}
	})
	each(inTunnel, func(i, j int) {
		if j-i+1 < max(cfg.Least, 2) {
			for k := i; k <= j; k++ {
				kinds[k] = onGround
			}
		}
	})
	return kinds
}

// structures lays the tunnels and the bridges of ways along their courses' grade lines: the
// tunnels dug, the decks to span, and in place of their ways the stretches of them lying on
// the ground, streets as any
func (s *streets) structures(ways []osmWay) (ground []osmWay, decks []*viaduct) {
	w := s.w
	r := w.roads
	for _, way := range ways {
		if !way.roundabout && !way.tunnel && !way.bridge {
			ground = append(ground, way)
		}
	}
	courses := s.courses(ways)
	// the longest first: a tunnel's twin and a tunnel under another's head yield to it
	slices.SortStableFunc(courses, func(a, b course) int { return len(b.cells) - len(a.cells) })
	// every line over the ground as it lay, before a tunnel's mouth is cut into it
	lines := make([][]float64, len(courses))
	lies := make([][]int, len(courses))
	for i, co := range courses {
		lines[i] = s.grade(co)
		lies[i] = s.lie(co, lines[i])
	}
	for i, co := range courses {
		line, kinds := lines[i], lies[i]
		if !slices.Contains(kinds, onDeck) && !slices.Contains(kinds, inTunnel) {
			ground = append(ground, co.way)
			continue
		}
		last := len(co.cells) - 1
		// the cells of a tunnel or a deck standing as its twin laid already, and the heads
		// either had, each to be joined to the twin's
		gone := make([]bool, len(co.cells))
		type join struct {
			k    int
			a, b cell.ID
		}
		var joins []join
		// each stretch of a kind, from the cell of ground before it to the one after
		stretches := func(kind int, do func(i, j, from, to int)) {
			for i := 0; i <= last; {
				j := i
				for j < last && kinds[j+1] == kinds[i] {
					j++
				}
				if kinds[i] == kind {
					do(i, j, max(i-1, 0), min(j+1, last))
				}
				i = j + 1
			}
		}
		stretches(inTunnel, func(i, j, from, to int) {
			run := make([][2]int, 0, to-from+1)
			for k := from; k <= to; k++ {
				x, y := w.xy(co.cells[k])
				run = append(run, [2]int{x, y})
			}
			ends := [2][2]int{run[0], run[len(run)-1]}
			if twin, ok := r.twinOf(ends); ok {
				joins = append(joins, join{from, co.cells[from], twin[0]},
					join{to, co.cells[to], twin[1]})
				for k := i; k <= j; k++ {
					gone[k] = true
				}
				return
			}
			var t tunnel
			var a, b int
			ok := !w.refused[ends]
			if ok {
				t, a, b, ok = r.dig(run, func(k int) float64 { return line[from+k] })
			}
			if !ok {
				// no tunnel the ground lets lie: the road runs through in a cutting
				r.undug++
				for k := i; k <= j; k++ {
					kinds[k] = onGround
				}
				return
			}
			t.ends, t.lanes = ends, co.way.lanes
			r.tunnels = append(r.tunnels, t)
			s.freed = append(s.freed, run[:a+1]...)
			s.freed = append(s.freed, run[len(run)-1-b:]...)
		})
		stretches(onDeck, func(i, j, from, to int) {
			v := &viaduct{cells: co.cells[from : to+1], lanes: min(co.way.lanes, deckLanes)}
			for k := from; k <= to; k++ {
				x, y := w.xy(co.cells[k])
				at := co.at[k]
				if k > from && k < to {
					at[0] = min(max(at[0], float64(x)+0.5-deckLeeway), float64(x)+0.5+deckLeeway)
					at[1] = min(max(at[1], float64(y)+0.5-deckLeeway), float64(y)+0.5+deckLeeway)
				}
				v.at = append(v.at, at)
			}
			twin := twinDeck(decks, v)
			if twin == nil {
				decks = append(decks, v)
				return
			}
			// no wider than its cell
			twin.lanes = min(twin.lanes+v.lanes, deckLanes)
			// each head to the twin's nearer one: the carriageways are driven opposite ways
			a, b := twin.cells[0], twin.cells[len(twin.cells)-1]
			far := func(p, q cell.ID) int {
				px, py := w.xy(p)
				qx, qy := w.xy(q)
				return max(abs(px-qx), abs(py-qy))
			}
			if far(co.cells[from], a)+far(co.cells[to], b) >
				far(co.cells[from], b)+far(co.cells[to], a) {
				a, b = b, a
			}
			joins = append(joins, join{from, co.cells[from], a}, join{to, co.cells[to], b})
			for k := i; k <= j; k++ {
				gone[k] = true
			}
		})
		// a stretch of ground between two structures gone into their twins is the twin's too
		merged := func(k int) bool {
			a, b := k, k
			for a > 0 && kinds[a-1] == onGround {
				a--
			}
			for b < last && kinds[b+1] == onGround {
				b++
			}
			return a > 0 && b < last && gone[a-1] && gone[b+1]
		}
		for _, jn := range joins {
			if !merged(jn.k) {
				s.freed = append(s.freed, w.between(jn.a, jn.b)...)
			}
		}
		stretches(onGround, func(i, j, _, _ int) {
			if j == i || merged(i) {
				return
			}
			way := co.way
			way.line = nil
			for k := i; k <= j; k++ {
				if k > i {
					// slantwise through the corner the two cells share, a hair inside the
					// second, clipping neither beside it
					ax, ay := w.xy(co.cells[k-1])
					bx, by := w.xy(co.cells[k])
					if ax != bx && ay != by {
						const hair = 1e-3
						lon, lat := w.lonLat(float64(max(ax, bx))+hair*float64(bx-ax),
							float64(max(ay, by))+hair*float64(by-ay))
						way.line = append(way.line, [2]float64{lon, lat})
					}
				}
				lon, lat := w.lonLat(co.at[k][0], co.at[k][1])
				way.line = append(way.line, [2]float64{lon, lat})
			}
			ground = append(ground, way)
		})
	}
	return ground, decks
}

// twinDeck is the deck of decks that v runs beside, over most of the same cells: the other
// carriageway of one road
func twinDeck(decks []*viaduct, v *viaduct) *viaduct {
	for _, o := range decks {
		shared := 0
		for _, c := range v.cells[1 : len(v.cells)-1] {
			if slices.Contains(o.cells, c) {
				shared++
			}
		}
		if 2*shared >= len(v.cells)-2 {
			return o
		}
	}
	return nil
}
