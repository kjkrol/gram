package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

func TestMarch_SendsTheSelectedInItsFormationOffOneField(t *testing.T) {
	g := grid.DefaultGrids{}.Square(12, 3, roadCell)
	units := make([]roadUnit, 6)
	for i := range units {
		units[i] = roadUnit{start: g.At(uint32(i), 1), selected: i < 5}
	}
	rw := newRoadWorld(t, 12, units)
	column := ColumnOf(1, true, 1)
	rw.nav.marches.Add(command.Nobody, March{Cell: rw.at(11, 1), In: column})
	rw.ecs.Tick(time.Second / 60)
	_, first := rw.state(rw.byRow[0])
	for row := range 5 {
		_, o := rw.state(rw.byRow[row])
		if o == nil || o.Formation != column || o.Party != 5 || o.Group != first.Group ||
			o.Place == 0 {
			t.Errorf("unit %d: order %+v, want one of a party of 5 in its column, laid out", row, o)
		}
	}
	if _, o := rw.state(rw.byRow[5]); o != nil {
		t.Errorf("the unit not selected was given %+v", o)
	}
	for range 30 {
		rw.ecs.Tick(time.Second / 60)
	}
	coord := rw.nav.module.navigationSystem.Routing.(*routing.Coordinator)
	if coord.Fills() != 1 || coord.Walks() == 0 {
		t.Errorf("%d fields filled, %d routes walked off them; want one field the column walks",
			coord.Fills(), coord.Walks())
	}
}

func TestMarch_InAShapeUnknownGoesAsAMoveTo(t *testing.T) {
	g := grid.DefaultGrids{}.Square(12, 3, roadCell)
	units := make([]roadUnit, 3)
	for i := range units {
		units[i] = roadUnit{start: g.At(uint32(i), 1), selected: true}
	}
	rw := newRoadWorld(t, 12, units)
	rw.nav.marches.Add(command.Nobody, March{Cell: rw.at(11, 1),
		In: Formation{Shape: Column + 1}})
	rw.ecs.Tick(time.Second / 60)
	for row := range units {
		_, o := rw.state(rw.byRow[row])
		if o == nil || o.Formation != (Formation{}) || o.Party != 3 {
			t.Errorf("unit %d: order %+v, want a MoveTo's of a party of 3, in no formation", row, o)
		}
	}
}

// six in a row along a corridor, 14 apart, the sixth nearest the goal, which the effect halted
// stops
func newCorridorColumn(t *testing.T) (*fieldWorld, geom.Vec) {
	t.Helper()
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(12, 1, fieldCell)}
	goal := probe.grid.CellCenter(probe.at(11, 0))
	var units []fieldUnit
	for i := range 6 {
		units = append(units, fieldUnit{at: geom.NewVec(16+14*float32(i), 16), side: 6,
			selected: true})
	}
	halting := func(p *Plugin) {
		p.worldPlugin.Write().Effects().Define("halted", effect.Spec{
			effect.Alter(func(s *steering.Steering) { s.Halted = true }),
		})
	}
	return newFieldWorldWith(t, 12, 1, BodySpacing, nil, units, halting), goal
}

func TestColumn_LaysItsMembersOutByTheWayToTheGoal(t *testing.T) {
	fw, goal := newCorridorColumn(t)
	fw.nav.marches.Add(command.Nobody, March{Cell: fw.at(11, 0), At: goal,
		In: ColumnOf(1, false, 0)})
	fw.ecs.Tick(time.Second / 60)
	for i := range 6 {
		_, o := fw.centre(i)
		if o == nil || o.Place != uint16(6-i) {
			t.Fatalf("unit %d: order %+v, want it %d-th from the goal", i, o, 6-i)
		}
		if i == 5 {
			if f32.Abs(o.Gap-14) > 0.5 {
				t.Errorf("the head's widest gap %v, want the 14 they stand apart", o.Gap)
			}
			continue
		}
		if o.AheadOf != fw.ids[i+1] || f32.Abs(o.Lag-14) > 0.5 {
			t.Errorf("unit %d walks %v behind %d, want 14 behind unit %d", i, o.Lag, o.AheadOf,
				i+1)
		}
	}

	fw.nav.worldPlugin.Write().Carrier().PutFrom(fw.ids[2], world.Despawn{})
	for gone := false; !gone; {
		fw.ecs.Tick(time.Second / 60)
		gone = true
		fw.each(func(id uid.UID64, _ *world.Base, _ cell.ID, _ *MoveOrder, _ *collision.Collider) {
			gone = gone && id != fw.ids[2]
		})
	}
	for k, i := range []int{5, 4, 3, 1, 0} {
		if _, o := fw.centre(i); o == nil || o.Place != uint16(k+1) {
			t.Errorf("with unit 2 gone, unit %d: order %+v, want it %d-th", i, o, k+1)
		}
	}
	if _, o := fw.centre(1); o.AheadOf != fw.ids[3] || f32.Abs(o.Lag-28) > 1 {
		t.Errorf("with unit 2 gone, unit 1 walks %v behind %d, want 28 behind unit 3", o.Lag,
			o.AheadOf)
	}
}

// what the column adds to a queue's trailing: the room kept before anyone is touched
func TestColumn_StopsARoomBehindItsHeadHalted(t *testing.T) {
	fw, goal := newCorridorColumn(t)
	halted := fw.nav.worldPlugin.Write().Effects().Named("halted")
	fw.nav.worldPlugin.Write().Carrier().Put(1, ceremony.Put(halted).On(ceremony.ID(fw.ids[5])))
	fw.nav.marches.Add(command.Nobody, March{Cell: fw.at(11, 0), At: goal,
		In: ColumnOf(1, false, 0)})
	room := spacing.Room(bodies.Member{Pos: world.Position{AABB: plane.NewAABB(geom.Vec{}, 6,
		6)}}, ColumnOf(1, false, 0))
	nearest := f32.Inf(1)
	for tick := 1; tick <= 4*60; tick++ {
		fw.ecs.Tick(time.Second / 60)
		var at [6]geom.Vec
		for i := range 6 {
			c, o := fw.centre(i)
			if o == nil || i < 5 && o.Stalls > 0 {
				t.Fatalf("tick %d: unit %d's order %+v, want it waiting without a stall", tick, i,
					o)
			}
			at[i] = c
		}
		for i := range 5 {
			nearest = min(nearest, f32.Hypot(at[i+1].X-at[i].X, at[i+1].Y-at[i].Y))
		}
	}
	if nearest < room-1 {
		t.Errorf("two came %v apart, want a room, %v, kept behind the one ahead", nearest, room)
	}
	head, _ := fw.centre(5)
	last, _ := fw.centre(0)
	if d := head.X - last.X; d > 5*room+5 {
		t.Errorf("the last stands %v behind the halted head, want the column closed up, %v",
			d, 5*room)
	}
}

func TestColumn_WalksAtThePaceOfTheOneAheadOverSlowGround(t *testing.T) {
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(12, 1, fieldCell)}
	goal := probe.grid.CellCenter(probe.at(11, 0))
	strip := func(b testBoard, at func(x, y uint32) cell.ID) {
		for _, x := range []uint32{5, 6} {
			b.Set(at(x, 0), cell.Kind{Pace: 0.01, Allows: cell.Land})
		}
	}
	var units []fieldUnit
	for i := range 6 {
		units = append(units, fieldUnit{at: geom.NewVec(16+14*float32(i), 16), side: 6,
			selected: true, paced: true})
	}
	fw := newFieldWorld(t, 12, 1, BodySpacing, strip, units)
	fw.nav.marches.Add(command.Nobody, March{Cell: fw.at(11, 0), At: goal,
		In: ColumnOf(1, false, 0)})
	inStrip := func(x float32) bool { return x > 5*fieldCell+6 && x < 7*fieldCell-6 }
	var arrived []int
	paced := false
	for tick := 1; len(arrived) < len(units); tick++ {
		if tick > 200*60 {
			t.Fatalf("after 200 s only %v arrived", arrived)
		}
		fw.ecs.Tick(time.Second / 60)
		var at [6]geom.Vec
		var speed [6]float32
		var orders [6]*MoveOrder
		fw.each(func(id uid.UID64, b *world.Base, _ cell.ID, o *MoveOrder, _ *collision.Collider) {
			for i := range units {
				if fw.ids[i] == id {
					at[i], speed[i], orders[i] = b.Pos.Center(), f32.Hypot(b.Vel.Delta().X,
						b.Vel.Delta().Y), o
				}
			}
		})
		for i, o := range orders {
			if o == nil {
				if !containsInt(arrived, i) {
					arrived = append(arrived, i)
				}
				continue
			}
			if o.Stalls > 0 {
				t.Fatalf("tick %d: unit %d stalled, want none at the pace of the one ahead", tick,
					i)
			}
			if i == 5 || orders[i+1] == nil || at[i+1].X > 10*fieldCell {
				continue
			}
			if at[i].X >= at[i+1].X {
				t.Fatalf("tick %d: unit %d at %v is past unit %d at %v", tick, i, at[i], i+1,
					at[i+1])
			}
			if o.AheadOf == fw.ids[i+1] && inStrip(at[i].X) && inStrip(at[i+1].X) {
				paced = true
				if speed[i] > speed[i+1]+0.05 {
					t.Errorf("tick %d: unit %d goes %v over the strip, the one ahead of it %v",
						tick, i, speed[i], speed[i+1])
				}
			}
		}
	}
	if !paced {
		t.Error("no two walked the strip one behind the other")
	}
	for k, i := range []int{5, 4, 3, 2, 1, 0} {
		if arrived[k] != i {
			t.Fatalf("arrived %v, want in the order they started, the head first", arrived)
		}
	}
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// what a column of six on the road from (0..5, 1) to (19, 1) did, its tail halted at a second
// for three: by tick, the head's cell, whether it was under its order and its widest gap
type halting struct {
	heads   []cell.ID
	ordered []bool
	gaps    []float32
	room    float32
	arrived []int
}

func haltTheTail(t *testing.T, closed bool) halting {
	t.Helper()
	units := make([]roadUnit, 6)
	rw := newRoadWorld(t, 20, units)
	for i := range units {
		units[i] = roadUnit{start: rw.at(uint32(i), 1), selected: true}
	}
	rw = newRoadWorld(t, 20, units)
	halted := rw.w.Write().Effects().Named("halted")
	column := ColumnOf(1, closed, 1)
	rw.nav.marches.Add(command.Nobody, March{Cell: rw.at(19, 1), In: column})
	h := halting{room: spacing.Room(bodies.Member{Pos: posAt(rw.grid, rw.at(0, 1))}, column)}
	for tick := 1; len(h.arrived) < len(units); tick++ {
		if tick > 30*60 {
			t.Fatalf("after 30 s only %v arrived", h.arrived)
		}
		if tick == 60 {
			rw.w.Write().Carrier().Put(1, ceremony.Put(halted).On(ceremony.ID(rw.byRow[0])).
				For(3*time.Second))
		}
		rw.ecs.Tick(time.Second / 60)
		c, o := rw.state(rw.byRow[5])
		h.heads, h.ordered = append(h.heads, c), append(h.ordered, o != nil)
		if o == nil {
			h.gaps = append(h.gaps, 0)
		} else {
			h.gaps = append(h.gaps, o.Gap)
		}
		for i := range units {
			if _, o := rw.state(rw.byRow[i]); o == nil && !containsInt(h.arrived, i) {
				h.arrived = append(h.arrived, i)
			}
		}
	}
	return h
}

// the longest the head stood in one cell under its order, between ticks from and to, and the
// tick it stepped on after
func (h halting) longestStand(from, to int) (ticks, left int) {
	run := 0
	for k := from; k < to && k < len(h.heads); k++ {
		if k > from && h.ordered[k] && h.heads[k] == h.heads[k-1] {
			run++
		} else {
			run = 0
		}
		if run > ticks {
			ticks, left = run, k+1
		}
	}
	return ticks, left
}

func TestColumn_AClosedHeadWaitsForItsTailWhileItComesOn(t *testing.T) {
	h := haltTheTail(t, true)
	const halt, halted = 60, 3 * 60
	if stood, _ := h.longestStand(halt, halt+150); stood < 48 {
		t.Errorf("with the tail halted the head stood %d ticks at most, want it waiting a while",
			stood)
	}
	if h.heads[halt+halted-1] == h.heads[halt+100] {
		t.Errorf("the head still stands at %v as the halt ends, want it gone on, the tail not "+
			"coming on", h.heads[halt+100])
	}
	stood, left := h.longestStand(halt+halted, len(h.heads))
	if stood < 60 {
		t.Errorf("with the tail coming on again the head stood %d ticks at most, want it "+
			"waiting till the tail closes up", stood)
	}
	if left < len(h.gaps) && h.gaps[left] > 2*h.room {
		t.Errorf("the head stepped on with a gap of %v, wider than the slack lets it, %v",
			h.gaps[left], 2*h.room)
	}
}

func TestColumn_AnOpenHeadDoesNotWait(t *testing.T) {
	h := haltTheTail(t, false)
	if stood, _ := h.longestStand(30, len(h.heads)); stood >= 30 {
		t.Errorf("the head of an open column stood %d ticks in one cell, want it never waiting",
			stood)
	}
}

func TestColumn_TenThroughAOneCellCorridorArriveInTheirOrder(t *testing.T) {
	const width = 30
	units := make([]roadUnit, 10)
	rw := newRoadWorld(t, width, units)
	for i := range units {
		units[i] = roadUnit{start: rw.at(uint32(i), 1), selected: true}
	}
	rw = newRoadWorld(t, width, units)
	for x := uint32(0); x < width; x++ {
		rw.walls(x, 0, x, 2)
	}
	rw.nav.marches.Add(command.Nobody, March{Cell: rw.at(width-1, 1), In: ColumnOf(1, true, 1)})
	var arrived []int
	for tick := 1; len(arrived) < len(units); tick++ {
		if tick > 60*60 {
			t.Fatalf("after 60 s only %v arrived", arrived)
		}
		rw.ecs.Tick(time.Second / 60)
		for i := range units {
			_, o := rw.state(rw.byRow[i])
			switch {
			case o == nil && !containsInt(arrived, i):
				arrived = append(arrived, i)
			case o != nil && o.Stalls > 0:
				t.Fatalf("tick %d: unit %d stalled in the corridor", tick, i)
			}
		}
	}
	for k := range units {
		if arrived[k] != len(units)-1-k {
			t.Fatalf("arrived %v, want in the order they started, the head first", arrived)
		}
	}
}

func TestColumn_TwoColumnsThroughOneGapBothArrive(t *testing.T) {
	const cols, rows = 16, 7
	probe := &fieldWorld{grid: grid.DefaultGrids{}.Square(cols, rows, fieldCell)}
	gap := func(b testBoard, at func(x, y uint32) cell.ID) {
		for y := uint32(0); y < rows; y++ {
			if y != 3 {
				b.Set(at(8, y), wall)
			}
		}
	}
	goals := [2]geom.Vec{probe.grid.CellCenter(probe.at(14, 1)),
		probe.grid.CellCenter(probe.at(14, 5))}
	var units []fieldUnit
	for g := range 2 {
		for i := range 5 {
			at := geom.NewVec(20+float32(i)*14, 20+float32(g)*120)
			units = append(units, fieldUnit{at: at, side: 6, selected: true,
				owner: command.Issuer(1 + g), paced: true})
		}
	}
	fw := newFieldWorld(t, cols, rows, BodySpacing, gap, units)
	for g, at := range goals {
		c, _ := fw.grid.CellAt(at)
		fw.nav.marches.Add(command.Issuer(1+g), March{Cell: c, At: at, In: ColumnOf(1, true, 1)})
	}
	if settled, _ := fw.run(60 * time.Second); !settled {
		t.Fatal("the two columns have not settled in 60 s")
	}
	for i := range units {
		at, _ := fw.centre(i)
		if g := goals[i/5]; f32.Hypot(at.X-g.X, at.Y-g.Y) > 2*fieldCell {
			t.Errorf("unit %d of column %d stands at %v, want it by its goal %v", i, i/5, at, g)
		}
	}
}

func TestColumn_AMemberKeepsThePaceOfTheOneAheadOfIt(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 1, 32)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	finder := spacing.NewFinder(g, terrain, nil, spacing.OpenOccupancy{})
	east := geom.NewVec(1, 0)
	from := g.At(1, 0)
	lead := bodies.Body{ID: 3, At: geom.NewVec(48+10, 16), Half: geom.NewVec(3, 3),
		Vel: geom.NewVec(0.96, 0), Facing: east, Domain: cell.Land, Moving: true}
	for _, k := range []spacing.Keeping{
		spacing.NewBodyKeeping(finder, nil, nil),
		spacing.NewCellKeeping(finder, &cell.SingleOccupancy{}),
	} {
		for _, c := range []struct {
			name    string
			lag     float32
			ahead   bodies.Body
			place   uint16
			pressed bool
			want    float32
		}{
			{"within its room", 10, lead, 2, false, 0.96},
			{"short of its room by a braking's way", 15, lead, 2, false, 48.0096},
			{"farther than its braking takes", 40, lead, 2, false, 96},
			{"pressed into its cell by them", 10, lead, 2, true, 96},
			{"them behind it", 10, movedTo(lead, geom.NewVec(30, 16)), 2, false, 96},
			{"them under no order", 10, standing(lead), 2, false, 96},
			{"the head", 10, lead, 1, false, 96},
		} {
			k.Begin(func(dst []bodies.Body) []bodies.Body { return append(dst, c.ahead) })
			o := MoveOrder{Formation: ColumnOf(1, false, 0), Place: c.place, AheadOf: c.ahead.ID,
				Lag: c.lag}
			m := bodies.Member{ID: 7, Cell: from, From: from, Domain: cell.Land,
				Pos: world.Position{AABB: plane.NewAABB(geom.NewVec(45, 13), 6, 6)}, Facing: east,
				Share: 1, MaxSpeed: 96, Brake: 384, Order: &o, Pressed: c.pressed,
				PressedBy: c.ahead.ID}
			helm := steering.Helm{Steering: &steering.Steering{MaxSpeed: 96},
				Course: &steering.Course{}}
			k.Steer(m, helm, east, 96)
			if f32.Abs(helm.WantSpeed-c.want) > 1e-3 {
				t.Errorf("%T, %s: asked for %v, want %v", k, c.name, helm.WantSpeed, c.want)
			}
		}
	}
}

func movedTo(b bodies.Body, p geom.Vec) bodies.Body {
	b.At = p
	return b
}

func standing(b bodies.Body) bodies.Body {
	b.Moving, b.Vel = false, geom.Vec{}
	return b
}

// keeping its room behind one standing in its formation is no stall; behind one under no order
// it is, as before
func TestColumn_WaitingOnTheOneAheadIsNoStall(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 1, 32)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	finder := spacing.NewFinder(g, terrain, nil, spacing.OpenOccupancy{})
	from, toward := g.At(1, 0), g.At(2, 0)
	waiting := bodies.Body{ID: 3, At: geom.NewVec(48+10, 16), Half: geom.NewVec(3, 3),
		Domain: cell.Land, Moving: true}
	for _, c := range []struct {
		name    string
		ahead   bodies.Body
		stalled bool
	}{
		{"one ahead standing in its order", waiting, false},
		{"one ahead under no order", standing(waiting), true},
	} {
		body := spacing.NewBodyKeeping(finder, nil, nil)
		cells := spacing.NewCellKeeping(finder, &cell.SingleOccupancy{})
		for _, k := range []spacing.Keeping{body, cells} {
			k.Begin(func(dst []bodies.Body) []bodies.Body { return append(dst, c.ahead) })
		}
		o := MoveOrder{Formation: ColumnOf(1, false, 0), Place: 2, AheadOf: c.ahead.ID, Lag: 10}
		held := o
		m := bodies.Member{ID: 7, Cell: from, From: from, Domain: cell.Land,
			Pos: world.Position{AABB: plane.NewAABB(geom.NewVec(45, 13), 6, 6)}, Share: 1,
			MaxSpeed: 96, Brake: 384}
		watched, blocked := false, false
		for range 2 * 60 {
			body.Watch(m, &o, toward, g.CellCenter(toward), time.Second/60)
			cells.Blocked(m, &held, toward, false, time.Second/60)
			watched, blocked = watched || o.Bumped, blocked || held.Bumped
		}
		if watched != c.stalled || blocked != c.stalled {
			t.Errorf("%s: stalled in bodies %v, in cells %v; want %v", c.name, watched, blocked,
				c.stalled)
		}
	}
}

// laying a formation out allocates nothing once warm: off its field kept, nor off one filled
// again after the board changed
func TestColumn_ALayOutAllocatesNothing(t *testing.T) {
	r := newGroupRig(t, nil, westOf(8))
	r.send(1, r.nw.grid.At(26, 16), every(8)...)
	for _, u := range every(8) {
		o, _ := r.order(r.nw.ids[u])
		o.Formation = ColumnOf(1, true, 1)
	}
	for _, tc := range []struct {
		name string
		form func()
	}{
		{"the field kept", func() { r.coord.Form(r.nw.ids) }},
		{"the board changed", func() { r.nw.board.Touch(0); r.coord.Form(r.nw.ids) }},
	} {
		for range 3 {
			tc.form()
		}
		if o, _ := r.order(r.nw.ids[0]); o.Place == 0 {
			t.Fatalf("%s: the formation was not laid out", tc.name)
		}
		if allocs := testing.AllocsPerRun(10, tc.form); allocs != 0 {
			t.Errorf("%s: a lay-out allocates %v times; want none", tc.name, allocs)
		}
	}
}
