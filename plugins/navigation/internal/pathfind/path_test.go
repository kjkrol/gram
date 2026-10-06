package pathfind

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/uid"
)

func TestPathFinder_FindPath_UnreachableTarget_ReportsNotFound(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occupancy := &cell.SingleOccupancy{}

	from := grid.CellIndex(0, 0)
	to := grid.CellIndex(2, 2)
	for _, n := range grid.Neighbors(to) {
		terrain.Set(n, cell.Kind{Cost: 1, Solid: true})
	}

	_, ok := newPathFinder(grid, terrain, nil, occupancy).findPath(uid.UID64(1), cell.Land, from, to)
	if ok {
		t.Error("expected findPath to report not-found for a target walled in on every side")
	}
}

func TestPathFinder_FindPath_ReusesSolverAcrossCalls(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occupancy := &cell.SingleOccupancy{}
	entity := uid.UID64(1)

	pf := newPathFinder(grid, terrain, nil, occupancy)

	firstFrom := grid.CellIndex(0, 0)
	firstTo := grid.CellIndex(4, 0)
	firstPath, ok := pf.findPath(entity, cell.Land, firstFrom, firstTo)
	if !ok {
		t.Fatal("expected the first query to find a path")
	}
	if firstPath.Length == 0 || firstPath.Steps[firstPath.Length-1] != firstTo {
		t.Errorf("first path = %+v, want it to end at %v", firstPath, firstTo)
	}

	secondFrom := grid.CellIndex(0, 4)
	secondTo := grid.CellIndex(4, 4)
	secondPath, ok := pf.findPath(entity, cell.Land, secondFrom, secondTo)
	if !ok {
		t.Fatal("expected the second query, on the same reused pathFinder, to find a path")
	}
	if secondPath.Length == 0 || secondPath.Steps[secondPath.Length-1] != secondTo {
		t.Errorf("second path = %+v, want it to end at %v", secondPath, secondTo)
	}
}

func TestPathFinder_FindPath_NeverCutsThroughABlockedCorner(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	wall := cell.Kind{Cost: 1, Solid: true}
	for _, y := range []uint32{1, 2, 3, 4} {
		c := grid.CellIndex(2, y)
		terrain.Set(c, wall)
	}
	occupancy := &cell.SingleOccupancy{}

	from := grid.CellIndex(1, 2)
	to := grid.CellIndex(3, 2)
	path, ok := newPathFinder(grid, terrain, nil, occupancy).findPath(uid.UID64(1), cell.Land, from, to)
	if !ok {
		t.Fatal("expected a path around the wall to exist")
	}
	full := append([]cell.ID{from}, path.Steps[:path.Length]...)
	for i := 1; i < len(full); i++ {
		if c1, c2, diag := grid.DiagonalNeighbors(full[i-1], full[i]); diag {
			if !terrain.Kind(c1).Admits(cell.Land) || !terrain.Kind(c2).Admits(cell.Land) {
				t.Errorf("step %d->%d cuts a diagonal through a blocked corner", i-1, i)
			}
		}
	}
}

func TestPathFinder_NearestFree_SkipsOccupiedTakenAndUnreachableCells(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(7, 1, legCellSize)
	terrain := openTerrain()
	occupancy := &cell.SingleOccupancy{}
	pf := newPathFinder(grid, terrain, nil, occupancy)
	at := func(x uint32) cell.ID { c := grid.CellIndex(x, 0); return c }

	const mover, other = uid.UID64(1), uid.UID64(2)
	occupancy.Enter(at(0), mover, cell.Land)
	occupancy.Enter(at(4), other, cell.Land)
	terrain.Set(at(2), cell.Kind{Cost: 1, Solid: true})
	taken := func(c cell.ID) bool { return c == at(5) }

	dest, _, ok := pf.nearestFree(mover, cell.Land, at(6), at(4), taken)
	if !ok || dest != at(6) {
		t.Errorf("nearestFree = %v, %v, want %v (only free, reachable cell near the target)", dest, ok, at(6))
	}

	if _, _, ok := pf.nearestFree(mover, cell.Land, at(0), at(4), taken); ok {
		t.Error("expected no result: every free cell near the target is behind the wall")
	}
}
