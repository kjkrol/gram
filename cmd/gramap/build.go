package main

import (
	"fmt"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/water"
)

// chart is a map being made: its cells, the heights of their corners in metres, what lies on them.
type chart struct {
	r          recipe
	flat       bool
	origin     [2]int // how far the area grew west and north for its sea
	cols, rows int
	grid       grid.Grid
	size       float64 // world units a cell
	perUnit    float64 // metres a world unit
	land       []bool
	corners    []float64 // (cols+1) x (rows+1), row by row
	soils      []string
	rivers     *water.Network
	roads      *roads
}

func build(r recipe, height elevation, flat bool) (*chart, error) {
	origin, cells, err := grow(r, height)
	if err != nil {
		return nil, err
	}
	cols, rows := cells[0], cells[1]
	w := &chart{
		r: r, flat: flat, origin: origin, cols: cols, rows: rows,
		grid:    grid.DefaultGrids{}.Square(uint32(cols), uint32(rows), r.Area.Units),
		size:    float64(r.Area.Units),
		perUnit: r.Area.Cell / float64(r.Area.Units),
		land:    make([]bool, cols*rows),
		corners: make([]float64, (cols+1)*(rows+1)),
		soils:   make([]string, cols*rows),
	}
	at := func(x, y float64) (float64, error) { return height(w.lonLat(x, y)) }
	for y := range rows {
		for x := range cols {
			h, err := at(float64(x)+0.5, float64(y)+0.5)
			if err != nil {
				return nil, err
			}
			w.land[y*cols+x] = h > 0
		}
	}
	for y := range rows + 1 {
		for x := range cols + 1 {
			if !w.ashore(x, y) {
				continue
			}
			h, err := at(float64(x), float64(y))
			if err != nil {
				return nil, err
			}
			w.corners[y*(cols+1)+x] = max(h, 0)
		}
	}
	if err := w.drain(); err != nil {
		return nil, err
	}
	w.soil()
	w.roads = newRoads(w)
	if err := w.roads.lay(); err != nil {
		return nil, err
	}
	return w, nil
}

// lonLat is where the point x, y cells into the map lies on the Earth: the recipe's area projected
// round its centre, however far it grew.
func (w *chart) lonLat(x, y float64) (lon, lat float64) {
	return lonLatOf(w.r.Area.Centre, w.east(x), w.south(y))
}

// east and south are how many metres east and south of the recipe's centre x and y cells into the
// map lie.
func (w *chart) east(x float64) float64 {
	return (x - float64(w.origin[0]) - float64(w.r.Area.Cells[0])/2) * w.r.Area.Cell
}

func (w *chart) south(y float64) float64 {
	return (y - float64(w.origin[1]) - float64(w.r.Area.Cells[1])/2) * w.r.Area.Cell
}

// grow is how far the area grows west and north, and its cells grown, so that Area.Sea cells of
// sea lie between its land and every edge, the land read anew over the grown area until it holds;
// the recipe's area as it is with no Sea.
func grow(r recipe, height elevation) (origin, cells [2]int, err error) {
	cells = r.Area.Cells
	sea := r.Area.Sea
	if sea <= 0 {
		return origin, cells, nil
	}
	for range 8 {
		w := &chart{r: r, origin: origin, cols: cells[0], rows: cells[1]}
		x0, y0, x1, y1 := cells[0], cells[1], -1, -1
		for y := range cells[1] {
			for x := range cells[0] {
				h, err := height(w.lonLat(float64(x)+0.5, float64(y)+0.5))
				if err != nil {
					return origin, cells, err
				}
				if h > 0 {
					x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
				}
			}
		}
		if x1 < 0 {
			return origin, cells, nil
		}
		west, north := max(sea-x0, 0), max(sea-y0, 0)
		east, south := max(sea-(cells[0]-1-x1), 0), max(sea-(cells[1]-1-y1), 0)
		if west+north+east+south == 0 {
			return origin, cells, nil
		}
		origin[0], origin[1] = origin[0]+west, origin[1]+north
		cells[0], cells[1] = cells[0]+west+east, cells[1]+north+south
	}
	return origin, cells, fmt.Errorf("the land runs on past the area: no %d cells of sea round it",
		sea)
}

// ashore reports whether the corner (x, y) has land all round it: the coast's corners stay at the
// sea's level, so the shore slopes down into water that lies level.
func (w *chart) ashore(x, y int) bool {
	for _, d := range [4][2]int{{-1, -1}, {0, -1}, {-1, 0}, {0, 0}} {
		cx, cy := x+d[0], y+d[1]
		if cx < 0 || cy < 0 || cx >= w.cols || cy >= w.rows || !w.land[cy*w.cols+cx] {
			return false
		}
	}
	return true
}

func (w *chart) cell(x, y int) cell.ID { return w.grid.At(uint32(x), uint32(y)) }

func (w *chart) xy(c cell.ID) (int, int) {
	x, y, _ := w.grid.Coords(c)
	return int(x), int(y)
}

func (w *chart) isLand(c cell.ID) bool {
	x, y := w.xy(c)
	return w.land[y*w.cols+x]
}

// cornersOf is c's corners: top-left, top-right, bottom-left, bottom-right.
func (w *chart) cornersOf(c cell.ID) [4]int {
	x, y := w.xy(c)
	k := y*(w.cols+1) + x
	return [4]int{k, k + 1, k + w.cols + 1, k + w.cols + 2}
}

// rim is the corners of the lattice direction i leaves c by: two for an edge, one for a corner.
func (w *chart) rim(c cell.ID, i int) []int {
	var buf [4]geom.Vec
	outline := w.grid.CellOutline(c, buf[:0])
	a, b := grid.Rim(w.grid, c, i)
	if a == b {
		return []int{w.corner(outline[a])}
	}
	return []int{w.corner(outline[a]), w.corner(outline[b])}
}

// corner is the lattice's corner at p.
func (w *chart) corner(p geom.Vec) int {
	return int(math.Round(p.Y/w.size))*(w.cols+1) + int(math.Round(p.X/w.size))
}

// outline is c's corners' heights in world units, round it as the grid draws its outline.
func (w *chart) outline(c cell.ID) []float64 {
	var heights []float64
	for _, p := range w.grid.CellOutline(c, nil) {
		heights = append(heights, w.heights(p))
	}
	return heights
}

func (w *chart) level(c cell.ID) float64 {
	sum := 0.0
	for _, k := range w.cornersOf(c) {
		sum += w.corners[k]
	}
	return sum / 4
}

func (w *chart) heights(p geom.Vec) float64 {
	x := min(max(int(math.Round(p.X/w.size)), 0), w.cols)
	y := min(max(int(math.Round(p.Y/w.size)), 0), w.rows)
	return w.corners[y*(w.cols+1)+x] / w.perUnit
}

func (w *chart) drain() error {
	rv := w.r.Rivers
	cellsOf := func(km2 float64) float64 { return km2 * 1e6 / (w.r.Area.Cell * w.r.Area.Cell) }
	var rain func(float64) float64
	if rv.Rain > 0 {
		rain = func(level float64) float64 { return 1 + level*w.perUnit/rv.Rain }
	}
	n, err := water.Drain(w.grid, w.heights, func(c cell.ID) bool { return !w.isLand(c) },
		water.Config{
			BrookAt:      cellsOf(rv.Brook),
			StreamAt:     cellsOf(rv.Stream),
			RiverAt:      cellsOf(rv.River),
			Rain:         rain,
			BrookDepth:   rv.Depth[0] / w.perUnit,
			StreamDepth:  rv.Depth[1] / w.perUnit,
			RiverDepth:   rv.Depth[2] / w.perUnit,
			FordEvery:    rv.Ford.Every,
			FordSlope:    rv.Ford.Slope,
			WidthPerRoot: rv.Width,
			Meander:      rv.Meander / w.perUnit,
		})
	if err != nil {
		return err
	}
	w.rivers = n
	carved := n.Carved(w.heights)
	for y := range w.rows + 1 {
		for x := range w.cols + 1 {
			p := geom.NewVec(float64(x)*w.size, float64(y)*w.size)
			w.corners[y*(w.cols+1)+x] = carved(p) * w.perUnit
		}
	}
	return nil
}

func (w *chart) wet(c cell.ID) bool { return w.rivers.Courses[c] != water.Dry }

// soil lays rock where the ground is steep or high, sand along the low coast, forest at the
// heights the recipe gives, earth elsewhere; the sand and the forest on the share of their
// candidates a noise picks.
func (w *chart) soil() {
	g := w.r.Ground
	inland := w.fromSea()
	type candidate struct {
		c cell.ID
		n float64
	}
	var sands, forests []candidate
	for y := range w.rows {
		for x := range w.cols {
			c := w.cell(x, y)
			if !w.isLand(c) {
				continue
			}
			top, steep := 0.0, 0.0
			ks := w.cornersOf(c)
			for i, a := range ks {
				top = max(top, w.corners[a])
				for _, b := range ks[i+1:] {
					steep = max(steep, math.Abs(w.corners[a]-w.corners[b]))
				}
			}
			steep /= w.r.Area.Cell
			w.soils[y*w.cols+x] = w.r.Kinds.Earth
			fx, fy := float64(x-w.origin[0])+0.5, float64(y-w.origin[1])+0.5
			switch {
			case steep >= g.Rock.Slope || top >= g.Rock.Above:
				w.soils[y*w.cols+x] = w.r.Kinds.Rock
			case float64(inland[c]) <= g.Sand.Within && top < g.Sand.Below:
				sands = append(sands, candidate{c, fbm(fx/6+50, fy/6+20)})
			case w.level(c) >= g.Forest.From && w.level(c) <= g.Forest.To:
				forests = append(forests, candidate{c, fbm(fx/5+31, fy/5+17)})
			}
		}
	}
	pick := func(cs []candidate, share float64, kind string) {
		slices.SortFunc(cs, func(a, b candidate) int { return -cmpFloat(a.n, b.n) })
		for _, cd := range cs[:int(math.Round(share*float64(len(cs))))] {
			x, y := w.xy(cd.c)
			w.soils[y*w.cols+x] = kind
		}
	}
	pick(sands, g.Sand.Share, w.r.Kinds.Sand)
	pick(forests, g.Forest.Share, w.r.Kinds.Forest)
}

func (w *chart) fromSea() map[cell.ID]int {
	steps := map[cell.ID]int{}
	var queue []cell.ID
	w.grid.EachCell(func(c cell.ID) {
		if !w.isLand(c) {
			steps[c] = 0
			queue = append(queue, c)
		}
	})
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		for _, m := range w.grid.Neighbors(c) {
			if _, seen := steps[m]; !seen {
				steps[m] = steps[c] + 1
				queue = append(queue, m)
			}
		}
	}
	return steps
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// fbm is smooth noise in [0, 1): three octaves of value noise.
func fbm(x, y float64) float64 {
	return (valueNoise(x, y) + 0.5*valueNoise(2*x, 2*y) + 0.25*valueNoise(4*x, 4*y)) / 1.75
}

func valueNoise(x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	u, v := x-x0, y-y0
	u, v = u*u*(3-2*u), v*v*(3-2*v)
	ix, iy := int64(x0), int64(y0)
	a, b := lattice(ix, iy), lattice(ix+1, iy)
	c, d := lattice(ix, iy+1), lattice(ix+1, iy+1)
	return (a*(1-u)+b*u)*(1-v) + (c*(1-u)+d*u)*v
}

func lattice(x, y int64) float64 {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F ^ 0x5DEECE66D
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return float64(h>>11) / (1 << 53)
}
