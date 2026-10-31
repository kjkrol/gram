package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
)

// asks answered on a pool's goroutines, many of them asked by several units at once, get the
// routes one goroutine alone gives them
func TestCoordinator_AsksOnAPoolGetTheRoutesOfOneGoroutine(t *testing.T) {
	samePoolRoutes(t, func(y uint32) bool { return y%8 == 4 }, 0)
}

// as asks answered on a pool's goroutines alike, of units bigger than a cell, each router keeping
// their footprints of its own
func TestCoordinator_AsksOfBodiesBiggerThanACellOnAPoolGetTheRoutesOfOneGoroutine(t *testing.T) {
	samePoolRoutes(t, func(y uint32) bool { return y%8 >= 3 && y%8 <= 5 }, 40)
}

// samePoolRoutes has 200 units of boxes side wide, none for 0, ask their routes across a wall down
// the middle of a board 32 cells by 32 of 16, open in the rows open says, on a pool of goroutines
// and on one, and wants the same routes; a unit with a box stands and is sent a cell off the
// board's edges, where its footprint stands
func samePoolRoutes(t *testing.T, open func(y uint32) bool, side uint32) {
	t.Helper()
	g := grid.DefaultGrids{}.Square(32, 32, 16)
	off := uint32(min(side, 1))
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	for y := range uint32(32) {
		if !open(y) {
			terrain.Set(g.At(16, y), cell.Kind{Pace: 1, Solid: true})
		}
	}
	routes := func(pool *parallel.Pool) []order.Path {
		occ := &cell.SingleOccupancy{}
		answers := &spacing.Answers{}
		coord := routing.NewCoordinator(pool, nil)
		for range 6 {
			f := spacing.NewFinder(g, terrain, nil, spacing.OpenOccupancy{})
			coord.Router(f, spacing.NewCellKeeping(f, occ).WithAnswers(answers))
		}
		var at goke.Comp[unit.At]
		var orders goke.Comp[MoveOrder]
		var base goke.Comp[world.Base]
		var q *goke.Query
		ecs := goke.New()
		ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
			f := si.NewFactory(&at, &orders, &base)
			f.Create(200)
			k := 0
			for f.Next() {
				for i := range f.Cursor.IDs {
					c := g.At(uint32(k%10)+off, uint32(k%30)+off)
					at.Slice(&f.Cursor)[i] = unit.At{Cell: c}
					orders.Slice(&f.Cursor)[i] = MoveOrder{Target: g.At(31-off,
						uint32(k%5*6)+off), Ask: order.AskRoute}
					if side > 0 {
						base.Slice(&f.Cursor)[i].Pos = world.Position{AABB: cellBox(g, c, side)}
					}
					k++
				}
			}
			q = si.NewQueryBuilder(&orders).Build()
		}})
		h := ecs.RegSys(coord)
		ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(h, d) })
		ecs.Tick(time.Second / 60)
		var paths []order.Path
		for q.All(); q.Next(); {
			for _, o := range orders.Slice(q.Cursor()) {
				if o.Ask != order.AskNone {
					t.Fatalf("an ask left unanswered: %v", o.Ask)
				}
				paths = append(paths, o.Path)
			}
		}
		return paths
	}
	one, many := routes(nil), routes(parallel.New(4))
	if len(one) != 200 || len(many) != len(one) {
		t.Fatalf("%d and %d routes, want 200 each", len(one), len(many))
	}
	for i := range one {
		if one[i] != many[i] {
			t.Fatalf("unit %d: route %+v on the pool, %+v on one goroutine", i, many[i], one[i])
		}
	}
}
