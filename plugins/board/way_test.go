package board_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
)

var (
	earth = cell.Kind{Name: cell.Named("earth"), Cost: 1, Allows: cell.Land | cell.Air, Veil: 0.2}
	river = cell.Kind{Name: cell.Named("river"), Cost: 1, Allows: cell.Water | cell.Air}
)

// A way decides who may cross its cell and what it costs; the ground keeps the rest.
func TestWay_OverTakesWhoMayCrossAndTheCostFromTheWay(t *testing.T) {
	got := cell.Way{Kind: river, Width: 8}.Over(earth)
	if got.Allows != river.Allows || got.Name != earth.Name || got.Veil != earth.Veil {
		t.Errorf("a river over earth is %+v, want the river's Allows, the rest the earth's", got)
	}
	if got := (cell.Way{Kind: river}).Over(earth); got != earth {
		t.Errorf("a way of no width over earth is %+v, want the earth as it is", got)
	}
	road := cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land, Graded: true}
	if got := (cell.Way{Kind: road, Width: 4}).Over(earth); !got.Graded {
		t.Error("a graded road over earth is not graded")
	}
	if got := (cell.Crossing{Way: cell.Way{Kind: road, Width: 4}}).Over(earth); !got.Graded {
		t.Error("a graded bridge over earth is not graded")
	}
}

// Bare is the ground under whatever runs across a cell.
func TestBoard_BareIsTheGroundUnderTheWay(t *testing.T) {
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	c, _ := grid.CellIndex(1, 1)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(earth)
	brd.SetWay(c, cell.Way{Kind: river, Width: 6})
	if brd.Kind(c).Allows != river.Allows || brd.Bare(c) != earth {
		t.Errorf("under the river the cell is %+v, bare %+v; want the river over the earth, the earth bare", brd.Kind(c), brd.Bare(c))
	}
}

// A step goes along a way where the way, or a crossing, links the two cells either way round.
func TestBoard_AlongFollowsTheLinksOfWaysAndCrossings(t *testing.T) {
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(earth)
	east, _ := board.Link(grid, at(1, 1), at(2, 1))
	brd.SetWay(at(1, 1), cell.Way{Kind: river, Width: 4, Links: east})
	south, _ := board.Link(grid, at(1, 1), at(1, 2))
	brd.SetCrossing(at(1, 1), cell.Crossing{Way: cell.Way{Kind: earth, Width: 4, Links: south}})
	for _, c := range []struct {
		from, to cell.ID
		want     bool
	}{
		{at(1, 1), at(2, 1), true}, {at(2, 1), at(1, 1), true}, // the way, and back along it
		{at(1, 1), at(1, 2), true}, {at(1, 2), at(1, 1), true}, // the crossing
		{at(1, 1), at(0, 1), false}, {at(1, 1), at(2, 2), false}, // no link that way
		{at(0, 0), at(2, 2), false}, // not neighbours
	} {
		if got := brd.Along(c.from, c.to); got != c.want {
			t.Errorf("Along(%v, %v) = %v, want %v", c.from, c.to, got, c.want)
		}
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
	way := cell.Way{Kind: river, Width: 6, Links: 1<<0 | 1<<1}

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
	brd.SetWay(cw.target, cell.Way{})
	if brd.Kind(cw.target) != earth {
		t.Errorf("with the way taken away the cell is %+v, want the earth", brd.Kind(cw.target))
	}
}
