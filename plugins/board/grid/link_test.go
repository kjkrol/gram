package grid_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/grid"
)

// Link names the way to each neighbour by one bit, and Toward finds the neighbour back by it.
func TestLink_FindsTheWayToEachNeighbourAndTowardFindsItBack(t *testing.T) {
	for _, g := range []grid.Grid{grid.DefaultGrids{}.Square(4, 4, 10), grid.DefaultGrids{}.Hex(4, 4, 10)} {
		c := g.CellIndex(1, 1)
		for _, n := range g.Neighbors(c) {
			l, ok := grid.Link(g, c, n)
			if !ok || l == 0 || l&(l-1) != 0 {
				t.Fatalf("link from %v to its neighbour %v: %b, %v; want one bit", c, n, l, ok)
			}
			i := 0
			for l>>i != 1 {
				i++
			}
			if back, ok := g.Toward(c, i); !ok || back != n {
				t.Errorf("toward %d from %v is %v, want %v", i, c, back, n)
			}
		}
		far := g.CellIndex(3, 3)
		if _, ok := grid.Link(g, c, far); ok {
			t.Errorf("a link from %v to %v, two cells off", c, far)
		}
	}
	g := grid.DefaultGrids{}.Square(4, 4, 10)
	c := g.CellIndex(1, 1)
	east := g.CellIndex(2, 1)
	south := g.CellIndex(1, 2)
	se := g.CellIndex(2, 2)
	if l, _ := grid.Link(g, c, east); l != 1<<3 {
		t.Errorf("east is bit %b, want %b", l, 1<<3)
	}
	if l, _ := grid.Link(g, c, south); l != 1<<1 {
		t.Errorf("south is bit %b, want %b", l, 1<<1)
	}
	if l, _ := grid.Link(g, c, se); l != 1<<7 {
		t.Errorf("south-east is bit %b, want %b", l, 1<<7)
	}
}
