package board_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

type meadow struct {
	*boardtest.World
	at                       func(x, y uint32) cell.ID
	lever, post, plate       cell.ID
	west, east               []cell.ID
	open                     effect.Effect
	openWest, openEast, flip rule.Command
}

func newMeadow(t *testing.T, standing ...cell.ID) *meadow {
	t.Helper()
	g := grid.DefaultGrids{}.Square(6, 6, boardtest.CellSize)
	m := &meadow{at: func(x, y uint32) cell.ID { c := g.At(x, y); return c }}
	m.lever, m.post, m.plate = m.at(0, 5), m.at(2, 5), m.at(5, 5)
	m.west, m.east = []cell.ID{m.at(1, 1), m.at(1, 2)}, []cell.ID{m.at(4, 1), m.at(4, 2)}
	var units []boardtest.Mover
	for _, c := range standing {
		units = append(units, boardtest.Mover{Here: c})
	}
	m.World = boardtest.NewWorldWith(t, g, 6*boardtest.CellSize, 6*boardtest.CellSize, func(w *world.Plugin, brd *board.Plugin) []rule.Rule {
		brd.Board().SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
		w.Effects().Define("open", effect.Spec{effect.Lasts(3 * time.Second / 60)})
		m.open = w.Effects().Named("open")
		w.Roles().Define("plate", rule.Branch[collision.Meeting]{
			Name: "press",
			On:   rule.Self,
			Do:   rule.Apply(rule.Triggered{}),
		})
		plate := w.Roles().Named("plate")
		plates := board.NewEntities(brd, board.Shape{Size: boardtest.UnitSize}, itself)
		plates.Define("plate", comp.Const(collision.Collider{}), rule.Plays(plate))
		w.Seed(plates.Named("plate").Entry(m.plate).Named("plate"))
		cells := []cell.Entry{{Cell: m.lever, Name: "lever"}, {Cell: m.post, Name: "post"}}
		for _, c := range m.west {
			cells = append(cells, cell.Entry{Cell: c, Group: "west doors"})
		}
		for _, c := range m.east {
			cells = append(cells, cell.Entry{Cell: c, Group: "east doors"})
		}
		brd.Seed(board.Layout{Cells: cells})
		if err := brd.Populate(); err != nil {
			t.Fatal(err)
		}
		m.openWest = rule.Put(m.open).On(entity.Group("west doors")).By(entity.Named("lever"))
		m.openEast = rule.Put(m.open).On(entity.Group("east doors")).By(entity.Named("plate"))
		m.flip = rule.Toggle(m.open).On(entity.Named("lever", "post")).For(time.Hour)
		for name, cmd := range map[string]rule.Command{"open west": m.openWest, "open east": m.openEast, "flip": m.flip} {
			w.Commands().Define(name, cmd)
		}
		return plate.Rules()
	}, units)
	return m
}

func (m *meadow) give(t *testing.T, cmd control.Command) {
	t.Helper()
	if !m.World.World.Carrier().Put(control.Nobody, cmd) {
		t.Fatalf("the world carries no %T", cmd)
	}
	m.Tick()
	m.Tick()
}

func (m *meadow) under(cs ...cell.ID) int {
	brd, n := m.Board.Board(), 0
	for _, c := range cs {
		if brd.States(c).Has(m.open.Mark()) {
			n++
		}
	}
	return n
}

func TestCast_ReachesTheCellsOfItsGroupAlone(t *testing.T) {
	m := newMeadow(t)
	m.give(t, m.openWest)
	w, e, l := m.under(m.west...), m.under(m.east...), m.under(m.lever, m.post)
	if w != 2 || e != 0 || l != 0 {
		t.Fatalf("open the west doors: %d west, %d east, %d of the lever and the post under it; "+
			"want 2, 0, 0", w, e, l)
	}
	for range 4 {
		m.Tick()
	}
	if w := m.under(m.west...); w != 0 {
		t.Errorf("%d west doors still open once the effect ran out, want none", w)
	}
	m.give(t, m.openEast)
	m.give(t, rule.Take(m.open).On(entity.Group("east doors")))
	if e := m.under(m.east...); e != 0 {
		t.Errorf("%d east doors open after a Lift, want none", e)
	}
}

func TestCast_ForSaysHowLongItLasts(t *testing.T) {
	m := newMeadow(t)
	m.give(t, rule.Put(m.open).On(entity.Named("lever")).For(time.Second))
	for range 8 { // well past the three steps the Spec gives it
		m.Tick()
	}
	if m.under(m.lever) != 1 {
		t.Fatal("cast for a second, gone within ten steps: For did not outlast the Spec")
	}
	for range 60 {
		m.Tick()
	}
	if m.under(m.lever) != 0 {
		t.Error("cast for a second, still on after more than a second")
	}
}

func TestToggle_SwitchesThoseItNames(t *testing.T) {
	m := newMeadow(t)
	m.give(t, m.flip)
	if n := m.under(m.lever, m.post); n != 2 {
		t.Fatalf("flipped on: %d of the two named under it, want both", n)
	}
	m.give(t, m.flip)
	if n := m.under(m.lever, m.post); n != 0 {
		t.Fatalf("flipped off: %d of the two named under it, want none", n)
	}
	m.World.World.Carrier().Put(control.Nobody, m.flip)
	m.give(t, m.flip)
	if n := m.under(m.lever, m.post); n != 0 {
		t.Errorf("flipped twice in a step: %d under it, want them as they were, off", n)
	}
}

func TestTrigger_GivesTheCommandsOfItsSource(t *testing.T) {
	m := newMeadow(t, (&meadow{}).plateCell())
	for range 4 {
		m.Tick()
	}
	if w, e := m.under(m.west...), m.under(m.east...); w != 0 || e != 2 {
		t.Errorf("a unit on the plate: %d west and %d east doors open; want 0 and 2", w, e)
	}
}

func (*meadow) plateCell() cell.ID {
	return grid.DefaultGrids{}.Square(6, 6, boardtest.CellSize).At(5, 5)
}

func TestCommands_RefuseANameNobodyBearsAndOneTwoBear(t *testing.T) {
	for name, tc := range map[string]struct {
		cells []cell.Entry
		cmd   func(e effect.Effect) rule.Command
		want  string
	}{
		"nobody is called so": {
			cells: []cell.Entry{{Cell: 0, Name: "lever"}},
			cmd: func(e effect.Effect) rule.Command {
				return rule.Put(e).On(entity.Named("levr"))
			},
			want: `"levr"`,
		},
		"two bear one name": {
			cells: []cell.Entry{{Cell: 0, Name: "lever"}, {Cell: 1, Name: "lever"}},
			cmd: func(e effect.Effect) rule.Command {
				return rule.Put(e).On(entity.Named("lever"))
			},
			want: "one name",
		},
	} {
		t.Run(name, func(t *testing.T) {
			g := grid.DefaultGrids{}.Square(3, 3, boardtest.CellSize)
			for i := range tc.cells {
				tc.cells[i].Cell = g.At(uint32(i), 0)
			}
			bw := boardtest.NewWorldWith(t, g, 3*boardtest.CellSize, 3*boardtest.CellSize, func(w *world.Plugin, brd *board.Plugin) []rule.Rule {
				brd.Board().SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
				brd.Seed(board.Layout{Cells: tc.cells})
				if err := brd.Populate(); err != nil {
					t.Fatal(err)
				}
				w.Effects().Define("open", effect.Spec{})
				w.Commands().Define("open", tc.cmd(w.Effects().Named("open")))
				return nil
			}, nil)
			defer func() {
				msg, _ := recover().(string)
				if !strings.Contains(msg, tc.want) {
					t.Errorf("the first step panicked with %q; want it to say %s", msg, tc.want)
				}
			}()
			bw.Tick()
		})
	}
}
