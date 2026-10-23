package navigation

import (
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/navigation/internal/traffic"
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type profiledWorld struct {
	grid      grid.Grid
	terrain   testBoard
	ecs       *goke.ECS
	id        uid.UID64
	pos       goke.Comp[world.Base]
	orderComp goke.OptComp[MoveOrder]
	course    goke.OptComp[steering.Course]
	q         *goke.Query
}

func newProfiledWorld(t *testing.T, w, h uint32, start cell.ID, mt MoveOrder, profile steering.Steering, withSteering bool) *profiledWorld {
	t.Helper()
	pw := &profiledWorld{}
	u := navUnit{box: cellBox(grid.DefaultGrids{}.Square(w, h, legCellSize), start, legEntitySize), at: start, order: &mt}
	if withSteering {
		u.profile = &profile
	}
	nw := newNavWorld(t, w, h, legCellSize, []navUnit{u}, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		pw.q = si.NewQueryBuilder(&pw.pos).Optional(&pw.orderComp).Optional(&pw.course).Build()
	}})
	pw.grid, pw.terrain, pw.ecs, pw.id = nw.grid, nw.board, nw.ecs, nw.ids[0]
	return pw
}

func (pw *profiledWorld) tick() (vel world.Velocity, centre geom.Vec, ordered bool) {
	pw.ecs.Tick(time.Second / 60)
	pw.q.All()
	for pw.q.Next() {
		cur := pw.q.Cursor()
		b := pw.pos.Slice(cur)[0]
		return b.Vel, b.Pos.Center(), pw.orderComp.Slice(cur) != nil
	}
	return
}

func (pw *profiledWorld) cellAt(x, y uint32) cell.ID {
	c := pw.grid.At(x, y)
	return c
}

func TestNavigation_TurnsBeforeTheBendAndNeverStops(t *testing.T) {
	const turnRate = 0.1
	pw := newProfiledWorld(t, 6, 6, cell.ID(0), MoveOrder{}, steering.Steering{}, true)
	pw = newProfiledWorld(t, 6, 6, pw.cellAt(0, 2), MoveOrder{Target: pw.cellAt(5, 4)}, steering.Steering{MaxSpeed: 64, TurnRate: turnRate}, true)

	var prev float64
	haveHeading := false
	turned := 0.0
	for tick := range 60 * 20 {
		vel, _, ordered := pw.tick()
		if tick == 0 {
			continue // the world steps before navigation: an order is carried out from the next tick
		}
		if !ordered {
			if turned < 0.2 {
				t.Errorf("arrived having turned only %.2f rad in total; the route should have bent", turned)
			}
			return
		}
		if vel.Value <= 0 {
			t.Fatalf("tick %d: Velocity.Value = %v on the way; a unit must never stop between waypoints", tick, vel.Value)
		}
		h := math.Atan2(vel.Dir.Y, vel.Dir.X)
		if haveHeading {
			delta := math.Abs(math.Mod(h-prev+3*math.Pi, 2*math.Pi) - math.Pi)
			if delta > turnRate+1e-9 {
				t.Fatalf("tick %d: heading swung by %.3f rad in one tick, want at most TurnRate %.3f", tick, delta, turnRate)
			}
			turned += delta
		}
		prev, haveHeading = h, true
	}
	t.Fatal("never arrived")
}

func TestNavigation_PassesAWaypointByProjectionNotDistance(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occupancy := &cell.SingleOccupancy{}
	nav := routed(spacing.NewFinder(grid, terrain, nil, occupancy), grid, terrain,
		occupancy)
	at := func(x uint32) cell.ID { c := grid.At(x, 0); return c }

	var here goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var orderComp goke.Comp[MoveOrder]
	var profile goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var q *goke.Query
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&here, &pos, &orderComp, &profile, &course)
		f.Create(1)
		f.Next()
		id := f.Cursor.IDs[0]
		here.Slice(&f.Cursor)[0] = unit.At{Cell: at(1)}
		// past the plane through cell 2's centre (x = 25), yet 3.2 units from that centre
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: geomBox(26, 8, 4)}
		var mt MoveOrder
		mt.Target = at(3)
		mt.Path.Steps[0], mt.Path.Steps[1] = at(2), at(3)
		mt.Path.Length = 2
		mt.Leg = Leg{From: at(1), To: at(2), Active: true}
		orderComp.Slice(&f.Cursor)[0] = mt
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
		cells, n := order.Cells(mt.Leg)
		for _, c := range cells[:n] {
			occupancy.Enter(c, false, id, cell.Land)
		}
		q = si.NewQueryBuilder(&here, &orderComp).Build()
	}})
	navHandle := ecs.RegSys(nav)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(navHandle, d); ctx.Sync() })

	ecs.Tick(time.Second / 60)

	c, mt := readCellAndMoveOrder(t, q, &here, &orderComp)
	if mt.Path.Index != 1 {
		t.Errorf("Path.Index = %d, want 1: the waypoint behind the plane counts as passed", mt.Path.Index)
	}
	if mt.Leg.Active {
		t.Error("the leg onto the passed waypoint is still active")
	}
	if c.Cell != at(2) {
		t.Errorf("Cell = %v, want %v", c.Cell, at(2))
	}
}

func TestNavigation_BrakesToRestOnTheGoal(t *testing.T) {
	pw := newProfiledWorld(t, 6, 1, cell.ID(0), MoveOrder{}, steering.Steering{}, true)
	target := pw.cellAt(5, 0)
	pw = newProfiledWorld(t, 6, 1, pw.cellAt(0, 0), MoveOrder{Target: target}, steering.Steering{MaxSpeed: 64, Accel: 128, V0: 16}, true)

	var speeds []float64
	for tick := range 60 * 20 {
		vel, centre, ordered := pw.tick()
		if tick == 0 {
			continue // the world steps before navigation: an order is carried out from the next tick
		}
		if ordered {
			if vel.Value <= 0 {
				t.Fatalf("Velocity.Value = %v before arriving; braking must not stop the unit short", vel.Value)
			}
			speeds = append(speeds, vel.Value)
			continue
		}
		arrivedAt := centre
		vel, centre, _ = pw.tick() // the stop is carried out by the world's next step
		if vel.Value != 0 {
			t.Errorf("Velocity.Value = %v a tick after arriving, want 0", vel.Value)
		}
		if want := pw.grid.CellCenter(target); centre != want || arrivedAt != want {
			t.Errorf("arrived at %v and came to rest at %v, want both at the goal's centre %v", arrivedAt, centre, want)
		}
		n := len(speeds)
		if n < 3 || speeds[n-1] >= speeds[n-3] {
			t.Errorf("last speeds %v: want them falling as the goal comes up", speeds[max(0, n-4):])
		}
		if peak := maxOf(speeds); peak != 64 {
			t.Errorf("peak speed %v, want the profile's 64 on the way", peak)
		}
		return
	}
	t.Fatal("never arrived")
}

func TestNavigation_LeavesAloneAnEntityWithoutSteering(t *testing.T) {
	pw := newProfiledWorld(t, 5, 1, cell.ID(0), MoveOrder{}, steering.Steering{}, true)
	start, target := pw.cellAt(0, 0), pw.cellAt(4, 0)
	pw = newProfiledWorld(t, 5, 1, start, MoveOrder{Target: target}, steering.Steering{}, false)

	before := pw.grid.CellCenter(start)
	for range 30 {
		vel, centre, ordered := pw.tick()
		if !ordered || centre != before || vel.Value != 0 {
			t.Fatalf("an entity without Steering was navigated: ordered=%v centre=%v vel=%v", ordered, centre, vel)
		}
	}
}

func geomBox(cx, cy, size float64) plane.AABB {
	return plane.NewAABB(geom.NewVec(cx-size/2, cy-size/2), size, size)
}

func maxOf(xs []float64) float64 {
	m := math.Inf(-1)
	for _, x := range xs {
		m = max(m, x)
	}
	return m
}

func TestNavigation_RunsThroughQueuedGoalsWithoutStopping(t *testing.T) {
	pw := newProfiledWorld(t, 8, 1, cell.ID(0), MoveOrder{}, steering.Steering{}, true)
	mid, last := pw.cellAt(3, 0), pw.cellAt(6, 0)
	mt := MoveOrder{Target: mid}
	mt.Enqueue(Goal{Cell: last})
	pw = newProfiledWorld(t, 8, 1, pw.cellAt(0, 0), mt, steering.Steering{MaxSpeed: 64, Accel: 128, V0: 16}, true)

	passedMid := false
	for tick := range 60 * 30 {
		vel, centre, ordered := pw.tick()
		if tick == 0 {
			continue // the world steps before navigation: an order is carried out from the next tick
		}
		if !ordered {
			if !passedMid {
				t.Fatal("the order ended before the unit had gone past the first goal")
			}
			if want := pw.grid.CellCenter(last); centre != want {
				t.Errorf("came to rest at %v, want the last goal's centre %v", centre, want)
			}
			return
		}
		if vel.Value <= 0 {
			t.Fatalf("Velocity.Value = %v while still ordered; a queued goal must not stop the unit", vel.Value)
		}
		if centre.X > pw.grid.CellCenter(mid).X {
			passedMid = true
		}
	}
	t.Fatal("never arrived")
}

func TestNavigation_QueuedGoalIsPassedByProjection(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 1, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occupancy := &cell.SingleOccupancy{}
	nav := routed(spacing.NewFinder(grid, terrain, nil, occupancy), grid, terrain,
		occupancy)
	at := func(x uint32) cell.ID { c := grid.At(x, 0); return c }

	var here goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var orderComp goke.Comp[MoveOrder]
	var profile goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var q *goke.Query
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&here, &pos, &orderComp, &profile, &course)
		f.Create(1)
		f.Next()
		id := f.Cursor.IDs[0]
		here.Slice(&f.Cursor)[0] = unit.At{Cell: at(1)}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: geomBox(26, 8, 4)} // past cell 2's centre plane, off its centre
		mt := MoveOrder{Target: at(2), Leg: Leg{From: at(1), To: at(2), Active: true}}
		mt.Path.Steps[0], mt.Path.Length = at(2), 1
		mt.Enqueue(Goal{Cell: at(5)})
		orderComp.Slice(&f.Cursor)[0] = mt
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
		cells, n := order.Cells(mt.Leg)
		for _, c := range cells[:n] {
			occupancy.Enter(c, false, id, cell.Land)
		}
		q = si.NewQueryBuilder(&here, &orderComp).Build()
	}})
	h := ecs.RegSys(nav)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(h, d); ctx.Sync() })

	ecs.Tick(time.Second / 60)

	_, mt := readCellAndMoveOrder(t, q, &here, &orderComp)
	if mt.Target != at(5) || mt.Queued != 0 {
		t.Errorf("order = target %v, queued %d; want the queued goal promoted and the queue empty", mt.Target, mt.Queued)
	}
	if mt.Path.Length != 0 {
		t.Errorf("Path.Length = %d, want 0 so the route to the new goal is planned afresh", mt.Path.Length)
	}
	if mt.Leg.Active {
		t.Error("the leg onto the passed goal is still active")
	}
}

// as if already under way
func (pw *profiledWorld) heading(dir geom.Vec, speed float64) {
	pw.q.All()
	for pw.q.Next() {
		cur := pw.q.Cursor()
		pw.pos.Slice(cur)[0].Vel = world.Velocity{Dir: dir, Value: speed}
		if courses := pw.course.Slice(cur); courses != nil {
			courses[0].Speed = speed
		}
	}
}

func TestNavigation_ASharpTurnSlowsTheUnit(t *testing.T) {
	pw := newProfiledWorld(t, 8, 1, cell.ID(0), MoveOrder{}, steering.Steering{}, true)
	// under way westwards at full speed, with the goal to the east: a U-turn before anything else
	profile := steering.Steering{MaxSpeed: 64, Accel: 400, V0: 64, TurnRate: 0.1}
	pw = newProfiledWorld(t, 8, 1, pw.cellAt(2, 0), MoveOrder{Target: pw.cellAt(7, 0)}, profile, true)
	pw.heading(geom.NewVec(-1, 0), 64)

	slowest, slowestAt := 64.0, 0
	recovered := false
	for tick := range 60 * 20 {
		vel, _, ordered := pw.tick()
		if !ordered {
			break
		}
		if tick < 60 && vel.Value < slowest {
			slowest, slowestAt = vel.Value, tick
		}
		if tick > slowestAt && vel.Value == 64 {
			recovered = true
		}
	}
	if slowest > 64*traffic.TurnCrawl*1.5 {
		t.Errorf("slowest speed through the U-turn was %v, want it down near the crawl (%v)", slowest, 64*traffic.TurnCrawl)
	}
	if !recovered {
		t.Error("the unit never got back to full speed once it faced its goal")
	}
}

func TestNavigation_ALegIsTurnedRoundWhenTheRouteGoesBack(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occupancy := &cell.SingleOccupancy{}
	nav := routed(spacing.NewFinder(grid, terrain, nil, occupancy), grid, terrain,
		occupancy)
	at := func(x uint32) cell.ID { c := grid.At(x, 0); return c }

	var here goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var orderComp goke.Comp[MoveOrder]
	var profile goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var q *goke.Query
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&here, &pos, &orderComp, &profile, &course)
		f.Create(1)
		f.Next()
		id := f.Cursor.IDs[0]
		here.Slice(&f.Cursor)[0] = unit.At{Cell: at(2)}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: geomBox(27, 5, 4)} // just into cell 2, on a leg from 1
		mt := MoveOrder{Target: at(0), Leg: Leg{From: at(1), To: at(2), Active: true}}
		mt.Path.Steps[0], mt.Path.Steps[1], mt.Path.Length = at(1), at(0), 2 // the route back home
		orderComp.Slice(&f.Cursor)[0] = mt
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
		cells, n := order.Cells(mt.Leg)
		for _, c := range cells[:n] {
			occupancy.Enter(c, false, id, cell.Land)
		}
		q = si.NewQueryBuilder(&here, &orderComp).Build()
	}})
	h := ecs.RegSys(nav)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(h, d); ctx.Sync() })

	ecs.Tick(time.Second / 60)

	_, mt := readCellAndMoveOrder(t, q, &here, &orderComp)
	if !mt.Leg.Active || mt.Leg.From != at(2) || mt.Leg.To != at(1) {
		t.Errorf("Leg = %+v, want it turned round to 2 -> 1 so the unit goes straight back", mt.Leg)
	}
}
