package pathfind

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/uid"
)

func TestPathFinder_FindPath_UnreachableTarget_ReportsNotFound(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occupancy := &cell.SingleOccupancy{}

	from := grid.At(0, 0)
	to := grid.At(2, 2)
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

	firstFrom := grid.At(0, 0)
	firstTo := grid.At(4, 0)
	firstPath, ok := pf.findPath(entity, cell.Land, firstFrom, firstTo)
	if !ok {
		t.Fatal("expected the first query to find a path")
	}
	if firstPath.Length == 0 || firstPath.Steps[firstPath.Length-1] != firstTo {
		t.Errorf("first path = %+v, want it to end at %v", firstPath, firstTo)
	}

	secondFrom := grid.At(0, 4)
	secondTo := grid.At(4, 4)
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
		c := grid.At(2, y)
		terrain.Set(c, wall)
	}
	occupancy := &cell.SingleOccupancy{}

	from := grid.At(1, 2)
	to := grid.At(3, 2)
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
	at := func(x uint32) cell.ID { c := grid.At(x, 0); return c }

	const mover, other = uid.UID64(1), uid.UID64(2)
	occupancy.Enter(at(0), false, mover, cell.Land)
	occupancy.Enter(at(4), false, other, cell.Land)
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

// a start off the grid has no ordinal: its slot in the solvers' dictionaries is one of its own
func TestPathFinder_Find_StartsOffTheGrid(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(5, 5, 10)
	terrain := openTerrain()
	below, to := cell.ID(5), g.At(0, 0) // below: axial (0, 5), the row past the last
	if _, ok := g.Ordinal(below); ok {
		t.Fatalf("%v lies on the grid", below)
	}
	finders := map[string]*Finder{
		"plain":   New(g, terrain, nil, &cell.MultipleOccupancy{}),
		"leveled": New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain}),
	}
	for name, f := range finders {
		var steps [8]cell.ID
		n, ok := f.Find(uid.UID64(1), cell.Land, Body{}, below, to, Either, steps[:])
		if !ok || n != 5 || steps[n-1] != to || !slices.Contains(g.Neighbors(below), steps[0]) {
			t.Errorf("%s: Find = %v, %v; want 5 steps from a neighbour of %v to %v",
				name, steps[:n], ok, below, to)
		}
	}
}

// one level everywhere
type flat struct{ terrain cell.Terrain }

func (flat) Deck(cell.ID) (top, under float32, carries cell.Domain, ok bool) {
	return 0, 0, 0, false
}
func (flat) Ground(geom.Vec) float32              { return 0 }
func (flat) Over(geom.Vec) (float32, bool)        { return 0, false }
func (f flat) KindAt(c cell.ID, _ bool) cell.Kind { return f.terrain.Kind(c) }

func TestFinder_FindAllocatesNothingOnceItsRoutesFit(t *testing.T) {
	g := grid.DefaultGrids{}.Square(32, 32, 10)
	terrain := openTerrain()
	for y := range uint32(31) {
		terrain.Set(g.At(16, y), cell.Kind{Cost: 1, Solid: true})
	}
	v := newValley()
	for name, tc := range map[string]struct {
		f        *Finder
		body     Body
		from, to cell.ID
	}{
		"plain": {f: New(g, terrain, nil, &cell.MultipleOccupancy{}),
			from: g.At(0, 0), to: g.At(31, 0)},
		"by levels": {f: New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).WithLevels(v),
			body: Body{Feet: 10, Step: 1, Height: 2}, from: v.g.At(0, 2), to: v.g.At(4, 2)},
	} {
		t.Run(name, func(t *testing.T) {
			var dst [64]cell.ID
			find := func() {
				if _, ok := tc.f.Find(1, cell.Land, tc.body, tc.from, tc.to, Either, dst[:]); !ok {
					t.Fatalf("no route from %v to %v", tc.from, tc.to)
				}
			}
			find()
			if allocs := testing.AllocsPerRun(10, find); allocs != 0 {
				t.Errorf("a route found again allocates %v times; want none", allocs)
			}
		})
	}
}
