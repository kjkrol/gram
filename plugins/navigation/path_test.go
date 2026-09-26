package navigation

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/uid"
)

func TestPathFinder_FindPath_UnreachableTarget_ReportsNotFound(t *testing.T) {
	grid := board.DefaultGrids{}.Square(5, 5, 10)
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	occupancy := &board.SingleOccupancy{}

	from, _ := grid.CellIndex(0, 0)
	to, _ := grid.CellIndex(2, 2)
	for _, n := range grid.Neighbors(to) {
		terrain.Set(n, board.CellKind{Cost: 1, Solid: true})
	}

	_, ok := newPathFinder(grid, terrain, nil, occupancy).findPath(uid.UID64(1), board.Land, from, to)
	if ok {
		t.Error("expected findPath to report not-found for a target walled in on every side")
	}
}

func TestPathFinder_FindPath_ReusesSolverAcrossCalls(t *testing.T) {
	grid := board.DefaultGrids{}.Square(5, 5, 10)
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	occupancy := &board.SingleOccupancy{}
	entity := uid.UID64(1)

	pf := newPathFinder(grid, terrain, nil, occupancy)

	firstFrom, _ := grid.CellIndex(0, 0)
	firstTo, _ := grid.CellIndex(4, 0)
	firstPath, ok := pf.findPath(entity, board.Land, firstFrom, firstTo)
	if !ok {
		t.Fatal("expected the first query to find a path")
	}
	if firstPath.Length == 0 || firstPath.Steps[firstPath.Length-1] != firstTo {
		t.Errorf("first path = %+v, want it to end at %v", firstPath, firstTo)
	}

	secondFrom, _ := grid.CellIndex(0, 4)
	secondTo, _ := grid.CellIndex(4, 4)
	secondPath, ok := pf.findPath(entity, board.Land, secondFrom, secondTo)
	if !ok {
		t.Fatal("expected the second query, on the same reused pathFinder, to find a path")
	}
	if secondPath.Length == 0 || secondPath.Steps[secondPath.Length-1] != secondTo {
		t.Errorf("second path = %+v, want it to end at %v", secondPath, secondTo)
	}
}

func TestPathFinder_FindPath_NeverCutsThroughABlockedCorner(t *testing.T) {
	grid := board.DefaultGrids{}.Square(5, 5, 10)
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	wall := board.CellKind{Cost: 1, Solid: true}
	for _, y := range []uint32{1, 2, 3, 4} {
		c, _ := grid.CellIndex(2, y)
		terrain.Set(c, wall)
	}
	occupancy := &board.SingleOccupancy{}

	from, _ := grid.CellIndex(1, 2)
	to, _ := grid.CellIndex(3, 2)
	path, ok := newPathFinder(grid, terrain, nil, occupancy).findPath(uid.UID64(1), board.Land, from, to)
	if !ok {
		t.Fatal("expected a path around the wall to exist")
	}
	full := append([]board.CellID{from}, path.Steps[:path.Length]...)
	for i := 1; i < len(full); i++ {
		if c1, c2, diag := grid.DiagonalNeighbors(full[i-1], full[i]); diag {
			if !terrain.Kind(c1).Admits(board.Land) || !terrain.Kind(c2).Admits(board.Land) {
				t.Errorf("step %d->%d cuts a diagonal through a blocked corner", i-1, i)
			}
		}
	}
}
