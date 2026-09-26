package navigation

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// commandWorld is a 10x1 board with a Selected unit in flight and a Selected idle unit, driven
// by the command system alone.
type commandWorld struct {
	grid  board.Grid
	moves *control.Queue[MoveTo]
	ecs   *goke.ECS
	order goke.OptComp[MoveOrder]
	q     *goke.Query

	moving, idle uid.UID64
	oldTarget    board.CellID
}

func newCommandWorld(t *testing.T) *commandWorld {
	t.Helper()
	cw := &commandWorld{grid: board.DefaultGrids{}.Square(10, 1, 10), moves: &control.Queue[MoveTo]{}}
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	cmds := newMoveCommandSystem(newPathFinder(cw.grid, terrain, nil, &board.SingleOccupancy{}), cw.moves, &control.Queue[LookAt]{}, selTags.Selected)
	cw.oldTarget = cw.cellAt(3)

	cw.ecs = goke.New()
	cw.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var cell goke.Comp[board.Cell]
		var pos goke.Comp[world.Base]
		var sel goke.Comp[plugin.Tags[selection.Family]]
		var order goke.Comp[MoveOrder]

		f := si.NewFactory(&cell, &pos, &sel, &order)
		f.Create(1)
		f.Next()
		sel.Slice(&f.Cursor)[0] = selectedMarks
		cw.moving = f.Cursor.IDs[0]
		cell.Slice(&f.Cursor)[0] = board.Cell{ID: cw.cellAt(0)}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: board.CellAABB(cw.grid, cw.cellAt(0), 8)}
		order.Slice(&f.Cursor)[0] = MoveOrder{Target: cw.oldTarget, Path: Path{Length: 1}}

		g := si.NewFactory(&cell, &pos, &sel)
		g.Create(1)
		g.Next()
		sel.Slice(&g.Cursor)[0] = selectedMarks
		cw.idle = g.Cursor.IDs[0]
		cell.Slice(&g.Cursor)[0] = board.Cell{ID: cw.cellAt(5)}
		pos.Slice(&g.Cursor)[0].Pos = world.Position{AABB: board.CellAABB(cw.grid, cw.cellAt(5), 8)}

		cw.q = si.NewQueryBuilder(&cell).Optional(&cw.order).Build()
		cmds.Init(si)
	}})
	h := cw.ecs.RegSys(cmds)
	cw.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(h, d); ctx.Sync() })
	return cw
}

func (cw *commandWorld) cellAt(x uint32) board.CellID { c, _ := cw.grid.CellIndex(x, 0); return c }

// issue runs one command through a tick.
func (cw *commandWorld) issue(cell board.CellID, appendIt bool) {
	cw.moves.Add(control.Nobody, MoveTo{Cell: cell, Append: appendIt})
	cw.ecs.Tick(time.Second)
}

func (cw *commandWorld) orders() map[uid.UID64]*MoveOrder {
	out := map[uid.UID64]*MoveOrder{}
	cw.q.All()
	for cw.q.Next() {
		cur := cw.q.Cursor()
		orders := cw.order.Slice(cur)
		for i, id := range cur.IDs {
			if orders != nil {
				o := orders[i]
				out[id] = &o
			}
		}
	}
	return out
}

func TestCommandSystem_Update_ShiftAppendsAWaypointToInFlightOrders(t *testing.T) {
	cw := newCommandWorld(t)
	next := cw.cellAt(8)

	cw.issue(next, true)

	got := cw.orders()[cw.moving]
	if got == nil || got.Target != cw.oldTarget {
		t.Fatalf("in-flight order = %+v, want its Target %v kept", got, cw.oldTarget)
	}
	if got.Queued != 1 || got.Waypoints[0] != next {
		t.Errorf("queue = %v (%d), want [%v]", got.Waypoints[:got.Queued], got.Queued, next)
	}
	if got.Path.Length != 1 {
		t.Errorf("Path.Length = %d, want the in-flight path left alone", got.Path.Length)
	}
}

func TestCommandSystem_Update_ShiftGivesAnIdleSelectedEntityAFreshOrder(t *testing.T) {
	cw := newCommandWorld(t)
	next := cw.cellAt(8)

	cw.issue(next, true)

	got := cw.orders()[cw.idle]
	if got == nil || got.Target != next || got.Queued != 0 {
		t.Fatalf("idle entity's order = %+v, want a fresh one to %v with nothing queued", got, next)
	}
}

func TestCommandSystem_Update_AFullQueueIgnoresAnotherWaypoint(t *testing.T) {
	cw := newCommandWorld(t)
	for i := range MaxWaypoints {
		cw.issue(cw.cellAt(uint32(i)), true)
	}
	cw.issue(cw.cellAt(9), true)

	got := cw.orders()[cw.moving]
	if int(got.Queued) != MaxWaypoints {
		t.Fatalf("Queued = %d, want the queue full at %d", got.Queued, MaxWaypoints)
	}
	if got.Waypoints[MaxWaypoints-1] != cw.cellAt(MaxWaypoints-1) {
		t.Errorf("last queued = %v, want the eighth goal kept and the ninth dropped", got.Waypoints[MaxWaypoints-1])
	}
}

func TestCommandSystem_Update_APlainClickReplacesTheQueue(t *testing.T) {
	cw := newCommandWorld(t)
	cw.issue(cw.cellAt(8), true)
	cw.issue(cw.cellAt(6), false)

	got := cw.orders()[cw.moving]
	if got.Queued != 0 {
		t.Errorf("queue = %v, want it dropped by a plain click", got.Waypoints[:got.Queued])
	}
	if got.Target == cw.oldTarget {
		t.Errorf("Target = %v, want it replaced by a cell at or beside %v", got.Target, cw.cellAt(6))
	}
}
