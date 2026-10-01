package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

func chebyshev(grid grid.Grid, a, b cell.ID) float64 {
	ca, cb := grid.CellCenter(a), grid.CellCenter(b)
	return max(abs(ca.X-cb.X), abs(ca.Y-cb.Y)) / float64(grid.CellSpan())
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func openTerrain() *cell.TerrainMap {
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	return terrain
}

func TestCommandSystem_Update_SpreadsGroupOverDistinctFreeCells(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(10, 10, legCellSize)
	occupancy := &cell.SingleOccupancy{}
	moves := &control.Queue[MoveTo]{}
	cmds := newMoveCommandSystem(newPathFinder(grid, openTerrain(), nil, occupancy), moves, &control.Queue[LookAt]{}, selTags.Selected)
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	target := at(5, 5)
	starts := []cell.ID{at(5, 0), at(5, 4), at(5, 2)}

	var here goke.Comp[unit.At]
	var selected goke.Comp[tag.Tags[selection.Family]]
	var order goke.OptComp[MoveOrder]
	var q *goke.Query
	var nearest uid.UID64

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&here, &selected)
		f.Create(len(starts))
		f.Next()
		for i := range f.Cursor.IDs {
			selected.Slice(&f.Cursor)[i] = selectedMarks
		}
		for i, id := range f.Cursor.IDs {
			here.Slice(&f.Cursor)[i] = unit.At{Cell: starts[i]}
			occupancy.Enter(starts[i], id, cell.Land)
			if starts[i] == at(5, 4) {
				nearest = id
			}
		}
		q = si.NewQueryBuilder(&here).Optional(&order).Build()
		cmds.Init(si)
	}})
	cmdHandle := ecs.RegSys(cmds)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(cmdHandle, d)
		ctx.Sync()
	})

	moves.Add(control.Nobody, MoveTo{Cell: target})
	ecs.Tick(time.Second)

	seen := make(map[cell.ID]bool)
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
	grid := grid.DefaultGrids{}.Square(5, 5, legCellSize)
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
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
	grid := grid.DefaultGrids{}.Square(5, 1, legCellSize)
	at := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }
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
