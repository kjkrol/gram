package network

import (
	"slices"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Node is a cell a network runs through: the board kind its way is laid of, named, how wide it
// runs in world units, and how far it has faded out, 0 not at all to 1 gone.
type Node struct {
	Kind  string
	Width float64
	Fade  float64
}

// Network is what runs from cell to cell over a board's grid — a river, a road: the cells it runs
// through and which of their neighbours each runs on to. A link that Flows has a way down: water
// running from cell to cell towards where it leaves the network, the sea.
type Network struct {
	grid  board.Grid
	nodes map[cell.ID]Node
	links map[cell.ID]cell.Links
	down  map[cell.ID]cell.ID
	up    map[cell.ID][]cell.ID
	runs  map[cell.ID][2]int // Along's: how many cells above and below, worked out once
}

// New is an empty network over grid.
func New(grid board.Grid) *Network {
	return &Network{grid: grid, nodes: map[cell.ID]Node{}, links: map[cell.ID]cell.Links{},
		down: map[cell.ID]cell.ID{}, up: map[cell.ID][]cell.ID{}}
}

// Set has the network run through c as node says.
func (n *Network) Set(c cell.ID, node Node) {
	n.nodes[c], n.runs = node, nil
}

// Node is what runs through c; false where nothing does.
func (n *Network) Node(c cell.ID) (Node, bool) {
	node, ok := n.nodes[c]
	return node, ok
}

// Link has a and b, neighbours, run on to each other; false where they are no neighbours.
func (n *Network) Link(a, b cell.ID) bool {
	ab, ok := board.Link(n.grid, a, b)
	ba, back := board.Link(n.grid, b, a)
	if !ok || !back {
		return false
	}
	n.links[a] |= ab
	n.links[b] |= ba
	return true
}

// Flow has from run down on to its neighbour to: linked to it, and to linked back where the
// network runs through it — a river's last cell runs on to the sea it leaves by, which is no
// part of it. False where they are no neighbours.
func (n *Network) Flow(from, to cell.ID) bool {
	l, ok := board.Link(n.grid, from, to)
	if !ok {
		return false
	}
	n.links[from] |= l
	n.down[from] = to
	if _, runs := n.nodes[to]; runs {
		if back, ok := board.Link(n.grid, to, from); ok {
			n.links[to] |= back
		}
		n.up[to] = append(n.up[to], from)
	}
	n.runs = nil
	return true
}

// Links is which of c's neighbours the network runs on to from it; none where it does not run.
func (n *Network) Links(c cell.ID) cell.Links {
	if _, runs := n.nodes[c]; !runs {
		return 0
	}
	return n.links[c]
}

// Down is where c runs down on to, Flow's; false where it flows nowhere.
func (n *Network) Down(c cell.ID) (cell.ID, bool) {
	d, ok := n.down[c]
	return d, ok
}

// Along is how far down its flow c lies: 0 at the head of the longest flow running into it, 1 at
// the last cell before it leaves the network, by the cells above and below it; 0 where the network
// does not run, or does not flow.
func (n *Network) Along(c cell.ID) float64 {
	if _, runs := n.nodes[c]; !runs {
		return 0
	}
	if _, flows := n.down[c]; !flows && len(n.up[c]) == 0 {
		return 0
	}
	r := n.run(c)
	if r[0]+r[1] == 0 {
		return 1
	}
	return float64(r[0]) / float64(r[0]+r[1])
}

// run is how many cells lie above c, along the longest flow into it, and below it on to where it
// leaves the network.
func (n *Network) run(c cell.ID) [2]int {
	if r, ok := n.runs[c]; ok {
		return r
	}
	if n.runs == nil {
		n.runs = map[cell.ID][2]int{}
	}
	var r [2]int
	for _, u := range n.up[c] {
		r[0] = max(r[0], n.run(u)[0]+1)
	}
	for at := c; r[1] < len(n.nodes); r[1]++ { // a flow runs down a tree: it never comes back
		next, ok := n.down[at]
		if _, runs := n.nodes[next]; !ok || !runs {
			break
		}
		at = next
	}
	n.runs[c] = r
	return r
}

// Cells is every cell the network runs through, in order.
func (n *Network) Cells() []cell.ID {
	cells := make([]cell.ID, 0, len(n.nodes))
	for c := range n.nodes {
		cells = append(cells, c)
	}
	slices.Sort(cells)
	return cells
}

// Crossings is every cell both n and o run through: where a road meets a river, a ford or a
// bridge.
func (n *Network) Crossings(o *Network) []cell.ID {
	var out []cell.ID
	for _, c := range n.Cells() {
		if _, both := o.nodes[c]; both {
			out = append(out, c)
		}
	}
	return out
}

// Ways is the network laid on a board: a Way across every cell it runs through, of its node's
// kind, as wide and faded, running on as Links says, its look turned as far down its flow as it
// lies (Along) — a river taking on the sea's where it reaches it.
func (n *Network) Ways() []board.WayEntry {
	cells := n.Cells()
	out := make([]board.WayEntry, 0, len(cells))
	for _, c := range cells {
		node := n.nodes[c]
		out = append(out, board.WayEntry{Kind: node.Kind, Cell: c, Width: float32(node.Width),
			Links: n.Links(c), Fade: float32(node.Fade), Mix: float32(n.Along(c))})
	}
	return out
}
