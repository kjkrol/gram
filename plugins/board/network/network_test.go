package network_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/network"
)

var g = grid.DefaultGrids{}.Square(5, 5, 10)

func at(x, y uint32) cell.ID { c := g.CellIndex(x, y); return c }

const (
	north = cell.Links(1 << 0)
	south = cell.Links(1 << 1)
	west  = cell.Links(1 << 2)
	east  = cell.Links(1 << 3)
)

// A road links its cells both ways, only neighbours, and does not flow.
func TestNetwork_ARoadLinksItsCellsBothWays(t *testing.T) {
	road := network.New(g)
	for x := range uint32(3) {
		road.Set(at(x, 1), network.Node{Kind: "road", Width: 4})
	}
	road.Link(at(0, 1), at(1, 1))
	road.Link(at(1, 1), at(2, 1))
	if road.Link(at(0, 1), at(2, 1)) {
		t.Error("two cells apart linked, want neighbours alone")
	}
	if l := road.Links(at(1, 1)); l != west|east {
		t.Errorf("the middle of the road links %08b, want west and east", l)
	}
	if l := road.Links(at(0, 1)); l != east {
		t.Errorf("the road's end links %08b, want east alone", l)
	}
	if _, flows := road.Down(at(1, 1)); flows || road.Along(at(1, 1)) != 0 {
		t.Error("a road flows")
	}
	if road.Links(at(4, 4)) != 0 {
		t.Error("a cell off the road links on")
	}
}

// Water flows down from cell to cell, its last cell on to where it leaves the network: it links
// there, and nothing links back. Along it the way down grows from 0 at the head of the longest
// flow in to 1 at the last cell.
func TestNetwork_WaterFlowsDownAndLiesAlongIt(t *testing.T) {
	river := network.New(g)
	for y := range uint32(4) {
		river.Set(at(2, y), network.Node{Kind: "river", Width: 2 + float64(y)})
	}
	river.Set(at(1, 1), network.Node{Kind: "brook", Width: 1})
	for y := range uint32(4) {
		river.Flow(at(2, y), at(2, y+1)) // the last on to (2, 4), the sea
	}
	river.Flow(at(1, 1), at(2, 1))
	if l := river.Links(at(2, 1)); l != north|south|west {
		t.Errorf("the confluence links %08b, want up, down and the brook", l)
	}
	if l := river.Links(at(2, 3)); l != north|south {
		t.Errorf("the last cell links %08b, want up and down on to the sea", l)
	}
	if d, ok := river.Down(at(2, 3)); !ok || d != at(2, 4) {
		t.Errorf("the last cell flows to %v, want the sea at %v", d, at(2, 4))
	}
	for y, want := range []float64{0, 1.0 / 3, 2.0 / 3, 1} {
		if a := river.Along(at(2, uint32(y))); a != want {
			t.Errorf("row %d lies %v along, want %v", y, a, want)
		}
	}
	if a := river.Along(at(1, 1)); a != 0 {
		t.Errorf("the brook's head lies %v along, want 0", a)
	}
}

// Laid on a board, a network is a Way across every cell it runs through, of its node's kind, as
// wide, running on as it links, its look turned as far down its flow as it lies.
func TestNetwork_WaysLayItAcrossItsCells(t *testing.T) {
	river := network.New(g)
	river.Set(at(0, 0), network.Node{Kind: "brook", Width: 2})
	river.Set(at(0, 1), network.Node{Kind: "river", Width: 5, Fade: 0.5})
	river.Flow(at(0, 0), at(0, 1))
	river.Flow(at(0, 1), at(0, 2))
	ways := river.Ways()
	if len(ways) != 2 {
		t.Fatalf("%d ways, want one on each cell", len(ways))
	}
	want := []cell.WayEntry{
		{Kind: "brook", Cell: at(0, 0), Width: 2, Links: south, Mix: 0},
		{Kind: "river", Cell: at(0, 1), Width: 5, Links: north | south, Fade: 0.5, Mix: 1},
	}
	for i, w := range ways {
		if w != want[i] {
			t.Errorf("way %d is %+v, want %+v", i, w, want[i])
		}
	}
}

// Two networks cross where both run through a cell: a road over a river.
func TestNetwork_CrossingsAreTheCellsBothRunThrough(t *testing.T) {
	river, road := network.New(g), network.New(g)
	for y := range uint32(5) {
		river.Set(at(2, y), network.Node{Kind: "river", Width: 6})
	}
	for x := range uint32(5) {
		road.Set(at(x, 3), network.Node{Kind: "road", Width: 4})
	}
	if got := road.Crossings(river); len(got) != 1 || got[0] != at(2, 3) {
		t.Errorf("the road crosses the river at %v, want %v", got, at(2, 3))
	}
}

// A route is the cheapest way from cell to cell, round what may not be crossed; none where
// nothing gets there.
func TestRoute_TakesTheCheapestWayRoundWhatMayNotBeCrossed(t *testing.T) {
	wall := map[cell.ID]bool{at(2, 0): true, at(2, 1): true, at(2, 2): true, at(2, 3): true}
	cost := func(a, b cell.ID) float64 {
		if wall[b] {
			return math.Inf(1)
		}
		return 1
	}
	path, ok := network.Route(g, at(0, 0), at(4, 0), cost)
	if !ok || path[0] != at(0, 0) || path[len(path)-1] != at(4, 0) {
		t.Fatalf("route %v, %v; want one from (0, 0) to (4, 0)", path, ok)
	}
	for _, c := range path {
		if wall[c] {
			t.Errorf("the route runs through the wall at %v", c)
		}
	}
	if len(path) != 9 {
		t.Errorf("the route is %d cells, want 9: down round the wall's end and back up", len(path))
	}
	wall[at(2, 4)] = true
	if _, ok := network.Route(g, at(0, 0), at(4, 0), cost); ok {
		t.Error("a route through a wall across the whole g")
	}
}

// A road laid over a river is its own way where the river does not run, and a crossing of the
// kind given where it does: a bridge.
func TestNetwork_ARoadAcrossARiverBridgesIt(t *testing.T) {
	river, road := network.New(g), network.New(g)
	for y := range uint32(5) {
		river.Set(at(2, y), network.Node{Kind: "river", Width: 6})
	}
	road.Path([]cell.ID{at(1, 2), at(2, 2), at(3, 2)}, network.Node{Kind: "road", Width: 4})
	ways, crossings := road.Across(river, "bridge")
	if len(ways) != 2 || len(crossings) != 1 {
		t.Fatalf("%d ways and %d crossings, want the road's 2 ends and a bridge", len(ways), len(crossings))
	}
	if b := crossings[0]; b.Cell != at(2, 2) || b.Kind != "bridge" || b.Links != west|east || b.Width != 4 {
		t.Errorf("the bridge is %+v, want one at (2, 2) running west and east, as wide as the road", b)
	}
}
