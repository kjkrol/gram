package main

import (
	"math"
	"slices"

	"github.com/kjkrol/gram/plugins/board/cell"
)

// guide is the roads of OpenStreetMap as a chart's planner follows them: the legs between
// their junctions, each a trace of cells, and the cells a bridge may span
type guide struct {
	legs      []leg
	spannable map[cell.ID]bool
}

// a leg of the guide, from a junction to the next: the cells its roads run over in order, and
// the class of the greatest of them
type leg struct {
	from, to cell.ID
	trace    []cell.ID
	class    int
}

// a spur shorter than this many cells is what thinning leaves of a wide road, no road
const spur = 3

// place is where a longitude and latitude lie on the chart, in cells
func (w *chart) place(lon, lat float64) (x, y float64) {
	a := w.r.Area
	mx, my := metresOf(a.Centre, lon, lat)
	return mx/a.Cell + float64(a.Cells[0])/2 + float64(w.origin[0]),
		my/a.Cell + float64(a.Cells[1])/2 + float64(w.origin[1])
}

// box is the chart's bounds: south, west, north, east
func (w *chart) box() [4]float64 {
	west, north := w.lonLat(0, 0)
	east, south := w.lonLat(float64(w.cols), float64(w.rows))
	return [4]float64{south, west, north, east}
}

// trace is the cells of the chart line passes, in order, each beside the one before; a line
// leaving the chart is cut there
func (w *chart) trace(line [][2]float64) [][2]int {
	var out [][2]int
	add := func(x, y float64) {
		c := [2]int{int(math.Floor(x)), int(math.Floor(y))}
		if c[0] < 0 || c[1] < 0 || c[0] >= w.cols || c[1] >= w.rows {
			return
		}
		if len(out) == 0 || out[len(out)-1] != c {
			out = append(out, c)
		}
	}
	for i := 1; i < len(line); i++ {
		ax, ay := w.place(line[i-1][0], line[i-1][1])
		bx, by := w.place(line[i][0], line[i][1])
		n := max(1, int(math.Ceil(4*math.Hypot(bx-ax, by-ay))))
		for k := 0; k <= n; k++ {
			t := float64(k) / float64(n)
			add(ax+(bx-ax)*t, ay+(by-ay)*t)
		}
	}
	return out
}

// unbent is the cells of a trace without those at its corners: each still beside the one
// before, across a corner where the line turns
func unbent(cells [][2]int) [][2]int {
	out := make([][2]int, 0, len(cells))
	for i, c := range cells {
		if i > 0 && i < len(cells)-1 {
			a, b := out[len(out)-1], cells[i+1]
			if abs(b[0]-a[0]) == 1 && abs(b[1]-a[1]) == 1 {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// a set of cells of the chart, row by row
type mask struct {
	cols, rows int
	on         []bool
	// bar says no line runs from the cell a to the cell b beside it, both of the set
	bar func(a, b [2]int) bool
}

func newMask(cols, rows int) *mask {
	return &mask{cols: cols, rows: rows, on: make([]bool, cols*rows)}
}

func (m *mask) has(x, y int) bool {
	return x >= 0 && y >= 0 && x < m.cols && y < m.rows && m.on[y*m.cols+x]
}

func (m *mask) set(x, y int, on bool) { m.on[y*m.cols+x] = on }

// thin leaves of m lines a cell wide running where its shapes ran (Zhang and Suen's thinning),
// the cells of keep staying
func (m *mask) thin(keep *mask) {
	type at struct{ x, y int }
	for changed := true; changed; {
		changed = false
		for pass := range 2 {
			var gone []at
			for y := range m.rows {
				for x := range m.cols {
					if !m.has(x, y) || keep != nil && keep.has(x, y) {
						continue
					}
					// clockwise from the north
					var p [8]bool
					for i, d := range ring {
						p[i] = m.has(x+d[0], y+d[1])
					}
					n, turns := 0, 0
					for i := range 8 {
						if p[i] {
							n++
						}
						if !p[i] && p[(i+1)%8] {
							turns++
						}
					}
					if n < 2 || n > 6 || turns != 1 {
						continue
					}
					if pass == 0 && !(p[0] && p[2] && p[4]) && !(p[2] && p[4] && p[6]) ||
						pass == 1 && !(p[0] && p[2] && p[6]) && !(p[0] && p[4] && p[6]) {
						gone = append(gone, at{x, y})
					}
				}
			}
			for _, g := range gone {
				m.set(g.x, g.y, false)
				changed = true
			}
		}
	}
}

// the cells round one, clockwise from the north
var ring = [8][2]int{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}

// next are the cells of m a line through (x, y) runs on to: those beside it, and those across
// a corner no cell beside both fills
func (m *mask) next(x, y int, dst [][2]int) [][2]int {
	for i, d := range ring {
		if !m.has(x+d[0], y+d[1]) {
			continue
		}
		if m.bar != nil && m.bar([2]int{x, y}, [2]int{x + d[0], y + d[1]}) {
			continue
		}
		if i%2 == 1 {
			a, b := ring[(i+7)%8], ring[(i+1)%8]
			if m.has(x+a[0], y+a[1]) || m.has(x+b[0], y+b[1]) {
				continue
			}
		}
		dst = append(dst, [2]int{x + d[0], y + d[1]})
	}
	return dst
}

// newGuide is the guide of ways over the chart: the ways' cells thinned to lines, cut into legs
// at their junctions, the greatest roads first; the cells of open are roads too, and those of
// keep stay, each a junction
func (w *chart) newGuide(ways []osmWay, open, keep [][2]int) *guide {
	roads, kept := newMask(w.cols, w.rows), newMask(w.cols, w.rows)
	class := make([]int8, w.cols*w.rows)
	for i := range class {
		class[i] = math.MaxInt8
	}
	g := &guide{spannable: map[cell.ID]bool{}}
	// the way driven one way that alone passes a cell, one more than its index, -1 where more
	// ways pass or one driven both ways, and the way it runs there
	solo := make([]int32, w.cols*w.rows)
	heading := make([][2]float64, w.cols*w.rows)
	for i, way := range ways {
		cells := w.trace(way.line)
		// a town's streets alone: a map of roads planned between the guide's legs has its
		// carriageways one road
		lone := way.oneway && !way.roundabout && w.r.Roads.Lay == "profile"
		if lone {
			cells = unbent(cells)
		}
		for k, c := range cells {
			at := c[1]*w.cols + c[0]
			switch {
			case !lone || solo[at] != 0 && solo[at] != int32(i+1):
				solo[at] = -1
			case solo[at] == 0:
				a, b := cells[max(k-1, 0)], cells[min(k+1, len(cells)-1)]
				if l := math.Hypot(float64(b[0]-a[0]), float64(b[1]-a[1])); l > 0 {
					solo[at] = int32(i + 1)
					heading[at] = [2]float64{float64(b[0]-a[0]) / l, float64(b[1]-a[1]) / l}
				}
			}
		}
		for _, c := range cells {
			roads.set(c[0], c[1], true)
			class[c[1]*w.cols+c[0]] = min(class[c[1]*w.cols+c[0]], int8(way.class))
			if way.bridge {
				for _, d := range ring {
					if x, y := c[0]+d[0], c[1]+d[1]; x >= 0 && y >= 0 && x < w.cols && y < w.rows {
						g.spannable[w.cell(x, y)] = true
					}
				}
				g.spannable[w.cell(c[0], c[1])] = true
			}
		}
	}
	for _, c := range open {
		solo[c[1]*w.cols+c[0]] = -1
		roads.set(c[0], c[1], true)
		class[c[1]*w.cols+c[0]] = min(class[c[1]*w.cols+c[0]], 0)
	}
	for _, c := range keep {
		solo[c[1]*w.cols+c[0]] = -1
		roads.set(c[0], c[1], true)
		kept.set(c[0], c[1], true)
	}
	roads.thin(kept)
	// the two carriageways of an avenue are two roads: where their cells come to lie side by
	// side, as two lines slanting across the grid a cell or two apart do at every step, no
	// road runs from one across to the other. Ways that do join them are ways of their own.
	// how many cells round c a way driven against c's passes alone
	against := func(c [2]int) int {
		n, h := 0, heading[c[1]*w.cols+c[0]]
		for _, d := range ring {
			x, y := c[0]+d[0], c[1]+d[1]
			if x < 0 || y < 0 || x >= w.cols || y >= w.rows {
				continue
			}
			i := y*w.cols + x
			if solo[i] > 0 && solo[i] != solo[c[1]*w.cols+c[0]] &&
				heading[i][0]*h[0]+heading[i][1]*h[1] < -0.5 {
				n++
			}
		}
		return n
	}
	// one whose only way on is across stays joined: thinning left it no other
	held := make([]bool, w.cols*w.rows)
	bar := func(a, b [2]int) bool {
		ia, ib := a[1]*w.cols+a[0], b[1]*w.cols+b[0]
		if solo[ia] <= 0 || solo[ib] <= 0 || solo[ia] == solo[ib] || held[ia] || held[ib] {
			return false
		}
		ha, hb := heading[ia], heading[ib]
		if ha[0]*hb[0]+ha[1]*hb[1] > -0.5 {
			return false
		}
		l := math.Hypot(float64(b[0]-a[0]), float64(b[1]-a[1]))
		x, y := float64(b[0]-a[0])/l, float64(b[1]-a[1])/l
		if math.Abs(x*ha[0]+y*ha[1]) >= 0.6 || math.Abs(x*hb[0]+y*hb[1]) >= 0.6 {
			return false
		}
		// carriageways a cell apart all along are one road, as thinning makes them: only
		// those touching in passing are kept apart
		return against(a) <= 1 && against(b) <= 1
	}
	var cut []int
	for y := range w.rows {
		for x := range w.cols {
			if !roads.has(x, y) {
				continue
			}
			roads.bar = nil
			joined := len(roads.next(x, y, nil))
			roads.bar = bar
			if left := len(roads.next(x, y, nil)); left < 2 && joined >= 2 {
				cut = append(cut, y*w.cols+x)
			}
		}
	}
	for _, i := range cut {
		held[i] = true
	}
	roads.bar = bar
	var buf [][2]int
	node := func(x, y int) bool {
		buf = roads.next(x, y, buf[:0])
		return len(buf) != 2 || kept.has(x, y)
	}
	walked := map[[2][2]int]bool{}
	walk := func(from, first [2]int) {
		if walked[[2][2]int{from, first}] {
			return
		}
		l := leg{class: int(class[from[1]*w.cols+from[0]])}
		run := [][2]int{from, first}
		for prev, at := from, first; ; {
			walked[[2][2]int{prev, at}], walked[[2][2]int{at, prev}] = true, true
			if node(at[0], at[1]) || at == from {
				break
			}
			buf = roads.next(at[0], at[1], buf[:0])
			to := buf[0]
			if to == prev {
				to = buf[1]
			}
			prev, at = at, to
			run = append(run, at)
		}
		for _, c := range run {
			l.trace = append(l.trace, w.cell(c[0], c[1]))
			l.class = min(l.class, int(class[c[1]*w.cols+c[0]]))
		}
		l.from, l.to = l.trace[0], l.trace[len(l.trace)-1]
		g.legs = append(g.legs, l)
	}
	for y := range w.rows {
		for x := range w.cols {
			if !roads.has(x, y) || !node(x, y) {
				continue
			}
			for _, n := range roads.next(x, y, nil) {
				walk([2]int{x, y}, n)
			}
		}
	}
	// a ring no junction cuts
	for y := range w.rows {
		for x := range w.cols {
			if !roads.has(x, y) || node(x, y) {
				continue
			}
			if n := roads.next(x, y, nil); !walked[[2][2]int{{x, y}, n[0]}] {
				walk([2]int{x, y}, n[0])
			}
		}
	}
	ends := map[cell.ID]int{}
	for _, l := range g.legs {
		ends[l.from]++
		ends[l.to]++
	}
	g.legs = slices.DeleteFunc(g.legs, func(l leg) bool {
		loose := ends[l.from] == 1 && !kept.on[w.at(l.from)] ||
			ends[l.to] == 1 && !kept.on[w.at(l.to)]
		return loose && len(l.trace) <= spur
	})
	slices.SortStableFunc(g.legs, func(a, b leg) int { return a.class - b.class })
	return g
}

// at is c's place in a mask of the chart
func (w *chart) at(c cell.ID) int {
	x, y := w.xy(c)
	return y*w.cols + x
}
