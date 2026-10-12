package moments_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

type placeWorld struct {
	w        *world.Plugin
	ecs      *goke.ECS
	brd      *board.Plugin
	fx       *effect.Effects
	scorched effect.Effect
	middle   cell.ID
	casting  func(cb *goke.CmdBuf)
}

func newPlaceWorld(t *testing.T, grid grid.Grid, withUnit bool, hook func(pw *placeWorld) []rule.Rule) *placeWorld {
	t.Helper()
	pw := &placeWorld{}
	pw.middle = grid.At(3, 3)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 8 * boardtest.CellSize, Height: 8 * boardtest.CellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 8, MaxSize: 8},
	})
	pw.w, pw.fx = w, w.Effects()
	pw.fx.Define("scorched", effect.Spec{})
	pw.scorched = pw.fx.Named("scorched")
	pw.brd = board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	pw.brd.Board().SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	rules := hook(pw)
	if err := pw.brd.Populate(); err != nil {
		t.Fatal(err)
	}
	if withUnit {
		kind.Define[struct{}](w.Kinds(), "unit", kind.Spec{
			comp.Const(world.Position{AABB: boardtest.CellBox(grid, pw.middle, 8)}),
			comp.Const(world.Velocity{}),
			comp.Const(unit.At{Cell: pw.middle}),
			comp.Const(unit.Mover{Domain: cell.Land}),
		})
		k := kind.Named[struct{}](w.Kinds(), "unit")
		w.Seed(k.Entry(struct{}{}))
		if err := w.Populate(); err != nil {
			t.Fatal(err)
		}
	}
	ctx := boardtest.NewInstallCtx()
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pw.brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Deliver(rules...); err != nil {
		t.Fatal(err)
	}
	systems := ctx.Systems()
	ctx.ECS().Setup(systems...)
	caster := ctx.ECS().RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		if pw.casting != nil {
			pw.casting(cb)
			pw.casting = nil
		}
	}})
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(caster, d)
		rc.Sync()
		w.RunPlan(rc, d)
		pw.brd.RunPlan(rc, d)
		w.Clock().Replay(rc, d)
		rc.Sync()
	})
	pw.ecs = ctx.ECS()
	return pw
}

func (pw *placeWorld) tick(n int) {
	for range n {
		pw.ecs.Tick(time.Second / 10)
	}
}

func (pw *placeWorld) under(e effect.Effect) map[cell.ID]bool {
	out := map[cell.ID]bool{}
	pw.brd.Board().EachCell(func(c cell.ID) {
		if id, ok := pw.brd.CellEntity(c); ok && pw.fx.Has(id, e) {
			out[c] = true
		}
	})
	return out
}

func within(grid grid.Grid, seed []cell.ID, n int) map[cell.ID]bool {
	out := map[cell.ID]bool{}
	edge := seed
	for _, c := range seed {
		out[c] = true
	}
	for range n {
		var next []cell.ID
		for _, c := range edge {
			for _, nb := range grid.Neighbors(c) {
				if !out[nb] {
					out[nb] = true
					next = append(next, nb)
				}
			}
		}
		edge = next
	}
	return out
}

func sameCells(a, b map[cell.ID]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for c := range a {
		if !b[c] {
			return false
		}
	}
	return true
}

func placeGrids() map[string]grid.Grid {
	return map[string]grid.Grid{
		"square": grid.DefaultGrids{}.Square(7, 7, boardtest.CellSize),
		"hex":    grid.DefaultGrids{}.Hex(7, 7, boardtest.CellSize/2),
	}
}

func TestAround_ReachesTheRingsRoundTheCellsUnderTheUnit(t *testing.T) {
	for name, grid := range placeGrids() {
		for _, rings := range []int{0, 1, 2} {
			pw := newPlaceWorld(t, grid, true, func(pw *placeWorld) []rule.Rule {
				scorch := rule.Around(rings, rule.Apply(pw.scorched))
				if rings == 0 {
					scorch = rule.Here(rule.Apply(pw.scorched))
				}
				return []rule.Rule{rule.Branch[unit.Standing]{Name: "scorch", Do: scorch}}
			})
			pw.tick(3)
			var seed []cell.ID
			grid.CellsUnder(boardtest.CellBox(grid, pw.middle, 8).AABB, func(c cell.ID) { seed = append(seed, c) })
			if got, want := pw.under(pw.scorched), within(grid, seed, rings); !sameCells(got, want) {
				t.Errorf("%s, %d rings: scorched %v, want %v", name, rings, got, want)
			}
		}
	}
}

func TestCell_FiresForEveryCell(t *testing.T) {
	for name, grid := range placeGrids() {
		pw := newPlaceWorld(t, grid, false, func(pw *placeWorld) []rule.Rule {
			return []rule.Rule{rule.Branch[cell.Now]{
				Name: "scorch all",
				Do:   rule.Apply(pw.scorched),
			}}
		})
		pw.tick(3)
		if got := len(pw.under(pw.scorched)); got != grid.CellCount() {
			t.Errorf("%s: %d cells scorched, want all %d", name, got, grid.CellCount())
		}
	}
}

func TestCell_FireSpreadsFromCellToCell(t *testing.T) {
	for name, grid := range placeGrids() {
		pw := newPlaceWorld(t, grid, false, func(pw *placeWorld) []rule.Rule {
			return []rule.Rule{rule.Branch[cell.Now]{
				Name: "fire spreads",
				When: rule.Self(pw.scorched.Mark()),
				Do:   rule.Around(1, rule.Apply(pw.scorched)),
			}}
		})
		start, _ := pw.brd.CellEntity(pw.middle)
		pw.casting = func(cb *goke.CmdBuf) { pw.fx.Cast(cb, start, pw.scorched) }
		grown := 0
		for tick := 1; tick <= 40; tick++ {
			pw.tick(1)
			burning := pw.under(pw.scorched)
			for c := range burning {
				if !within(grid, []cell.ID{pw.middle}, tick)[c] {
					t.Fatalf("%s, tick %d: cell %d burns, further than %d rings from the start", name, tick, c, tick)
				}
			}
			if len(burning) < grown {
				t.Fatalf("%s, tick %d: %d cells burn, fewer than %d before", name, tick, len(burning), grown)
			}
			grown = len(burning)
		}
		if grown != grid.CellCount() {
			t.Errorf("%s: %d cells burn in the end, want all %d", name, grown, grid.CellCount())
		}
	}
}

func TestPlaces_ARuleOfARoleFiresForTheCellsPlayingIt(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(7, 7, boardtest.CellSize)
	a := grid.At(1, 1)
	b := grid.At(5, 2)
	pw := newPlaceWorld(t, grid, false, func(pw *placeWorld) []rule.Rule {
		pw.w.Roles().Define("marked", rule.Branch[cell.Now]{
			Name: "scorch the marked",
			Do:   rule.Apply(pw.scorched),
		})
		marked := pw.w.Roles().Named("marked")
		pw.brd.CellKinds().Define("marked", cell.Kind{Cost: 1, Allows: cell.Land}, marked)
		pw.brd.Seed(board.Layout{Cells: []cell.Entry{{Kind: "marked", Cell: a}, {Kind: "marked", Cell: b}}})
		return marked.Rules()
	})
	pw.tick(3)
	if got := pw.under(pw.scorched); !sameCells(got, map[cell.ID]bool{a: true, b: true}) {
		t.Errorf("scorched %v, want the two marked cells %d and %d alone", got, a, b)
	}
}

func TestPlaces_ARuleOfARoleFiresForTheOneCellTheEntryHasPlayIt(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(7, 7, boardtest.CellSize)
	a := grid.At(1, 1)
	b := grid.At(5, 2)
	pw := newPlaceWorld(t, grid, false, func(pw *placeWorld) []rule.Rule {
		pw.w.Roles().Define("lake", rule.Branch[cell.Now]{
			Name: "scorch the lake",
			Do:   rule.Apply(pw.scorched),
		})
		lake := pw.w.Roles().Named("lake")
		pw.brd.CellKinds().Define("water", cell.Kind{Cost: 1, Allows: cell.Water})
		water := pw.brd.CellKinds().Named("water")
		pw.brd.Seed(board.Layout{Cells: []cell.Entry{water.Entry(a).Plays(lake), water.Entry(b)}})
		return lake.Rules()
	})
	pw.tick(3)
	if got := pw.under(pw.scorched); !sameCells(got, map[cell.ID]bool{a: true}) {
		t.Errorf("scorched %v, want the lake's cell %d alone", got, a)
	}
}

func TestStanding_ReachesThePlaceUnderTheUnitByItsRole(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(7, 7, boardtest.CellSize)
	middle := grid.At(3, 3)
	pw := newPlaceWorld(t, grid, true, func(pw *placeWorld) []rule.Rule {
		pw.w.Roles().Define("plate")
		plate := pw.w.Roles().Named("plate")
		pw.brd.CellKinds().Define("plate", cell.Kind{Cost: 1, Allows: cell.Land}, plate)
		pw.brd.Seed(board.Layout{Cells: []cell.Entry{{Kind: "plate", Cell: middle}}})
		return []rule.Rule{rule.Branch[unit.Standing]{
			Name: "press",
			Do: rule.Here(rule.Branch[any]{
				When: rule.Playing(plate),
				Do:   rule.Apply(pw.scorched),
			}),
		}}
	})
	pw.tick(3)
	if got := pw.under(pw.scorched); len(got) != 1 || !got[middle] {
		t.Errorf("pressed %v, want the plate %d under the unit alone", got, middle)
	}
}
