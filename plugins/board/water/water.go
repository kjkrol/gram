package water

import (
	"cmp"
	"container/heap"
	"errors"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/network"
)

// Course is what runs through a cell: nothing, a brook, a stream, a river, a ford across a river,
// or the mouth of a course out at sea, where it runs out into it.
type Course uint8

const (
	Dry Course = iota
	Brook
	Stream
	River
	Ford
	Mouth
)

// Config is how a relief drains: how much gathered water makes a brook, a stream and a river; the
// rain on a cell at a level (nil: 1 a cell); how deep their beds lie; every how many cells from its
// mouth a ford crosses a river (0: none), and the steepest a ford may lie on, rise over run; how
// wide a course is by the square root of the water gathered in it, up to a cell; how far, up or
// down, each cell's level is nudged for the draining alone, so courses wander instead of running
// straight down an even slope; how many cells a course runs on out to sea, by the square root of
// the water gathered in it.
type Config struct {
	BrookAt, StreamAt, RiverAt          float64
	Rain                                func(level float64) float64
	BrookDepth, StreamDepth, RiverDepth float64
	FordEvery                           int
	FordSlope                           float64
	WidthPerRoot                        float64
	Meander                             float64
	Plume                               float64
}

// Network is the brooks, streams and rivers draining a relief to the sea: each wet cell's course,
// where every land cell's water goes and how much has gathered in it. Net is it as a
// network.Network, to lay on a board.
type Network struct {
	Courses  map[cell.ID]Course
	Down     map[cell.ID]cell.ID
	Gathered map[cell.ID]float64

	grid    grid.Grid
	perRoot float64           // WidthPerRoot
	mouths  map[cell.ID]mouth // the cells of the sea a course runs out into

	cw, ch       float64
	width        int64
	height       int64
	wrapX, wrapY bool
	beds         map[[2]int64]float64 // a course's bed at each lattice corner it touches
}

// ErrNotSquare is Drain's answer for a grid other than square: it has no corners to carve.
var ErrNotSquare = errors.New("water: drains a square grid alone")

// Drain works out the network over grid for heights, water leaving where sea says.
func Drain(grid grid.Grid, heights func(geom.Vec) float64, sea func(cell.ID) bool, cfg Config) (*Network, error) {
	var any cell.ID
	grid.EachCell(func(c cell.ID) { any = c })
	if len(grid.CellOutline(any, nil)) != 4 {
		return nil, ErrNotSquare
	}
	cw, ch := grid.CellBounds()
	n := &Network{
		Courses: map[cell.ID]Course{}, Down: map[cell.ID]cell.ID{}, Gathered: map[cell.ID]float64{},
		grid: grid, cw: cw, ch: ch, beds: map[[2]int64]float64{}, perRoot: cfg.WidthPerRoot, mouths: map[cell.ID]mouth{},
	}
	grid.EachCell(func(c cell.ID) {
		x, y, _ := grid.Coords(c)
		n.width, n.height = max(n.width, int64(x)+1), max(n.height, int64(y)+1)
	})
	n.wrapX, n.wrapY = wrapsOf(grid)

	shore := stepsFromSea(grid, sea)
	level := map[cell.ID]float64{}
	grid.EachCell(func(c cell.ID) {
		x, y, _ := grid.Coords(c)
		sum := 0.0
		for _, d := range corners {
			sum += heights(geom.NewVec(float64(int64(x)+d[0])*cw, float64(int64(y)+d[1])*ch))
		}
		calm := min(float64(shore[c])/calmNear, 1) // near the sea a course runs straight for it
		level[c] = sum/4 + cfg.Meander*(nudge(c)-0.5)*calm
	})

	// Flood from the sea up, the lowest first: each cell drains to the one that reached it.
	filled := map[cell.ID]float64{}
	var q queue
	grid.EachCell(func(c cell.ID) {
		if sea(c) {
			filled[c] = level[c]
			heap.Push(&q, flooded{c, level[c], q.seq})
			q.seq++
		}
	})
	var order []cell.ID
	for q.Len() > 0 {
		c := heap.Pop(&q).(flooded)
		for _, m := range grid.Neighbors(c.id) {
			if _, seen := filled[m]; seen {
				continue
			}
			filled[m] = max(level[m], c.at+1)
			n.Down[m] = c.id
			order = append(order, m)
			heap.Push(&q, flooded{m, filled[m], q.seq})
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

	bed := map[cell.ID]float64{}
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
	n.ford(cfg, level, sea)
	n.plume(grid, cfg, sea)

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

// calmNear is how many cells from the sea a course starts to wander: nearer, the Meander eases off
// and it runs straight for the sea instead of along the shore into another.
const calmNear = 4

// stepsFromSea is how many steps between neighbours each cell lies from the sea, 0 the sea's own.
// wrapsOf is the axes g wraps along, by its shape; a grid of another make wraps along none.
func wrapsOf(g grid.Grid) (x, y bool) {
	sh, ok := grid.ShapeOf(g)
	return ok && sh.WrapX, ok && sh.WrapY
}

func stepsFromSea(grid grid.Grid, sea func(cell.ID) bool) map[cell.ID]int {
	steps := map[cell.ID]int{}
	var ring []cell.ID
	grid.EachCell(func(c cell.ID) {
		if sea(c) {
			steps[c], ring = 0, append(ring, c)
		}
	})
	for len(ring) > 0 {
		var next []cell.ID
		for _, c := range ring {
			for _, m := range grid.Neighbors(c) {
				if _, ok := steps[m]; !ok {
					steps[m], next = steps[c]+1, append(next, m)
				}
			}
		}
		ring = next
	}
	return steps
}

// ford lays a ford on every river cell FordEvery cells up from the mouth where the river runs no
// steeper than FordSlope, across the whole river there.
func (n *Network) ford(cfg Config, level map[cell.ID]float64, sea func(cell.ID) bool) {
	if cfg.FordEvery <= 0 {
		return
	}
	mouth := map[cell.ID]int{}
	var from func(c cell.ID) int
	from = func(c cell.ID) int {
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
	var fords []cell.ID
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

// mouth is a cell of the sea a course runs out into: the course's last cell ashore, how many cells
// out it lies and how many the mouth runs.
type mouth struct {
	from       cell.ID
	out, reach int
}

// plume runs every course reaching the sea on out into it, on the way of its last step, over sea
// no other mouth has taken: Plume by the square root of its water cells, each wider and more
// faded than the last.
func (n *Network) plume(grid grid.Grid, cfg Config, sea func(cell.ID) bool) {
	if cfg.Plume <= 0 {
		return
	}
	// the biggest first, so a river's mouth takes the sea before a brook's
	var reaching []cell.ID
	for c, k := range n.Courses {
		if k != Dry && sea(n.Down[c]) {
			reaching = append(reaching, c)
		}
	}
	slices.SortFunc(reaching, func(a, b cell.ID) int {
		if d := cmp.Compare(n.Gathered[b], n.Gathered[a]); d != 0 {
			return d
		}
		return cmp.Compare(a, b)
	})
	for _, c := range reaching {
		way := -1
		for i := range 8 {
			if d, ok := grid.Toward(c, i); ok && d == n.Down[c] {
				way = i
			}
		}
		reach := int(math.Ceil(cfg.Plume * math.Sqrt(n.Gathered[c])))
		prev, at := c, n.Down[c]
		for out := 1; out <= reach && way >= 0; out++ {
			if !sea(at) || n.Courses[at] != Dry {
				break
			}
			n.Courses[at], n.Gathered[at] = Mouth, n.Gathered[c]
			n.mouths[at] = mouth{from: c, out: out, reach: reach}
			if prev != c {
				n.Down[prev] = at
			}
			next, ok := grid.Toward(at, way)
			if !ok {
				break
			}
			prev, at = at, next
		}
	}
}

// Fade is how far c's course has faded out: 0 ashore, and out at sea from a little beyond the mouth
// to nearly all at the last cell a course reaches.
func (n *Network) Fade(c cell.ID) float64 {
	m, ok := n.mouths[c]
	if !ok {
		return 0
	}
	return float64(m.out) / float64(m.reach+1)
}

// Net is the courses as a network.Network over the grid drained, to lay on a board: a node on
// every wet cell of the board kind kinds names for its course, as wide as Width and faded as Fade
// say, each flowing down to where its water goes — the last cell ashore on to the sea.
func (n *Network) Net(kinds map[Course]string) *network.Network {
	net := network.New(n.grid)
	for c, k := range n.Courses {
		if k != Dry {
			net.Set(c, network.Node{Kind: kinds[k], Width: n.Width(c, n.cw), Fade: n.Fade(c)})
		}
	}
	for c, k := range n.Courses {
		if d, ok := n.Down[c]; ok && k != Dry {
			net.Flow(c, d)
		}
	}
	return net
}

// Width is how wide c's course runs, cell wide at most: WidthPerRoot by the square root of the
// water gathered in it, so a brook is a thread and a river fills its cell.
// Out at sea a course widens as it runs out, by more than a half again each cell.
func (n *Network) Width(c cell.ID, cell float64) float64 {
	if n.Courses[c] == Dry {
		return 0
	}
	if m, ok := n.mouths[c]; ok {
		return min(n.Width(m.from, cell)*(1+0.6*float64(m.out)), cell)
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
func nudge(c cell.ID) float64 {
	h := uint64(c)*0x9E3779B97F4A7C15 + 0x632BE59BD9B4E019
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return float64(h>>11) / (1 << 53)
}

// corners are a cell's corners from its top-left, in relief.Relief's order.
var corners = [4][2]int64{{0, 0}, {1, 0}, {0, 1}, {1, 1}}

// flooded is a cell on the flood's queue, at the level it was filled to, seq keeping ties in the
// order they came.
type flooded struct {
	id  cell.ID
	at  float64
	seq int
}

type queue struct {
	cells []flooded
	seq   int
}

func (q *queue) Len() int { return len(q.cells) }
func (q *queue) Less(i, j int) bool {
	a, b := q.cells[i], q.cells[j]
	return a.at < b.at || a.at == b.at && a.seq < b.seq
}
func (q *queue) Swap(i, j int) { q.cells[i], q.cells[j] = q.cells[j], q.cells[i] }
func (q *queue) Push(x any)    { q.cells = append(q.cells, x.(flooded)) }
func (q *queue) Pop() any {
	c := q.cells[len(q.cells)-1]
	q.cells = q.cells[:len(q.cells)-1]
	return c
}
