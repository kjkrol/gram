package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// enteredWorld is one navigated entity in a world with a board, observing its Entered marker
// after every step.
type enteredWorld struct {
	ecs  *goke.ECS
	id   uid.UID64
	grid grid.Grid

	// per-tick observations, refreshed by the observer system
	entered  map[uid.UID64]cell.ID
	hasOrder map[uid.UID64]bool
}

// newEnteredWorld makes the world; with marks the entity carries its markers from the start, else it
// gets them at the first cell it enters.
func newEnteredWorld(t *testing.T, w, h uint32, start, target cell.ID, marks bool) *enteredWorld {
	t.Helper()
	ew := &enteredWorld{entered: map[uid.UID64]cell.ID{}, hasOrder: map[uid.UID64]bool{}}
	g := grid.DefaultGrids{}.Square(w, h, legCellSize)
	profile := steering.Steering{MaxSpeed: float64(legCellSize * 2)}

	var statesComp goke.OptComp[tag.Tags[States]]
	var orderComp goke.OptComp[MoveOrder]
	var cellComp goke.Comp[unit.At]
	var enteredQ, orderQ *goke.Query
	nw := newNavWorld(t, w, h, legCellSize, []navUnit{{
		box: cellBox(g, start, legEntitySize), at: start, profile: &profile, order: &MoveOrder{Target: target}, marks: marks,
	}}, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		enteredQ = si.NewQueryBuilder(&cellComp).Optional(&statesComp).Build()
		orderQ = si.NewQueryBuilder(&cellComp).Optional(&orderComp).Build()
	}})
	ew.ecs, ew.id, ew.grid = nw.ecs, nw.ids[0], nw.grid

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
		nw.step(ctx, d)
		ctx.Run(observer, d)
		ctx.Sync()
	})
	return ew
}

func (ew *enteredWorld) cellAt(x, y uint32) cell.ID {
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
	grid := grid.DefaultGrids{}.Square(6, 1, legCellSize)
	start, _ := grid.CellIndex(0, 0)
	target, _ := grid.CellIndex(3, 0)
	ew := newEnteredWorld(t, 6, 1, start, target, marks)

	const maxTicks = 600
	var reported []cell.ID
	for tick := range maxTicks {
		ew.ecs.Tick(time.Second / 60)
		if c, ok := ew.entered[ew.id]; ok {
			reported = append(reported, c)
		}
		if !ew.hasOrder[ew.id] {
			want := []cell.ID{}
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
