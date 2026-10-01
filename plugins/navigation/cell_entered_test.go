package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// enteredWorld is a one-entity navigation ECS that also observes its Entered marker.
type enteredWorld struct {
	ecs  *goke.ECS
	id   uid.UID64
	grid board.Grid

	// per-tick observations, refreshed by the observer system
	entered  map[uid.UID64]board.CellID
	hasOrder map[uid.UID64]bool
}

// newEnteredWorld makes the world; with marks the entity carries its markers from the start, else it
// gets them at the first cell it enters.
func newEnteredWorld(t *testing.T, w, h uint32, start, target board.CellID, marks bool) *enteredWorld {
	t.Helper()
	ew := &enteredWorld{
		grid:     board.DefaultGrids{}.Square(w, h, legCellSize),
		entered:  map[uid.UID64]board.CellID{},
		hasOrder: map[uid.UID64]bool{},
	}
	occupancy := &board.SingleOccupancy{}
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})

	steer := newNavigationSystem(
		newPathFinder(ew.grid, terrain, nil, occupancy), ew.grid, terrain, occupancy)
	space := testSpace(t)
	steer.BindSpace(space)

	var statesComp goke.OptComp[tag.Tags[States]]
	var orderComp goke.OptComp[MoveOrder]
	var cellComp goke.Comp[board.At]
	var enteredQ, orderQ *goke.Query

	ew.ecs = goke.New()
	ew.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var cell goke.Comp[board.At]
		var pos goke.Comp[world.Base]
		var order goke.Comp[MoveOrder]
		var profile goke.Comp[steering.Steering]

		var states goke.Comp[tag.Tags[States]]
		f := si.NewFactory(&cell, &pos, &order, &profile)
		if marks {
			f = si.NewFactory(&cell, &pos, &order, &profile, &states)
		}
		f.Create(1)
		f.Next()
		ew.id = f.Cursor.IDs[0]
		p := world.Position{AABB: board.CellAABB(ew.grid, start, legEntitySize)}
		cell.Slice(&f.Cursor)[0] = board.At{Cell: start}
		pos.Slice(&f.Cursor)[0].Pos = p
		order.Slice(&f.Cursor)[0] = MoveOrder{Target: target}
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: float64(legCellSize * 2)}
		occupancy.Enter(start, ew.id, board.Land)

		enteredQ = si.NewQueryBuilder(&cellComp).Optional(&statesComp).Build()
		orderQ = si.NewQueryBuilder(&cellComp).Optional(&orderComp).Build()
	}})

	steerHandle := ew.ecs.RegSys(steer)
	steeringHandle := ew.ecs.RegSys(steering.NewSystem())
	moveHandle := ew.ecs.RegSys(world.NewMoveSystem(space))
	observer := ew.ecs.RegSys(goke.SystemFn{OnUpdate: func(*goke.CmdBuf, time.Duration) {
		clear(ew.entered)
		clear(ew.hasOrder)
		enteredQ.All()
		for enteredQ.Next() {
			cur := enteredQ.Cursor()
			states, cells := statesComp.Slice(cur), cellComp.Slice(cur)
			for i, id := range cur.IDs {
				if states != nil && states[i].Has(Entered) {
					ew.entered[id] = cells[i].Cell
				}
			}
		}
		orderQ.All()
		for orderQ.Next() {
			cur := orderQ.Cursor()
			orders := orderComp.Slice(cur)
			for _, id := range cur.IDs {
				ew.hasOrder[id] = orders != nil
			}
		}
	}})

	ew.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(steerHandle, d)
		ctx.Run(steeringHandle, d)
		ctx.Run(moveHandle, d)
		ctx.Sync()
		ctx.Run(observer, d)
		ctx.Sync()
	})
	return ew
}

func (ew *enteredWorld) cellAt(x, y uint32) board.CellID {
	c, _ := ew.grid.CellIndex(x, y)
	return c
}

// Entered is on for the one step a unit comes into each cell on its way, and off between: one
// report a cell, whether the unit carried its markers from the start or got them at its first cell.
func TestEntered_ReportsEveryCellOnTheWayToTheTarget(t *testing.T) {
	for _, marks := range []bool{true, false} {
		t.Run(map[bool]string{true: "carried", false: "got on the way"}[marks], func(t *testing.T) {
			enteredOnTheWay(t, marks)
		})
	}
}

func enteredOnTheWay(t *testing.T, marks bool) {
	grid := board.DefaultGrids{}.Square(6, 1, legCellSize)
	start, _ := grid.CellIndex(0, 0)
	target, _ := grid.CellIndex(3, 0)
	ew := newEnteredWorld(t, 6, 1, start, target, marks)

	const maxTicks = 600
	var reported []board.CellID
	for tick := range maxTicks {
		ew.ecs.Tick(time.Second / 60)
		if c, ok := ew.entered[ew.id]; ok {
			reported = append(reported, c)
		}
		if !ew.hasOrder[ew.id] {
			want := []board.CellID{}
			for x := uint32(1); x <= 3; x++ {
				c, _ := grid.CellIndex(x, 0)
				want = append(want, c)
			}
			if len(reported) != len(want) {
				t.Fatalf("tick %d: reported cells %v, want one report per cell entered %v", tick, reported, want)
			}
			for i := range want {
				if reported[i] != want[i] {
					t.Fatalf("reported cells %v, want %v", reported, want)
				}
			}
			return
		}
	}
	t.Fatalf("entity never arrived within %d ticks; reported %v", maxTicks, reported)
}
