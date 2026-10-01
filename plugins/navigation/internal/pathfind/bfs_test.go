package pathfind

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

func TestBreadthFirst_VisitsRingByRing(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, legCellSize)
	start, _ := grid.CellIndex(2, 2)
	all := func(cell.ID) bool { return true }

	var order []cell.ID
	record := func(c cell.ID) bool { order = append(order, c); return false }
	if _, ok := breadthFirst(start, grid.Neighbors, all, record, 100); ok {
		t.Fatal("match never accepts, expected no result")
	}
	if len(order) != 25 || order[0] != start {
		t.Fatalf("visited %d cells starting at %v, want all 25 starting at %v", len(order), order[0], start)
	}
	for i := 1; i < len(order); i++ {
		if chebyshev(grid, start, order[i]) < chebyshev(grid, start, order[i-1]) {
			t.Fatalf("cell %v (ring %v) visited after a farther ring", order[i], chebyshev(grid, start, order[i]))
		}
	}
}

func TestBreadthFirst_DoesNotCrossRejectedCells(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, legCellSize)
	start, _ := grid.CellIndex(0, 0)
	wall, _ := grid.CellIndex(1, 0)
	beyond, _ := grid.CellIndex(3, 0)
	notWall := func(c cell.ID) bool { return c != wall }

	if _, ok := breadthFirst(start, grid.Neighbors, notWall, func(c cell.ID) bool { return c == beyond }, 100); ok {
		t.Error("found a cell behind a rejected one")
	}
}

func TestBreadthFirst_RespectsMaxVisited(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, legCellSize)
	start, _ := grid.CellIndex(0, 0)
	next, _ := grid.CellIndex(1, 0)
	all := func(cell.ID) bool { return true }

	if _, ok := breadthFirst(start, grid.Neighbors, all, func(c cell.ID) bool { return c == next }, 1); ok {
		t.Error("found a cell beyond maxVisited=1")
	}
}
