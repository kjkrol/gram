package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

const (
	legCellSize   = uint32(32)
	legEntitySize = uint32(22)
)

// legUnit seeds one entity: its start cell and, if hasOrder, a MoveOrder toward target.
type legUnit struct {
	start, target board.CellID
	hasOrder      bool
}

// legWorld is a real navigation + movement ECS over an open grid, for Leg reservation tests.
type legWorld struct {
	grid      board.Grid
	terrain   *board.TerrainMap
	occupancy *board.SingleOccupancy
	ecs       *goke.ECS
	ids       []uid.UID64

	pos   goke.Comp[world.Base]
	cell  goke.Comp[board.Cell]
	order goke.OptComp[MoveOrder]
	q     *goke.Query
}

func newLegWorld(t *testing.T, w, h uint32, units ...legUnit) *legWorld {
	t.Helper()
	lw := &legWorld{grid: board.DefaultGrids{}.Square(w, h, legCellSize), occupancy: &board.SingleOccupancy{}}
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	lw.terrain = terrain
	steer := newNavigationSystem(newPathFinder(lw.grid, terrain, nil, lw.occupancy), lw.grid, terrain, lw.occupancy)
	space := testSpace(t)
	steer.BindSpace(space)

	lw.ecs = goke.New()
	lw.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for _, u := range units {
			var cell goke.Comp[board.Cell]
			var pos goke.Comp[world.Base]
			var profile goke.Comp[steering.Steering]
			comps := []goke.Addable{&cell, &pos, &profile}
			var order goke.Comp[MoveOrder]
			if u.hasOrder {
				comps = append(comps, &order)
			}
			f := si.NewFactory(comps...)
			f.Create(1)
			f.Next()
			id := f.Cursor.IDs[0]
			p := world.Position{AABB: board.CellAABB(lw.grid, u.start, legEntitySize)}
			cell.Slice(&f.Cursor)[0] = board.Cell{ID: u.start}
			pos.Slice(&f.Cursor)[0].Pos = p
			profile.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: float64(legCellSize * 2)}
			if u.hasOrder {
				order.Slice(&f.Cursor)[0] = MoveOrder{Target: u.target}
			}
			lw.occupancy.Enter(u.start, id, board.Land)
			lw.ids = append(lw.ids, id)
		}
		lw.q = si.NewQueryBuilder(&lw.pos, &lw.cell).Optional(&lw.order).Build()
	}})

	steerHandle := lw.ecs.RegSys(steer)
	steeringHandle := lw.ecs.RegSys(steering.NewSystem())
	moveHandle := lw.ecs.RegSys(world.NewMoveSystem(space))
	lw.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(steerHandle, d)
		ctx.Run(steeringHandle, d)
		ctx.Run(moveHandle, d)
		ctx.Sync()
	})
	return lw
}

// legState is one entity's observable navigation state after a tick.
type legState struct {
	pos      world.Position
	cell     board.CellID
	order    MoveOrder
	hasOrder bool
}

func (lw *legWorld) read() map[uid.UID64]legState {
	out := make(map[uid.UID64]legState)
	lw.q.All()
	for lw.q.Next() {
		cur := lw.q.Cursor()
		positions, cells, orders := lw.pos.Slice(cur), lw.cell.Slice(cur), lw.order.Slice(cur)
		for i, id := range cur.IDs {
			st := legState{pos: positions[i].Pos, cell: cells[i].ID}
			if orders != nil {
				st.order, st.hasOrder = orders[i], true
			}
			out[id] = st
		}
	}
	return out
}

func (lw *legWorld) tick() map[uid.UID64]legState {
	lw.ecs.Tick(time.Second / 60)
	return lw.read()
}

// walls makes cells impassable, taking effect from the next tick.
func (lw *legWorld) walls(cells ...board.CellID) {
	for _, c := range cells {
		lw.terrain.Set(c, board.CellKind{Cost: 1, Solid: true})
	}
}

func (lw *legWorld) cellAt(x, y uint32) board.CellID {
	c, _ := lw.grid.CellIndex(x, y)
	return c
}

func overlaps(a, b world.Position) bool {
	return a.TopLeft.X < b.TopLeft.X+b.Size.X && b.TopLeft.X < a.TopLeft.X+a.Size.X &&
		a.TopLeft.Y < b.TopLeft.Y+b.Size.Y && b.TopLeft.Y < a.TopLeft.Y+a.Size.Y
}

const otherEntity = uid.UID64(1 << 40)

func TestNavigation_HeadOn_ResolvesWithoutOverlap(t *testing.T) {
	grid := board.DefaultGrids{}.Square(5, 5, legCellSize)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	lw := newLegWorld(t, 5, 5,
		legUnit{start: at(2, 1), target: at(2, 4), hasOrder: true},
		legUnit{start: at(2, 3), target: at(2, 0), hasOrder: true},
	)

	for tick := range 60 * 20 {
		st := lw.tick()
		a, b := st[lw.ids[0]], st[lw.ids[1]]
		if overlaps(a.pos, b.pos) {
			t.Fatalf("tick %d: AABBs overlap: %+v vs %+v", tick, a.pos.AABB, b.pos.AABB)
		}
		if !a.hasOrder && !b.hasOrder {
			if a.cell != at(2, 4) || b.cell != at(2, 0) {
				t.Fatalf("arrived at cells %v, %v, want %v, %v", a.cell, b.cell, at(2, 4), at(2, 0))
			}
			return
		}
	}
	t.Fatal("units never both reached their targets — head-on block was not resolved")
}

func TestNavigation_BlockedDeparture_RepathsAroundStationaryEntity(t *testing.T) {
	grid := board.DefaultGrids{}.Square(5, 5, legCellSize)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	blocker := at(1, 2)
	lw := newLegWorld(t, 5, 5,
		legUnit{start: at(0, 2), target: at(4, 2), hasOrder: true},
		legUnit{start: blocker},
	)

	for range 60 * 20 {
		st := lw.tick()
		a := st[lw.ids[0]]
		if a.cell == blocker {
			t.Fatal("mover entered the stationary entity's cell")
		}
		if !a.hasOrder {
			if a.cell != at(4, 2) {
				t.Fatalf("arrived at %v, want %v", a.cell, at(4, 2))
			}
			if st[lw.ids[1]].cell != blocker {
				t.Errorf("stationary entity moved to %v", st[lw.ids[1]].cell)
			}
			return
		}
	}
	t.Fatal("mover never reached its target around the stationary entity")
}

func TestNavigation_Leg_HoldsFromAndToUntilArrival(t *testing.T) {
	grid := board.DefaultGrids{}.Square(5, 1, legCellSize)
	start, _ := grid.CellIndex(0, 0)
	target, _ := grid.CellIndex(1, 0)
	lw := newLegWorld(t, 5, 1, legUnit{start: start, target: target, hasOrder: true})

	st := lw.tick()[lw.ids[0]]
	if !st.order.Leg.Active || st.order.Leg.From != start || st.order.Leg.To != target {
		t.Fatalf("Leg after departure = %+v, want active %v→%v", st.order.Leg, start, target)
	}
	if lw.occupancy.CanEnter(start, otherEntity, board.Land) || lw.occupancy.CanEnter(target, otherEntity, board.Land) {
		t.Fatal("expected both From and To held while the step is in progress")
	}

	for range 60 * 5 {
		if st = lw.tick()[lw.ids[0]]; !st.hasOrder {
			break
		}
	}
	if st.hasOrder {
		t.Fatal("entity never arrived")
	}
	if !lw.occupancy.CanEnter(start, otherEntity, board.Land) {
		t.Error("expected From released after arrival")
	}
	if lw.occupancy.CanEnter(target, otherEntity, board.Land) {
		t.Error("expected To still held after arrival")
	}
}

func TestNavigation_Leg_DiagonalHoldsCorners(t *testing.T) {
	grid := board.DefaultGrids{}.Square(3, 3, legCellSize)
	start, _ := grid.CellIndex(0, 0)
	target, _ := grid.CellIndex(1, 1)
	c1, c2, diag := grid.DiagonalNeighbors(start, target)
	if !diag {
		t.Fatal("sanity: (0,0)→(1,1) must be a diagonal step")
	}
	lw := newLegWorld(t, 3, 3, legUnit{start: start, target: target, hasOrder: true})

	st := lw.tick()[lw.ids[0]]
	if !st.order.Leg.Diagonal {
		t.Fatalf("Leg after departure = %+v, want a diagonal step", st.order.Leg)
	}
	if lw.occupancy.CanEnter(c1, otherEntity, board.Land) || lw.occupancy.CanEnter(c2, otherEntity, board.Land) {
		t.Fatal("expected both corner cells held during a diagonal step")
	}

	for range 60 * 5 {
		if st = lw.tick()[lw.ids[0]]; !st.hasOrder {
			break
		}
	}
	if st.hasOrder {
		t.Fatal("entity never arrived")
	}
	if !lw.occupancy.CanEnter(c1, otherEntity, board.Land) || !lw.occupancy.CanEnter(c2, otherEntity, board.Land) {
		t.Error("expected corner cells released after arrival")
	}
}

func TestCommandSystem_Update_RetargetMidLegKeepsLeg(t *testing.T) {
	grid := board.DefaultGrids{}.Square(10, 1, legCellSize)
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	occupancy := &board.SingleOccupancy{}
	moves := &control.Queue[MoveTo]{}
	cmds := newMoveCommandSystem(newPathFinder(grid, terrain, nil, occupancy), moves, &control.Queue[LookAt]{}, selTags.Selected)

	from, _ := grid.CellIndex(0, 0)
	to, _ := grid.CellIndex(1, 0)
	newTarget, _ := grid.CellIndex(5, 0)
	leg := Leg{From: from, To: to, Active: true}

	var cell goke.Comp[board.Cell]
	var order goke.Comp[MoveOrder]
	var selected goke.Comp[plugin.Tags[selection.Family]]
	var q *goke.Query

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&cell, &order, &selected)
		f.Create(1)
		f.Next()
		selected.Slice(&f.Cursor)[0] = selectedMarks
		id := f.Cursor.IDs[0]
		cell.Slice(&f.Cursor)[0] = board.Cell{ID: from}
		order.Slice(&f.Cursor)[0] = MoveOrder{Target: to, Leg: leg}
		for _, c := range leg.cells() {
			occupancy.Enter(c, id, board.Land)
		}
		q = si.NewQueryBuilder(&cell, &order).Build()
		cmds.Init(si)
	}})
	cmdHandle := ecs.RegSys(cmds)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(cmdHandle, d)
		ctx.Sync()
	})

	moves.Add(control.Nobody, MoveTo{Cell: newTarget})
	ecs.Tick(time.Second)

	_, mt := readCellAndMoveOrder(t, q, &cell, &order)
	if mt.Target != newTarget {
		t.Fatalf("Target = %v, want %v", mt.Target, newTarget)
	}
	if mt.Leg != leg {
		t.Errorf("Leg = %+v, want the in-progress step %+v carried over", mt.Leg, leg)
	}
	next, _ := grid.CellIndex(2, 0)
	if mt.Path.Length == 0 || mt.Path.Steps[0] != next {
		t.Errorf("Path = %+v, want it to continue from Leg.To (first step %v)", mt.Path, next)
	}
}

func TestModule_Setup_RestoresLegCells(t *testing.T) {
	grid := board.DefaultGrids{}.Square(3, 3, legCellSize)
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	occupancy := &board.SingleOccupancy{}
	m := &module{navigationSystem: newNavigationSystem(newPathFinder(grid, terrain, nil, occupancy), grid, terrain, occupancy)}

	from, _ := grid.CellIndex(0, 0)
	to, _ := grid.CellIndex(1, 1)
	c1, c2, _ := grid.DiagonalNeighbors(from, to)
	leg := Leg{From: from, To: to, C1: c1, C2: c2, Diagonal: true, Active: true}

	goke.New().Setup(
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			var cell goke.Comp[board.Cell]
			var order goke.Comp[MoveOrder]
			f := si.NewFactory(&cell, &order)
			f.Create(1)
			f.Next()
			cell.Slice(&f.Cursor)[0] = board.Cell{ID: from}
			order.Slice(&f.Cursor)[0] = MoveOrder{Target: to, Leg: leg}
		}},
		m.SetupSystems()[0],
	)

	for _, c := range leg.cells() {
		if occupancy.CanEnter(c, otherEntity, board.Land) {
			t.Errorf("cell %v not held after Setup, want every Leg cell restored", c)
		}
	}
}
