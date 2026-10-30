package board_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
)

var gap = cell.Gap{Width: 6, Height: 2}

// walled is a board of grass, 5 cells by 3 of 10, a wall keeping Land off down column 2, its
// middle cell's wall with g in it
func walled(t *testing.T, g cell.Gap) (*board.BoardRead, collision.Field, grid.Grid) {
	t.Helper()
	gr := grid.DefaultGrids{}.Square(5, 3, 10)
	wd := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 30},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	p := board.NewPlugin(gr, &cell.MultipleOccupancy{}, wd)
	p.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land | cell.Air})
	walls := board.NewEntities(p, board.Shape{Size: 10}, itself)
	walls.Define("wall", comp.Const(cell.Wall{Keeps: cell.Land}))
	walls.Define("gap", comp.Const(cell.Wall{Keeps: cell.Land, Gap: g}))
	for y := range uint32(3) {
		name := "wall"
		if y == 1 {
			name = "gap"
		}
		wd.Write().Seed(walls.Named(name).Entry(gr.At(2, y)))
	}
	ctx := boardtest.NewInstallCtx()
	if err := wd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := wd.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx.ECS().Setup(ctx.Systems()...)
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		wd.RunPlan(rc, d)
		p.RunPlan(rc, d)
		rc.Sync()
		wd.Write().Clock().Replay(rc, d)
	})
	ctx.ECS().Tick(time.Second / 60)
	return p.Read(0).Board(), p.Read(0).Cover().(collision.Field), gr
}

func TestWall_AGapAdmitsWhomItKeepsOffAndTellsWhomItFits(t *testing.T) {
	brd, _, g := walled(t, gap)
	at, wall := g.At(2, 1), g.At(2, 0)
	if k := brd.Kind(at); !k.Admits(cell.Land) || !k.Solid {
		t.Errorf("a wall with a gap: Allows %v Solid %v, want Land through it, solid",
			k.Allows, k.Solid)
	}
	if brd.Kind(wall).Admits(cell.Land) {
		t.Error("a wall without a gap admits Land")
	}
	if !brd.Limited() {
		t.Error("a board with a gap in a wall limits nothing")
	}
	if load, got := brd.Limits(at, cell.Land, false); load != 0 || got != gap {
		t.Errorf("the gap's cell limits Land by %v kg and %+v, want the gap alone", load, got)
	}
	if _, got := brd.Limits(at, cell.Air, false); got != (cell.Gap{}) {
		t.Errorf("the gap limits Air, which the wall keeps not, by %+v", got)
	}
	if _, got := brd.Limits(wall, cell.Land, false); got != (cell.Gap{}) {
		t.Errorf("a wall without a gap limits by %+v, want nothing: it admits none", got)
	}
}

// collision holds the gap's cell solid for whoever navigation would not lead through it
func TestWall_ItsGapIsSolidForWhomItDoesNotFit(t *testing.T) {
	_, f, g := walled(t, gap)
	at := g.CellCenter(g.At(2, 1))
	low := collision.Band{Bottom: 0, Top: 1.5}
	for _, tc := range []struct {
		name  string
		side  geom.Vec
		from  geom.Vec // its top left; zero, centred on the gap
		band  collision.Band
		solid bool
		// the cell takes whom its gap is wide enough for, holding back no push into it
		bare bool
	}{
		{"small", geom.NewVec(4, 4), geom.Vec{}, low, false, false},
		{"as wide and as tall", geom.NewVec(gap.Width, gap.Width), geom.Vec{},
			collision.Band{Bottom: 0, Top: gap.Height}, false, false},
		// its right edge past 32, where float32 rounds the box's width up past the gap's
		{"as wide, leaving it", geom.NewVec(gap.Width, 8), geom.NewVec(26.500006, 11), low,
			false, false},
		{"of no height", geom.NewVec(4, 4), geom.Vec{}, collision.Everywhere, false, false},
		{"long", geom.NewVec(4, 8), geom.Vec{}, low, false, false},
		{"wide", geom.NewVec(8, 8), geom.Vec{}, low, true, true},
		{"tall", geom.NewVec(4, 4), geom.Vec{}, collision.Band{Bottom: 0, Top: 3}, true, false},
	} {
		from := tc.from
		if from == (geom.Vec{}) {
			from = geom.NewVec(at.X-tc.side.X/2, at.Y-tc.side.Y/2)
		}
		box := geom.NewAABBAt(from, tc.side.X, tc.side.Y)
		var cells []uint64
		f.Solid(world.Layers(cell.Land), tc.band, box, func(fb collide.FieldBox) bool {
			cells = append(cells, fb.Cell)
			return true
		})
		if solid := len(cells) > 0; solid != tc.solid {
			t.Errorf("%s: solid %v (%v), want %v", tc.name, solid, cells, tc.solid)
		}
		if got := f.Overhang(world.Layers(cell.Land), box); (got > 0) != tc.bare {
			t.Errorf("%s: overhang %v over the gap's cell, want some %v", tc.name, got, tc.bare)
		}
	}
}

// a unit driven at the wall walks through the gap where it fits, else stands at the wall
func TestWall_AUnitWalksThroughTheGapItFits(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 3, boardtest.CellSize)
	opening := cell.Gap{Width: 16}
	prepare := func(w *world.Plugin, brd *board.Plugin) []act.Rule {
		brd.Write().Board().SetAll(grass)
		walls := board.NewEntities(brd, board.Shape{Size: boardtest.UnitSize}, itself)
		walls.Define("wall", comp.Const(cell.Wall{Keeps: cell.Land}))
		walls.Define("gap", comp.Const(cell.Wall{Keeps: cell.Land, Gap: opening}))
		for y := range uint32(3) {
			name := "wall"
			if y == 1 {
				name = "gap"
			}
			w.Write().Seed(walls.Named(name).Entry(g.At(3, y)))
		}
		return nil
	}
	wall := float32(3 * boardtest.CellSize)
	for _, tc := range []struct {
		name    string
		size    uint32
		through bool
	}{
		{"small", 12, true},
		{"as wide as the gap", 16, true},
		{"wide", boardtest.UnitSize, false},
	} {
		units := []boardtest.Mover{{Here: g.At(1, 1), Heading: geom.NewVec(1, 0), Size: tc.size}}
		bw := boardtest.NewWorldWith(t, g, 6*boardtest.CellSize, 3*boardtest.CellSize, prepare,
			units)
		for range 120 {
			bw.Tick()
		}
		boxes := bw.Snapshot() // the walls' kinds first, defined before the unit's
		box := boxes[len(boxes)-1]
		switch {
		case tc.through && box.TopLeft.X < wall+boardtest.CellSize:
			t.Errorf("%s: at %v, want it through the gap past x %v", tc.name, box,
				wall+boardtest.CellSize)
		case !tc.through && box.BottomRight.X > wall+0.01:
			t.Errorf("%s: at %v, want it at the wall, x %v", tc.name, box, wall)
		}
	}
}

func TestLimits_AFloorBearsWhereItsGroundWouldNot(t *testing.T) {
	rope := cell.Floor{Allows: cell.Land, Load: 100}
	raised := cell.Floor{Allows: cell.Land, Height: 5, Thickness: 1, Load: 100}
	for _, tc := range []struct {
		name             string
		under            cell.Kind
		floor            cell.Floor
		onGround, onDeck float32
	}{
		{"lying over a pit", pit, rope, 100, 0},
		{"lying over grass", grass, rope, 0, 0},
		{"raised over grass", grass, raised, 0, 100},
	} {
		s := newStrip(t, tc.under, effect.Spec{}, comp.Const(tc.floor))
		brd := s.Board.Read(0).Board()
		onGround, _ := brd.Limits(s.middle, cell.Land, false)
		onDeck, _ := brd.Limits(s.middle, cell.Land, true)
		if onGround != tc.onGround || onDeck != tc.onDeck {
			t.Errorf("%s: bears %v on the ground and %v on the deck, want %v and %v", tc.name,
				onGround, onDeck, tc.onGround, tc.onDeck)
		}
		if load, _ := brd.Limits(s.middle, cell.Air, false); load != 0 {
			t.Errorf("%s: bears Air, which it carries not, %v kg", tc.name, load)
		}
	}
}

// a step reads whom a floor bears and what a gap lets through: an effect changing either moves
// the passage of the cell alone
func TestLimits_AnEffectChangingALoadOrAGapCountsAsPassage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ground cell.Kind
		spec   effect.Spec
		built  comp.Comp
		after  func(cell.Built) bool
	}{
		{"a rope fraying", pit, effect.Spec{effect.Alter(func(f *cell.Floor) { f.Load = 50 })},
			comp.Const(cell.Floor{Allows: cell.Land, Load: 200}),
			func(b cell.Built) bool { return b.Floor.Load == 50 }},
		{"a gap widened", grass,
			effect.Spec{effect.Alter(func(w *cell.Wall) { w.Gap.Width = 12 })},
			comp.Const(cell.Wall{Keeps: cell.Land, Gap: gap}),
			func(b cell.Built) bool { return b.Wall.Gap.Width == 12 }},
		{"a hole knocked through", grass,
			effect.Spec{effect.Alter(func(w *cell.Wall) { w.Gap = gap })},
			comp.Const(cell.Wall{Keeps: cell.Land}),
			func(b cell.Built) bool { return b.Wall.Gap == gap }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStrip(t, tc.ground, tc.spec, tc.built)
			brd := s.Board.Read(0).Board()
			passage, cellPassage := brd.Passage(), brd.CellPassage(s.middle)
			beside := brd.CellPassage(s.middle - 1)
			s.cast(t)
			if b := brd.Built(s.middle); !tc.after(b) {
				t.Fatalf("built once the effect began: %+v", b)
			}
			if brd.Passage() <= passage || brd.CellPassage(s.middle) <= cellPassage {
				t.Errorf("the Passage moved %d to %d and the cell's %d to %d, want both up",
					passage, brd.Passage(), cellPassage, brd.CellPassage(s.middle))
			}
			if brd.CellPassage(s.middle-1) != beside {
				t.Error("the passage of the cell beside it moved")
			}
		})
	}
}
