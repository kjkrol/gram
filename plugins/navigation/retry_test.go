package navigation

import (
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// a board nobody holds, counting what the searches ask of it
type askedOccupancy struct {
	openOccupancy
	asked int
}

func (o *askedOccupancy) CanEnter(cell.ID, bool, uid.UID64, cell.Domain) bool {
	o.asked++
	return true
}

const retryTick = time.Second / 60

// five cells across, a wall down the fourth: nothing gets from the first three to the fifth
func walled(rows uint32) (grid.Grid, *cell.TerrainMap, func(x, y uint32) cell.ID) {
	g := grid.DefaultGrids{}.Square(5, rows, 10)
	at := func(x, y uint32) cell.ID { c := g.At(x, y); return c }
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	for y := range rows {
		terrain.Set(at(3, y), wall)
	}
	return g, terrain, at
}

func within(got, want time.Duration) bool { return got >= want && got < want+retryTick }

func TestRetry_ADetourFindingNoWayIsSearchedAgainHalfASecondOneAndTwoSecondsLater(t *testing.T) {
	g, terrain, at3 := walled(3)
	at := func(x uint32) cell.ID { return at3(x, 1) }
	occ := &askedOccupancy{}
	k := newCellKeeping(newPathFinder(g, terrain, nil, occ), occ)
	m := bodies.Member{ID: 7, Cell: at(0), From: at(0), Domain: cell.Land, Pos: posAt(g, at(0))}
	o := MoveOrder{Target: at(4), Toward: at(1)}
	other := bodies.Body{ID: 3, Cell: at(1)}
	var searched []time.Duration
	detour := func(tick int) {
		o.Retry = max(o.Retry-retryTick, 0) // as the system's pass does, every order a tick
		asked := occ.asked
		k.detour(m, &o, other)
		if occ.asked > asked {
			searched = append(searched, time.Duration(tick)*retryTick)
		}
	}
	for tick := range 6 * 60 {
		detour(tick)
	}
	want := []time.Duration{0, 500 * time.Millisecond, 1500 * time.Millisecond,
		3500 * time.Millisecond, 5500 * time.Millisecond}
	if len(searched) != len(want) {
		t.Fatalf("told to go round every tick for 6 s it searched at %v, want at %v",
			searched, want)
	}
	for i := 1; i < len(want); i++ {
		if gap := searched[i] - searched[i-1]; !within(gap, want[i]-want[i-1]) {
			t.Errorf("searched at %v, want %v apart", searched, want[i]-want[i-1])
		}
	}
	if !o.Cornered {
		t.Error("not cornered after every search found no way round")
	}

	terrain.Set(at3(3, 0), cell.Kind{Cost: 1, Allows: cell.Land})
	searched = searched[:0]
	tick := 6 * 60
	for ; len(searched) == 0; tick++ {
		detour(tick)
	}
	if o.Cornered || o.Fails != 0 {
		t.Fatalf("a gap in the wall, it was cornered %v with %d failed in a row, want a way found",
			o.Cornered, o.Fails)
	}
	terrain.Set(at3(3, 0), wall)
	searched = searched[:0]
	for end := tick + 60; tick < end; tick++ {
		detour(tick)
	}
	if len(searched) != 2 || !within(searched[1]-searched[0], 500*time.Millisecond) {
		t.Errorf("the gap closed after a way round was found it searched at %v, want at once "+
			"and half a second later", searched)
	}
}

// a new order searches at once; the order's own way out, its target given up at
// targetWaitTimeout, comes as before
func TestRetry_AnOrderWithNoWaySearchesOnceTillItGivesItsTargetUp(t *testing.T) {
	g, terrain, at1 := walled(1)
	at := func(x uint32) cell.ID { return at1(x, 0) }
	occupancy := &cell.SingleOccupancy{}
	asked := &askedOccupancy{}
	nav := newNavigationSystem(newPathFinder(g, terrain, nil, asked), g, terrain, occupancy)

	var here goke.Comp[unit.At]
	var base goke.Comp[world.Base]
	var orderComp goke.Comp[MoveOrder]
	var profile goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var q *goke.Query
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&here, &base, &orderComp, &profile, &course)
		f.Create(1)
		f.Next()
		here.Slice(&f.Cursor)[0] = unit.At{Cell: at(0)}
		base.Slice(&f.Cursor)[0].Pos = posAt(g, at(0))
		orderComp.Slice(&f.Cursor)[0] = MoveOrder{Target: at(4)}
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
		occupancy.Enter(at(0), false, f.Cursor.IDs[0], cell.Land)
		q = si.NewQueryBuilder(&here, &orderComp).Build()
	}})
	navHandle := ecs.RegSys(nav)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(navHandle, d); ctx.Sync() })
	ordered := func() bool { q.All(); return q.Next() }

	const reordered = 10
	var searched []int
	tick := 0
	for ; tick < 2*60 && ordered(); tick++ {
		if tick == reordered {
			orderComp.Slice(q.Cursor())[0] = MoveOrder{Target: at(4)}
		}
		before := asked.asked
		ecs.Tick(retryTick)
		if asked.asked > before {
			searched = append(searched, tick)
		}
	}
	waits := reordered + int(targetWaitTimeout/retryTick)
	if ordered() || tick < waits || tick > waits+2 {
		t.Fatalf("the order lasted %d ticks, want it given up %v after the new one, about %d",
			tick, targetWaitTimeout, waits)
	}
	if want := []int{0, reordered, tick - 1}; !slices.Equal(searched, want) {
		t.Errorf("searched on ticks %v, want %v: as ordered, as ordered anew, as it gives up",
			searched, want)
	}
}

func TestRetry_TheNextGoalOrAnotherCellLearntSearchesAtOnce(t *testing.T) {
	g, terrain, at1 := walled(1)
	at := func(x uint32) cell.ID { return at1(x, 0) }
	occ := &askedOccupancy{}
	k := newCellKeeping(newPathFinder(g, terrain, nil, occ), occ)
	m := bodies.Member{ID: 7, Cell: at(0), From: at(0), Domain: cell.Land, Pos: posAt(g, at(0))}
	other := bodies.Body{ID: 3, Cell: at(1)}
	for _, c := range []struct {
		name string
		next func(o *MoveOrder)
	}{
		{"the next goal", func(o *MoveOrder) { order.Advance(o) }},
		{"another cell learnt", func(o *MoveOrder) { o.Toward = at(2) }},
	} {
		o := MoveOrder{Target: at(4), Toward: at(1)}
		o.Enqueue(Goal{Cell: at(4)})
		searches := func() bool {
			asked := occ.asked
			k.detour(m, &o, other)
			return occ.asked > asked
		}
		searches()
		if searches() {
			t.Fatalf("%s: searched again at once after it found no way round", c.name)
		}
		c.next(&o)
		if !searches() {
			t.Errorf("after %s it waited, want a search at once", c.name)
		}
	}
}
