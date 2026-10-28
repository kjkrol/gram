package navigation

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// the routes' board: 200x200 cells of 16 px, a sixth of them rock, walls every 40 columns with
// three gaps each
const (
	routeSide = 200
	routeCell = 16
)

// routeBench is the routes' board with a unit standing at each start, the plugin's routing
// answering their asks on a pool
type routeBench struct {
	nw      *navWorld
	coord   *routing.Coordinator
	orders  goke.Comp[MoveOrder]
	ordered *goke.Query
	goals   map[uid.UID64][2]cell.ID
	group   uint32 // the units sent as one MoveTo, its point the goal's middle; 0 each alone
}

func newRouteBench(b *testing.B, pool *parallel.Pool, group uint32,
	place func(g grid.Grid) (starts []cell.ID, goals [][2]cell.ID)) *routeBench {
	b.Helper()
	g := grid.DefaultGrids{}.Square(routeSide, routeSide, routeCell)
	starts, goals := place(g)
	units := make([]navUnit, len(starts))
	for i, c := range starts {
		units[i] = navUnit{box: cellBox(g, c, 12), at: c, order: &MoveOrder{Target: goals[i][0]}}
	}
	r := &routeBench{goals: map[uid.UID64][2]cell.ID{}, group: group}
	r.nw = newNavWorldIn(b, pool, routeSide, routeSide, routeCell, units, world.SectorsCfg{}, nil,
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			r.ordered = si.NewQueryBuilder(&r.orders).Build()
		}})
	for i, id := range r.nw.ids {
		r.goals[id] = goals[i]
	}
	r.coord = r.nw.nav.module.navigationSystem.Routing.(*routing.Coordinator)
	clear := append([]cell.ID(nil), starts...)
	for _, gs := range goals {
		clear = append(clear, gs[0], gs[1])
	}
	layRoutes(r.nw, clear)
	r.route(0)
	for r.ordered.All(); r.ordered.Next(); {
		for _, o := range r.orders.Slice(r.ordered.Cursor()) {
			if o.Ask != order.AskNone || o.Path.Length == 0 {
				b.Fatalf("a unit with no route across the board: ask %v, %d steps", o.Ask,
					o.Path.Length)
			}
		}
	}
	return r
}

// route has every unit ask for a route to its k-th goal anew, the routes kept let go, and the
// routing answer
func (r *routeBench) route(k int) {
	r.nw.board.Touch(0)
	for r.ordered.All(); r.ordered.Next(); {
		cur := r.ordered.Cursor()
		orders := r.orders.Slice(cur)
		for i, id := range cur.IDs {
			to := r.goals[id][k%2]
			orders[i] = MoveOrder{Target: to, Ask: order.AskRoute}
			if r.group != 0 {
				orders[i].At, orders[i].Group = r.nw.grid.CellCenter(to), r.group
				orders[i].Party = uint16(len(r.goals))
			}
		}
	}
	r.coord.Update(nil, time.Second/60)
}

// layRoutes lays the routes' board, the cells round the walls' gaps and round keep clear
func layRoutes(nw *navWorld, keep []cell.ID) {
	g, brd := nw.grid, nw.board
	rng := rand.New(rand.NewPCG(7, 9))
	rock := cell.Kind{Pace: 1, Solid: true}
	clear := keep
	for y := range uint32(routeSide) {
		for x := range uint32(routeSide) {
			wall := x%40 == 20
			switch {
			case wall && y%67 == 33:
				clear = append(clear, g.At(x, y))
			case wall || rng.IntN(6) == 0:
				brd.Set(g.At(x, y), rock)
			}
		}
	}
	for _, c := range clear {
		x, y, _ := g.Coords(c)
		for dy := range uint32(5) {
			for dx := range uint32(5) {
				brd.Set(g.At(x+dx-2, y+dy-2), cell.Kind{Pace: 1, Allows: cell.Land})
			}
		}
	}
}

// Benchmark_Routing_Path is the A* of one route across the board: a unit in the north-west corner
// asks for a route to the far corner, and the next time to the south-west one, round the rocks
// and through the walls' gaps, nothing kept between two.
func Benchmark_Routing_Path(b *testing.B) {
	r := newRouteBench(b, nil, 0, func(g grid.Grid) ([]cell.ID, [][2]cell.ID) {
		return []cell.ID{g.At(2, 2)},
			[][2]cell.ID{{g.At(routeSide-3, routeSide-3), g.At(2, routeSide-3)}}
	})
	b.ReportAllocs()
	for k := 1; b.Loop(); k++ {
		r.route(k)
	}
}

// Benchmark_Routing_Routes is 32 routes across the board at once: units on its west edge ask each
// for a route to the east edge, answered by the plugin's routers on one goroutine (workers=1) or
// on a pool of every CPU (workers=all), nothing kept between two.
func Benchmark_Routing_Routes(b *testing.B) {
	for _, all := range []bool{false, true} {
		name, pool := "workers=1", (*parallel.Pool)(nil)
		if all {
			name, pool = "workers=all", parallel.New(0)
		}
		b.Run(name, func(b *testing.B) {
			r := newRouteBench(b, pool, 0, func(g grid.Grid) ([]cell.ID, [][2]cell.ID) {
				var starts []cell.ID
				var goals [][2]cell.ID
				for i := range uint32(32) {
					east := g.At(routeSide-3, 3+6*i)
					starts = append(starts, g.At(2, 3+6*i))
					goals = append(goals, [2]cell.ID{east, east})
				}
				return starts, goals
			})
			b.ReportAllocs()
			for k := 1; b.Loop(); k++ {
				r.route(k)
			}
		})
	}
}

// Benchmark_Routing_Group is 32 routes across the board to one goal at once: the units of
// Routing_Routes sent to the middle of the east edge as one group, answered on one goroutine
// (workers=1) or on a pool of every CPU (workers=all), nothing kept between two.
func Benchmark_Routing_Group(b *testing.B) {
	for _, all := range []bool{false, true} {
		name, pool := "workers=1", (*parallel.Pool)(nil)
		if all {
			name, pool = "workers=all", parallel.New(0)
		}
		b.Run(name, func(b *testing.B) {
			r := newRouteBench(b, pool, 1, func(g grid.Grid) ([]cell.ID, [][2]cell.ID) {
				goal := g.At(routeSide-3, routeSide/2)
				var starts []cell.ID
				var goals [][2]cell.ID
				for i := range uint32(32) {
					starts = append(starts, g.At(2, 3+6*i))
					goals = append(goals, [2]cell.ID{goal, goal})
				}
				return starts, goals
			})
			b.ReportAllocs()
			for k := 1; b.Loop(); k++ {
				r.route(k)
			}
		})
	}
}
