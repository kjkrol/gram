package board_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
)

var bridge = cell.Kind{Name: cell.Named("bridge"), Cost: 1, Allows: cell.Land | cell.Air}.Costing(cell.Land, 0.5)

// A crossing lets whoever it admits over the cell at its cost, and leaves the way under it as it
// is: a bridge over a river admits walkers and still the water.
func TestCrossing_OverLetsWhoeverItAdmitsOverTheWay(t *testing.T) {
	under := cell.Way{Kind: river, Width: 8}.Over(earth)
	got := cell.Crossing{Way: cell.Way{Kind: bridge, Width: 6}}.Over(under)
	if !got.Admits(cell.Land) || !got.Admits(cell.Water) || got.Name != earth.Name {
		t.Errorf("a bridge over a river is %+v, want walkers and water both, the ground's name", got)
	}
	if c := got.CostFor(cell.Land); c != 0.5 {
		t.Errorf("a walker pays %v on the bridge, want the bridge's 0.5", c)
	}
	if c := got.CostFor(cell.Water); c != under.CostFor(cell.Water) {
		t.Errorf("the water pays %v under the bridge, want the river's %v", c, under.CostFor(cell.Water))
	}
	if got := (cell.Crossing{}).Over(under); got != under {
		t.Errorf("no crossing over a river is %+v, want the river as it is", got)
	}
}

// Before the ECS and after, the board lays a crossing over its cell's way and counts the change.
func TestBoard_LaysACrossingOverTheWay(t *testing.T) {
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	c, _ := grid.CellIndex(1, 1)
	way := cell.Way{Kind: river, Width: 6, Links: 1<<0 | 1<<1}
	over := cell.Crossing{Way: cell.Way{Kind: bridge, Width: 4, Links: 1<<2 | 1<<3}}

	seeded := board.NewBoard(grid, board.NewTerrainMap())
	seeded.SetAll(earth)
	seeded.SetWay(c, way)
	v := seeded.Version()
	seeded.SetCrossing(c, over)
	if seeded.Version() == v || seeded.Crossing(c) != over || !seeded.Kind(c).Admits(cell.Land) || seeded.Way(c) != way {
		t.Errorf("seed: version %d→%d, crossing %+v, kind %+v; want the change counted and the bridge over the river",
			v, seeded.Version(), seeded.Crossing(c), seeded.Kind(c))
	}

	cw := newCellWorld(t, false)
	brd := cw.board()
	brd.SetAll(earth)
	brd.SetWay(cw.target, way)
	v, cv := brd.Version(), brd.CellVersion(cw.target)
	brd.SetCrossing(cw.target, over)
	if brd.Version() == v || brd.CellVersion(cw.target) == cv || brd.Crossing(cw.target) != over || !brd.Kind(cw.target).Admits(cell.Land|cell.Water) {
		t.Errorf("entities: crossing %+v, kind %+v; want the change counted and the bridge over the river", brd.Crossing(cw.target), brd.Kind(cw.target))
	}
	brd.SetCrossing(cw.target, cell.Crossing{})
	if brd.Kind(cw.target).Admits(cell.Land) {
		t.Errorf("with the bridge taken away walkers may still cross: %+v", brd.Kind(cw.target))
	}
}
