package navigation

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// driveRig is driveSystem over a 10x1 row of 10-unit land cells with water at x 6, a 4x4 walker
// in cell 2 facing east, and a stranger holding cell 3 when asked.
type driveRig struct {
	t         *testing.T
	ecs       *goke.ECS
	grid      grid.Grid
	occupancy *cell.SingleOccupancy
	walker    uid.UID64

	cell   goke.Comp[unit.At]
	base   goke.Comp[world.Base]
	steer  goke.Comp[steering.Steering]
	course goke.Comp[steering.Course]
	driven goke.Comp[steering.Driven]
	order  goke.OptComp[MoveOrder]
	states goke.OptComp[tag.Tags[States]]
	mover  goke.OptComp[unit.Mover]
	q      *goke.Query
}

func newDriveRig(t *testing.T, order *MoveOrder, mover ...unit.Mover) *driveRig {
	t.Helper()
	r := &driveRig{t: t, ecs: goke.New(), grid: grid.DefaultGrids{}.Square(10, 1, 10), occupancy: &cell.SingleOccupancy{}}
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	water := r.grid.CellIndex(6, 0)
	terrain.Set(water, cell.Kind{Cost: 1, Allows: cell.Water})
	nav := newNavigationSystem(newPathFinder(r.grid, terrain, nil, r.occupancy), r.grid, terrain, r.occupancy)
	sys := &driveSystem{nav: nav}

	var at goke.Comp[unit.At]
	var base goke.Comp[world.Base]
	var steer goke.Comp[steering.Steering]
	var course goke.Comp[steering.Course]
	var driven goke.Comp[steering.Driven]
	var ord goke.Comp[MoveOrder]
	var mov goke.Comp[unit.Mover]
	r.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.q = si.NewQueryBuilder(&r.cell, &r.base, &r.steer, &r.course, &r.driven).Optional(&r.order).Optional(&r.states).Optional(&r.mover).Build()
		comps := []goke.Addable{&at, &base, &steer, &course, &driven}
		if order != nil {
			comps = append(comps, &ord)
		}
		if len(mover) > 0 {
			comps = append(comps, &mov)
		}
		f := si.NewFactory(comps...)
		f.Create(1)
		for f.Next() {
			r.walker = f.Cursor.IDs[0]
			start := r.grid.CellIndex(2, 0)
			at.Slice(&f.Cursor)[0] = unit.At{Cell: start}
			b := &base.Slice(&f.Cursor)[0]
			b.Pos = world.Position{AABB: plane.NewAABB(geom.NewVec(23, 3), 4, 4)}
			b.Vel.Dir = geom.NewVec(1, 0)
			steer.Slice(&f.Cursor)[0] = steering.Steering{MaxSpeed: 20}
			r.occupancy.Enter(start, r.walker, cell.Land)
			if order != nil {
				ord.Slice(&f.Cursor)[0] = *order
			}
			if len(mover) > 0 {
				mov.Slice(&f.Cursor)[0] = mover[0]
			}
		}
	}})
	run := r.ecs.RegSys(sys)
	r.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(run, d); ctx.Sync() })
	return r
}

// with calls fn with the walker's components.
func (r *driveRig) with(fn func(c *unit.At, b *world.Base, st steering.Helm, d *steering.Driven, o *MoveOrder, entered bool)) {
	r.q.All()
	for r.q.Next() {
		cur := r.q.Cursor()
		var o *MoveOrder
		if orders := r.order.Slice(cur); orders != nil {
			o = &orders[0]
		}
		fn(&r.cell.Slice(cur)[0], &r.base.Slice(cur)[0], steering.Helm{Steering: &r.steer.Slice(cur)[0], Course: &r.course.Slice(cur)[0]}, &r.driven.Slice(cur)[0], o, entered(r.states.Slice(cur)))
	}
}

func (r *driveRig) drive(in steering.Driven) {
	r.with(func(_ *unit.At, _ *world.Base, _ steering.Helm, d *steering.Driven, _ *MoveOrder, _ bool) {
		*d = in
	})
	r.ecs.Tick(time.Second / 60)
}

// place puts the walker's centre at (x, 5).
func (r *driveRig) place(x float64) {
	r.with(func(_ *unit.At, b *world.Base, _ steering.Helm, _ *steering.Driven, _ *MoveOrder, _ bool) {
		b.Pos.AABB = plane.NewAABB(geom.NewVec(x-2, 3), 4, 4)
	})
}

func (r *driveRig) steering() steering.Course {
	var st steering.Course
	r.with(func(_ *unit.At, _ *world.Base, s steering.Helm, _ *steering.Driven, _ *MoveOrder, _ bool) {
		st = *s.Course
	})
	return st
}

func TestDrive_WalksOnTheWayItFacesAndTurnsByHand(t *testing.T) {
	r := newDriveRig(t, nil)
	r.drive(steering.Driven{Ahead: 1})
	if st := r.steering(); st.WantSpeed != 20 || st.Want != geom.NewVec(1, 0) {
		t.Errorf("Up held: asks %v at %v, want its top speed east", st.WantSpeed, st.Want)
	}
	r.drive(steering.Driven{Turn: 1})
	st := r.steering()
	if want := geom.NewVec(math.Cos(driveTurn), math.Sin(driveTurn)); math.Abs(st.Want.X-want.X) > 1e-9 || math.Abs(st.Want.Y-want.Y) > 1e-9 || st.WantSpeed != 0 {
		t.Errorf("Right held alone: asks %v at %v, want turning clockwise by %v and standing", st.WantSpeed, st.Want, driveTurn)
	}
}

func TestDrive_StopsAtTheWaterAndAtACellTakenAndBrakesWithNoHand(t *testing.T) {
	r := newDriveRig(t, nil)
	r.place(57) // in cell 5, 3 short of the water
	r.drive(steering.Driven{Ahead: 1})
	if st := r.steering(); st.WantSpeed != 0 || st.Speed != 0 {
		t.Errorf("Up at the water's edge: asks %v, speed %v; want stopped", st.WantSpeed, st.Speed)
	}
	r.place(45)
	r.drive(steering.Driven{Ahead: 1})
	if st := r.steering(); st.WantSpeed != 20 {
		t.Fatalf("Up in the middle of cell 4 asks %v, want walking on", st.WantSpeed)
	}
	r.drive(steering.Driven{})
	if st := r.steering(); st.WantSpeed != 0 {
		t.Errorf("no key held asks %v, want braking", st.WantSpeed)
	}
	stranger := uid.UID64(99)
	next := r.grid.CellIndex(5, 0)
	r.occupancy.Enter(next, stranger, cell.Land)
	r.place(47)
	r.drive(steering.Driven{Ahead: 1})
	if st := r.steering(); st.WantSpeed != 0 {
		t.Errorf("Up towards a cell another holds asks %v, want stopped", st.WantSpeed)
	}
}

func TestDrive_KeepsTheCellAndTheOccupancyWithTheWalker(t *testing.T) {
	r := newDriveRig(t, nil)
	r.place(35)
	r.drive(steering.Driven{Ahead: 1})
	cell3 := r.grid.CellIndex(3, 0)
	cell2 := r.grid.CellIndex(2, 0)
	r.with(func(c *unit.At, _ *world.Base, _ steering.Helm, _ *steering.Driven, _ *MoveOrder, entered bool) {
		if c.Cell != cell3 || !entered {
			t.Errorf("walked into cell 3 the walker stands on %v, entered %v; want cell 3, entered", c.Cell, entered)
		}
	})
	if !r.occupancy.CanEnter(cell2, uid.UID64(99), cell.Land) || r.occupancy.CanEnter(cell3, uid.UID64(99), cell.Land) {
		t.Error("the occupancy did not follow the walker from cell 2 to cell 3")
	}
}

func TestDrive_AHandEndsAnOrderAndNoHandLetsItGoOn(t *testing.T) {
	order := &MoveOrder{Target: 8}
	r := newDriveRig(t, order)
	r.drive(steering.Driven{})
	r.with(func(_ *unit.At, _ *world.Base, st steering.Helm, _ *steering.Driven, o *MoveOrder, _ bool) {
		if o == nil || st.WantSpeed != 0 || st.Want != (geom.Vec{}) {
			t.Errorf("no hand on an ordered walker: order %v, steering %+v; want the order kept, the steering untouched", o, st)
		}
	})
	r.drive(steering.Driven{Ahead: 1})
	r.with(func(_ *unit.At, _ *world.Base, st steering.Helm, _ *steering.Driven, o *MoveOrder, _ bool) {
		if o != nil || st.WantSpeed != 20 {
			t.Errorf("a hand on an ordered walker: order %v, asks %v; want the order gone, walking on", o, st.WantSpeed)
		}
	})
}

func TestDrive_AHandGivesUpTheCellsTheOrdersStepHeld(t *testing.T) {
	cell2 := grid.DefaultGrids{}.Square(10, 1, 10).CellIndex(2, 0)
	cell3 := grid.DefaultGrids{}.Square(10, 1, 10).CellIndex(3, 0)
	r := newDriveRig(t, &MoveOrder{Target: 8, Leg: Leg{From: cell2, To: cell3, Active: true}})
	r.occupancy.Enter(cell3, r.walker, cell.Land)
	r.drive(steering.Driven{Turn: 1})
	if !r.occupancy.CanEnter(cell3, uid.UID64(99), cell.Land) {
		t.Error("the cell the order's step was heading into is still held")
	}
	if r.occupancy.CanEnter(cell2, uid.UID64(99), cell.Land) {
		t.Error("the cell the walker stands on is no longer held")
	}
}

// Face turns the walker to face a way — where an eye riding in it looks — whatever Turn says, a
// hand that ends an order as any other.
func TestDrive_FaceTurnsItToFaceAWay(t *testing.T) {
	r := newDriveRig(t, nil)
	r.drive(steering.Driven{Face: geom.NewVec(0, -3), Turn: 1})
	if st := r.steering(); st.Want != geom.NewVec(0, -1) || st.WantSpeed != 0 {
		t.Errorf("driven to face north: asks %v at %v, want north, standing", st.WantSpeed, st.Want)
	}
	r.drive(steering.Driven{Face: geom.NewVec(1, 1), Ahead: 1})
	if st := r.steering(); math.Abs(st.Want.X-math.Sqrt2/2) > 1e-9 || math.Abs(st.Want.Y-math.Sqrt2/2) > 1e-9 || st.WantSpeed != 20 {
		t.Errorf("driven on to face south-east: asks %v at %v, want its top speed south-east", st.WantSpeed, st.Want)
	}
	o := r2order(t)
	o.drive(steering.Driven{Face: geom.NewVec(0, 1)})
	gone := true
	o.with(func(_ *unit.At, _ *world.Base, _ steering.Helm, _ *steering.Driven, ord *MoveOrder, _ bool) {
		gone = ord == nil
	})
	if !gone {
		t.Error("a Face left the walker's order on")
	}
}

// r2order is the drive rig with the walker on an order to the far end of the row.
func r2order(t *testing.T) *driveRig {
	far := grid.DefaultGrids{}.Square(10, 1, 10).CellIndex(9, 0)
	return newDriveRig(t, &MoveOrder{Target: far})
}

// S held while walking brakes the walker to a stop, never at once, then backs it away at its V0,
// facing as it does — and stops it at the edge behind it.
func TestDrive_BrakesThenBacksAwayFacingOn(t *testing.T) {
	r := newDriveRig(t, nil)
	r.with(func(_ *unit.At, _ *world.Base, st steering.Helm, _ *steering.Driven, _ *MoveOrder, _ bool) {
		st.Accel, st.Brake, st.V0, st.Speed = 40, 80, 5, 20
	})
	r.place(45)
	r.drive(steering.Driven{Ahead: -1})
	if st := r.steering(); st.WantSpeed != 0 || st.Speed != 20 {
		t.Fatalf("S held walking at 20: asks %v, speed %v; want braking, not stopped at once", st.WantSpeed, st.Speed)
	}
	r.with(func(_ *unit.At, _ *world.Base, st steering.Helm, _ *steering.Driven, _ *MoveOrder, _ bool) {
		st.Speed = 0 // braked to a stop
	})
	r.drive(steering.Driven{Ahead: -1})
	if st := r.steering(); st.WantSpeed != -5 || st.Want != geom.NewVec(1, 0) {
		t.Errorf("S held standing: asks %v facing %v, want backing at V0 5, still facing east", st.WantSpeed, st.Want)
	}
	r.place(3) // at the board's west edge: nothing behind
	r.drive(steering.Driven{Ahead: -1})
	if st := r.steering(); st.WantSpeed != 0 || st.Speed != 0 {
		t.Errorf("S held with the edge behind: asks %v, speed %v; want stopped", st.WantSpeed, st.Speed)
	}
}

// lift is the walker's Lift, 0 without a Mover.
func (r *driveRig) lift() float64 {
	var lift float64
	r.q.All()
	for r.q.Next() {
		if movers := r.mover.Slice(r.q.Cursor()); movers != nil {
			lift = movers[0].Lift
		}
	}
	return lift
}

// W with Shift urges the walker to its Sprint; without one to its top speed alone.
func TestDrive_SprintsWhereUrged(t *testing.T) {
	r := newDriveRig(t, nil)
	r.with(func(_ *unit.At, _ *world.Base, st steering.Helm, _ *steering.Driven, _ *MoveOrder, _ bool) {
		st.Sprint = 4
	})
	r.place(35)
	r.drive(steering.Driven{Ahead: 1, Sprint: true})
	if st := r.steering(); st.WantSpeed != 80 {
		t.Errorf("urged on: asks %v, want 4 times its top speed 20", st.WantSpeed)
	}
	r.drive(steering.Driven{Ahead: 1})
	if st := r.steering(); st.WantSpeed != 20 {
		t.Errorf("walked on: asks %v, want its top speed", st.WantSpeed)
	}
}

// A flyer flown by hand and steered up goes the less along the ground, the more steeply, sprinting
// or not; its height is the topography's to change, so the drive leaves its Lift be. Neither a
// flyer steered from behind nor a walker slows for the look.
func TestDrive_AFlyerFlownUpGoesTheLessAlongTheGround(t *testing.T) {
	r := newDriveRig(t, nil, unit.Mover{Domain: cell.Land | cell.Air, Lift: 10}) // over the rig's land
	r.with(func(_ *unit.At, _ *world.Base, st steering.Helm, _ *steering.Driven, _ *MoveOrder, _ bool) {
		st.Sprint, st.Speed = 4, 20
	})
	r.place(35)
	r.drive(steering.Driven{Ahead: 1, Flown: true, Climb: 0.6})
	if st := r.steering(); math.Abs(st.WantSpeed-16) > 1e-9 {
		t.Errorf("flown up at a rise of 0.6: asks %v along the ground, want 20 times 0.8", st.WantSpeed)
	}
	r.drive(steering.Driven{Ahead: 1, Sprint: true, Flown: true, Climb: -0.6})
	if st := r.steering(); math.Abs(st.WantSpeed-64) > 1e-9 {
		t.Errorf("flown down at a rise of -0.6, sprinting: asks %v along the ground, want 80 times 0.8", st.WantSpeed)
	}
	if lift := r.lift(); lift != 10 {
		t.Errorf("the drive changed the lift to %v, want it left at 10", lift)
	}
	r.drive(steering.Driven{Ahead: 1, Climb: 0.6})
	if st := r.steering(); st.WantSpeed != 20 {
		t.Errorf("steered from behind: asks %v, want its top speed", st.WantSpeed)
	}
	w := newDriveRig(t, nil, unit.Mover{Domain: cell.Land})
	w.place(35)
	w.drive(steering.Driven{Ahead: 1, Flown: true, Climb: 0.6})
	if st := w.steering(); st.WantSpeed != 20 {
		t.Errorf("a walker ridden looking up asks %v, want its top speed", st.WantSpeed)
	}
}

// entered reports whether the chunk's one unit has its Entered on.
func entered(states []tag.Tags[States]) bool { return states != nil && states[0].Has(Entered) }
