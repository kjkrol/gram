package navigation

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// driveRig is driveSystem over a 10x1 row of 10-unit land cells with water at x 6, a 4x4 walker
// in cell 2 facing east, and a stranger holding cell 3 when asked.
type driveRig struct {
	t         *testing.T
	ecs       *goke.ECS
	grid      board.Grid
	occupancy *board.SingleOccupancy
	walker    uid.UID64

	cell    goke.Comp[board.Cell]
	base    goke.Comp[world.Base]
	steer   goke.Comp[world.Steering]
	driven  goke.Comp[world.Driven]
	order   goke.OptComp[MoveOrder]
	entered goke.OptComp[CellEntered]
	q       *goke.Query
}

func newDriveRig(t *testing.T, order *MoveOrder) *driveRig {
	t.Helper()
	r := &driveRig{t: t, ecs: goke.New(), grid: board.DefaultGrids{}.Square(10, 1, 10), occupancy: &board.SingleOccupancy{}}
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	water, _ := r.grid.CellIndex(6, 0)
	terrain.Set(water, board.CellKind{Cost: 1, Allows: board.Water})
	nav := newNavigationSystem(newPathFinder(r.grid, terrain, nil, r.occupancy), r.grid, terrain, r.occupancy)
	sys := &driveSystem{nav: nav}

	var cell goke.Comp[board.Cell]
	var base goke.Comp[world.Base]
	var steer goke.Comp[world.Steering]
	var driven goke.Comp[world.Driven]
	var ord goke.Comp[MoveOrder]
	r.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.q = si.NewQueryBuilder(&r.cell, &r.base, &r.steer, &r.driven).Optional(&r.order).Optional(&r.entered).Build()
		comps := []goke.Addable{&cell, &base, &steer, &driven}
		if order != nil {
			comps = append(comps, &ord)
		}
		f := si.NewFactory(comps...)
		f.Create(1)
		for f.Next() {
			r.walker = f.Cursor.IDs[0]
			start, _ := r.grid.CellIndex(2, 0)
			cell.Slice(&f.Cursor)[0] = board.Cell{ID: start}
			b := &base.Slice(&f.Cursor)[0]
			b.Pos = world.Position{AABB: plane.NewAABB(geom.NewVec(23, 3), 4, 4)}
			b.Vel.Dir = geom.NewVec(1, 0)
			steer.Slice(&f.Cursor)[0] = world.Steering{MaxSpeed: 20}
			r.occupancy.Enter(start, r.walker, board.Land)
			if order != nil {
				ord.Slice(&f.Cursor)[0] = *order
			}
		}
	}})
	run := r.ecs.RegSys(sys)
	r.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(run, d); ctx.Sync() })
	return r
}

// with calls fn with the walker's components.
func (r *driveRig) with(fn func(c *board.Cell, b *world.Base, st *world.Steering, d *world.Driven, o *MoveOrder, entered bool)) {
	r.q.All()
	for r.q.Next() {
		cur := r.q.Cursor()
		var o *MoveOrder
		if orders := r.order.Slice(cur); orders != nil {
			o = &orders[0]
		}
		fn(&r.cell.Slice(cur)[0], &r.base.Slice(cur)[0], &r.steer.Slice(cur)[0], &r.driven.Slice(cur)[0], o, r.entered.Slice(cur) != nil)
	}
}

func (r *driveRig) drive(in world.Driven) {
	r.with(func(_ *board.Cell, _ *world.Base, _ *world.Steering, d *world.Driven, _ *MoveOrder, _ bool) { *d = in })
	r.ecs.Tick(time.Second / 60)
}

// place puts the walker's centre at (x, 5).
func (r *driveRig) place(x float64) {
	r.with(func(_ *board.Cell, b *world.Base, _ *world.Steering, _ *world.Driven, _ *MoveOrder, _ bool) {
		b.Pos.AABB = plane.NewAABB(geom.NewVec(x-2, 3), 4, 4)
	})
}

func (r *driveRig) steering() world.Steering {
	var st world.Steering
	r.with(func(_ *board.Cell, _ *world.Base, s *world.Steering, _ *world.Driven, _ *MoveOrder, _ bool) { st = *s })
	return st
}

func TestDrive_WalksOnTheWayItFacesAndTurnsByHand(t *testing.T) {
	r := newDriveRig(t, nil)
	r.drive(world.Driven{Ahead: 1})
	if st := r.steering(); st.WantSpeed != 20 || st.Want != geom.NewVec(1, 0) {
		t.Errorf("Up held: asks %v at %v, want its top speed east", st.WantSpeed, st.Want)
	}
	r.drive(world.Driven{Turn: 1})
	st := r.steering()
	if want := geom.NewVec(math.Cos(driveTurn), math.Sin(driveTurn)); math.Abs(st.Want.X-want.X) > 1e-9 || math.Abs(st.Want.Y-want.Y) > 1e-9 || st.WantSpeed != 0 {
		t.Errorf("Right held alone: asks %v at %v, want turning clockwise by %v and standing", st.WantSpeed, st.Want, driveTurn)
	}
}

func TestDrive_StopsAtTheWaterAndAtACellTakenAndBrakesWithNoHand(t *testing.T) {
	r := newDriveRig(t, nil)
	r.place(57) // in cell 5, 3 short of the water
	r.drive(world.Driven{Ahead: 1})
	if st := r.steering(); st.WantSpeed != 0 || st.Speed != 0 {
		t.Errorf("Up at the water's edge: asks %v, speed %v; want stopped", st.WantSpeed, st.Speed)
	}
	r.place(45)
	r.drive(world.Driven{Ahead: 1})
	if st := r.steering(); st.WantSpeed != 20 {
		t.Fatalf("Up in the middle of cell 4 asks %v, want walking on", st.WantSpeed)
	}
	r.drive(world.Driven{})
	if st := r.steering(); st.WantSpeed != 0 {
		t.Errorf("no key held asks %v, want braking", st.WantSpeed)
	}
	stranger := uid.UID64(99)
	next, _ := r.grid.CellIndex(5, 0)
	r.occupancy.Enter(next, stranger, board.Land)
	r.place(47)
	r.drive(world.Driven{Ahead: 1})
	if st := r.steering(); st.WantSpeed != 0 {
		t.Errorf("Up towards a cell another holds asks %v, want stopped", st.WantSpeed)
	}
}

func TestDrive_KeepsTheCellAndTheOccupancyWithTheWalker(t *testing.T) {
	r := newDriveRig(t, nil)
	r.place(35)
	r.drive(world.Driven{Ahead: 1})
	cell3, _ := r.grid.CellIndex(3, 0)
	cell2, _ := r.grid.CellIndex(2, 0)
	r.with(func(c *board.Cell, _ *world.Base, _ *world.Steering, _ *world.Driven, _ *MoveOrder, entered bool) {
		if c.ID != cell3 || !entered {
			t.Errorf("walked into cell 3 the walker stands on %v, entered %v; want cell 3, entered", c.ID, entered)
		}
	})
	if !r.occupancy.CanEnter(cell2, uid.UID64(99), board.Land) || r.occupancy.CanEnter(cell3, uid.UID64(99), board.Land) {
		t.Error("the occupancy did not follow the walker from cell 2 to cell 3")
	}
}

func TestDrive_AHandEndsAnOrderAndNoHandLetsItGoOn(t *testing.T) {
	order := &MoveOrder{Target: 8}
	r := newDriveRig(t, order)
	r.drive(world.Driven{})
	r.with(func(_ *board.Cell, _ *world.Base, st *world.Steering, _ *world.Driven, o *MoveOrder, _ bool) {
		if o == nil || st.WantSpeed != 0 || st.Want != (geom.Vec{}) {
			t.Errorf("no hand on an ordered walker: order %v, steering %+v; want the order kept, the steering untouched", o, st)
		}
	})
	r.drive(world.Driven{Ahead: 1})
	r.with(func(_ *board.Cell, _ *world.Base, st *world.Steering, _ *world.Driven, o *MoveOrder, _ bool) {
		if o != nil || st.WantSpeed != 20 {
			t.Errorf("a hand on an ordered walker: order %v, asks %v; want the order gone, walking on", o, st.WantSpeed)
		}
	})
}

func TestDrive_AHandGivesUpTheCellsTheOrdersStepHeld(t *testing.T) {
	cell2, _ := board.DefaultGrids{}.Square(10, 1, 10).CellIndex(2, 0)
	cell3, _ := board.DefaultGrids{}.Square(10, 1, 10).CellIndex(3, 0)
	r := newDriveRig(t, &MoveOrder{Target: 8, Leg: Leg{From: cell2, To: cell3, Active: true}})
	r.occupancy.Enter(cell3, r.walker, board.Land)
	r.drive(world.Driven{Turn: 1})
	if !r.occupancy.CanEnter(cell3, uid.UID64(99), board.Land) {
		t.Error("the cell the order's step was heading into is still held")
	}
	if r.occupancy.CanEnter(cell2, uid.UID64(99), board.Land) {
		t.Error("the cell the walker stands on is no longer held")
	}
}
