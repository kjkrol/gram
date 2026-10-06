package navigation

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/plan"
	"github.com/kjkrol/uid"
)

// A road one cell wide through a field, SingleOccupancy and collision as the board demos have them: the
// scene of the reported deadlock, where units pushed each other for ever.
type roadUnit struct {
	start, target cell.ID
	ordered       bool
	domain        cell.Domain      // zero: Land
	wide          bool             // the hawk's profile: faster, turning slower, looking further ahead
	selected      bool             // Selectable and Selected, for the commands of a player
	owner         control.PlayerID // who owns it; nobody for Nobody
	plan          *writtenPlan     // it acts by this plan
	group         uint32           // the group of its order
	sensor        bool             // a Collider without Physics: only ever detected
	loose         bool             // no At, no Mover: a body in the world, not a unit of the board
}

type roadWorld struct {
	t     *testing.T
	grid  grid.Grid
	ecs   *goke.ECS
	nav   *Plugin
	cell  goke.Comp[unit.At]
	base  goke.Comp[world.Base]
	order goke.OptComp[MoveOrder]
	q     *goke.Query
	kinds map[uid.UID64]kind.ID
	byRow map[int]uid.UID64 // row index → entity, in the order given
}

const roadCell = 32

func newRoadWorld(t *testing.T, width uint32, units []roadUnit) *roadWorld {
	t.Helper()
	rw := &roadWorld{t: t, grid: grid.DefaultGrids{}.Square(width, 3, roadCell), byRow: map[int]uid.UID64{}}
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width * roadCell, Height: 3 * roadCell},
		Entities: world.EntitiesCfg{MaxCount: len(units), MinSize: 22, MaxSize: 22},
	})
	occupancy := &cell.SingleOccupancy{}
	c := collision.NewPlugin(w)
	brd := board.NewPlugin(rw.grid, occupancy, w)
	brd.Res.Logic.Board.SetAll(cell.Kind{Cost: 2, Allows: cell.Land | cell.Air}) // field
	for x := uint32(0); x < width; x++ {
		brd.Res.Logic.Board.Set(rw.at(x, 1), cell.Kind{Cost: 1, Allows: cell.Land | cell.Air}) // the road
	}
	sel := selection.NewPlugin(w)
	rw.nav = NewPlugin(brd, w, sel).WithCollision(c)
	if err := w.Carry(rw.nav); err != nil { // as the engine does with Use
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
	if err := rw.nav.Install(ctx); err != nil {
		t.Fatal(err)
	}

	spec := func(ordered bool, domain cell.Domain, wide, selected bool, u roadUnit) kind.Spec {
		if domain == 0 {
			domain = cell.Land
		}
		profile := steering.Steering{MaxSpeed: 96, Accel: 192, Brake: 384, V0: 48, TurnRate: 0.15}
		if wide {
			profile.MaxSpeed, profile.TurnRate = 144, 0.1
		}
		s := kind.Spec{
			comp.Load(func(u roadUnit) world.Position { return world.Position{AABB: cellBox(rw.grid, u.start, 22)} }),
			comp.Const(world.Velocity{}),
			comp.Const(profile),
			comp.Const(collision.Collider{}),
			comp.Const(world.Layers(domain)),
		}
		if !u.loose {
			s = append(s, comp.Load(func(u roadUnit) unit.At { return unit.At{Cell: u.start} }), comp.Const(unit.Mover{Domain: domain}))
		}
		if !u.sensor {
			physics := collision.Physics{}
			if u.loose { // scenery: nothing shifts it
				physics.Mass = math.Inf(1)
			}
			s = append(s, comp.Const(physics))
		}
		if ordered {
			s = append(s, comp.Load(func(u roadUnit) MoveOrder { return MoveOrder{Target: u.target, Group: u.group} }))
		}
		if selected {
			s = append(s, comp.Tagged(sel.Tags().Selectable, sel.Tags().Selected))
		}
		if u.owner != control.Nobody {
			s = append(s, comp.Tagged(owner.Of(u.owner)))
		}
		if u.plan != nil {
			w.Plans().Define(u.plan.name, u.plan.body)
			s = append(s, w.Plans().Named(u.plan.name))
		}
		return s
	}
	kindIDs := make([]kind.ID, len(units))
	for i, u := range units {
		kind.Define[roadUnit](w.Kinds(), string(rune('a'+i)), spec(u.ordered, u.domain, u.wide, u.selected, u))
		k := kind.Named[roadUnit](w.Kinds(), string(rune('a'+i)))
		kindIDs[i] = k.ID()
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
		rw.q = si.NewQueryBuilder(&rw.cell, &rw.base).Optional(&rw.order).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		c.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		rw.nav.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	})
	rw.ecs = ctx.ecs
	for rw.q.All(); rw.q.Next(); {
		cur := rw.q.Cursor()
		for i, id := range cur.IDs {
			for row, k := range kindIDs {
				if rw.base.Slice(cur)[i].TypeID == k {
					rw.byRow[row] = id
				}
			}
		}
	}
	return rw
}

func (rw *roadWorld) at(x, y uint32) cell.ID { c := rw.grid.CellIndex(x, y); return c }

// state is one unit's cell and order, if it still has one.
func (rw *roadWorld) state(id uid.UID64) (cell cell.ID, order *MoveOrder) {
	for rw.q.All(); rw.q.Next(); {
		cur := rw.q.Cursor()
		for i, got := range cur.IDs {
			if got != id {
				continue
			}
			cell = rw.cell.Slice(cur)[i].Cell
			if orders := rw.order.Slice(cur); orders != nil {
				o := orders[i]
				order = &o
			}
		}
	}
	return
}

// run ticks until every ordered unit has arrived — on its target, or beside it when someone stands
// there — or the time is up; it reports the ticks taken and how many times each unit's route changed.
func (rw *roadWorld) run(units []roadUnit, limit time.Duration) (ticks int, replans map[uid.UID64]int) {
	replans = map[uid.UID64]int{}
	last := map[uid.UID64][]cell.ID{}
	for ticks = 0; time.Duration(ticks)*time.Second/60 < limit; ticks++ {
		rw.ecs.Tick(time.Second / 60)
		done := true
		for row, u := range units {
			if !u.ordered {
				continue
			}
			id := rw.byRow[row]
			here, o := rw.state(id)
			if o == nil {
				if here != u.target && rw.grid.Distance(here, u.target) > 1.5 {
					rw.t.Fatalf("unit %d lost its order at %v, neither on nor beside its target %v", id, here, u.target)
				}
				continue
			}
			done = false
			steps := append([]cell.ID(nil), o.Path.Steps[:o.Path.Length]...)
			if !equalSteps(steps, last[id]) {
				replans[id]++
				last[id] = steps
			}
		}
		if done {
			return ticks, replans
		}
	}
	return ticks, replans
}

func TestBump_TwoUnitsHeadOnOnARoadPassEachOther(t *testing.T) {
	units := []roadUnit{{start: 0, ordered: true}, {start: 0, ordered: true}}
	rw := newRoadWorld(t, 10, units)
	units[0] = roadUnit{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true}
	units[1] = roadUnit{start: rw.at(9, 1), target: rw.at(0, 1), ordered: true}
	rw = newRoadWorld(t, 10, units)

	ticks, replans := rw.run(units, 10*time.Second)
	if ticks >= 60*10 {
		t.Fatalf("the two units did not both arrive within 10 s; replans %v", replans)
	}
	for id, n := range replans {
		if n > 10 {
			t.Errorf("unit %d re-planned %d times, want a handful: one per bump, at most one per bumpInterval", id, n)
		}
	}
}

func TestBump_ThreeInARowUntangle(t *testing.T) {
	rw := newRoadWorld(t, 12, []roadUnit{{start: 0, ordered: true}})
	units := []roadUnit{
		{start: rw.at(0, 1), target: rw.at(11, 1), ordered: true},
		{start: rw.at(1, 1), target: rw.at(11, 1), ordered: true},
		{start: rw.at(11, 1), target: rw.at(0, 1), ordered: true},
	}
	rw = newRoadWorld(t, 12, units)
	if ticks, replans := rw.run(units, 12*time.Second); ticks >= 60*12 {
		t.Fatalf("three units did not all arrive within 12 s; replans %v", replans)
	}
}

// A stranger standing on a unit's target never makes way: the unit stands beside it.
func TestBump_ATargetAStrangerStandsOnIsSettledBeside(t *testing.T) {
	rw := newRoadWorld(t, 6, []roadUnit{{start: 0, ordered: true}})
	units := []roadUnit{
		{start: rw.at(0, 1), target: rw.at(4, 1), ordered: true, owner: 1},
		{start: rw.at(4, 1), owner: 2}, // standing on the target, going nowhere
	}
	rw = newRoadWorld(t, 6, units)
	for tick := range 60 * 4 {
		rw.ecs.Tick(time.Second / 60)
		cell, o := rw.state(rw.byRow[0])
		if o == nil {
			if cell == rw.at(4, 1) || rw.grid.Distance(cell, rw.at(4, 1)) > 1.5 {
				t.Fatalf("tick %d: settled at %v, want a cell beside the occupied target", tick, cell)
			}
			return
		}
	}
	t.Fatal("the unit never settled beside the occupied target within 4 s")
}

// A hawk over a road full of walkers flies straight through: they hold the Land layer of their
// cells, it moves in Air, and neither the planner, the leg nor the solver mind the other.
func TestBump_AFlyerPassesOverWalkersUntouched(t *testing.T) {
	rw := newRoadWorld(t, 8, []roadUnit{{start: 0, ordered: true}})
	units := []roadUnit{
		{start: rw.at(0, 1), target: rw.at(7, 1), ordered: true, domain: cell.Air},
		{start: rw.at(3, 1)}, // walkers parked across the road
		{start: rw.at(4, 1)},
		{start: rw.at(5, 1)},
	}
	rw = newRoadWorld(t, 8, units)
	hawk := rw.byRow[0]
	_, o := rw.state(hawk)
	if o == nil {
		t.Fatal("the hawk has no order")
	}
	ticks, replans := rw.run(units[:1], 6*time.Second)
	if ticks >= 60*6 {
		t.Fatalf("the hawk did not reach the far end within 6 s; replans %v", replans)
	}
	if replans[hawk] > 1 {
		t.Errorf("the hawk re-planned %d times, want its first straight route kept: nothing on the Land layer stops it", replans[hawk])
	}
	for _, row := range []int{1, 2, 3} {
		if cell, _ := rw.state(rw.byRow[row]); cell != units[row].start {
			t.Errorf("walker %d was moved to %v by the hawk passing over", row, cell)
		}
	}
}

// A wide turner looks further ahead than half a cell, so it passes a cell of its route before its
// centre is in it; it must keep the route it was given — here one row down along the way — rather
// than find itself short of its cell every step and plan again.
func TestNavigation_AWideTurnerKeepsItsRoute(t *testing.T) {
	rw := newRoadWorld(t, 24, []roadUnit{{start: 0, ordered: true}})
	units := []roadUnit{{start: rw.at(0, 0), target: rw.at(23, 1), ordered: true, domain: cell.Air, wide: true}}
	rw = newRoadWorld(t, 24, units)
	hawk := rw.byRow[0]
	ticks, replans := rw.run(units, 10*time.Second)
	if ticks >= 60*10 {
		t.Fatalf("the hawk did not arrive within 10 s; replans %v", replans)
	}
	if replans[hawk] > 1 {
		t.Errorf("the hawk's route changed %d times on the way, want the first one kept", replans[hawk]-1)
	}
}

// heading is the unit's heading after ticking for d.
func (rw *roadWorld) heading(id uid.UID64, d time.Duration) geom.Vec {
	for range int(d / (time.Second / 60)) {
		rw.ecs.Tick(time.Second / 60)
	}
	for rw.q.All(); rw.q.Next(); {
		cur := rw.q.Cursor()
		for i, got := range cur.IDs {
			if got == id {
				return rw.base.Slice(cur)[i].Vel.Dir
			}
		}
	}
	return geom.Vec{}
}

func TestLook_AClickOnTheUnitsOwnCellTurnsItThere(t *testing.T) {
	rw := newRoadWorld(t, 1, nil)
	units := []roadUnit{{start: 0, selected: true}}
	rw = newRoadWorld(t, 6, units)
	units[0].start = rw.at(2, 1)
	rw = newRoadWorld(t, 6, units)
	unit := rw.byRow[0]
	centre := rw.grid.CellCenter(rw.at(2, 1))
	north := geom.NewVec(centre.X+3, centre.Y-12) // inside its own cell

	rw.nav.moves.Add(control.Nobody, MoveTo{Cell: rw.at(2, 1), At: north})
	dir := rw.heading(unit, 2*time.Second)
	if cell, o := rw.state(unit); o != nil || cell != rw.at(2, 1) {
		t.Fatalf("after the click the unit is on %v with order %v, want it still on its cell with none", cell, o)
	}
	if dir.Y > -0.9 {
		t.Errorf("heading %v, want it turned north towards the point clicked", dir)
	}
}

func TestLook_LookAtStopsAWalkingUnitAtTheEndOfItsStepAndTurnsIt(t *testing.T) {
	rw := newRoadWorld(t, 1, nil)
	units := []roadUnit{{start: 0, selected: true, ordered: true}}
	rw = newRoadWorld(t, 12, units)
	units[0].start, units[0].target = rw.at(0, 1), rw.at(11, 1)
	rw = newRoadWorld(t, 12, units)
	unit := rw.byRow[0]

	rw.heading(unit, 500*time.Millisecond) // under way
	here, _ := rw.state(unit)
	south := geom.NewVec(rw.grid.CellCenter(here).X, 3000) // far south, whichever cell the step ends on
	rw.nav.looks.Add(control.Nobody, LookAt{At: south})
	dir := rw.heading(unit, 3*time.Second)

	cell, o := rw.state(unit)
	if o != nil {
		t.Fatalf("the unit still has an order %+v, want it stopped", *o)
	}
	if rw.grid.Distance(cell, here) > 1.5 {
		t.Errorf("the unit stopped on %v, %v cells from where it was told to look; want the end of its step", cell, rw.grid.Distance(cell, here))
	}
	if dir.Y < 0.9 {
		t.Errorf("heading %v, want it turned south towards the point", dir)
	}
}

// A contact only sensed — a sensor, a shot — bumps nobody: the unit under orders walks on through
// it on its first route and arrives; a body in the way, one the planner knew nothing of, still bumps
// it into stopping and planning again (and again: the planner never sees a body off the occupancy).
func TestBump_ASensedContactBumpsNobody(t *testing.T) {
	for _, tc := range []struct {
		name   string
		sensor bool
		bumped bool
	}{{"a sensor on the road", true, false}, {"a body on the road", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			rw := newRoadWorld(t, 10, []roadUnit{{start: 0}})
			units := []roadUnit{
				{start: rw.at(0, 1), target: rw.at(9, 1), ordered: true},
				{start: rw.at(3, 1), sensor: tc.sensor, loose: true},
			}
			rw = newRoadWorld(t, 10, units)
			ticks, replans := rw.run(units, 10*time.Second)
			if bumped := replans[rw.byRow[0]] > 1; bumped != tc.bumped {
				t.Errorf("the walker went round what it struck: %v, want %v (replans %v)", bumped, tc.bumped, replans)
			}
			if arrived := ticks < 60*10; arrived == tc.bumped {
				t.Errorf("the walker arrived within 10 s: %v, want %v (through a sensor, never past a body)", arrived, !tc.bumped)
			}
		})
	}
}

// writtenPlan is a plan as a test writes it, for the road's world to define (world.Plans).
type writtenPlan struct {
	name string
	body func(a *plan.Actor) rule.Step
}
