package navigation_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation"
)

func TestPathCells_NoPathYet_StraightToTarget(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	start, _ := grid.CellIndex(0, 0)
	target, _ := grid.CellIndex(4, 0)

	cells := navigation.PathCells(unit.At{Cell: start}, navigation.MoveOrder{Target: target})

	assertCells(t, cells, []cell.ID{start, target})
}

func TestPathCells_PartiallyConsumedPath_SkipsPassedSteps(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	start, _ := grid.CellIndex(0, 0)
	c1, _ := grid.CellIndex(1, 0)
	c2, _ := grid.CellIndex(2, 0)
	target, _ := grid.CellIndex(3, 0)

	var p navigation.Path
	p.Steps[0] = c1
	p.Steps[1] = c2
	p.Steps[2] = target
	p.Length = 3
	p.Index = 1

	cells := navigation.PathCells(unit.At{Cell: start}, navigation.MoveOrder{Target: target, Path: p})

	assertCells(t, cells, []cell.ID{start, c2, target})
}

func TestPathCells_LastCellAlwaysTarget(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	start, _ := grid.CellIndex(0, 0)
	target, _ := grid.CellIndex(2, 0)

	var p navigation.Path
	p.Steps[0] = target
	p.Length = 1
	p.Index = 0

	cells := navigation.PathCells(unit.At{Cell: start}, navigation.MoveOrder{Target: target, Path: p})

	if last := cells[len(cells)-1]; last != target {
		t.Errorf("last cell = %v, want %v (Target)", last, target)
	}
}

func TestPathCells_AtIntermediateWaypoint_DoesNotDuplicateIt(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	mid, _ := grid.CellIndex(1, 0)
	target, _ := grid.CellIndex(2, 0)

	var p navigation.Path
	p.Steps[0] = mid
	p.Steps[1] = target
	p.Length = 2
	p.Index = 0

	cells := navigation.PathCells(unit.At{Cell: mid}, navigation.MoveOrder{Target: target, Path: p})

	assertCells(t, cells, []cell.ID{mid, target})
}

func TestPathCells_AtTarget_DoesNotDuplicateIt(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	target, _ := grid.CellIndex(2, 0)

	cells := navigation.PathCells(unit.At{Cell: target}, navigation.MoveOrder{Target: target})

	assertCells(t, cells, []cell.ID{target})
}

func assertCells(t *testing.T, got, want []cell.ID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("pathCells returned %d cells, want %d: got=%v want=%v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("pathCells[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
