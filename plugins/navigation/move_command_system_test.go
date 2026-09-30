package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

func TestCommandSystem_Update_RetargetsOnlySelectedEntities(t *testing.T) {
	grid := board.DefaultGrids{}.Square(10, 1, 10)
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	occupancy := &board.SingleOccupancy{}

	start, _ := grid.CellIndex(0, 0)
	oldTarget, _ := grid.CellIndex(3, 0)
	newTarget, _ := grid.CellIndex(8, 0)

	moves := &control.Queue[MoveTo]{}
	cmds := newMoveCommandSystem(newPathFinder(grid, terrain, nil, occupancy), moves, &control.Queue[LookAt]{}, selTags.Selected)
	selects := &control.Queue[selection.Select]{}
	selSys := selection.NewSelectionSystem(selects, nil, selTags, nil)

	var cell goke.Comp[board.Cell]
	var pos goke.Comp[world.Base]
	var order goke.Comp[MoveOrder]
	var selectable goke.Comp[plugin.Tags[selection.Family]]
	var readQuery *goke.Query
	var selectedID, otherID uid.UID64

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&cell, &pos, &order, &selectable)
		f.Create(2)
		f.Next()
		for i := range f.Cursor.IDs {
			selectable.Slice(&f.Cursor)[i] = selectableMarks
		}
		ids := f.Cursor.IDs
		selectedID, otherID = ids[0], ids[1]

		cells := cell.Slice(&f.Cursor)
		positions := pos.Slice(&f.Cursor)
		orders := order.Slice(&f.Cursor)
		for i := range ids {
			cells[i] = board.Cell{ID: start}
			positions[i].Pos = world.Position{AABB: board.CellAABB(grid, start, 8)}
			orders[i] = MoveOrder{Target: oldTarget, Path: Path{Length: 1}}
		}

		readQuery = si.NewQueryBuilder(&order).Build()
		selSys.Init(si)
		cmds.Init(si)
	}})

	selHandle := ecs.RegSys(selSys)
	cmdHandle := ecs.RegSys(cmds)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(selHandle, d)
		ctx.Run(cmdHandle, d)
		ctx.Sync()
	})

	selects.Add(control.Nobody, selection.Select{IDs: []uid.UID64{selectedID}})
	ecs.Tick(time.Second)

	moves.Add(control.Nobody, MoveTo{Cell: newTarget})
	ecs.Tick(time.Second)

	got := map[uid.UID64]MoveOrder{}
	readQuery.All()
	for readQuery.Next() {
		cur := readQuery.Cursor()
		orders := order.Slice(cur)
		for i, id := range cur.IDs {
			got[id] = orders[i]
		}
	}

	if got[selectedID].Target != newTarget {
		t.Errorf("selected entity's MoveOrder.Target = %v, want %v", got[selectedID].Target, newTarget)
	}
	if got[selectedID].Path.Length != 8 {
		t.Errorf("selected entity's Path.Length = %d, want 8 (freshly computed by CommandSystem, straight 8-cell line to newTarget)", got[selectedID].Path.Length)
	} else if got[selectedID].Path.Steps[7] != newTarget {
		t.Errorf("selected entity's last path step = %v, want newTarget %v", got[selectedID].Path.Steps[7], newTarget)
	}

	if got[otherID].Target != oldTarget {
		t.Errorf("unselected entity's MoveOrder.Target = %v, want unchanged %v", got[otherID].Target, oldTarget)
	}
	if got[otherID].Path.Length != 1 {
		t.Errorf("unselected entity's Path.Length = %d, want unchanged 1", got[otherID].Path.Length)
	}
}

func TestCommandSystem_Update_AssignsFreshOrderToIdleSelectedEntity(t *testing.T) {
	grid := board.DefaultGrids{}.Square(10, 1, 10)
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	occupancy := &board.SingleOccupancy{}

	start, _ := grid.CellIndex(0, 0)
	newTarget, _ := grid.CellIndex(8, 0)

	moves := &control.Queue[MoveTo]{}
	cmds := newMoveCommandSystem(newPathFinder(grid, terrain, nil, occupancy), moves, &control.Queue[LookAt]{}, selTags.Selected)
	selects := &control.Queue[selection.Select]{}
	selSys := selection.NewSelectionSystem(selects, nil, selTags, nil)

	var cell goke.Comp[board.Cell]
	var pos goke.Comp[world.Base]
	var order goke.OptComp[MoveOrder]
	var readQuery *goke.Query
	var idleID uid.UID64

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var selectable goke.Comp[plugin.Tags[selection.Family]]
		f := si.NewFactory(&cell, &pos, &selectable)
		f.Create(1)
		f.Next()
		selectable.Slice(&f.Cursor)[0] = selectableMarks
		idleID = f.Cursor.IDs[0]

		cells := cell.Slice(&f.Cursor)
		positions := pos.Slice(&f.Cursor)
		cells[0] = board.Cell{ID: start}
		positions[0].Pos = world.Position{AABB: board.CellAABB(grid, start, 8)}

		readQuery = si.NewQueryBuilder().Optional(&order).Build()
		selSys.Init(si)
		cmds.Init(si)
	}})

	selHandle := ecs.RegSys(selSys)
	cmdHandle := ecs.RegSys(cmds)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(selHandle, d)
		ctx.Run(cmdHandle, d)
		ctx.Sync()
	})

	selects.Add(control.Nobody, selection.Select{IDs: []uid.UID64{idleID}})
	ecs.Tick(time.Second)

	readQuery.All()
	for readQuery.Next() {
		cur := readQuery.Cursor()
		if order.Present(cur) {
			t.Fatal("sanity check failed: expected the idle entity to start without MoveOrder")
		}
	}

	moves.Add(control.Nobody, MoveTo{Cell: newTarget})
	ecs.Tick(time.Second)

	var gotMoveTo MoveOrder
	found := false
	readQuery.All()
	for readQuery.Next() {
		cur := readQuery.Cursor()
		if !order.Present(cur) {
			continue
		}
		orders := order.Slice(cur)
		for i, id := range cur.IDs {
			if id == idleID {
				gotMoveTo, found = orders[i], true
			}
		}
	}

	if !found {
		t.Fatal("expected the idle entity to have MoveOrder after a right-click order")
	}
	if gotMoveTo.Target != newTarget {
		t.Errorf("idle entity's MoveOrder.Target = %v, want %v", gotMoveTo.Target, newTarget)
	}
	if gotMoveTo.Path.Length != 8 {
		t.Errorf("idle entity's Path.Length = %d, want 8 (freshly computed by CommandSystem, straight 8-cell line to newTarget)", gotMoveTo.Path.Length)
	} else if gotMoveTo.Path.Steps[7] != newTarget {
		t.Errorf("idle entity's last path step = %v, want newTarget %v", gotMoveTo.Path.Steps[7], newTarget)
	}
}

func TestCommandSystem_Update_UnreachableTargetLeavesInFlightEntityUntouched(t *testing.T) {
	grid := board.DefaultGrids{}.Square(10, 1, 10)
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	occupancy := &board.SingleOccupancy{}

	start, _ := grid.CellIndex(0, 0)
	oldTarget, _ := grid.CellIndex(3, 0)
	wall, _ := grid.CellIndex(8, 0)
	terrain.Set(wall, board.CellKind{Cost: 1, Solid: true})

	moves := &control.Queue[MoveTo]{}
	cmds := newMoveCommandSystem(newPathFinder(grid, terrain, nil, occupancy), moves, &control.Queue[LookAt]{}, selTags.Selected)
	selects := &control.Queue[selection.Select]{}
	selSys := selection.NewSelectionSystem(selects, nil, selTags, nil)

	var cell goke.Comp[board.Cell]
	var pos goke.Comp[world.Base]
	var order goke.Comp[MoveOrder]
	var readQuery *goke.Query
	var movingID uid.UID64

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var selectable goke.Comp[plugin.Tags[selection.Family]]
		f := si.NewFactory(&cell, &pos, &order, &selectable)
		f.Create(1)
		f.Next()
		selectable.Slice(&f.Cursor)[0] = selectableMarks
		movingID = f.Cursor.IDs[0]

		cells := cell.Slice(&f.Cursor)
		positions := pos.Slice(&f.Cursor)
		orders := order.Slice(&f.Cursor)
		cells[0] = board.Cell{ID: start}
		positions[0].Pos = world.Position{AABB: board.CellAABB(grid, start, 8)}
		orders[0] = MoveOrder{Target: oldTarget, Path: Path{Length: 1}}

		readQuery = si.NewQueryBuilder(&order).Build()
		selSys.Init(si)
		cmds.Init(si)
	}})

	selHandle := ecs.RegSys(selSys)
	cmdHandle := ecs.RegSys(cmds)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(selHandle, d)
		ctx.Run(cmdHandle, d)
		ctx.Sync()
	})

	selects.Add(control.Nobody, selection.Select{IDs: []uid.UID64{movingID}})
	ecs.Tick(time.Second)

	moves.Add(control.Nobody, MoveTo{Cell: wall})
	ecs.Tick(time.Second)

	readQuery.All()
	found := false
	for readQuery.Next() {
		cur := readQuery.Cursor()
		orders := order.Slice(cur)
		for i, id := range cur.IDs {
			if id != movingID {
				continue
			}
			found = true
			if orders[i].Target != oldTarget {
				t.Errorf("MoveOrder.Target = %v, want unchanged %v (order to a wall must be dropped)", orders[i].Target, oldTarget)
			}
			if orders[i].Path.Length != 1 {
				t.Errorf("Path.Length = %d, want unchanged 1 (order to a wall must be dropped)", orders[i].Path.Length)
			}
		}
	}
	if !found {
		t.Fatal("expected the moving entity to still have MoveOrder")
	}
}
