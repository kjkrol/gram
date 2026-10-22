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
)

// asks answered on a pool's goroutines, many of them asked by several units at once, get the
// routes one goroutine alone gives them
func TestCoordinator_AsksOnAPoolGetTheRoutesOfOneGoroutine(t *testing.T) {
	g := grid.DefaultGrids{}.Square(32, 32, 16)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	for y := range uint32(32) {
		if y%8 != 4 {
			terrain.Set(g.At(16, y), cell.Kind{Cost: 1, Solid: true})
		}
	}
	routes := func(pool *parallel.Pool) []order.Path {
		occ := &cell.SingleOccupancy{}
		answers := &spacing.Answers{}
		coord := routing.NewCoordinator(pool, nil)
		for range 4 {
			f := spacing.NewFinder(g, terrain, nil, spacing.OpenOccupancy{})
			coord.Router(f, spacing.NewCellKeeping(f, occ).WithAnswers(answers))
		}
		var at goke.Comp[unit.At]
		var orders goke.Comp[MoveOrder]
		var q *goke.Query
		ecs := goke.New()
		ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
			f := si.NewFactory(&at, &orders)
			f.Create(200)
			k := 0
			for f.Next() {
				for i := range f.Cursor.IDs {
					at.Slice(&f.Cursor)[i] = unit.At{Cell: g.At(uint32(k%10), uint32(k%30))}
					orders.Slice(&f.Cursor)[i] = MoveOrder{Target: g.At(31, uint32(k%5*6)),
						Ask: order.AskRoute}
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
