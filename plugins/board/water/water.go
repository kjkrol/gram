package water

import (
	"container/heap"
	"errors"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
)

// Course is what runs through a cell: nothing, a brook, a stream, a river, or a ford across a
// river.
type Course uint8

const (
	Dry Course = iota
	Brook
	Stream
	River
	Ford
)

// Config is how a relief drains: how much gathered water makes a brook, a stream and a river; the
// rain on a cell at a level (nil: 1 a cell); how deep their beds lie; every how many cells from its
// mouth a ford crosses a river (0: none), and the steepest a ford may lie on, rise over run; how
// wide a course is by the square root of the water gathered in it, up to a cell; how far, up or
// down, each cell's level is nudged for the draining alone, so courses wander instead of running
// straight down an even slope.
type Config struct {
	BrookAt, StreamAt, RiverAt          float64
	Rain                                func(level float64) float64
	BrookDepth, StreamDepth, RiverDepth float64
	FordEvery                           int
	FordSlope                           float64
	WidthPerRoot                        float64
	Meander                             float64
}

// Network is the brooks, streams and rivers draining a relief to the sea: each wet cell's course,
// where every land cell's water goes and how much has gathered in it.
type Network struct {
	Courses  map[board.CellID]Course
	Down     map[board.CellID]board.CellID
	Gathered map[board.CellID]float64

	up      map[board.CellID][]board.CellID // the cells draining straight into each one
	perRoot float64                         // WidthPerRoot

	cw, ch       float64
	width        int64
	height       int64
	wrapX, wrapY bool
	beds         map[[2]int64]float64 // a course's bed at each lattice corner it touches
}

// ErrNotSquare is Drain's answer for a grid other than square: it has no corners to carve.
var ErrNotSquare = errors.New("water: drains a square grid alone")

// Drain works out the network over grid for heights, water leaving where sea says.
func Drain(grid board.Grid, heights func(geom.Vec) float64, sea func(board.CellID) bool, cfg Config) (*Network, error) {
	var any board.CellID
	grid.EachCell(func(c board.CellID) { any = c })
	if len(grid.CellOutline(any, nil)) != 4 {
		return nil, ErrNotSquare
	}
	cw, ch := grid.CellBounds()
	n := &Network{
		Courses: map[board.CellID]Course{}, Down: map[board.CellID]board.CellID{}, Gathered: map[board.CellID]float64{},
		cw: cw, ch: ch, beds: map[[2]int64]float64{}, up: map[board.CellID][]board.CellID{}, perRoot: cfg.WidthPerRoot,
	}
	grid.EachCell(func(c board.CellID) {
		x, y, _ := grid.Coords(c)
		n.width, n.height = max(n.width, int64(x)+1), max(n.height, int64(y)+1)
	})
	_, n.wrapX = grid.CellIndex(uint32(n.width), 0)
	_, n.wrapY = grid.CellIndex(0, uint32(n.height))

	level := map[board.CellID]float64{}
	grid.EachCell(func(c board.CellID) {
		x, y, _ := grid.Coords(c)
		sum := 0.0
		for _, d := range corners {
			sum += heights(geom.NewVec(float64(int64(x)+d[0])*cw, float64(int64(y)+d[1])*ch))
		}
		level[c] = sum/4 + cfg.Meander*(nudge(c)-0.5)
	})

	// Flood from the sea up, the lowest first: each cell drains to the one that reached it.
	filled := map[board.CellID]float64{}
	var q queue
	grid.EachCell(func(c board.CellID) {
		if sea(c) {
			filled[c] = level[c]
			heap.Push(&q, cell{c, level[c], q.seq})
			q.seq++
		}
	})
	var order []board.CellID
	for q.Len() > 0 {
		c := heap.Pop(&q).(cell)
		for _, m := range grid.Neighbors(c.id) {
			if _, seen := filled[m]; seen {
				continue
			}
			filled[m] = max(level[m], c.at+1)
			n.Down[m] = c.id
			order = append(order, m)
			heap.Push(&q, cell{m, filled[m], q.seq})
			q.seq++
		}
	}

	// The rain gathers downstream, the highest first.
	rain := cfg.Rain
	if rain == nil {
		rain = func(float64) float64 { return 1 }
	}
	for i := len(order) - 1; i >= 0; i-- {
		c := order[i]
		n.Gathered[c] += rain(level[c])
		if d := n.Down[c]; !sea(d) {
			n.Gathered[d] += n.Gathered[c]
		}
	}

	bed := map[board.CellID]float64{}
	for _, c := range order {
		switch g := n.Gathered[c]; {
		case g >= cfg.RiverAt:
			n.Courses[c], bed[c] = River, filled[c]-cfg.RiverDepth
		case g >= cfg.StreamAt:
			n.Courses[c], bed[c] = Stream, filled[c]-cfg.StreamDepth
		case g >= cfg.BrookAt:
			n.Courses[c], bed[c] = Brook, filled[c]-cfg.BrookDepth
		}
	}
	for c, k := range n.Courses {
		if k != Dry {
			d := n.Down[c]
			n.up[d] = append(n.up[d], c)
		}
	}
	n.ford(cfg, level, sea)

	for c, b := range bed {
		x, y, _ := grid.Coords(c)
		for _, d := range corners {
			k := n.key(int64(x)+d[0], int64(y)+d[1])
			if old, ok := n.beds[k]; !ok || b < old {
				n.beds[k] = max(b, 0)
			}
		}
	}
	return n, nil
}

// ford lays a ford on every river cell FordEvery cells up from the mouth where the river runs no
// steeper than FordSlope, across the whole river there.
func (n *Network) ford(cfg Config, level map[board.CellID]float64, sea func(board.CellID) bool) {
	if cfg.FordEvery <= 0 {
		return
	}
	mouth := map[board.CellID]int{}
	var from func(c board.CellID) int
	from = func(c board.CellID) int {
		if v, ok := mouth[c]; ok {
			return v
		}
		d := n.Down[c]
		v := 0
		if !sea(d) && n.Courses[d] != Dry {
			v = from(d) + 1
		}
		mouth[c] = v
		return v
	}
	var fords []board.CellID
	for c, k := range n.Courses {
		if k != River || from(c)%cfg.FordEvery != cfg.FordEvery/2 {
			continue
		}
		if d := n.Down[c]; math.Abs(level[c]-level[d])/n.cw > cfg.FordSlope {
			continue
		}
		fords = append(fords, c)
	}
	for _, c := range fords {
		n.Courses[c] = Ford
	}
}

// Links is which of c's neighbours its course runs on to: down to where its water goes, the sea
// at a mouth included, and up to each course draining into it; none where no course runs.
func (n *Network) Links(grid board.Grid, c board.CellID) board.Links {
	if n.Courses[c] == Dry {
		return 0
	}
	var out board.Links
	if l, ok := board.Link(grid, c, n.Down[c]); ok {
		out |= l
	}
	for _, u := range n.up[c] {
		if l, ok := board.Link(grid, c, u); ok {
			out |= l
		}
	}
	return out
}

// Width is how wide c's course runs, cell wide at most: WidthPerRoot by the square root of the
// water gathered in it, so a brook is a thread and a river fills its cell.
func (n *Network) Width(c board.CellID, cell float64) float64 {
	if n.Courses[c] == Dry {
		return 0
	}
	return min(n.perRoot*math.Sqrt(n.Gathered[c]), cell)
}

// Carved is heights with the channels cut: every corner touching a course at its bed, the corners
// at the sea as they are.
func (n *Network) Carved(heights func(geom.Vec) float64) func(geom.Vec) float64 {
	return func(p geom.Vec) float64 {
		h := heights(p)
		if h <= 0 {
			return h
		}
		gx, gy := p.X/n.cw, p.Y/n.ch
		rx, ry := math.Round(gx), math.Round(gy)
		if math.Abs(gx-rx) > 1e-6 || math.Abs(gy-ry) > 1e-6 {
			return h
		}
		if b, ok := n.beds[n.key(int64(rx), int64(ry))]; ok {
			return b
		}
		return h
	}
}

// key is the lattice corner (x, y), folded where the grid wraps.
func (n *Network) key(x, y int64) [2]int64 {
	if n.wrapX {
		x = (x%n.width + n.width) % n.width
	}
	if n.wrapY {
		y = (y%n.height + n.height) % n.height
	}
	return [2]int64{x, y}
}

// nudge is a fixed number in [0, 1) for c.
func nudge(c board.CellID) float64 {
	h := uint64(c)*0x9E3779B97F4A7C15 + 0x632BE59BD9B4E019
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return float64(h>>11) / (1 << 53)
}

// corners are a cell's corners from its top-left, in board.Relief's order.
var corners = [4][2]int64{{0, 0}, {1, 0}, {0, 1}, {1, 1}}

// cell is a cell on the flood's queue, at the level it was filled to, seq keeping ties in the
// order they came.
type cell struct {
	id  board.CellID
	at  float64
	seq int
}

type queue struct {
	cells []cell
	seq   int
}

func (q *queue) Len() int { return len(q.cells) }
func (q *queue) Less(i, j int) bool {
	a, b := q.cells[i], q.cells[j]
	return a.at < b.at || a.at == b.at && a.seq < b.seq
}
func (q *queue) Swap(i, j int) { q.cells[i], q.cells[j] = q.cells[j], q.cells[i] }
func (q *queue) Push(x any)    { q.cells = append(q.cells, x.(cell)) }
func (q *queue) Pop() any {
	c := q.cells[len(q.cells)-1]
	q.cells = q.cells[:len(q.cells)-1]
	return c
}
