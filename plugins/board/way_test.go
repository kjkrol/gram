package board_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
)

var (
	earth = board.CellKind{Name: board.Named("earth"), Cost: 1, Allows: board.Land | board.Air, Veil: 0.2}
	river = board.CellKind{Name: board.Named("river"), Cost: 1, Allows: board.Water | board.Air}
)

// A way decides who may cross its cell and what it costs; the ground keeps the rest.
func TestWay_OverTakesWhoMayCrossAndTheCostFromTheWay(t *testing.T) {
	got := board.Way{Kind: river, Width: 8}.Over(earth)
	if got.Allows != river.Allows || got.Name != earth.Name || got.Veil != earth.Veil {
		t.Errorf("a river over earth is %+v, want the river's Allows, the rest the earth's", got)
	}
	if got := (board.Way{Kind: river}).Over(earth); got != earth {
		t.Errorf("a way of no width over earth is %+v, want the earth as it is", got)
	}
}

func TestLink_FindsTheWayToEachNeighbourAndTowardFindsItBack(t *testing.T) {
	for _, grid := range []board.Grid{board.DefaultGrids{}.Square(4, 4, 10), board.DefaultGrids{}.Hex(4, 4, 10)} {
		c, _ := grid.CellIndex(1, 1)
		for _, n := range grid.Neighbors(c) {
			l, ok := board.Link(grid, c, n)
			if !ok || l == 0 || l&(l-1) != 0 {
				t.Fatalf("link from %v to its neighbour %v: %b, %v; want one bit", c, n, l, ok)
			}
			i := 0
			for l>>i != 1 {
				i++
			}
			if back, ok := board.Toward(grid, c, i); !ok || back != n {
				t.Errorf("toward %d from %v is %v, want %v", i, c, back, n)
			}
		}
		far, _ := grid.CellIndex(3, 3)
		if _, ok := board.Link(grid, c, far); ok {
			t.Errorf("a link from %v to %v, two cells off", c, far)
		}
	}
	grid := board.DefaultGrids{}.Square(4, 4, 10)
	c, _ := grid.CellIndex(1, 1)
	east, _ := grid.CellIndex(2, 1)
	south, _ := grid.CellIndex(1, 2)
	se, _ := grid.CellIndex(2, 2)
	if l, _ := board.Link(grid, c, east); l != 1<<3 {
		t.Errorf("east is bit %b, want %b", l, 1<<3)
	}
	if l, _ := board.Link(grid, c, south); l != 1<<1 {
		t.Errorf("south is bit %b, want %b", l, 1<<1)
	}
	if l, _ := board.Link(grid, c, se); l != 1<<7 {
		t.Errorf("south-east is bit %b, want %b", l, 1<<7)
	}
}

// Before the ECS and after, the board lays a way over its cell's ground and counts the change.
func TestBoard_LaysAWayOverTheGround(t *testing.T) {
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	c, _ := grid.CellIndex(1, 1)
	way := board.Way{Kind: river, Width: 6, Links: 1<<0 | 1<<1}

	seeded := board.NewBoard(grid, board.NewTerrainMap())
	seeded.SetAll(earth)
	v := seeded.Version()
	seeded.SetWay(c, way)
	if seeded.Version() == v || seeded.Way(c) != way || seeded.Kind(c).Allows != river.Allows {
		t.Errorf("seed: version %d→%d, way %+v, kind %+v; want the change counted and the river over the earth",
			v, seeded.Version(), seeded.Way(c), seeded.Kind(c))
	}

	cw := newCellWorld(t, false)
	brd := cw.board()
	brd.SetAll(earth)
	v = brd.Version()
	brd.SetWay(cw.target, way)
	if brd.Version() == v || brd.Way(cw.target) != way || brd.Kind(cw.target).Allows != river.Allows {
		t.Errorf("entities: version %d→%d, way %+v, kind %+v; want the change counted and the river over the earth",
			v, brd.Version(), brd.Way(cw.target), brd.Kind(cw.target))
	}
	v = brd.Version()
	brd.SetWay(cw.target, way)
	if brd.Version() != v {
		t.Error("laying the same way again counted as a change")
	}
	brd.SetWay(cw.target, board.Way{})
	if brd.Kind(cw.target) != earth {
		t.Errorf("with the way taken away the cell is %+v, want the earth", brd.Kind(cw.target))
	}
}
