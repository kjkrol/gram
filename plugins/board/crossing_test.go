package board_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
)

var bridge = board.CellKind{Name: board.Named("bridge"), Cost: 1, Allows: board.Land | board.Air}.Costing(board.Land, 0.5)

// A crossing lets whoever it admits over the cell at its cost, and leaves the way under it as it
// is: a bridge over a river admits walkers and still the water.
func TestCrossing_OverLetsWhoeverItAdmitsOverTheWay(t *testing.T) {
	under := board.Way{Kind: river, Width: 8}.Over(earth)
	got := board.Crossing{Way: board.Way{Kind: bridge, Width: 6}}.Over(under)
	if !got.Admits(board.Land) || !got.Admits(board.Water) || got.Name != earth.Name {
		t.Errorf("a bridge over a river is %+v, want walkers and water both, the ground's name", got)
	}
	if c := got.CostFor(board.Land); c != 0.5 {
		t.Errorf("a walker pays %v on the bridge, want the bridge's 0.5", c)
	}
	if c := got.CostFor(board.Water); c != under.CostFor(board.Water) {
		t.Errorf("the water pays %v under the bridge, want the river's %v", c, under.CostFor(board.Water))
	}
	if got := (board.Crossing{}).Over(under); got != under {
		t.Errorf("no crossing over a river is %+v, want the river as it is", got)
	}
}

// Before the ECS and after, the board lays a crossing over its cell's way and counts the change.
func TestBoard_LaysACrossingOverTheWay(t *testing.T) {
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	c, _ := grid.CellIndex(1, 1)
	way := board.Way{Kind: river, Width: 6, Links: 1<<0 | 1<<1}
	over := board.Crossing{Way: board.Way{Kind: bridge, Width: 4, Links: 1<<2 | 1<<3}}

	seeded := board.NewBoard(grid, board.NewTerrainMap())
	seeded.SetAll(earth)
	seeded.SetWay(c, way)
	v := seeded.Version()
	seeded.SetCrossing(c, over)
	if seeded.Version() == v || seeded.Crossing(c) != over || !seeded.Kind(c).Admits(board.Land) || seeded.Way(c) != way {
		t.Errorf("seed: version %d→%d, crossing %+v, kind %+v; want the change counted and the bridge over the river",
			v, seeded.Version(), seeded.Crossing(c), seeded.Kind(c))
	}

	cw := newCellWorld(t, false)
	brd := cw.board()
	brd.SetAll(earth)
	brd.SetWay(cw.target, way)
	v, cv := brd.Version(), brd.CellVersion(cw.target)
	brd.SetCrossing(cw.target, over)
	if brd.Version() == v || brd.CellVersion(cw.target) == cv || brd.Crossing(cw.target) != over || !brd.Kind(cw.target).Admits(board.Land|board.Water) {
		t.Errorf("entities: crossing %+v, kind %+v; want the change counted and the bridge over the river", brd.Crossing(cw.target), brd.Kind(cw.target))
	}
	brd.SetCrossing(cw.target, board.Crossing{})
	if brd.Kind(cw.target).Admits(board.Land) {
		t.Errorf("with the bridge taken away walkers may still cross: %+v", brd.Kind(cw.target))
	}
}
