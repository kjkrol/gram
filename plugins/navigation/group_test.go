package navigation

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// the groups' board: 32x32 cells of 16, a wall down its middle with gaps at rows 8 and 24
const (
	groupSide = 32
	groupCell = 16
)

func groupGrid() grid.Grid { return grid.DefaultGrids{}.Square(groupSide, groupSide, groupCell) }

// groupRig is the groups' board with a unit standing at each start, the plugin's routing
// answering their asks
type groupRig struct {
	nw      *navWorld
	coord   *routing.Coordinator
	orders  goke.Comp[MoveOrder]
	at      goke.Comp[unit.At]
	ordered *goke.Query
}

func newGroupRig(t testing.TB, pool *parallel.Pool, starts []cell.ID) *groupRig {
	t.Helper()
	g := groupGrid()
	units := make([]navUnit, len(starts))
	for i, c := range starts {
		units[i] = navUnit{box: cellBox(g, c, 12), at: c, order: &MoveOrder{}}
	}
	r := &groupRig{}
	r.nw = newNavWorldIn(t, pool, groupSide, groupSide, groupCell, units, world.SectorsCfg{}, nil,
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			r.ordered = si.NewQueryBuilder(&r.orders, &r.at).Build()
		}})
	r.coord = r.nw.nav.module.navigationSystem.Routing.(*routing.Coordinator)
	for y := range uint32(groupSide) {
		if y != 8 && y != 24 {
			r.nw.board.Set(g.At(16, y), wall)
		}
	}
	return r
}

// the units at starts in the west, n of them, four a column
func westOf(n int) []cell.ID {
	g := groupGrid()
	var starts []cell.ID
	for i := range n {
		starts = append(starts, g.At(uint32(2+i%4), uint32(6+3*(i/4))))
	}
	return starts
}

func (r *groupRig) order(id uid.UID64) (*MoveOrder, cell.ID) {
	for r.ordered.All(); r.ordered.Next(); {
		cur := r.ordered.Cursor()
		for i, got := range cur.IDs {
			if got == id {
				return &r.orders.Slice(cur)[i], r.at.Slice(cur)[i].Cell
			}
		}
	}
	panic("no such unit")
}

// send gives the units, by their starts' order, one MoveTo to goal as the plugin's cell keeping
// orders them, each asking for its route
func (r *groupRig) send(group uint32, goal cell.ID, units ...int) {
	keep := spacing.NewCellKeeping(spacing.NewFinder(r.nw.grid, r.nw.board.BoardRead, nil,
		spacing.OpenOccupancy{}), &cell.SingleOccupancy{})
	var members []bodies.Member
	for _, u := range units {
		_, at := r.order(r.nw.ids[u])
		members = append(members, bodies.Member{ID: r.nw.ids[u], Cell: at, From: at,
			Domain: cell.Land})
	}
	keep.Orders(members, MoveTo{Cell: goal}, func(m bodies.Member, o MoveOrder) {
		o.Group, o.Ask = group, order.AskRoute
		mine, _ := r.order(m.ID)
		*mine = o
	})
}

// ask has the units ask for their routes anew where they stand, as a unit does whose route broke
func (r *groupRig) ask(units ...int) {
	for _, u := range units {
		o, _ := r.order(r.nw.ids[u])
		o.Path, o.Ask = Path{}, order.AskRoute
	}
}

// walk stands the unit k steps along its route
func (r *groupRig) walk(u, k int) {
	id := r.nw.ids[u]
	for r.ordered.All(); r.ordered.Next(); {
		cur := r.ordered.Cursor()
		for i, got := range cur.IDs {
			if got == id {
				r.at.Slice(cur)[i].Cell = r.orders.Slice(cur)[i].Path.Steps[k-1]
			}
		}
	}
}

func (r *groupRig) update() { r.coord.Update(nil, time.Second/60) }

// routed checks every unit given has its route from where it stands, step by step to its target
func (r *groupRig) routed(t *testing.T, units ...int) {
	t.Helper()
	for _, u := range units {
		o, at := r.order(r.nw.ids[u])
		p := o.Path
		if o.Ask != order.AskNone || p.From != at || p.Length == 0 ||
			p.Steps[p.Length-1] != o.Target {
			t.Errorf("unit %d at %v: ask %v, a route from %v of %d steps to %v, want one to %v",
				u, at, o.Ask, p.From, p.Length, p.Steps[:p.Length], o.Target)
			continue
		}
		prev := at
		for _, s := range p.Steps[:p.Length] {
			if _, ok := grid.Way(r.nw.grid, prev, s); !ok || !r.nw.board.Kind(s).Admits(cell.Land) {
				t.Errorf("unit %d: a step %v->%v of its route %v", u, prev, s,
					p.Steps[:p.Length])
			}
			prev = s
		}
	}
}

func every(n int) []int {
	units := make([]int, n)
	for i := range units {
		units[i] = i
	}
	return units
}

func TestGroup_OneFieldServesEveryMembersRoute(t *testing.T) {
	r := newGroupRig(t, nil, westOf(8))
	goal := r.nw.grid.At(26, 16)
	r.send(1, goal, every(8)...)
	before := r.coord.Searches()
	r.update()
	if fills := r.coord.Fills(); fills != 1 {
		t.Errorf("%d fields filled, want one for the group", fills)
	}
	r.routed(t, every(8)...)
	legs := uint64(0)
	for _, u := range every(8) {
		if o, _ := r.order(r.nw.ids[u]); o.Target != goal {
			legs++
		}
	}
	if searched := r.coord.Searches() - before; searched != legs {
		t.Errorf("%d searches, want the %d last legs to targets beside the goal alone", searched,
			legs)
	}
}

func TestGroup_ABumpRefillsTheRouteWithoutASearch(t *testing.T) {
	r := newGroupRig(t, nil, westOf(8))
	goal := r.nw.grid.At(26, 16)
	r.send(1, goal, every(8)...)
	r.update()
	bumped := slices.IndexFunc(every(8), func(u int) bool {
		o, _ := r.order(r.nw.ids[u])
		return o.Target == goal
	})
	if bumped < 0 {
		t.Fatal("no unit sent to the goal itself")
	}
	fills, searches := r.coord.Fills(), r.coord.Searches()
	r.walk(bumped, 5)
	r.ask(bumped)
	r.update()
	if r.coord.Fills() != fills || r.coord.Searches() != searches {
		t.Errorf("%d fills and %d searches more, want the route walked off the field kept",
			r.coord.Fills()-fills, r.coord.Searches()-searches)
	}
	r.routed(t, bumped)
}

func TestGroup_AChangedBoardRebuildsTheFieldOnce(t *testing.T) {
	r := newGroupRig(t, nil, westOf(8))
	r.send(1, r.nw.grid.At(26, 16), every(8)...)
	r.update()
	gap := r.nw.grid.At(16, 8)
	r.nw.board.Set(gap, wall)
	r.ask(every(8)...)
	r.update()
	if fills := r.coord.Fills(); fills != 2 {
		t.Errorf("%d fields filled, want the group's once and once more for the wall", fills)
	}
	r.routed(t, every(8)...)
}

// three, one fewer than a party
func TestGroup_FewerThanAPartyKeepTheSearch(t *testing.T) {
	r := newGroupRig(t, nil, westOf(3))
	r.send(1, r.nw.grid.At(26, 16), every(3)...)
	before := r.coord.Searches()
	r.update()
	if fills := r.coord.Fills(); fills != 0 {
		t.Errorf("%d fields filled for three units, want none", fills)
	}
	if searched := r.coord.Searches() - before; searched != 3 {
		t.Errorf("%d searches, want one a unit", searched)
	}
	r.routed(t, every(3)...)
}

// a unit of the group asking from far past where the field reaches searches its route as before
func TestGroup_AMemberOffTheFieldSearches(t *testing.T) {
	g := groupGrid()
	starts := []cell.ID{g.At(22, 14), g.At(22, 18), g.At(30, 14), g.At(30, 18), g.At(2, 16)}
	r := newGroupRig(t, nil, starts)
	r.send(1, g.At(26, 16), every(len(starts))...)
	far, _ := r.order(r.nw.ids[4])
	far.Ask = order.AskNone
	r.update()
	fills, searches := r.coord.Fills(), r.coord.Searches()
	r.ask(4)
	r.update()
	if r.coord.Fills() != fills || r.coord.Searches() != searches+1 {
		t.Errorf("%d fills and %d searches more, want the far unit's own search alone",
			r.coord.Fills()-fills, r.coord.Searches()-searches)
	}
	r.routed(t, every(len(starts))...)
}

// groups' asks answered on a pool's goroutines, one filling a field the others wait on, get the
// routes one goroutine alone gives them
func TestGroup_AsksOnAPoolGetTheRoutesOfOneGoroutine(t *testing.T) {
	routes := func(pool *parallel.Pool) ([]Path, uint64) {
		r := newGroupRig(t, pool, westOf(24))
		g := r.nw.grid
		r.send(1, g.At(26, 16), 0, 3, 6, 9, 12, 15, 18, 21)
		r.send(2, g.At(28, 4), 1, 4, 7, 10, 13, 16, 19, 22)
		r.send(3, g.At(24, 28), 2, 5, 8, 11, 14, 17, 20, 23)
		r.update()
		r.routed(t, every(24)...)
		var paths []Path
		for _, id := range r.nw.ids {
			o, _ := r.order(id)
			paths = append(paths, o.Path)
		}
		return paths, r.coord.Fills()
	}
	one, oneFills := routes(nil)
	many, manyFills := routes(parallel.New(4))
	if oneFills != 3 || manyFills != oneFills {
		t.Errorf("%d fields filled on one goroutine, %d on the pool; want one a group",
			oneFills, manyFills)
	}
	for i := range one {
		if one[i] != many[i] {
			t.Fatalf("unit %d: route %v on the pool, %v on one goroutine", i,
				many[i].Steps[:many[i].Length], one[i].Steps[:one[i].Length])
		}
	}
}
