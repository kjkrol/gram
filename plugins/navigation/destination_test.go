package navigation

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/uid"
)

func chebyshev(grid board.Grid, a, b board.CellID) float64 {
	ca, cb := grid.CellCenter(a), grid.CellCenter(b)
	return max(abs(ca.X-cb.X), abs(ca.Y-cb.Y)) / float64(grid.CellSpan())
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func openTerrain() *board.TerrainMap {
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	return terrain
}

func TestBreadthFirst_VisitsRingByRing(t *testing.T) {
	grid := board.DefaultGrids{}.Square(5, 5, legCellSize)
	start, _ := grid.CellIndex(2, 2)
	all := func(board.CellID) bool { return true }

	var order []board.CellID
	record := func(c board.CellID) bool { order = append(order, c); return false }
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
	grid := board.DefaultGrids{}.Square(5, 1, legCellSize)
	start, _ := grid.CellIndex(0, 0)
	wall, _ := grid.CellIndex(1, 0)
	beyond, _ := grid.CellIndex(3, 0)
	notWall := func(c board.CellID) bool { return c != wall }

	if _, ok := breadthFirst(start, grid.Neighbors, notWall, func(c board.CellID) bool { return c == beyond }, 100); ok {
		t.Error("found a cell behind a rejected one")
	}
}

func TestBreadthFirst_RespectsMaxVisited(t *testing.T) {
	grid := board.DefaultGrids{}.Square(5, 1, legCellSize)
	start, _ := grid.CellIndex(0, 0)
	next, _ := grid.CellIndex(1, 0)
	all := func(board.CellID) bool { return true }

	if _, ok := breadthFirst(start, grid.Neighbors, all, func(c board.CellID) bool { return c == next }, 1); ok {
		t.Error("found a cell beyond maxVisited=1")
	}
}

func TestPathFinder_NearestFree_SkipsOccupiedTakenAndUnreachableCells(t *testing.T) {
	grid := board.DefaultGrids{}.Square(7, 1, legCellSize)
	terrain := openTerrain()
	occupancy := &board.SingleOccupancy{}
	pf := newPathFinder(grid, terrain, occupancy)
	at := func(x uint32) board.CellID { c, _ := grid.CellIndex(x, 0); return c }

	const mover, other = uid.UID64(1), uid.UID64(2)
	occupancy.Enter(at(0), mover, board.Land)
	occupancy.Enter(at(4), other, board.Land)
	terrain.Set(at(2), board.CellKind{Cost: 1, Solid: true})
	taken := map[board.CellID]bool{at(5): true}

	dest, _, ok := pf.nearestFree(mover, board.Land, at(6), at(4), taken)
	if !ok || dest != at(6) {
		t.Errorf("nearestFree = %v, %v, want %v (only free, reachable cell near the target)", dest, ok, at(6))
	}

	if _, _, ok := pf.nearestFree(mover, board.Land, at(0), at(4), taken); ok {
		t.Error("expected no result: every free cell near the target is behind the wall")
	}
}

func TestCommandSystem_Update_SpreadsGroupOverDistinctFreeCells(t *testing.T) {
	grid := board.DefaultGrids{}.Square(10, 10, legCellSize)
	occupancy := &board.SingleOccupancy{}
	moves := &control.Queue[MoveTo]{}
	cmds := newMoveCommandSystem(newPathFinder(grid, openTerrain(), occupancy), moves, &control.Queue[LookAt]{}, selTags.Selected)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	target := at(5, 5)
	starts := []board.CellID{at(5, 0), at(5, 4), at(5, 2)}

	var cell goke.Comp[board.Cell]
	var selected goke.Comp[plugin.Tags[selection.Family]]
	var order goke.OptComp[MoveOrder]
	var q *goke.Query
	var nearest uid.UID64

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&cell, &selected)
		f.Create(len(starts))
		f.Next()
		for i := range f.Cursor.IDs {
			selected.Slice(&f.Cursor)[i] = selectedMarks
		}
		for i, id := range f.Cursor.IDs {
			cell.Slice(&f.Cursor)[i] = board.Cell{ID: starts[i]}
			occupancy.Enter(starts[i], id, board.Land)
			if starts[i] == at(5, 4) {
				nearest = id
			}
		}
		q = si.NewQueryBuilder(&cell).Optional(&order).Build()
		cmds.Init(si)
	}})
	cmdHandle := ecs.RegSys(cmds)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(cmdHandle, d)
		ctx.Sync()
	})

	moves.Add(control.Nobody, MoveTo{Cell: target})
	ecs.Tick(time.Second)

	seen := make(map[board.CellID]bool)
	q.All()
	for q.Next() {
		cur := q.Cursor()
		orders := order.Slice(cur)
		if orders == nil {
			t.Fatal("expected every selected entity to get a MoveOrder")
		}
		for i, id := range cur.IDs {
			dest := orders[i].Target
			if seen[dest] {
				t.Errorf("destination %v assigned twice", dest)
			}
			seen[dest] = true
			if id == nearest && dest != target {
				t.Errorf("nearest entity got %v, want the clicked cell %v", dest, target)
			}
		}
	}
	if len(seen) != len(starts) {
		t.Errorf("got %d distinct destinations, want %d", len(seen), len(starts))
	}
}

func TestNavigation_OccupiedTarget_WaitsThenSettlesNextToIt(t *testing.T) {
	grid := board.DefaultGrids{}.Square(5, 5, legCellSize)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	target := at(4, 2)
	lw := newLegWorld(t, 5, 5,
		legUnit{start: at(0, 2), target: target, hasOrder: true},
		legUnit{start: target},
	)

	waitTicks := int(targetWaitTimeout / (time.Second / 60))
	for range waitTicks - 1 {
		if st := lw.tick()[lw.ids[0]]; !st.hasOrder || st.order.Target != target {
			t.Fatalf("before the wait timeout: order = %+v (has %v), want it still aimed at %v", st.order, st.hasOrder, target)
		}
	}

	for range 60 * 10 {
		st := lw.tick()[lw.ids[0]]
		if !st.hasOrder {
			if d := chebyshev(grid, st.cell, target); d != 1 {
				t.Fatalf("settled at %v (%v cells from target), want a neighbor of %v", st.cell, d, target)
			}
			return
		}
	}
	t.Fatal("mover never settled next to the occupied target")
}

func TestNavigation_OccupiedUnreachableTarget_GivesUp(t *testing.T) {
	grid := board.DefaultGrids{}.Square(5, 1, legCellSize)
	at := func(x uint32) board.CellID { c, _ := grid.CellIndex(x, 0); return c }
	lw := newLegWorld(t, 5, 1,
		legUnit{start: at(0), target: at(3), hasOrder: true},
		legUnit{start: at(3)},
	)
	lw.walls(at(1))

	waitTicks := int(targetWaitTimeout / (time.Second / 60))
	var st legState
	for range waitTicks + 5 {
		st = lw.tick()[lw.ids[0]]
	}
	if st.hasOrder {
		t.Fatalf("still has MoveOrder %+v after the wait timeout with no reachable free cell", st.order)
	}
	if st.cell != at(0) {
		t.Errorf("cell = %v, want %v (gave up in place)", st.cell, at(0))
	}
}
