package navigation

import (
	"testing"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/world"
)

// a unit standing at (x, y), going speed cells a second
func (r *abstractRig) stander(x, y uint32, speed float32) navUnit {
	at := r.g.At(x, y)
	profile := steering.Steering{MaxSpeed: speed * float32(legCellSize)}
	return navUnit{box: cellBox(r.g, at, legEntitySize), at: at, profile: &profile}
}

// the order unit k carries, a copy, false with none
func (r *abstractRig) orderOf(t *testing.T, k int) (MoveOrder, bool) {
	t.Helper()
	o, _, _, _, _ := r.unit(t, k)
	if o == nil {
		return MoveOrder{}, false
	}
	return *o, true
}

func (r *abstractRig) give(t *testing.T, k int, cmd Inherit) {
	t.Helper()
	if !r.nw.nav.worldPlugin.Write().Carrier().PutFrom(r.nw.ids[k], cmd) {
		t.Fatalf("unit %d's %v carried by nobody", k, cmd)
	}
}

// has the units' orders far off, a step or more of their routes behind them, within ticks
func (r *abstractRig) underWay(t *testing.T, ticks int, ks ...int) {
	t.Helper()
	for range ticks {
		r.tick()
		ready := true
		for _, k := range ks {
			o, ok := r.orderOf(t, k)
			ready = ready && ok && o.Path.Index > 0 && r.unitMarked(t, k)
		}
		if ready {
			return
		}
	}
	t.Fatalf("units %v not along their routes far off in %d ticks", ks, ticks)
}

func TestInherit_TakesTheOrderOfAnotherWithItsRoute(t *testing.T) {
	r := &abstractRig{g: grid.DefaultGrids{}.Square(128, 16, legCellSize)}
	far := r.walker(2, 8, 120, 8, 16)
	far.order.Enqueue(Goal{Cell: r.g.At(120, 2), At: r.g.CellCenter(r.g.At(120, 2))})
	// the one sent far; one standing; one sent elsewhere, behind the first's way
	r = newAbstractRig(t, 128, 16, []navUnit{far, r.stander(4, 3, 8), r.walker(1, 12, 1, 2, 8)},
		nil)
	r.underWay(t, 120, 0)
	was, _ := r.orderOf(t, 0)
	r.give(t, 1, Inherit{From: ceremony.ID(r.nw.ids[0])})
	r.give(t, 2, Inherit{From: ceremony.ID(r.nw.ids[0])})
	r.tick()

	if got, ok := r.orderOf(t, 1); !ok || got != order.Inherited(&was, Leg{}) {
		t.Errorf("standing, took over %+v (%v);\nwant the first's goals and route at step %d, "+
			"nothing of its walk", got, ok, was.Path.Index)
	}
	got, _ := r.orderOf(t, 2)
	if got.Target != was.Target || got.Queued != was.Queued || got.Waypoints != was.Waypoints ||
		got.Group != was.Group || got.Path.Steps != was.Path.Steps ||
		got.Path.Length != was.Path.Length || got.Path.Index < was.Path.Index {
		t.Errorf("under way, took over %v and %d goals on %+v;\nwant %v and %d on %+v", got.Target,
			got.Queued, got.Path, was.Target, was.Queued, was.Path)
	}
	if r.tick(); !r.unitMarked(t, 2) {
		t.Error("under way, its route taken over, it is not in the far mode")
	}
}

func (r *abstractRig) unitMarked(t *testing.T, k int) bool {
	t.Helper()
	_, _, _, _, marked := r.unit(t, k)
	return marked
}

// the rig's units, the ones given as parts of the first, who is an aggregate
func newWholeRig(t *testing.T, cols, rows uint32, units []navUnit, parts ...int) *abstractRig {
	t.Helper()
	units[0].spec = append(units[0].spec, comp.Const(world.Aggregate{}))
	for _, k := range parts {
		units[k].spec = append(units[k].spec, comp.Const(world.PartOf{}))
	}
	var q *goke.Query
	var of goke.Comp[world.PartOf]
	r := newAbstractRigOn(t, nil, cols, rows, units, nil, goke.SystemFn{
		OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&of).Build() },
	})
	for _, k := range parts {
		if !q.Seek(r.nw.ids[k]) {
			t.Fatalf("part %d is gone", k)
		}
		of.At(q.Cursor()).Whole = r.nw.ids[0]
	}
	return r
}

func TestInherit_FromNoneIsTheWholes(t *testing.T) {
	r := &abstractRig{g: grid.DefaultGrids{}.Square(128, 16, legCellSize)}
	second := r.walker(4, 4, 120, 4, 8)
	second.order.Enqueue(Goal{Cell: r.g.At(120, 12)})
	// the aggregate and a part, standing; two parts sent far
	r = newWholeRig(t, 128, 16, []navUnit{r.stander(3, 8, 8), r.stander(2, 8, 8),
		r.walker(4, 12, 110, 12, 8), second}, 1, 2, 3)
	r.underWay(t, 120, 2, 3)
	first := 2
	if r.nw.ids[3] < r.nw.ids[2] {
		first = 3
	}
	was, _ := r.orderOf(t, first)
	r.give(t, 0, Inherit{})
	r.tick()
	whole, ok := r.orderOf(t, 0)
	if !ok || whole != order.Inherited(&was, Leg{}) {
		t.Fatalf("the aggregate took over %+v (%v);\nwant the order of its part of the least id",
			whole, ok)
	}

	r.give(t, 1, Inherit{})
	r.tick()
	if got, ok := r.orderOf(t, 1); !ok || got != order.Inherited(&whole, Leg{}) {
		t.Errorf("a part took over %+v (%v);\nwant its aggregate's", got, ok)
	}
}

func TestInherit_OfAnEntityWithoutAnOrderStands(t *testing.T) {
	r := &abstractRig{g: grid.DefaultGrids{}.Square(32, 8, legCellSize)}
	units := []navUnit{r.walker(2, 4, 28, 4, 4), r.stander(2, 1, 4), r.stander(2, 6, 4)}
	var q *goke.Query
	var base goke.Comp[world.Base]
	var orders goke.OptComp[MoveOrder]
	nw := newNavWorld(t, 32, 8, legCellSize, units, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base).Optional(&orders).Build()
	}})
	r.nw = nw
	of := func(k int) (*MoveOrder, float32) {
		if !q.Seek(nw.ids[k]) {
			t.Fatalf("unit %d is gone", k)
		}
		return orders.At(q.Cursor()), base.At(q.Cursor()).Pos.Center().X
	}
	for range 20 {
		r.tick()
	}
	if o, _ := of(0); o == nil {
		t.Fatal("the one sent is not under way")
	}
	_, x := of(0)
	r.give(t, 0, Inherit{From: ceremony.ID(nw.ids[1])})
	r.give(t, 2, Inherit{From: ceremony.ID(nw.ids[1])})
	for range 30 {
		r.tick()
	}
	if o, nx := of(0); o != nil || nx-x > float32(legCellSize) {
		t.Errorf("under way, took over a standing one's: an order %v, from %v to %v; want it "+
			"standing within its step", o != nil, x, nx)
	}
	if o, _ := of(2); o != nil {
		t.Errorf("standing, took over a standing one's: %+v, want none", *o)
	}
}
