package terrain_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

var (
	earth = cell.Kind{Name: cell.Named("earth"), Cost: 1, Allows: cell.Land | cell.Air, Veil: 0.2}
	river = cell.Kind{Name: cell.Named("river"), Cost: 1, Allows: cell.Water | cell.Air}
)
var bridge = cell.Kind{Name: cell.Named("bridge"), Cost: 1, Allows: cell.Land | cell.Air}.Costing(cell.Land, 0.5)

// Bare is the ground under whatever runs across a cell.
func TestBoard_BareIsTheGroundUnderTheWay(t *testing.T) {
	g := grid.DefaultGrids{}.Square(3, 3, 10)
	c, _ := g.CellIndex(1, 1)
	brd := board.NewBoard(g)
	brd.SetAll(earth)
	brd.SetWay(c, cell.Way{Kind: river, Width: 6})
	if brd.Kind(c).Allows != river.Allows || brd.Bare(c) != earth {
		t.Errorf("under the river the cell is %+v, bare %+v; want the river over the earth, the earth bare", brd.Kind(c), brd.Bare(c))
	}
}

// Before the ECS and after, the board lays a way over its cell's ground and counts the change.
func TestBoard_LaysAWayOverTheGround(t *testing.T) {
	g := grid.DefaultGrids{}.Square(3, 3, 10)
	c, _ := g.CellIndex(1, 1)
	way := cell.Way{Kind: river, Width: 6, Links: 1<<0 | 1<<1}

	seeded := board.NewBoard(g)
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

// Before the ECS and after, the board lays a crossing over its cell's way and counts the change.
func TestBoard_LaysACrossingOverTheWay(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(3, 3, 10)
	c, _ := grid.CellIndex(1, 1)
	way := cell.Way{Kind: river, Width: 6, Links: 1<<0 | 1<<1}
	over := cell.Crossing{Way: cell.Way{Kind: bridge, Width: 4, Links: 1<<2 | 1<<3}}

	seeded := board.NewBoard(grid)
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
