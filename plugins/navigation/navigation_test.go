package navigation

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

func testSpace(t *testing.T) *aabbworld.Space {
	t.Helper()
	space, err := aabbworld.NewSpace(aabbworld.Config{
		Width: 1000, Height: 1000,
		BucketSize: 64,
	})
	if err != nil {
		t.Fatalf("aabbworld.NewSpace: %v", err)
	}
	return space
}

// pushOnce jumps every entity's Position to a fixed cell the first time it is armed and run.
type pushOnce struct {
	grid  grid.Grid
	to    cell.ID
	size  uint32
	armed bool

	pos   goke.Comp[world.Base]
	query *goke.Query
}

func (p *pushOnce) Init(si *goke.SysInit) { p.query = si.NewQueryBuilder(&p.pos).Build() }

func (p *pushOnce) Update(_ *goke.CmdBuf, _ time.Duration) {
	if !p.armed {
		return
	}
	p.armed = false
	p.query.All()
	for p.query.Next() {
		positions := p.pos.Slice(p.query.Cursor())
		for i := range positions {
			positions[i].Pos = world.Position{AABB: cellBox(p.grid, p.to, p.size)}
		}
	}
}

func TestNavigationSystem_Update_DeviationTriggersRepath(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occupancy := &cell.SingleOccupancy{}
	steer := newNavigationSystem(newPathFinder(grid, terrain, nil, occupancy), grid, terrain, occupancy)
	pusher := &pushOnce{grid: grid, size: 8}

	start := grid.CellIndex(0, 0)
	target := grid.CellIndex(4, 0)
	pushed := grid.CellIndex(3, 0)
	pusher.to = pushed

	var at goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var order goke.Comp[MoveOrder]
	var profile goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var q *goke.Query

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&at, &pos, &order, &profile, &course)
		f.Create(1)
		f.Next()
		id := f.Cursor.IDs[0]
		at.Slice(&f.Cursor)[0] = unit.At{Cell: start}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: cellBox(grid, start, 8)}
		order.Slice(&f.Cursor)[0] = MoveOrder{Target: target}
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
		occupancy.Enter(start, id, cell.Land)

		q = si.NewQueryBuilder(&at, &order).Build()
	}})

	pusherHandle := ecs.RegSys(pusher)
	steerHandle := ecs.RegSys(steer)
	steeringHandle := ecs.RegSys(steering.NewSystem())
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(pusherHandle, d)
		ctx.Run(steerHandle, d)
		ctx.Run(steeringHandle, d)
		ctx.Sync()
	})

	ecs.Tick(time.Second)

	c, mt := readCellAndMoveOrder(t, q, &at, &order)
	if mt.Path.Length == 0 {
		t.Fatal("expected a path to have been computed on the first tick")
	}
	if c.Cell != start {
		t.Fatalf("At.Cell = %v, want %v (no movement yet, no deviation)", c.Cell, start)
	}
	firstStep := mt.Path.Steps[0]
	if firstStep == pushed || firstStep == target {
		t.Fatalf("first path step %v should be an intermediate cell — grid too small for this test", firstStep)
	}

	pusher.armed = true
	ecs.Tick(time.Second)

	c, mt = readCellAndMoveOrder(t, q, &at, &order)
	if c.Cell != pushed {
		t.Errorf("At.Cell = %v, want %v (deviation should resync bookkeeping to the actual cell)", c.Cell, pushed)
	}
	if mt.Path.Length == 0 {
		t.Fatal("expected the path to have been recomputed after the deviation")
	}
	if mt.Path.Steps[mt.Path.Index] == firstStep {
		t.Error("recomputed path should route onward from the pushed-to cell, not resume the stale pre-deviation path")
	}
}

func TestNavigationSystem_Update_TransientFlankerCellDoesNotInvalidatePath(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occupancy := &cell.SingleOccupancy{}
	steer := newNavigationSystem(newPathFinder(grid, terrain, nil, occupancy), grid, terrain, occupancy)

	previous := grid.CellIndex(0, 1)
	expected := grid.CellIndex(1, 0)

	var at goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var order goke.Comp[MoveOrder]
	var profile goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var q *goke.Query

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&at, &pos, &order, &profile, &course)
		f.Create(1)
		f.Next()
		id := f.Cursor.IDs[0]
		at.Slice(&f.Cursor)[0] = unit.At{Cell: previous}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: plane.NewAABB(geom.NewVec(11, 11), 8, 8)}
		var mt MoveOrder
		mt.Target = expected
		mt.Path.Steps[0] = expected
		mt.Path.Length = 1
		c1, c2, _ := grid.DiagonalNeighbors(previous, expected)
		mt.Leg = Leg{From: previous, To: expected, C1: c1, C2: c2, Diagonal: true, Active: true}
		order.Slice(&f.Cursor)[0] = mt
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
		for _, c := range mt.Leg.cells() {
			occupancy.Enter(c, id, cell.Land)
		}

		q = si.NewQueryBuilder(&at, &order).Build()
	}})

	steerHandle := ecs.RegSys(steer)
	steeringHandle := ecs.RegSys(steering.NewSystem())
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(steerHandle, d)
		ctx.Run(steeringHandle, d)
		ctx.Sync()
	})

	ecs.Tick(time.Second)

	c, mt := readCellAndMoveOrder(t, q, &at, &order)
	if c.Cell != previous {
		t.Fatalf("At.Cell = %v, want %v (bookkeeping should ignore a transient flanker read, not just tolerate it)", c.Cell, previous)
	}
	if mt.Path.Length != 1 || mt.Path.Steps[0] != expected {
		t.Errorf("Path = %+v, want unchanged (Length=1, Steps[0]=%v) — a transient flanker cell shouldn't invalidate the path", mt.Path, expected)
	}
}

func TestNavigationSystem_Update_ArrivalStopsEntity(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occupancy := &cell.SingleOccupancy{}
	steer := newNavigationSystem(newPathFinder(grid, terrain, nil, occupancy), grid, terrain, occupancy)

	start := grid.CellIndex(2, 0)
	target := start

	var at goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var order goke.Comp[MoveOrder]
	var profile goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var q *goke.Query

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&at, &pos, &order, &profile, &course)
		f.Create(1)
		f.Next()
		id := f.Cursor.IDs[0]
		at.Slice(&f.Cursor)[0] = unit.At{Cell: start}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: cellBox(grid, start, 8)}
		pos.Slice(&f.Cursor)[0].Vel = world.Velocity{Dir: geom.NewVec(1, 0), Value: 50}
		order.Slice(&f.Cursor)[0] = MoveOrder{Target: target}
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
		occupancy.Enter(start, id, cell.Land)

		q = si.NewQueryBuilder(&at, &pos).Build()
	}})

	steerHandle := ecs.RegSys(steer)
	steeringHandle := ecs.RegSys(steering.NewSystem())
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(steerHandle, d)
		ctx.Run(steeringHandle, d)
		ctx.Sync()
	})

	ecs.Tick(time.Second)

	q.All()
	found := false
	for q.Next() {
		cur := q.Cursor()
		velocities := pos.Slice(cur)
		for i := range cur.IDs {
			found = true
			if velocities[i].Vel.Value != 0 {
				t.Errorf("Velocity.Value = %v, want 0 after arriving at target", velocities[i].Vel.Value)
			}
		}
	}
	if !found {
		t.Fatal("expected to find the seeded entity")
	}
}

func TestNavigationSystem_Update_ArrivalSnapsToCellCenter(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occupancy := &cell.SingleOccupancy{}
	steer := newNavigationSystem(newPathFinder(grid, terrain, nil, occupancy), grid, terrain, occupancy)
	space := testSpace(t)
	steer.BindSpace(space)

	target := grid.CellIndex(2, 0)
	offCenter := world.Position{AABB: plane.NewAABB(geom.NewVec(20, 1), 8, 8)}

	var at goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var order goke.Comp[MoveOrder]
	var profile goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var q *goke.Query

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&at, &pos, &order, &profile, &course)
		f.Create(1)
		f.Next()
		id := f.Cursor.IDs[0]
		at.Slice(&f.Cursor)[0] = unit.At{Cell: target}
		pos.Slice(&f.Cursor)[0].Pos = offCenter
		order.Slice(&f.Cursor)[0] = MoveOrder{Target: target}
		profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
		occupancy.Enter(target, id, cell.Land)

		q = si.NewQueryBuilder(&pos).Build()
	}})

	steerHandle := ecs.RegSys(steer)
	steeringHandle := ecs.RegSys(steering.NewSystem())
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(steerHandle, d)
		ctx.Run(steeringHandle, d)
		ctx.Sync()
	})

	ecs.Tick(time.Second)

	want := grid.CellCenter(target)
	q.All()
	found := false
	for q.Next() {
		cur := q.Cursor()
		positions := pos.Slice(cur)
		for i := range cur.IDs {
			found = true
			p := positions[i].Pos
			gotX := float64(p.TopLeft.X) + float64(p.Size.X)/2
			gotY := float64(p.TopLeft.Y) + float64(p.Size.Y)/2
			if gotX != want.X || gotY != want.Y {
				t.Errorf("center = (%v, %v), want (%v, %v) — should snap exactly to the cell center on arrival", gotX, gotY, want.X, want.Y)
			}
		}
	}
	if !found {
		t.Fatal("expected to find the seeded entity")
	}
}

func TestNavigationSystem_Update_ArrivalGlidesSmoothlyToCellCenter(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 1, 10)
	target := grid.CellIndex(2, 0)
	offCenter := plane.NewAABB(geom.NewVec(17, 1), 8, 8)

	var pos goke.Comp[world.Base]
	var q *goke.Query
	nw := newNavWorld(t, 5, 1, 10, []navUnit{{
		box: offCenter, at: target, profile: &steering.Steering{MaxSpeed: 20}, order: &MoveOrder{Target: target},
	}}, goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&pos).Build() }})
	ecs := nw.ecs

	readCenter := func() (float64, float64) {
		t.Helper()
		q.All()
		for q.Next() {
			cur := q.Cursor()
			positions := pos.Slice(cur)
			for i := range cur.IDs {
				p := positions[i].Pos
				return float64(p.TopLeft.X) + float64(p.Size.X)/2, float64(p.TopLeft.Y) + float64(p.Size.Y)/2
			}
		}
		t.Fatal("expected to find the seeded entity")
		return 0, 0
	}

	prevX, prevY := readCenter()
	maxStep := 0.0
	for range 200 {
		ecs.Tick(20 * time.Millisecond)
		x, y := readCenter()
		if step := math.Hypot(x-prevX, y-prevY); step > maxStep {
			maxStep = step
		}
		prevX, prevY = x, y
	}

	if maxStep > 3 {
		t.Errorf("largest single-tick step = %.2f, want <= 3 (arrival must glide smoothly, not jump)", maxStep)
	}

	want := grid.CellCenter(target)
	if prevX != want.X || prevY != want.Y {
		t.Errorf("final center = (%v, %v), want (%v, %v)", prevX, prevY, want.X, want.Y)
	}
}

func TestNavigationSystem_Update_ReproducesBoardDemoWallScenario(t *testing.T) {
	const (
		gridWidth, gridHeight, cellSize = uint32(24), uint32(16), uint32(32)
		wallCol                         = uint32(12)
		entitySize                      = uint32(22)
		speed                           = float64(cellSize * 2)
	)
	grid := grid.DefaultGrids{}.Square(gridWidth, gridHeight, cellSize)
	start := grid.CellIndex(2, 4)
	target := grid.CellIndex(gridWidth-3, 4)

	var at goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var order goke.Comp[MoveOrder]
	var q *goke.Query
	nw := newNavWorld(t, gridWidth, gridHeight, cellSize, []navUnit{{
		box: cellBox(grid, start, entitySize), at: start, profile: &steering.Steering{MaxSpeed: speed}, order: &MoveOrder{Target: target},
	}}, goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&at, &pos, &order).Build() }})
	terrain, ecs := nw.board, nw.ecs
	for y := uint32(2); y < gridHeight; y++ {
		c := grid.CellIndex(wallCol, y)
		terrain.Set(c, cell.Kind{Cost: 1, Solid: true})
	}

	dt := time.Second / 60
	var lastSteps []cell.ID
	var prevCellID cell.ID
	var prevExpected cell.ID
	var havePrev bool
	replans := 0
	for tick := 0; tick < 60*15; tick++ {
		ecs.Tick(dt)

		q.All()
		for q.Next() {
			cur := q.Cursor()
			cells := at.Slice(cur)
			positions := pos.Slice(cur)
			orders := order.Slice(cur)
			for i := range cur.IDs {
				if !terrain.Kind(cells[i].Cell).Admits(cell.Land) {
					t.Fatalf("tick %d: entity's logical Cell is inside impassable terrain: %v", tick, cells[i].Cell)
				}
				path := orders[i].Path
				steps := append([]cell.ID(nil), path.Steps[:path.Length]...)
				if !equalSteps(steps, lastSteps) {
					replans++
					if havePrev {
						t.Logf("tick %d: INVALIDATED — actual(new cell)=%v, prevCell=%v, prevExpected=%v, pos=%v",
							tick, cells[i].Cell, prevCellID, prevExpected, positions[i].Pos.TopLeft)
					}
					t.Logf("tick %d: path changed (len %d -> %d) steps=%v", tick, len(lastSteps), len(steps), steps)
					lastSteps = steps
				}
				prevCellID = cells[i].Cell
				if path.Length > 0 && path.Index < path.Length {
					prevExpected = path.Steps[path.Index]
				}
				havePrev = true
			}
		}
	}
	t.Logf("total path changes observed: %d", replans)
	if replans != 1 {
		t.Errorf("total path changes = %d, want 1 (only the initial FindPath — no spurious mid-route invalidation)", replans)
	}
}

func TestShortestAxisDelta(t *testing.T) {
	cases := []struct {
		name       string
		have, want float64
		size       uint32
		toroidal   bool
		wantDelta  float64
	}{
		{"non-toroidal ignores size", 10, 50, 100, false, 40},
		{"toroidal direct is already shortest", 10, 50, 100, true, 40},
		{"toroidal wraps forward when shorter", 90, 5, 100, true, 15},
		{"toroidal wraps backward when shorter", 5, 90, 100, true, -15},
		{"zero size disables wrap", 90, 5, 0, true, -85},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shortestAxisDelta(c.have, c.want, c.size, c.toroidal); got != c.wantDelta {
				t.Errorf("shortestAxisDelta(%v, %v, %v, %v) = %v, want %v", c.have, c.want, c.size, c.toroidal, got, c.wantDelta)
			}
		})
	}
}

func equalSteps(a, b []cell.ID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func readCellAndMoveOrder(t *testing.T, q *goke.Query, cell *goke.Comp[unit.At], order *goke.Comp[MoveOrder]) (unit.At, MoveOrder) {
	t.Helper()
	q.All()
	for q.Next() {
		cur := q.Cursor()
		cells := cell.Slice(cur)
		orders := order.Slice(cur)
		if len(cells) > 0 {
			return cells[0], orders[0]
		}
	}
	t.Fatal("expected to find the seeded entity")
	return unit.At{}, MoveOrder{}
}
