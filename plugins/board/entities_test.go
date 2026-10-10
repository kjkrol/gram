package board_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

var (
	grass = cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land | cell.Air}
	pit   = cell.Kind{Name: cell.Named("pit"), Cost: 1, Allows: cell.Air}
)

func itself(c cell.ID) cell.ID { return c }

// strip is a 5 x 1 board of grass with ground in the middle and a thing called "it" standing on
// it, carrying extra; open is the effect opening it, as spec says.
type strip struct {
	*boardtest.World
	middle cell.ID
	open   effect.Effect
}

func newStrip(t *testing.T, ground cell.Kind, spec effect.Spec, extra ...comp.Comp) *strip {
	t.Helper()
	g := grid.DefaultGrids{}.Square(5, 1, boardtest.CellSize)
	s := &strip{middle: g.At(2, 0)}
	prepare := func(w *world.Plugin, brd *board.Plugin) []rule.Rule {
		brd.Board().SetAll(grass)
		brd.Board().Set(s.middle, ground)
		w.Effects().Define("open", spec)
		s.open = w.Effects().Named("open")
		things := board.NewEntities(brd, board.Shape{Size: boardtest.UnitSize}, itself)
		things.Define("thing", extra...)
		w.Seed(things.Named("thing").Entry(s.middle).Named("it"))
		return nil
	}
	s.World = boardtest.NewWorldWith(t, g, 5*boardtest.CellSize, boardtest.CellSize, prepare, nil)
	s.Tick()
	return s
}

// cast puts open on the thing standing on the strip and ticks it through.
func (s *strip) cast(t *testing.T) {
	t.Helper()
	if !s.World.World.Carrier().Put(control.Nobody, rule.Cast(s.open).On(entity.Named("it"))) {
		t.Fatal("the world carries no command about an effect")
	}
	s.Tick()
	s.Tick()
}

// A Floor carries its domains over a pit: the cell admits a walker while it is shut; opened by
// an effect, the pit takes over again.
func TestFloor_CarriesItsDomainsOverThePitUntilOpened(t *testing.T) {
	opens := effect.Spec{effect.Alter(func(f *cell.Floor) { f.Allows = 0 })}
	s := newStrip(t, pit, opens, comp.Const(cell.Floor{Allows: cell.Land}))
	brd := s.Board.Board()
	if k := brd.Kind(s.middle); !k.Admits(cell.Land) || !k.Admits(cell.Air) {
		t.Fatalf("a shut trapdoor over a pit admits %v, want Land and Air", k.Allows)
	}
	if b := brd.Built(s.middle); b.Floor.Allows != cell.Land {
		t.Fatalf("built over the pit: %+v, want a Floor of Land", b)
	}
	v := brd.CellVersion(s.middle)
	s.cast(t)
	if k := brd.Kind(s.middle); k.Admits(cell.Land) || !k.Admits(cell.Air) {
		t.Errorf("an open trapdoor over a pit admits %v, want Air alone", k.Allows)
	}
	if brd.CellVersion(s.middle) == v {
		t.Error("opening the trapdoor counted no change of the cell")
	}
	if brd.Kind(s.middle-1).Allows != grass.Allows {
		t.Error("the trapdoor changed the cell beside it")
	}
}

// A Wall keeps its domains off the cell: the cell admits them no more and stands solid for them,
// not for the others; opened, it is the grass it stands on.
func TestWall_KeepsItsDomainsOffTheCell(t *testing.T) {
	opens := effect.Spec{effect.Alter(func(w *cell.Wall) { w.Keeps = 0 })}
	s := newStrip(t, grass, opens, comp.Const(cell.Wall{Keeps: cell.Land}))
	brd := s.Board.Board()
	if k := brd.Kind(s.middle); k.Admits(cell.Land) || !k.Admits(cell.Air) || !k.Solid {
		t.Fatalf("a shut gate: Allows %v Solid %v, want Air alone, solid", k.Allows, k.Solid)
	}
	if got := s.Solid(world.Layers(cell.Land)); len(got) != 1 {
		t.Errorf("solid for a walker: %v, want the gate's cell", got)
	}
	if got := s.Solid(world.Layers(cell.Air)); len(got) != 0 {
		t.Errorf("solid for a flyer: %v, want nothing", got)
	}
	s.cast(t)
	if k := brd.Kind(s.middle); !k.Admits(cell.Land) || k.Solid {
		t.Errorf("an open gate: Allows %v Solid %v, want the grass's", k.Allows, k.Solid)
	}
	if got := s.Solid(world.Layers(cell.Land)); len(got) != 0 {
		t.Errorf("solid for a walker through the open gate: %v, want nothing", got)
	}
}

// Around visits the entities standing on the cells round where a moment stands, not the units:
// a walker beside a lever pulls it, and a unit beside the walker playing a lever is pulled by
// nobody.
func TestAround_VisitsTheEntitiesStandingOnTheCellsNotTheUnits(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 1, boardtest.CellSize)
	doors, decoys := g.At(4, 0), g.At(3, 0)
	var open effect.Effect
	prepare := func(w *world.Plugin, brd *board.Plugin) []rule.Rule {
		brd.Board().SetAll(grass)
		brd.Seed(board.Layout{Cells: []cell.Entry{
			{Cell: doors, Group: "doors"},
			{Cell: decoys, Group: "decoys"},
		}})
		if err := brd.Populate(); err != nil {
			t.Fatal(err)
		}
		w.Effects().Define("open", effect.Spec{})
		open = w.Effects().Named("open")
		w.Roles().Define("lever")
		lever := w.Roles().Named("lever")
		pull := rule.Around(1, rule.Playing(lever, rule.Trigger()))
		w.Roles().Define("walker", rule.Then[unit.Standing]("pull", rule.All, pull))
		walker := w.Roles().Named("walker")

		levers := board.NewEntities(brd, board.Shape{Size: boardtest.UnitSize}, itself)
		levers.Define("lever", rule.Plays(lever))
		w.Seed(levers.Named("lever").Entry(g.At(2, 0)).Named("lever"))
		units := board.NewUnits(brd, board.Shape{Size: boardtest.UnitSize}, g.CellCenter)
		walks := unit.Mover{Domain: cell.Land}
		units.Define("walker", walks, steering.Steering{}, rule.Plays(walker))
		units.Define("decoy", walks, steering.Steering{}, rule.Plays(lever))
		w.Seed(units.Named("walker").Entry(g.At(1, 0)))
		w.Seed(units.Named("decoy").Entry(g.At(0, 0)).Named("decoy"))

		w.Commands().Define("open the doors",
			rule.Cast(open).On(entity.Group("doors")).By(entity.Named("lever")))
		w.Commands().Define("open the decoys",
			rule.Cast(open).On(entity.Group("decoys")).By(entity.Named("decoy")))
		return walker.Rules()
	}
	bw := boardtest.NewWorldWith(t, g, 5*boardtest.CellSize, boardtest.CellSize, prepare, nil)
	for range 4 {
		bw.Tick()
	}
	brd := bw.Board.Board()
	if !brd.States(doors).Has(open.Mark()) {
		t.Error("the walker beside the lever did not pull it: the doors are shut")
	}
	if brd.States(decoys).Has(open.Mark()) {
		t.Error("the unit beside the walker was pulled as a lever: Around visited a unit")
	}
}

// A flat world refuses a Floor or a Wall at a Height the first step it meets one.
func TestBuilt_AFlatWorldRefusesAHeight(t *testing.T) {
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "flat world") {
			t.Errorf("a Wall at a Height in a flat world: %q, want a panic saying so", msg)
		}
	}()
	newStrip(t, grass, effect.Spec{}, comp.Const(cell.Wall{Keeps: cell.Land, Height: 3}))
}
