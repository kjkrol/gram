package navigation

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// profiledWorld is one navigated entity with a given Steering profile on an open square grid,
// ticked at 60 TPS through navigation, steering and movement.
type profiledWorld struct {
	grid    grid.Grid
	terrain *cell.TerrainMap
	ecs     *goke.ECS
	id      uid.UID64
	pos     goke.Comp[world.Base]
	order   goke.OptComp[MoveOrder]
	course  goke.OptComp[steering.Course]
	q       *goke.Query
}

func newProfiledWorld(t *testing.T, w, h uint32, start cell.ID, mt MoveOrder, profile steering.Steering, withSteering bool) *profiledWorld {
	t.Helper()
	pw := &profiledWorld{grid: grid.DefaultGrids{}.Square(w, h, legCellSize)}
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	pw.terrain = terrain
	occupancy := &cell.SingleOccupancy{}
	nav := newNavigationSystem(newPathFinder(pw.grid, terrain, nil, occupancy), pw.grid, terrain, occupancy)
	space := testSpace(t)
	nav.BindSpace(space)

	pw.ecs = goke.New()
	pw.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var at goke.Comp[unit.At]
		var pos goke.Comp[world.Base]
		var order goke.Comp[MoveOrder]
		var steer goke.Comp[steering.Steering]
		var course goke.Comp[steering.Course]
		comps := []goke.Addable{&at, &pos, &order}
		if withSteering {
			comps = append(comps, &steer, &course)
		}
		f := si.NewFactory(comps...)
		f.Create(1)
		f.Next()
		pw.id = f.Cursor.IDs[0]
		at.Slice(&f.Cursor)[0] = unit.At{Cell: start}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: cellBox(pw.grid, start, legEntitySize)}
		order.Slice(&f.Cursor)[0] = mt
		if withSteering {
			steer.Slice(&f.Cursor)[0] = profile
		}
		occupancy.Enter(start, pw.id, cell.Land)
		pw.q = si.NewQueryBuilder(&pw.pos).Optional(&pw.order).Optional(&pw.course).Build()
	}})

	navHandle := pw.ecs.RegSys(nav)
	steeringHandle := pw.ecs.RegSys(steering.NewSystem())
	moveHandle := pw.ecs.RegSys(world.NewMoveSystem(space))
	pw.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(navHandle, d)
		ctx.Run(steeringHandle, d)
		ctx.Run(moveHandle, d)
		ctx.Sync()
	})
	return pw
}

// tick advances one tick and reports the entity's velocity, centre and whether it still has an order.
func (pw *profiledWorld) tick() (vel world.Velocity, centre geom.Vec, ordered bool) {
	pw.ecs.Tick(time.Second / 60)
	pw.q.All()
	for pw.q.Next() {
		cur := pw.q.Cursor()
		b := pw.pos.Slice(cur)[0]
		return b.Vel, b.Pos.Center(), pw.order.Slice(cur) != nil
	}
	return
}

func (pw *profiledWorld) cellAt(x, y uint32) cell.ID {
	c, _ := pw.grid.CellIndex(x, y)
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
	nav := newNavigationSystem(newPathFinder(grid, terrain, nil, occupancy), grid, terrain, occupancy)
	at := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }

	var here goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var order goke.Comp[MoveOrder]
	var profile goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var q *goke.Query
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&here, &pos, &order, &profile, &course)
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
		order.Slice(&f.Cursor)[0] = mt
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
		for _, c := range mt.Leg.cells() {
			occupancy.Enter(c, id, cell.Land)
		}
		q = si.NewQueryBuilder(&here, &order).Build()
	}})
	navHandle := ecs.RegSys(nav)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(navHandle, d); ctx.Sync() })

	ecs.Tick(time.Second / 60)

	c, mt := readCellAndMoveOrder(t, q, &here, &order)
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
	for range 60 * 20 {
		vel, centre, ordered := pw.tick()
		if ordered {
			if vel.Value <= 0 {
				t.Fatalf("Velocity.Value = %v before arriving; braking must not stop the unit short", vel.Value)
			}
			speeds = append(speeds, vel.Value)
			continue
		}
		if vel.Value != 0 {
			t.Errorf("Velocity.Value = %v after arriving, want 0", vel.Value)
		}
		if want := pw.grid.CellCenter(target); centre != want {
			t.Errorf("came to rest at %v, want the goal's centre %v", centre, want)
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

func TestLookahead(t *testing.T) {
	have := geom.NewVec(0, 0)
	route := []geom.Vec{geom.NewVec(10, 0), geom.NewVec(10, 10)}
	for name, tc := range map[string]struct {
		reach float64
		want  geom.Vec
	}{
		"no reach aims at the first waypoint":     {0, geom.NewVec(10, 0)},
		"within the first segment":                {4, geom.NewVec(4, 0)},
		"past the bend":                           {14, geom.NewVec(10, 4)},
		"beyond the route ends at its last point": {40, geom.NewVec(10, 10)},
	} {
		if got := lookahead(have, route, tc.reach); got != tc.want {
			t.Errorf("%s: lookahead(reach %v) = %v, want %v", name, tc.reach, got, tc.want)
		}
	}
	if got := lookahead(have, nil, 5); got != have {
		t.Errorf("an empty route gave %v, want the position itself", got)
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
	for range 60 * 30 {
		vel, centre, ordered := pw.tick()
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
	nav := newNavigationSystem(newPathFinder(grid, terrain, nil, occupancy), grid, terrain, occupancy)
	at := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }

	var here goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var order goke.Comp[MoveOrder]
	var profile goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var q *goke.Query
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&here, &pos, &order, &profile, &course)
		f.Create(1)
		f.Next()
		id := f.Cursor.IDs[0]
		here.Slice(&f.Cursor)[0] = unit.At{Cell: at(1)}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: geomBox(26, 8, 4)} // past cell 2's centre plane, off its centre
		mt := MoveOrder{Target: at(2), Leg: Leg{From: at(1), To: at(2), Active: true}}
		mt.Path.Steps[0], mt.Path.Length = at(2), 1
		mt.Enqueue(Goal{Cell: at(5)})
		order.Slice(&f.Cursor)[0] = mt
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
		for _, c := range mt.Leg.cells() {
			occupancy.Enter(c, id, cell.Land)
		}
		q = si.NewQueryBuilder(&here, &order).Build()
	}})
	h := ecs.RegSys(nav)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(h, d); ctx.Sync() })

	ecs.Tick(time.Second / 60)

	_, mt := readCellAndMoveOrder(t, q, &here, &order)
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

// heading turns the unit to face dir at the given speed, as if already under way.
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
	if slowest > 64*turnCrawl*1.5 {
		t.Errorf("slowest speed through the U-turn was %v, want it down near the crawl (%v)", slowest, 64*turnCrawl)
	}
	if !recovered {
		t.Error("the unit never got back to full speed once it faced its goal")
	}
}

func TestTurnFactor(t *testing.T) {
	east, west, north := geom.NewVec(1, 0), geom.NewVec(-1, 0), geom.NewVec(0, 1)
	for name, tc := range map[string]struct {
		heading, dir geom.Vec
		want         float64
	}{
		"straight on keeps full speed": {east, east, 1},
		"a right angle crawls":         {east, north, turnCrawl},
		"a U-turn crawls":              {east, west, turnCrawl},
		"no heading yet keeps full":    {geom.Vec{}, east, 1},
	} {
		if got := turnFactor(tc.heading, tc.dir); got != tc.want {
			t.Errorf("%s: turnFactor = %v, want %v", name, got, tc.want)
		}
	}
}

func TestPassed_WithinTheLookaheadCountsAsPassed(t *testing.T) {
	from, w := geom.NewVec(0, 0), geom.NewVec(10, 0)
	if passed(geom.NewVec(7, 0), w, from, 0) {
		t.Error("3 short of the plane with no reach counts as passed")
	}
	if !passed(geom.NewVec(7, 0), w, from, 4) {
		t.Error("3 short of the plane but within a reach of 4 does not count as passed")
	}
	if !passed(geom.NewVec(12, 3), w, from, 0) {
		t.Error("beyond the plane does not count as passed")
	}
}

func TestNavigation_ALegIsTurnedRoundWhenTheRouteGoesBack(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occupancy := &cell.SingleOccupancy{}
	nav := newNavigationSystem(newPathFinder(grid, terrain, nil, occupancy), grid, terrain, occupancy)
	at := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }

	var here goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var order goke.Comp[MoveOrder]
	var profile goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var q *goke.Query
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&here, &pos, &order, &profile, &course)
		f.Create(1)
		f.Next()
		id := f.Cursor.IDs[0]
		here.Slice(&f.Cursor)[0] = unit.At{Cell: at(2)}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: geomBox(27, 5, 4)} // just into cell 2, on a leg from 1
		mt := MoveOrder{Target: at(0), Leg: Leg{From: at(1), To: at(2), Active: true}}
		mt.Path.Steps[0], mt.Path.Steps[1], mt.Path.Length = at(1), at(0), 2 // the route back home
		order.Slice(&f.Cursor)[0] = mt
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
		for _, c := range mt.Leg.cells() {
			occupancy.Enter(c, id, cell.Land)
		}
		q = si.NewQueryBuilder(&here, &order).Build()
	}})
	h := ecs.RegSys(nav)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(h, d); ctx.Sync() })

	ecs.Tick(time.Second / 60)

	_, mt := readCellAndMoveOrder(t, q, &here, &order)
	if !mt.Leg.Active || mt.Leg.From != at(2) || mt.Leg.To != at(1) {
		t.Errorf("Leg = %+v, want it turned round to 2 -> 1 so the unit goes straight back", mt.Leg)
	}
}

// A patrol walks to its goals in turn and round again, standing its pause on each, and its order
// never ends.
func TestNavigation_APatrolGoesRoundStandingItsPauseOnEachGoal(t *testing.T) {
	const pause = time.Second
	grid := grid.DefaultGrids{}.Square(8, 1, legCellSize)
	west, _ := grid.CellIndex(0, 0)
	east, _ := grid.CellIndex(7, 0)
	pw := newProfiledWorld(t, 8, 1, west, Patrol(pause, east, west), steering.Steering{MaxSpeed: 64, Accel: 256, V0: 32}, true)

	type stay struct {
		cell  cell.ID
		ticks int
	}
	var stays []stay
	standing := 0
	at := cell.ID(0)
	for range 60 * 30 {
		vel, centre, ordered := pw.tick()
		if !ordered {
			t.Fatal("the patrol's order ended")
		}
		end, ok := grid.CellAt(centre)
		if ok && (end == west || end == east) && vel.Value == 0 {
			standing, at = standing+1, end
			continue
		}
		if standing > 0 {
			stays = append(stays, stay{at, standing})
			standing = 0
		}
	}
	if len(stays) < 3 {
		t.Fatalf("stood at the ends %d times in thirty seconds, want the round walked again and again: %v", len(stays), stays)
	}
	for k, st := range stays {
		want := east
		if k%2 == 1 {
			want = west
		}
		if st.cell != want {
			t.Errorf("stay %d at cell %d, want %d: east, west, east, …", k, st.cell, want)
		}
		if secs := float64(st.ticks) / 60; secs < 0.9*pause.Seconds() || secs > 1.5*pause.Seconds() {
			t.Errorf("stay %d lasted %.2f s, want about the pause, %v", k, secs, pause)
		}
	}
}
