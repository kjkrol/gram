package navigation

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// fieldUnit is a unit of a fieldWorld: a box side a side round at, under order from the start
// when it has one.
type fieldUnit struct {
	at       geom.Vec
	side     float64
	selected bool
	driven   steering.Driven // non-zero: steered by hand
	order    *MoveOrder
	owner    control.PlayerID // who owns it; nobody for Nobody
}

// standAt is the order to stand at p, as a click on p with the unit alone selected gives.
func (fw *fieldWorld) standAt(p geom.Vec) *MoveOrder {
	c, _ := fw.grid.CellAt(p)
	return &MoveOrder{Target: c, Spot: p, At: p}
}

// fieldWorld is a cols x rows board of 32-unit cells with collision, its units kept apart by
// their boxes.
type fieldWorld struct {
	t     *testing.T
	grid  grid.Grid
	ecs   *goke.ECS
	nav   *Plugin
	base  goke.Comp[world.Base]
	cell  goke.Comp[unit.At]
	order goke.OptComp[MoveOrder]
	coll  goke.OptComp[collision.Collider]
	q     *goke.Query
	ids   []uid.UID64 // in the order given
}

const fieldCell = 32

// newFieldWorld builds the field, lays its kinds and puts units on it.
func newFieldWorld(t *testing.T, cols, rows uint32, spacing Spacing, lay func(b *board.Board, at func(x, y uint32) cell.ID), units []fieldUnit) *fieldWorld {
	t.Helper()
	return newFieldWorldWith(t, cols, rows, spacing, lay, units, nil)
}

// newFieldWorldWith is newFieldWorld with navigation set up by configure before it is used.
func newFieldWorldWith(t *testing.T, cols, rows uint32, spacing Spacing, lay func(b *board.Board, at func(x, y uint32) cell.ID), units []fieldUnit, configure func(*Plugin)) *fieldWorld {
	t.Helper()
	fw := &fieldWorld{t: t, grid: grid.DefaultGrids{}.Square(cols, rows, fieldCell)}
	largest := uint32(1)
	for _, u := range units {
		largest = max(largest, uint32(math.Ceil(u.side)))
	}
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: cols * fieldCell, Height: rows * fieldCell},
		Entities: world.EntitiesCfg{MaxCount: len(units), MinSize: 1, MaxSize: largest},
	})
	c := collision.NewPlugin(w)
	brd := board.NewPlugin(fw.grid, &cell.SingleOccupancy{}, w).WithCollision(c)
	brd.Res.Logic.Board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	if lay != nil {
		lay(brd.Res.Logic.Board, fw.at)
	}
	sel := selection.NewPlugin(w)
	drv := driving.NewPlugin(w, sel).WithGround(brd)
	fw.nav = NewPlugin(brd, w, sel, drv).WithCollision(c).WithSpacing(spacing)
	if configure != nil {
		configure(fw.nav)
	}
	if err := w.Carry(fw.nav); err != nil { // as the engine does with Use
		t.Fatal(err)
	}
	ctx := &stubInstallCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := fw.nav.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := drv.Install(ctx); err != nil {
		t.Fatal(err)
	}
	kinds := make([]kind.ID, len(units))
	for i, u := range units {
		s := kind.Spec{
			comp.Load(func(u fieldUnit) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(u.at.X-u.side/2, u.at.Y-u.side/2), u.side, u.side)}
			}),
			comp.Const(world.Velocity{}),
			comp.Const(steering.Steering{MaxSpeed: 96, Accel: 192, Brake: 384, V0: 48, TurnRate: 0.15}),
			comp.Load(func(u fieldUnit) unit.At { c, _ := fw.grid.CellAt(u.at); return unit.At{Cell: c} }),
			comp.Const(collision.Collider{}),
			comp.Const(world.Layers(cell.Land)),
			comp.Const(collision.Physics{}),
			comp.Const(unit.Mover{Domain: cell.Land}),
		}
		if u.selected {
			s = append(s, comp.Tagged(sel.Tags().Selectable, sel.Tags().Selected))
		}
		if u.driven != (steering.Driven{}) {
			s = append(s, comp.Load(func(u fieldUnit) steering.Driven { return u.driven }))
		}
		if u.order != nil {
			s = append(s, comp.Load(func(u fieldUnit) MoveOrder { return *u.order }))
		}
		if u.owner != control.Nobody {
			s = append(s, comp.Tagged(owner.Of(u.owner)))
		}
		kind.Define[fieldUnit](w.Kinds(), fmt.Sprintf("u%d", i), s)
		k := kind.Named[fieldUnit](w.Kinds(), fmt.Sprintf("u%d", i))
		kinds[i] = k.ID()
		w.Seed(k.Entry(u))
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		fw.q = si.NewQueryBuilder(&fw.base, &fw.cell).Optional(&fw.order).Optional(&fw.coll).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		c.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		fw.nav.RunPlan(rc, d)
		drv.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	})
	fw.ecs = ctx.ecs
	fw.ids = make([]uid.UID64, len(units))
	fw.each(func(id uid.UID64, b *world.Base, _ cell.ID, _ *MoveOrder, _ *collision.Collider) {
		for i, k := range kinds {
			if b.TypeID == k {
				fw.ids[i] = id
			}
		}
	})
	return fw
}

func (fw *fieldWorld) at(x, y uint32) cell.ID { c := fw.grid.CellIndex(x, y); return c }

// each calls fn with every unit.
func (fw *fieldWorld) each(fn func(id uid.UID64, b *world.Base, cell cell.ID, o *MoveOrder, c *collision.Collider)) {
	for fw.q.All(); fw.q.Next(); {
		cur := fw.q.Cursor()
		bases, cells, orders, colls := fw.base.Slice(cur), fw.cell.Slice(cur), fw.order.Slice(cur), fw.coll.Slice(cur)
		for i, id := range cur.IDs {
			var o *MoveOrder
			if orders != nil {
				o = &orders[i]
			}
			var c *collision.Collider
			if colls != nil {
				c = &colls[i]
			}
			fn(id, &bases[i], cells[i].Cell, o, c)
		}
	}
}

// centre is where unit i stands, and its order if it has one.
func (fw *fieldWorld) centre(i int) (geom.Vec, *MoveOrder) {
	var at geom.Vec
	var order *MoveOrder
	fw.each(func(id uid.UID64, b *world.Base, _ cell.ID, o *MoveOrder, _ *collision.Collider) {
		if id == fw.ids[i] {
			at = b.Pos.Center()
			if o != nil {
				copied := *o
				order = &copied
			}
		}
	})
	return at, order
}

// run ticks until no unit has an order, or limit; it reports whether they all settled, and how
// many contacts were struck on the way.
func (fw *fieldWorld) run(limit time.Duration) (settled bool, contacts int) {
	settled, contacts, _ = fw.runLongest(limit)
	return settled, contacts
}

// runLongest is run, and the most ticks any two units touched on end.
func (fw *fieldWorld) runLongest(limit time.Duration) (settled bool, contacts, longest int) {
	touching := map[[2]uid.UID64]int{}
	for tick := 0; time.Duration(tick)*time.Second/60 < limit; tick++ {
		fw.ecs.Tick(time.Second / 60)
		busy := false
		now := map[[2]uid.UID64]bool{}
		fw.each(func(id uid.UID64, _ *world.Base, _ cell.ID, o *MoveOrder, c *collision.Collider) {
			busy = busy || o != nil
			if c != nil {
				contacts += len(c.Contacts())
				for _, k := range c.Contacts() {
					if !k.Terrain {
						now[[2]uid.UID64{min(id, k.Other), max(id, k.Other)}] = true
					}
				}
			}
		})
		for pair := range touching {
			if !now[pair] {
				delete(touching, pair)
			}
		}
		for pair := range now {
			touching[pair]++
			longest = max(longest, touching[pair])
		}
		if !busy {
			return true, contacts, longest
		}
	}
	return false, contacts, longest
}

// overlaps lists the pairs of units whose boxes overlap.
func (fw *fieldWorld) overlaps() []string {
	type box struct {
		id  uid.UID64
		box geom.AABB
	}
	var boxes []box
	fw.each(func(id uid.UID64, b *world.Base, _ cell.ID, _ *MoveOrder, _ *collision.Collider) {
		boxes = append(boxes, box{id, geom.NewAABBAt(b.Pos.TopLeft, b.Pos.Size.X, b.Pos.Size.Y)})
	})
	var out []string
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i].box, boxes[j].box
			if a.TopLeft.X < b.BottomRight.X-1e-6 && b.TopLeft.X < a.BottomRight.X-1e-6 && a.TopLeft.Y < b.BottomRight.Y-1e-6 && b.TopLeft.Y < a.BottomRight.Y-1e-6 {
				out = append(out, fmt.Sprintf("%v %v", a, b))
			}
		}
	}
	return out
}

func TestBodySpacing_AutoPicksBodiesForUnitsSmallAgainstTheCells(t *testing.T) {
	small := newFieldWorld(t, 4, 1, AutoSpacing, nil, []fieldUnit{{at: geom.NewVec(16, 16), side: 4}})
	if got := small.nav.Spacing(); got != BodySpacing {
		t.Errorf("units 4 a side on cells 32 keep apart by %v, want bodies", got)
	}
	large := newFieldWorld(t, 4, 1, AutoSpacing, nil, []fieldUnit{{at: geom.NewVec(16, 16), side: 22}})
	if got := large.nav.Spacing(); got != CellSpacing {
		t.Errorf("units 22 a side on cells 32 keep apart by %v, want cells", got)
	}
}

// A group sent to a point gathers round it: one on it, the others stopping short of their spots
// where they touch one of the group that has arrived.
func TestBodySpacing_AGroupStandsRoundThePointClicked(t *testing.T) {
	var units []fieldUnit
	for i := range 6 {
		units = append(units, fieldUnit{at: geom.NewVec(16+float64(i%2)*10, 40+float64(i)*12), side: 4, selected: true})
	}
	fw := newFieldWorld(t, 8, 5, BodySpacing, nil, units)
	target := fw.at(6, 2)
	c := fw.grid.CellCenter(target)
	point := geom.NewVec(c.X+7, c.Y-5)
	fw.nav.moves.Add(control.Nobody, MoveTo{Cell: target, At: point})
	settled, contacts := fw.run(15 * time.Second)
	if !settled {
		t.Fatal("the group has not settled")
	}
	nearest := math.Inf(1)
	for i := range units {
		at, _ := fw.centre(i)
		nearest = min(nearest, math.Hypot(at.X-point.X, at.Y-point.Y))
		if math.Hypot(at.X-point.X, at.Y-point.Y) > fieldCell {
			t.Errorf("unit %d stands at %v, more than a cell from the point %v", i, at, point)
		}
	}
	if nearest > 4 {
		t.Errorf("the nearest unit stands %v from the point clicked, want one on it, within its side", nearest)
	}
	for range 30 {
		fw.ecs.Tick(time.Second / 60)
	}
	if o := fw.overlaps(); len(o) > 0 {
		t.Errorf("boxes overlap: %v", o)
	}
	if contacts > 80*len(units) {
		t.Errorf("the group struck each other for %d ticks on the way, want a few strikes each", contacts)
	}
}

func TestBodySpacing_TwoHeadOnPassEachOtherAndOneWalksPastAStandingOne(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 5, fieldCell)}
	goal0, goal1 := geom.NewVec(9*fieldCell+16, 80), geom.NewVec(16, 80)
	units := []fieldUnit{
		{at: goal1, side: 6, order: probe.standAt(goal0)},
		{at: goal0, side: 6, order: probe.standAt(goal1)},
		{at: geom.NewVec(5*fieldCell, 80), side: 6}, // standing in the way
	}
	fw := newFieldWorld(t, 10, 5, BodySpacing, nil, units)
	settled, contacts := fw.run(15 * time.Second)
	if !settled {
		t.Fatal("the two have not arrived")
	}
	for i, want := range []geom.Vec{goal0, goal1} {
		if at, _ := fw.centre(i); math.Hypot(at.X-want.X, at.Y-want.Y) > 1e-6 {
			t.Errorf("unit %d stands at %v, want %v", i, at, want)
		}
	}
	if contacts > 40 {
		t.Errorf("they struck someone for %d ticks on the way, want a strike each and past", contacts)
	}
}

// sendCrowd orders n units side a side, laid out as layout says on a 12 x 8 field, to a point off the
// middle of the cell (6, 4), and runs them until they stand; it reports whether they did, the most
// ticks two touched on end, the pairs overlapping at the end and the furthest any stands from the
// point.
func sendCrowd(t *testing.T, n int, layout string, side float64) (settled bool, longest, overlaps int, spread float64) {
	t.Helper()
	var units []fieldUnit
	for i := range n {
		var at geom.Vec
		switch layout {
		case "column from the west":
			at = geom.NewVec(20+float64(i%3)*side*2, 20+float64(i/3)*side*2)
		case "block from the south":
			at = geom.NewVec(150+float64(i%5)*side*2, 240-float64(i/5)*side*2)
		case "scattered":
			at = geom.NewVec(20+math.Mod(float64(i)*97.3, 340), 20+math.Mod(float64(i)*57.1, 220))
		case "from the north-east":
			at = geom.NewVec(360-float64(i%4)*side*2, 16+float64(i/4)*side*2)
		}
		units = append(units, fieldUnit{at: at, side: side, selected: true})
	}
	fw := newFieldWorld(t, 12, 8, BodySpacing, nil, units)
	c := fw.grid.CellCenter(fw.at(6, 4))
	point := geom.NewVec(c.X+5, c.Y-3)
	fw.nav.moves.Add(control.Nobody, MoveTo{Cell: fw.at(6, 4), At: point})
	settled, _, longest = fw.runLongest(30 * time.Second)
	for range 30 { // the last to stop are let settle
		fw.ecs.Tick(time.Second / 60)
	}
	for i := range units {
		at, _ := fw.centre(i)
		spread = max(spread, math.Hypot(at.X-point.X, at.Y-point.Y))
	}
	return settled, longest, len(fw.overlaps()), spread
}

// Groups of every size and from every side stand round the point with nothing overlapping: units a
// tenth of a cell, a fifth, and all but a third. Nobody knows where the others are, so they touch
// on the way and at the end, but only in passing: no two press on each other for as long as
// navigation takes for a stall.
func TestBodySpacing_CrowdsStandRoundThePointWithoutPushing(t *testing.T) {
	for _, n := range []int{4, 9, 16, 25} {
		for _, layout := range []string{"column from the west", "block from the south", "scattered", "from the north-east"} {
			for _, side := range []float64{3, 6, 10} {
				settled, longest, overlaps, spread := sendCrowd(t, n, layout, side)
				spacing := side * (1 + spacingGap)
				if !settled || overlaps > 0 {
					t.Errorf("%d units %v a side, %s: settled %v, %d pairs overlapping", n, side, layout, settled, overlaps)
				}
				if limit := spacing * (2 + math.Sqrt(float64(n))) * 1.5; spread > limit {
					t.Errorf("%d units %v a side, %s: one stands %.0f off the point, want within %.0f", n, side, layout, spread, limit)
				}
				if limit := int(stallAfter / (time.Second / 60)); longest >= limit {
					t.Errorf("%d units %v a side, %s: two touched for %d ticks on end, want under %d: in passing", n, side, layout, longest, limit)
				}
			}
		}
	}
}

// A road east with a longer way round north of it and water south: a unit nearly a cell large
// standing on the road leaves no room to pass, so the unit on its way east goes round by the north.
func TestBodySpacing_OneStandingInTheWayIsGoneRoundAndWithNoWayRoundTheUnitStands(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(9, 3, fieldCell)}
	road := func(b *board.Board, at func(x, y uint32) cell.ID) {
		for x := uint32(0); x < 9; x++ {
			b.Set(at(x, 0), cell.Kind{Cost: 3, Allows: cell.Land})
			b.Set(at(x, 2), cell.Kind{Cost: 1, Allows: cell.Water})
		}
	}
	from, goal := probe.grid.CellCenter(probe.at(0, 1)), probe.grid.CellCenter(probe.at(8, 1))
	blocker := probe.grid.CellCenter(probe.at(4, 1))
	fw := newFieldWorld(t, 9, 3, BodySpacing, road, []fieldUnit{{at: from, side: 6, order: probe.standAt(goal)}, {at: blocker, side: 28}})
	settled, _ := fw.run(30 * time.Second)
	if at, _ := fw.centre(0); !settled || math.Hypot(at.X-goal.X, at.Y-goal.Y) > 1e-6 {
		t.Errorf("the unit stands at %v, settled %v, want round the one in the way to %v", at, settled, goal)
	}
	if o := fw.overlaps(); len(o) > 0 {
		t.Errorf("boxes overlap: %v", o)
	}

	lane := &fieldWorld{grid: grid.DefaultGrids{}.Square(9, 1, fieldCell)}
	from, goal, blocker = lane.grid.CellCenter(lane.at(0, 0)), lane.grid.CellCenter(lane.at(8, 0)), lane.grid.CellCenter(lane.at(4, 0))
	fw = newFieldWorld(t, 9, 1, BodySpacing, nil, []fieldUnit{{at: from, side: 6, order: lane.standAt(goal)}, {at: blocker, side: 28}})
	settled, _ = fw.run(30 * time.Second)
	for range 30 {
		fw.ecs.Tick(time.Second / 60)
	}
	at, _ := fw.centre(0)
	b, _ := fw.centre(1)
	if !settled || at.X >= b.X || len(fw.overlaps()) > 0 {
		t.Errorf("with no way round the unit stands at %v, settled %v, want it given up short of the one in the way at %v", at, settled, b)
	}
	if math.Hypot(b.X-blocker.X, b.Y-blocker.Y) > 6 {
		t.Errorf("the one in the way was pushed to %v from %v: struck, not pushed along", b, blocker)
	}
}

// Two sent alone to one point each go for it, not knowing of the other: the one there first gives
// way to the late one striking it, the late one takes the point, and the first, back to find its
// place taken, stands beside it round the same point.
func TestBodySpacing_ASpotTakenMeanwhileGivesAnother(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 3, fieldCell)}
	goal := probe.grid.CellCenter(probe.at(8, 1))
	fw := newFieldWorld(t, 10, 3, BodySpacing, nil, []fieldUnit{
		{at: probe.grid.CellCenter(probe.at(0, 1)), side: 6, order: probe.standAt(goal)},
		{at: probe.grid.CellCenter(probe.at(6, 0)), side: 6, order: probe.standAt(goal)},
	})
	if settled, _ := fw.run(20 * time.Second); !settled {
		t.Fatal("the two have not settled")
	}
	for range 30 {
		fw.ecs.Tick(time.Second / 60)
	}
	onPoint, beside := 0, 0
	for i := range 2 {
		at, _ := fw.centre(i)
		switch d := math.Hypot(at.X-goal.X, at.Y-goal.Y); {
		case d <= 3:
			onPoint++
		case d <= 3*6*(1+spacingGap):
			beside++
		}
	}
	if onPoint != 1 || beside != 1 {
		t.Errorf("%d stand on the point and %d beside it, want one each", onPoint, beside)
	}
	if o := fw.overlaps(); len(o) > 0 {
		t.Errorf("boxes overlap: %v", o)
	}
}

// A click inside the cell a unit stands on moves it to the point; Shift queues the next point,
// passed on the way; a LookAt stops a unit on the move where its braking ends and turns it.
func TestBodySpacing_ClicksInItsOwnCellShiftAndLookAt(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 3, fieldCell)}
	home := probe.grid.CellCenter(probe.at(1, 1))
	fw := newFieldWorld(t, 10, 3, BodySpacing, nil, []fieldUnit{{at: home, side: 4, selected: true}})
	inside := geom.NewVec(home.X+9, home.Y-8)
	fw.nav.moves.Add(control.Nobody, MoveTo{Cell: fw.at(1, 1), At: inside})
	if settled, _ := fw.run(5 * time.Second); !settled {
		t.Fatal("the unit has not settled in its own cell")
	}
	if at, _ := fw.centre(0); at != inside {
		t.Errorf("clicked inside its own cell the unit stands at %v, want the point %v", at, inside)
	}

	first, last := geom.NewVec(5*fieldCell+7, 1*fieldCell+20), geom.NewVec(8*fieldCell+10, 2*fieldCell+9)
	fw.nav.moves.Add(control.Nobody, MoveTo{Cell: fw.at(5, 1), At: first})
	fw.ecs.Tick(time.Second / 60)
	fw.nav.moves.Add(control.Nobody, MoveTo{Cell: fw.at(8, 2), At: last, Append: true})
	fw.ecs.Tick(time.Second / 60)
	if _, o := fw.centre(0); o == nil || o.Queued != 1 || o.Waypoints[0].Spot != last {
		t.Fatalf("after Shift the order is %+v, want the last point queued as a spot", o)
	}
	if settled, _ := fw.run(10 * time.Second); !settled {
		t.Fatal("the unit has not settled at the last point")
	}
	if at, _ := fw.centre(0); at != last {
		t.Errorf("the unit stands at %v, want the last point %v", at, last)
	}

	fw.nav.moves.Add(control.Nobody, MoveTo{Cell: fw.at(0, 0), At: fw.grid.CellCenter(fw.at(0, 0))})
	for range 60 {
		fw.ecs.Tick(time.Second / 60)
	}
	told, _ := fw.centre(0)
	south := geom.NewVec(told.X, told.Y+1000)
	fw.nav.looks.Add(control.Nobody, LookAt{At: south})
	if settled, _ := fw.run(5 * time.Second); !settled {
		t.Fatal("the unit told to look has not stopped")
	}
	stopped, _ := fw.centre(0)
	braking := 96.0 * 96 / (2 * 384)
	if d := math.Hypot(stopped.X-told.X, stopped.Y-told.Y); d > braking+1 {
		t.Errorf("the unit stopped %.1f from where it was told to look, want its braking %.1f at most", d, braking)
	}
	for range 60 {
		fw.ecs.Tick(time.Second / 60)
	}
	var heading geom.Vec
	fw.each(func(id uid.UID64, b *world.Base, _ cell.ID, _ *MoveOrder, _ *collision.Collider) {
		heading = b.Vel.Dir
	})
	if heading.Y < 0.9 {
		t.Errorf("the unit heads %v, want south towards the point looked at", heading)
	}
}

// A unit walked by hand stops short of one standing ahead and does not push it.
func TestBodySpacing_ByHandAUnitStopsShortOfAnother(t *testing.T) {
	fw := newFieldWorld(t, 10, 3, BodySpacing, nil, []fieldUnit{
		{at: geom.NewVec(40, 48), side: 6, driven: steering.Driven{Ahead: 1, Face: geom.NewVec(1, 0)}},
		{at: geom.NewVec(90, 48), side: 6},
	})
	for range 180 {
		fw.ecs.Tick(time.Second / 60)
	}
	walker, _ := fw.centre(0)
	other, _ := fw.centre(1)
	if other != geom.NewVec(90, 48) {
		t.Errorf("the one standing was pushed to %v", other)
	}
	if walker.X+3 > 87 || walker.X < 70 {
		t.Errorf("the walker stands at %v, want just short of the one ahead at %v", walker, other)
	}
}

// Walking along the shore into one standing in its way, a unit steps round it by the land, never
// over the water: the demo drowns whoever stands where its domain may not.
func TestBodySpacing_StrikingSomeoneByTheShoreItStepsRoundByLand(t *testing.T) {
	shore := func(b *board.Board, at func(x, y uint32) cell.ID) {
		for x := uint32(0); x < 10; x++ {
			b.Set(at(x, 0), cell.Kind{Cost: 1, Allows: cell.Water})
		}
	}
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 3, fieldCell)}
	from, goal := geom.NewVec(16, 36), geom.NewVec(9*fieldCell+16, 36)
	fw := newFieldWorld(t, 10, 3, BodySpacing, shore, []fieldUnit{
		{at: from, side: 6, order: probe.standAt(goal)},
		{at: geom.NewVec(5*fieldCell, 38), side: 6},
	})
	wet := 0
	for tick := 0; tick < 20*60; tick++ {
		fw.ecs.Tick(time.Second / 60)
		if at, _ := fw.centre(0); at.Y-3 < fieldCell {
			wet++
		}
	}
	if wet > 0 {
		t.Errorf("the unit stood over the water for %d ticks", wet)
	}
	if at, o := fw.centre(0); o != nil || math.Hypot(at.X-goal.X, at.Y-goal.Y) > 6*(1+spacingGap) {
		t.Errorf("the unit stands at %v with order %v, want by the goal %v", at, o, goal)
	}
}

// One standing in the way of one on the move, struck by it, steps off its way — square to it, not
// pushed along it — lets it pass and stays there.
func TestBodySpacing_OneStandingStepsAsideAndStays(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(10, 5, fieldCell)}
	from, goal, home := geom.NewVec(16, 80), geom.NewVec(9*fieldCell+16, 80), geom.NewVec(5*fieldCell, 80)
	fw := newFieldWorld(t, 10, 5, BodySpacing, nil, []fieldUnit{
		{at: from, side: 6, order: probe.standAt(goal)},
		{at: home, side: 6},
	})
	gaveWay := false
	for tick := 0; tick < 20*60; tick++ {
		fw.ecs.Tick(time.Second / 60)
		if _, o := fw.centre(1); o != nil && o.GivingWay {
			gaveWay = true
		}
	}
	if !gaveWay {
		t.Error("the one standing in the way never gave way")
	}
	if at, o := fw.centre(0); o != nil || at != goal {
		t.Errorf("the one on the move stands at %v with order %v, want at %v", at, o, goal)
	}
	if at, o := fw.centre(1); o != nil || math.Abs(at.Y-home.Y) < 6 || math.Abs(at.X-home.X) > 3 {
		t.Errorf("the one that gave way stands at %v with order %v, want aside of %v, square off the way", at, o, home)
	}
}
