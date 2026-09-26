package water

import (
	"container/heap"
	"errors"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
)

// Course is what runs through a cell: nothing, a stream, a river, or a ford across a river.
type Course uint8

const (
	Dry Course = iota
	Stream
	River
	Ford
)

// Config is how a relief drains: how much gathered water makes a stream, a river and a river two
// cells wide; the rain on a cell at a level (nil: 1 a cell); how deep a stream's and a river's
// beds lie; every how many cells from its mouth a ford crosses a river (0: none), and the
// steepest a ford may lie on, rise over run.
type Config struct {
	StreamAt, RiverAt, WideAt float64
	Rain                      func(level float64) float64
	StreamDepth, RiverDepth   float64
	FordEvery                 int
	FordSlope                 float64
}

// Network is the streams and rivers draining a relief to the sea: each wet cell's course, where
// every land cell's water goes and how much has gathered in it.
type Network struct {
	Courses  map[board.CellID]Course
	Down     map[board.CellID]board.CellID
	Gathered map[board.CellID]float64

	cw, ch       float64
	width        int64
	height       int64
	wrapX, wrapY bool
	beds         map[[2]int64]float64          // a course's bed at each lattice corner it touches
	wide         map[board.CellID]board.CellID // the side a river widened to, and its channel
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
		cw: cw, ch: ch, beds: map[[2]int64]float64{}, wide: map[board.CellID]board.CellID{},
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
		level[c] = sum / 4
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
		for _, d := range sides {
			m, ok := n.beside(grid, c.id, d)
			if !ok {
				continue
			}
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
		}
	}
	// A wide river takes the lower of its sides across the current too.
	for _, c := range order {
		if n.Courses[c] != River || n.Gathered[c] < cfg.WideAt {
			continue
		}
		dx, dy := n.step(grid, c, n.Down[c])
		best, found := board.CellID(0), false
		for _, s := range [2][2]int64{{dy, dx}, {-dy, -dx}} {
			m, ok := n.beside(grid, c, s)
			if !ok || sea(m) || n.Courses[m] != Dry {
				continue
			}
			if !found || level[m] < level[best] {
				best, found = m, true
			}
		}
		if found {
			n.Courses[best], n.Down[best], n.Gathered[best], bed[best] = River, c, n.Gathered[c], bed[c]
			n.wide[best] = c
		}
	}
	n.ford(grid, cfg, level, sea)

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
func (n *Network) ford(grid board.Grid, cfg Config, level map[board.CellID]float64, sea func(board.CellID) bool) {
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
		if _, side := n.wide[c]; side || k != River || from(c)%cfg.FordEvery != cfg.FordEvery/2 {
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
	for side, c := range n.wide {
		if n.Courses[c] == Ford {
			n.Courses[side] = Ford // the river's other half, where it is two cells wide
		}
	}
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

// beside is c's neighbour d away, across the seam where the grid wraps; false off the grid.
func (n *Network) beside(grid board.Grid, c board.CellID, d [2]int64) (board.CellID, bool) {
	x, y, _ := grid.Coords(c)
	nx, ny := int64(x)+d[0], int64(y)+d[1]
	if n.wrapX {
		nx = (nx%n.width + n.width) % n.width
	}
	if n.wrapY {
		ny = (ny%n.height + n.height) % n.height
	}
	if nx < 0 || ny < 0 || nx >= n.width || ny >= n.height {
		return 0, false
	}
	return grid.CellIndex(uint32(nx), uint32(ny))
}

// step is the way from a to its side neighbour b.
func (n *Network) step(grid board.Grid, a, b board.CellID) (dx, dy int64) {
	for _, d := range sides {
		if m, ok := n.beside(grid, a, d); ok && m == b {
			return d[0], d[1]
		}
	}
	return 0, 0
}

// corners are a cell's corners from its top-left, in board.Relief's order; sides its four
// neighbours.
var (
	corners = [4][2]int64{{0, 0}, {1, 0}, {0, 1}, {1, 1}}
	sides   = [4][2]int64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
)

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
