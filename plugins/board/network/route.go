package network

import (
	"container/heap"
	"math"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Route is the cheapest way over grid from one cell to another, stepping from a cell to a
// neighbour at what cost says — +Inf where the step may not be taken: the cells in order, from
// and to included; false where no way gets there.
func Route(grid board.Grid, from, to cell.ID, cost func(a, b cell.ID) float64) ([]cell.ID, bool) {
	dist := map[cell.ID]float64{from: 0}
	prev := map[cell.ID]cell.ID{}
	q := &frontier{}
	heap.Push(q, step{from, 0})
	for q.Len() > 0 {
		s := heap.Pop(q).(step)
		if s.at > dist[s.cell] {
			continue // reached cheaper since
		}
		if s.cell == to {
			path := []cell.ID{to}
			for c := to; c != from; {
				c = prev[c]
				path = append(path, c)
			}
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
				path[i], path[j] = path[j], path[i]
			}
			return path, true
		}
		for _, m := range grid.Neighbors(s.cell) {
			c := cost(s.cell, m)
			if math.IsInf(c, 1) || math.IsNaN(c) {
				continue
			}
			if d, seen := dist[m]; !seen || s.at+c < d {
				dist[m], prev[m] = s.at+c, s.cell
				heap.Push(q, step{m, s.at + c})
			}
		}
	}
	return nil, false
}

// Path has the network run along cells, each a node as given, each linked to the next.
func (n *Network) Path(cells []cell.ID, node Node) {
	for i, c := range cells {
		n.Set(c, node)
		if i > 0 {
			n.Link(cells[i-1], c)
		}
	}
}

// Across is the network laid on a board over o — a road over a river: a Way of its own on every
// cell o does not run through, and a Crossing of kind, over o's way, on every cell it does.
func (n *Network) Across(o *Network, kind string) (ways, crossings []board.WayEntry) {
	for _, w := range n.Ways() {
		if _, both := o.nodes[w.Cell]; both {
			w.Kind = kind
			crossings = append(crossings, w)
		} else {
			ways = append(ways, w)
		}
	}
	return ways, crossings
}

// step is a cell on Route's frontier and what reaching it has cost.
type step struct {
	cell cell.ID
	at   float64
}

type frontier []step

func (f frontier) Len() int           { return len(f) }
func (f frontier) Less(i, j int) bool { return f[i].at < f[j].at }
func (f frontier) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f *frontier) Push(x any)        { *f = append(*f, x.(step)) }
func (f *frontier) Pop() any {
	s := (*f)[len(*f)-1]
	*f = (*f)[:len(*f)-1]
	return s
}
