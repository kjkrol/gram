package board_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

var (
	grass = cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land | cell.Air}
	pit   = cell.Kind{Name: cell.Named("pit"), Cost: 1, Allows: cell.Air}
)

func itself(c cell.ID) cell.ID { return c }

type strip struct {
	*boardtest.World
	middle cell.ID
	open   effect.Effect
}

func newStrip(t *testing.T, ground cell.Kind, spec effect.Spec, extra ...comp.Comp) *strip {
	t.Helper()
	g := grid.DefaultGrids{}.Square(5, 1, boardtest.CellSize)
	s := &strip{middle: g.At(2, 0)}
	prepare := func(w *world.Plugin, brd *board.Plugin) []act.Rule {
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

func (s *strip) cast(t *testing.T) {
	t.Helper()
	if !s.World.World.Carrier().Put(command.Nobody, ceremony.Put(s.open).On(ceremony.Named("it"))) {
		t.Fatal("the world carries no command about an effect")
	}
	s.Tick()
	s.Tick()
}

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

func TestAround_VisitsTheEntitiesStandingOnTheCellsNotTheUnits(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 1, boardtest.CellSize)
	doors, decoys := g.At(4, 0), g.At(3, 0)
	var open effect.Effect
	prepare := func(w *world.Plugin, brd *board.Plugin) []act.Rule {
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
		w.Roles().Define("walker", act.Branch[unit.Standing]{
			Name: "pull",
			When: cond.Around(1).If(lever),
			Then: act.Around(1).Do(ceremony.Trigger{}),
		})
		walker := w.Roles().Named("walker")

		levers := board.NewEntities(brd, board.Shape{Size: boardtest.UnitSize}, itself)
		levers.Define("lever", ceremony.Plays(lever))
		w.Seed(levers.Named("lever").Entry(g.At(2, 0)).Named("lever"))
		units := board.NewUnits(brd, board.Shape{Size: boardtest.UnitSize}, g.CellCenter)
		walks := unit.Mover{Domain: cell.Land}
		units.Define("walker", walks, steering.Steering{}, ceremony.Plays(walker))
		units.Define("decoy", walks, steering.Steering{}, ceremony.Plays(lever))
		w.Seed(units.Named("walker").Entry(g.At(1, 0)))
		w.Seed(units.Named("decoy").Entry(g.At(0, 0)).Named("decoy"))

		w.Commands().Define("open the doors",
			ceremony.Put(open).On(ceremony.Group("doors")).By(ceremony.Named("lever")))
		w.Commands().Define("open the decoys",
			ceremony.Put(open).On(ceremony.Group("decoys")).By(ceremony.Named("decoy")))
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

func TestFloor_CostsWhatItSaysAndSparesTheSlope(t *testing.T) {
	river := cell.Kind{Name: cell.Named("river"), Cost: 1, Allows: cell.Water}
	ford := newStrip(t, river, effect.Spec{},
		comp.Const(cell.Floor{Allows: cell.Land, Cost: 6.25}))
	k := ford.Board.Board().Kind(ford.middle)
	if !k.Admits(cell.Land) || k.CostFor(cell.Land) != 6.25 || k.CostFor(cell.Water) != 1 {
		t.Errorf("a ford: Land at %v, Water at %v, want 6.25 and the river's 1",
			k.CostFor(cell.Land), k.CostFor(cell.Water))
	}
	if k.Graded {
		t.Error("a ford is no graded way")
	}
	bridge := newStrip(t, river, effect.Spec{},
		comp.Const(cell.Floor{Allows: cell.Land, Cost: 1, Graded: true}))
	if k := bridge.Board.Board().Kind(bridge.middle); !k.Graded || k.CostFor(cell.Land) != 1 {
		t.Errorf("a bridge: graded %v at %v, want graded at 1", k.Graded, k.CostFor(cell.Land))
	}
}

func TestEntities_AVelocityGivenTakesTheStillOnesPlace(t *testing.T) {
	south := geom.NewVec(0, 1)
	s := newStrip(t, grass, effect.Spec{}, comp.Const(world.Velocity{Dir: south}))
	var base goke.Comp[world.Base]
	var q *goke.Query
	s.ECS.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base).Build()
	}})
	var dirs []geom.Vec
	for q.All(); q.Next(); {
		for _, b := range base.Slice(q.Cursor()) {
			dirs = append(dirs, b.Vel.Dir)
		}
	}
	if len(dirs) != 1 || dirs[0] != south {
		t.Errorf("the thing heads %v, want %v", dirs, south)
	}
}

func TestBare_LeavesABridgeOutAndATrapdoorIn(t *testing.T) {
	river := cell.Kind{Name: cell.Named("river"), Cost: 1, Allows: cell.Water}
	bridge := newStrip(t, river, effect.Spec{},
		comp.Const(cell.Floor{Allows: cell.Land, Cost: 1, Graded: true}))
	if bridge.Board.Board().Bare(bridge.middle).Admits(cell.Land) {
		t.Error("the ground beside a bridge takes a walker")
	}
	trapdoor := newStrip(t, pit, effect.Spec{}, comp.Const(cell.Floor{Allows: cell.Land}))
	if !trapdoor.Board.Board().Bare(trapdoor.middle).Admits(cell.Land) {
		t.Error("a shut trapdoor is no ground to cut across")
	}
}

func TestRail_AnEffectTakesTheRailingsDown(t *testing.T) {
	// a band west to east with railings along it
	opens := effect.Spec{effect.Alter(func(r *cell.Rail) { r.Keeps = 0 })}
	s := newStrip(t, grass, opens,
		comp.Const(cell.Floor{Allows: cell.Land, Links: 1<<2 | 1<<3, Width: 4}),
		comp.Const(cell.Rail{Keeps: cell.Land}))
	brd := s.Board.Board()
	if b := brd.Built(s.middle); b.Rail.Keeps != cell.Land {
		t.Fatalf("built: %+v, want a Rail keeping Land", b)
	}
	s.cast(t)
	if b := brd.Built(s.middle); b.Rail.Keeps != 0 {
		t.Errorf("built under the effect: %+v, want the railings gone", b)
	}
}

func TestEntities_CastBakedShadowsUnlessTheirKindSaysOtherwise(t *testing.T) {
	for _, c := range []struct {
		extra []comp.Comp
		want  world.Casting
	}{
		{nil, world.Baked},
		{[]comp.Comp{comp.Const(world.Shadow{Cast: world.NoShadow})}, world.NoShadow},
	} {
		s := newStrip(t, grass, effect.Spec{}, c.extra...)
		var shadow goke.Comp[world.Shadow]
		var q *goke.Query
		s.ECS.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&shadow).Build()
		}})
		var got []world.Casting
		for q.All(); q.Next(); {
			for _, sh := range shadow.Slice(q.Cursor()) {
				got = append(got, sh.Cast)
			}
		}
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("a thing given %d components casts %v, want %v", len(c.extra), got, c.want)
		}
	}
}
