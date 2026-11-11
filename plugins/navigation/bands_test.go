package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// two observers looking at (x, y) over a world w by h: the one a pixel a world unit, the other a
// quarter of a pixel seeing twice as far; under a ladder seeing from half a pixel a world unit,
// L2 within its sight of the point, L3 within twice that, L6 beyond
type observers [2]camera.Camera

var _ sector.Observers = (*observers)(nil)

func newObservers(w, h uint32, x, y float32) *observers {
	o := &observers{cameratest.NewFromSpace(w, h, 0), cameratest.NewFromSpace(w, h, 0)}
	for _, cam := range o {
		cam.SetViewport(2, 2)
	}
	o[1].ZoomOut(4, 0, 0)
	o[1].(interface{ See(factor float32) }).See(2)
	o.look(x, y)
	return o
}

// has both observers look at (x, y)
func (o *observers) look(x, y float32) {
	for _, cam := range o {
		cam.CenterOn(x, y, 0)
	}
}

func (o *observers) Observers(each func(cam camera.Camera)) {
	for _, cam := range o {
		each(cam)
	}
}

func TestNavigation_GoesOnAtL3AndStandsAsleep(t *testing.T) {
	const cols, rows = 128, 4
	g := grid.DefaultGrids{}.Square(cols, rows, legCellSize)
	profile := steering.Steering{MaxSpeed: float32(legCellSize * 2)}
	walker := func(from, to uint32) navUnit {
		start := g.At(from, 1)
		return navUnit{box: cellBox(g, start, legEntitySize), at: start, profile: &profile,
			order: &MoveOrder{Target: g.At(to, 1)}}
	}
	// at L2, at L3, asleep
	units := []navUnit{walker(2, 8), walker(40, 46), walker(70, 76)}
	var base goke.Comp[world.Base]
	var q *goke.Query
	ladder := lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5}, Sight: 600}
	nw := newNavWorldIn(t, nil, cols, rows, legCellSize, units, world.SectorsCfg{Buckets: 2},
		ladder, func(w *world.Plugin, _ *board.Plugin) {
			w.Write().Observe(newObservers(cols*legCellSize, rows*legCellSize, 80, 48))
		},
		goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base).Build() }})
	xs := func() map[int]float32 {
		got := map[int]float32{}
		for q.All(); q.Next(); {
			cur := q.Cursor()
			for i, id := range cur.IDs {
				for u, of := range nw.ids {
					if of == id {
						got[u] = base.Slice(cur)[i].Pos.Center().X
					}
				}
			}
		}
		return got
	}
	before := xs()
	for range 120 {
		nw.ecs.Tick(time.Second / 60)
	}
	after := xs()
	if cell := float32(legCellSize); after[0] <= before[0]+cell || after[1] <= before[1]+cell {
		t.Errorf("at L2 from %v to %v, at L3 from %v to %v; want both on their way",
			before[0], after[0], before[1], after[1])
	}
	if after[2] != before[2] {
		t.Errorf("asleep from %v to %v, want it standing", before[2], after[2])
	}
}

func TestMoveTo_SendsNoSelectedUnitAsleep(t *testing.T) {
	const cols, rows = 128, 4
	g := grid.DefaultGrids{}.Square(cols, rows, legCellSize)
	profile := steering.Steering{MaxSpeed: float32(legCellSize * 2)}
	selected := func(x uint32) navUnit {
		at := g.At(x, 1)
		return navUnit{box: cellBox(g, at, legEntitySize), at: at, profile: &profile,
			selected: true}
	}
	// at L2, asleep
	units := []navUnit{selected(2), selected(70)}
	var order goke.OptComp[MoveOrder]
	var q *goke.Query
	ladder := lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5}, Sight: 600}
	nw := newNavWorldIn(t, nil, cols, rows, legCellSize, units, world.SectorsCfg{Buckets: 2},
		ladder, func(w *world.Plugin, _ *board.Plugin) {
			w.Write().Observe(newObservers(cols*legCellSize, rows*legCellSize, 80, 48))
		},
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&goke.Comp[unit.At]{}).Optional(&order).Build()
		}})
	nw.ecs.Tick(time.Second / 60)
	nw.nav.moves.Add(command.Nobody, MoveTo{Cell: g.At(8, 1)})
	nw.ecs.Tick(time.Second / 60)
	ordered := func(k int) bool {
		if !q.Seek(nw.ids[k]) {
			t.Fatalf("unit %d is gone", k)
		}
		return order.At(q.Cursor()) != nil
	}
	if !ordered(0) || ordered(1) {
		t.Errorf("ordered at L2 %v, asleep %v; want only the first", ordered(0), ordered(1))
	}
}

// a keeping noting the level of every unit it is asked about
type levelKeeping struct {
	spacing.Keeping
	seen map[uid.UID64]lod.Level
}

func (k *levelKeeping) Watch(m bodies.Member, o *MoveOrder, toward cell.ID, want geom.Vec,
	d time.Duration) {
	k.seen[m.ID] = m.Level
	k.Keeping.Watch(m, o, toward, want, d)
}

func (k *levelKeeping) Route(m bodies.Member, from cell.ID, o *MoveOrder) (Path, bool) {
	k.seen[m.ID] = m.Level
	return k.Keeping.Route(m, from, o)
}

func (k *levelKeeping) Orders(members []bodies.Member, cmd MoveTo,
	issue func(bodies.Member, MoveOrder)) {
	for _, m := range members {
		k.seen[m.ID] = m.Level
	}
	k.Keeping.Orders(members, cmd, issue)
}

func (k *levelKeeping) MayStep(m bodies.Member, ahead cell.ID, deck cell.Level, heading geom.Vec,
	near *bodies.Nearby) bool {
	k.seen[m.ID] = m.Level
	return k.Keeping.MayStep(m, ahead, deck, heading, near)
}

func TestMember_KnowsTheLevelItsSectorIsTickedAt(t *testing.T) {
	const cols, rows = 128, 4
	g := grid.DefaultGrids{}.Square(cols, rows, legCellSize)
	profile := steering.Steering{MaxSpeed: float32(legCellSize * 2)}
	walker := func(from, to uint32) navUnit {
		start := g.At(from, 1)
		return navUnit{box: cellBox(g, start, legEntitySize), at: start, profile: &profile,
			order: &MoveOrder{Target: g.At(to, 1)}, selected: true}
	}
	// at L2, at L4
	units := []navUnit{walker(2, 8), walker(40, 46)}
	// the near observer sees L2 within 600 of the point, the far one L4 within 1200
	ladder := lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.4, 0.2}, Sight: 600}
	var orders goke.Comp[MoveOrder]
	var q *goke.Query
	coord := routing.NewCoordinator(nil, nil)
	nw := newNavWorldIn(t, nil, cols, rows, legCellSize, units, world.SectorsCfg{Buckets: 2},
		ladder, func(w *world.Plugin, _ *board.Plugin) {
			w.Write().Observe(newObservers(cols*legCellSize, rows*legCellSize, 80, 48))
		},
		coord,
		goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&orders).Build() }})
	for range 2 {
		nw.ecs.Tick(time.Second / 60)
	}
	keep := &levelKeeping{Keeping: nw.nav.handKeeping.Keep, seen: map[uid.UID64]lod.Level{}}
	near, far := nw.ids[0], nw.ids[1]
	check := func(by string) {
		t.Helper()
		l0, ok0 := keep.seen[near]
		l2, ok2 := keep.seen[far]
		if !ok0 || !ok2 || l0 != lod.L2 || l2 != lod.L4 {
			t.Errorf("%s: the near unit at %v (%v), the far one at %v (%v); want L2 and L4",
				by, l0, ok0, l2, ok2)
		}
		clear(keep.seen)
	}

	nw.nav.module.navigationSystem.WithKeeping(keep)
	for range 10 {
		nw.ecs.Tick(time.Second / 60)
	}
	check("the traffic")
	for _, id := range []uid.UID64{near, far} {
		if b, ok := keep.Other(id); ok {
			keep.seen[id] = b.Member().Level
		}
	}
	check("the bodies the others meet")

	coord.Router(nw.nav.finder, keep)
	for q.All(); q.Next(); {
		asked := orders.Slice(q.Cursor())
		for i := range asked {
			asked[i].Ask = order.AskRoute
		}
	}
	coord.Update(nil, 0)
	check("the routing")

	nw.nav.module.moveCommandSystem.WithKeeping(keep)
	nw.nav.moves.Add(command.Nobody, MoveTo{Cell: g.At(20, 1)})
	nw.ecs.Tick(time.Second / 60)
	check("a MoveTo")

	nw.nav.handKeeping.Keep = keep
	for _, id := range []uid.UID64{near, far} {
		nw.nav.handKeeping.MayStep(id, world.Position{}, g.At(2, 1), g.At(3, 1), cell.OnGround, cell.Land,
			geom.NewVec(1, 0))
	}
	check("the hand")
}
