package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// A road one cell wide through a field, SingleOccupancy and collision as the board demos have them: the
// scene of the reported deadlock, where units pushed each other for ever.
type roadUnit struct {
	start, target board.CellID
	ordered       bool
	domain        board.Domain // zero: Land
	wide          bool         // the hawk's profile: faster, turning slower, looking further ahead
	selected      bool         // Selectable and Selected, for the commands of a player
}

type roadWorld struct {
	t     *testing.T
	grid  board.Grid
	ecs   *goke.ECS
	nav   *Plugin
	cell  goke.Comp[board.Cell]
	base  goke.Comp[world.Base]
	order goke.OptComp[MoveOrder]
	q     *goke.Query
	kinds map[uid.UID64]kind.ID
	byRow map[int]uid.UID64 // row index → entity, in the order given
}

const roadCell = 32

func newRoadWorld(t *testing.T, width uint32, units []roadUnit) *roadWorld {
	t.Helper()
	rw := &roadWorld{t: t, grid: board.DefaultGrids{}.Square(width, 3, roadCell), byRow: map[int]uid.UID64{}}
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width * roadCell, Height: 3 * roadCell},
		Entities: world.EntitiesCfg{MaxCount: len(units), MinSize: 22, MaxSize: 22},
	})
	occupancy := &board.SingleOccupancy{}
	c := collision.NewPlugin(w)
	brd := board.NewPlugin(rw.grid, occupancy, w)
	brd.Res.Logic.Board.SetAll(board.CellKind{Cost: 2, Allows: board.Land | board.Air}) // field
	for x := uint32(0); x < width; x++ {
		brd.Res.Logic.Board.Set(rw.at(x, 1), board.CellKind{Cost: 1, Allows: board.Land | board.Air}) // the road
	}
	sel := selection.NewPlugin(w)
	rw.nav = NewPlugin(brd, w, sel).WithCollision(c)

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

	spec := func(ordered bool, domain board.Domain, wide, selected bool) kind.Spec {
		if domain == 0 {
			domain = board.Land
		}
		profile := steering.Steering{MaxSpeed: 96, Accel: 192, Brake: 384, V0: 48, TurnRate: 0.15}
		if wide {
			profile.MaxSpeed, profile.TurnRate = 144, 0.1
		}
		s := kind.Spec{
			comp.Load(func(u roadUnit) world.Position { return world.Position{AABB: board.CellAABB(rw.grid, u.start, 22)} }),
			comp.Const(world.Velocity{}),
			comp.Const(profile),
			comp.Load(func(u roadUnit) board.Cell { return board.Cell{ID: u.start} }),
			comp.Const(collision.Collider{}),
			comp.Const(world.Layers(domain)),
			comp.Const(collision.Physics{}),
			comp.Const(board.Mover{Domain: domain}),
		}
		if ordered {
			s = append(s, comp.Load(func(u roadUnit) MoveOrder { return MoveOrder{Target: u.target} }))
		}
		if selected {
			s = append(s, comp.Tagged(sel.Tags().Selectable, sel.Tags().Selected))
		}
		return s
	}
	kindIDs := make([]kind.ID, len(units))
	for i, u := range units {
		k := kind.Define[roadUnit](w.Kinds(), string(rune('a'+i)), spec(u.ordered, u.domain, u.wide, u.selected))
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

func (rw *roadWorld) at(x, y uint32) board.CellID { c, _ := rw.grid.CellIndex(x, y); return c }

// state is one unit's cell and order, if it still has one.
func (rw *roadWorld) state(id uid.UID64) (cell board.CellID, order *MoveOrder) {
	for rw.q.All(); rw.q.Next(); {
		cur := rw.q.Cursor()
		for i, got := range cur.IDs {
			if got != id {
				continue
			}
			cell = rw.cell.Slice(cur)[i].ID
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
	last := map[uid.UID64][]board.CellID{}
	for ticks = 0; time.Duration(ticks)*time.Second/60 < limit; ticks++ {
		rw.ecs.Tick(time.Second / 60)
		done := true
		for row, u := range units {
			if !u.ordered {
				continue
			}
			id := rw.byRow[row]
			cell, o := rw.state(id)
			if o == nil {
				if cell != u.target && rw.grid.Distance(cell, u.target) > 1.5 {
					rw.t.Fatalf("unit %d lost its order at %v, neither on nor beside its target %v", id, cell, u.target)
				}
				continue
			}
			done = false
			steps := append([]board.CellID(nil), o.Path.Steps[:o.Path.Length]...)
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

func TestBump_ATargetSomeoneStandsOnIsSettledBeside(t *testing.T) {
	rw := newRoadWorld(t, 6, []roadUnit{{start: 0, ordered: true}})
	units := []roadUnit{
		{start: rw.at(0, 1), target: rw.at(4, 1), ordered: true},
		{start: rw.at(4, 1)}, // standing on the target, going nowhere
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
		{start: rw.at(0, 1), target: rw.at(7, 1), ordered: true, domain: board.Air},
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
	units := []roadUnit{{start: rw.at(0, 0), target: rw.at(23, 1), ordered: true, domain: board.Air, wide: true}}
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
