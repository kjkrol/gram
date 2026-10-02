package moments_test

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

var grass = cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land}

// A cell.Now is Trodden exactly while a unit's centre stands on the cell, every step: under a
// walker as it goes, under one standing, under one straddling two cells only the one its centre
// is on.
func TestCellNow_TroddenExactlyUnderTheUnitsCentres(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, boardtest.CellSize)
	at := func(x, y uint32) cell.ID { c, _ := g.CellIndex(x, y); return c }
	stood := rule.On("stood", rule.All, func(m *rule.Moment[cell.Now]) rule.Step {
		return m.If(cell.Now.Stood, m.Order(heard{Rule: "stood"}))
	})
	bw := boardtest.NewWorld(t, g, 8*boardtest.CellSize, 8*boardtest.CellSize,
		func(brd *board.Board) { brd.SetAll(grass) },
		[]boardtest.Mover{
			{Here: at(0, 1), Heading: east},
			{Here: at(5, 5)},
			{Here: at(1, 3), Offset: boardtest.CellSize / 2},
		}, stood)
	orders := carried(t, bw.World, &heards{})
	walked := map[cell.ID]bool{}
	for tick := range 150 {
		bw.Tick()
		want := map[uid.UID64]bool{}
		for i, box := range bw.Snapshot() {
			c, ok := g.CellAt(world.Position{AABB: toPlane(box)}.Center())
			if !ok {
				t.Fatalf("tick %d: unit %d's centre is off the board", tick, i)
			}
			if i == 0 {
				walked[c] = true
			}
			id, _ := bw.Board.CellEntity(c)
			want[id] = true
		}
		got := map[uid.UID64]bool{}
		for id, rules := range orders.told() {
			if rules["stood"] {
				got[id] = true
			}
		}
		if !maps.Equal(got, want) {
			t.Fatalf("tick %d: trodden cells' entities %v, want %v, those under the units' centres",
				tick, slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)))
		}
	}
	if len(walked) < 4 {
		t.Fatalf("the walker stood on %d cells, want 4 or more: the test needs it to cross cells", len(walked))
	}
}

// signal gives the world sig as nobody does — a key pulling a lever.
func signal(t *testing.T, w *world.Plugin, sig rule.Signal) {
	t.Helper()
	if !w.Commands().Put(control.Nobody, sig) {
		t.Fatal("the world does not carry a rule.Signal")
	}
}

// A trapdoor — a role whose rule keeps its cell open WhileWire — opens with the wire its cell is
// wired to and closes when the wire goes off: a pulse while it lasts, a switch until flipped again.
// A trapdoor on another wire, one wired to none and a cell on the wire playing no trapdoor stay
// shut.
func TestRole_ATrapdoorIsOpenWhileItsWireIsOn(t *testing.T) {
	g := grid.DefaultGrids{}.Square(7, 7, boardtest.CellSize)
	at := func(x, y uint32) cell.ID { c, _ := g.CellIndex(x, y); return c }
	westA, westB, eastDoor, idle, unwired := at(1, 1), at(5, 1), at(1, 5), at(3, 1), at(5, 5)
	pit := cell.Kind{Name: cell.Named("pit"), Cost: 1}
	var on, lit, open effect.Effect
	var west, eastWire *rule.Wire
	pw := newPlaceWorld(t, g, false, func(pw *placeWorld) []rule.Rule {
		on = pw.fx.Define("on", effect.Spec{effect.Lasts(3 * time.Second / 10)})
		lit = pw.fx.Define("lit", effect.Spec{})
		open = pw.fx.Define("open", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind = pit })})
		trapdoor := rule.Role("trapdoor").Obeys(
			rule.On("open while on", rule.All, func(m *rule.Moment[cell.Now]) rule.Step { return m.WhileWire(on, m.Keep(open)) }),
			rule.On("open while lit", rule.All, func(m *rule.Moment[cell.Now]) rule.Step { return m.WhileWire(lit, m.Keep(open)) }),
		)
		west, eastWire = pw.w.Wire("west"), pw.w.Wire("east")
		door := []*rule.Part{trapdoor}
		pw.brd.Seed(board.Layout{Cells: []cell.Entry{
			{Cell: westA, Roles: door, Wired: west},
			{Cell: westB, Roles: door, Wired: west},
			{Cell: eastDoor, Roles: door, Wired: eastWire},
			{Cell: idle, Wired: west},
			{Cell: unwired, Roles: door},
		}})
		return trapdoor.Rules()
	})
	// pits are the cells the trapdoors' effect has made pits of.
	pits := func() map[cell.ID]bool {
		out := map[cell.ID]bool{}
		pw.brd.Res.Logic.Board.EachCell(func(c cell.ID) {
			if pw.brd.Res.Logic.Board.Kind(c) == pit {
				out[c] = true
			}
		})
		return out
	}
	opened := func(want ...cell.ID) bool {
		w := map[cell.ID]bool{}
		for _, c := range want {
			w[c] = true
		}
		return maps.Equal(pits(), w)
	}
	// within ticks up to n times until the open cells are want, failing the test if they never are.
	within := func(what string, n int, want ...cell.ID) {
		t.Helper()
		for range n {
			pw.tick(1)
			if opened(want...) {
				return
			}
		}
		t.Fatalf("%s: open %v after %d ticks, want %v", what, pits(), n, want)
	}

	pw.tick(3)
	if !opened() {
		t.Fatalf("no wire on: open %v, want none", pits())
	}

	signal(t, pw.w, rule.Signal{Wire: west, Effect: on})
	within("west pulsed", 3, westA, westB)
	within("west's pulse over", 6)
	if k := pw.brd.Res.Logic.Board.Kind(westA); k != grass {
		t.Errorf("a shut trapdoor is %q, want the grass it was", k.Name)
	}
	if got := pw.under(open); len(got) != 0 {
		t.Errorf("west's pulse over: the trapdoors' effect still on %v", got)
	}

	signal(t, pw.w, rule.Signal{Wire: eastWire, Effect: lit, Toggle: true})
	within("east switched on", 3, eastDoor)
	for tick := range 20 {
		pw.tick(1)
		if !opened(eastDoor) {
			t.Fatalf("tick %d with east switched on: open %v, want the east trapdoor %d alone", tick, pits(), eastDoor)
		}
	}
	signal(t, pw.w, rule.Signal{Wire: eastWire, Effect: lit, Toggle: true})
	within("east switched off", 4)
	pw.tick(10)
	if !opened() {
		t.Errorf("east switched off: open %v, want none", pits())
	}
}

// A plate — a role whose rule pulses its wire OnWire while stood on — opens the trapdoors on its
// wire as a walker's centre comes onto it, holds them open while the walker stands there and for
// as long as the pulse lasts after it steps off, then lets them shut; a plate nobody stands on
// opens nothing.
func TestRole_APlateStoodOnPulsesItsWire(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 3, boardtest.CellSize)
	at := func(x, y uint32) cell.ID { c, _ := g.CellIndex(x, y); return c }
	plateCell, doorA, doorB, quietPlate, quietDoor := at(2, 1), at(5, 0), at(5, 2), at(2, 2), at(7, 0)
	const pulse = 30 // ticks the pulse lasts
	bw := boardtest.NewWorldWith(t, g, 8*boardtest.CellSize, 3*boardtest.CellSize, func(w *world.Plugin, brd *board.Plugin) []rule.Rule {
		fx := w.Effects()
		on := fx.Define("on", effect.Spec{effect.Lasts(pulse * time.Second / 60)})
		open := fx.Define("open", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind.Name = cell.Named("open") })})
		trapdoor := rule.Role("trapdoor").Obeys(rule.On("open while on", rule.All, func(m *rule.Moment[cell.Now]) rule.Step {
			return m.WhileWire(on, m.Keep(open))
		}))
		plate := rule.Role("plate").Obeys(rule.On("press", rule.All, func(m *rule.Moment[cell.Now]) rule.Step {
			return m.If(cell.Now.Stood, m.OnWire(m.Apply(on)))
		}))
		gate, quiet := w.Wire("gate"), w.Wire("quiet")
		brd.Res.Logic.Board.SetAll(grass)
		brd.Seed(board.Layout{Cells: []cell.Entry{
			{Cell: plateCell, Roles: []*rule.Part{plate}, Wired: gate},
			{Cell: doorA, Roles: []*rule.Part{trapdoor}, Wired: gate},
			{Cell: doorB, Roles: []*rule.Part{trapdoor}, Wired: gate},
			{Cell: quietPlate, Roles: []*rule.Part{plate}, Wired: quiet},
			{Cell: quietDoor, Roles: []*rule.Part{trapdoor}, Wired: quiet},
		}})
		if err := brd.Populate(); err != nil {
			t.Fatal(err)
		}
		return append(trapdoor.Rules(), plate.Rules()...)
	}, []boardtest.Mover{{Here: at(0, 1), Heading: east}})
	isOpen := func(c cell.ID) bool { return bw.Board.Res.Logic.Board.Kind(c).Name.String() == "open" }

	first, last := -1, -1 // the ticks the walker's centre is first and last on the plate
	type sample struct{ onPlate, a, b, quiet bool }
	var run []sample
	for tick := range 150 {
		bw.Tick()
		c, _ := g.CellAt(world.Position{AABB: toPlane(bw.Snapshot()[0])}.Center())
		s := sample{onPlate: c == plateCell, a: isOpen(doorA), b: isOpen(doorB), quiet: isOpen(quietDoor)}
		if s.onPlate {
			if first < 0 {
				first = tick
			}
			last = tick
		}
		run = append(run, s)
	}
	if first < 0 || last+pulse+8 >= len(run) {
		t.Fatalf("the walker stood on the plate from tick %d to %d of %d: the test needs it on and off in time", first, last, len(run))
	}
	const lag = 3 // ticks from a press to the doors' open, through the wire's effect and the doors' Keep
	for tick, s := range run {
		switch {
		case s.quiet:
			t.Fatalf("tick %d: the door of the plate nobody stands on is open", tick)
		case s.a != s.b:
			t.Fatalf("tick %d: the gate's doors open %v and %v, want them together", tick, s.a, s.b)
		case tick < first && s.a:
			t.Fatalf("tick %d: the gate's doors open before the walker came onto the plate at %d", tick, first)
		case tick >= first+lag && tick <= last+pulse-lag && !s.a:
			t.Fatalf("tick %d: the gate's doors shut, the walker on the plate from %d to %d, the pulse lasting %d", tick, first, last, pulse)
		case tick > last+pulse+lag && s.a:
			t.Fatalf("tick %d: the gate's doors still open, the walker off the plate since %d, the pulse lasting %d", tick, last, pulse)
		}
	}
}
