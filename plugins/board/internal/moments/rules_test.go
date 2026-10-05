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

// placeWorld is world + effects + board over a 7x7 grid of grass, with one effect, scorched, a
// unit standing in the middle when asked, and the rules hook writes hooked on the board.
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
	pw.middle, _ = grid.CellIndex(3, 3)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 8 * boardtest.CellSize, Height: 8 * boardtest.CellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 8, MaxSize: 8},
	})
	pw.w, pw.fx = w, w.Effects()
	pw.scorched = pw.fx.Define("scorched", effect.Spec{})
	pw.brd = board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	pw.brd.Res.Logic.Board.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	rules := hook(pw)
	if err := pw.brd.Populate(); err != nil {
		t.Fatal(err)
	}
	if withUnit {
		k := kind.Define[struct{}](w.Kinds(), "unit", kind.Spec{
			comp.Const(world.Position{AABB: boardtest.CellBox(grid, pw.middle, 8)}),
			comp.Const(world.Velocity{}),
			comp.Const(unit.At{Cell: pw.middle}),
			comp.Const(unit.Mover{Domain: cell.Land}),
		})
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

// under is the set of cells whose entity is under e.
func (pw *placeWorld) under(e effect.Effect) map[cell.ID]bool {
	out := map[cell.ID]bool{}
	pw.brd.Res.Logic.Board.EachCell(func(c cell.ID) {
		if id, ok := pw.brd.CellEntity(c); ok && pw.fx.Has(id, e) {
			out[c] = true
		}
	})
	return out
}

// within is every cell n neighbours or fewer from the seed cells, by a walk of the grid's own.
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

// Here acts on the cells under the unit, Around on those and the rings of neighbours round them —
// each cell once, on square and hex grids alike.
func TestAround_ReachesTheRingsRoundTheCellsUnderTheUnit(t *testing.T) {
	for name, grid := range placeGrids() {
		for _, rings := range []int{0, 1, 2} {
			pw := newPlaceWorld(t, grid, true, func(pw *placeWorld) []rule.Rule {
				scorch := rule.Around(rings, rule.Apply(pw.scorched))
				if rings == 0 {
					scorch = rule.Here(rule.Apply(pw.scorched))
				}
				return []rule.Rule{rule.Then[unit.Standing]("scorch", rule.All, scorch)}
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

// A rule of a cell.Now fires for every cell of the board.
func TestCell_FiresForEveryCell(t *testing.T) {
	for name, grid := range placeGrids() {
		pw := newPlaceWorld(t, grid, false, func(pw *placeWorld) []rule.Rule {
			return []rule.Rule{rule.Then[cell.Now]("scorch all", rule.All, rule.Apply(pw.scorched))}
		})
		pw.tick(3)
		if got := len(pw.under(pw.scorched)); got != grid.CellCount() {
			t.Errorf("%s: %d cells scorched, want all %d", name, got, grid.CellCount())
		}
	}
}

// Fire spreads over the ground from cell to cell: a burning cell sets the ring round it burning,
// so it never runs ahead of the rings round where it began, and in the end it takes every cell.
func TestCell_FireSpreadsFromCellToCell(t *testing.T) {
	for name, grid := range placeGrids() {
		pw := newPlaceWorld(t, grid, false, func(pw *placeWorld) []rule.Rule {
			return []rule.Rule{rule.Then[cell.Now]("fire spreads", rule.Self(pw.scorched.Mark()), rule.Around(1, rule.Apply(pw.scorched)))}
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

// A cell plays the roles of the kind the Layout lays it as, and a rule a role obeys fires for
// those cells alone.
func TestPlaces_ARuleOfARoleFiresForTheCellsPlayingIt(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(7, 7, boardtest.CellSize)
	a, _ := grid.CellIndex(1, 1)
	b, _ := grid.CellIndex(5, 2)
	pw := newPlaceWorld(t, grid, false, func(pw *placeWorld) []rule.Rule {
		marked := rule.Role("marked").Obeys(rule.Then[cell.Now]("scorch the marked", rule.All, rule.Apply(pw.scorched)))
		pw.brd.CellKinds().Create(cell.Kind{Name: cell.Named("marked"), Cost: 1, Allows: cell.Land})
		pw.brd.Plays("marked", marked)
		pw.brd.Seed(board.Layout{Cells: []cell.Entry{{Kind: "marked", Cell: a}, {Kind: "marked", Cell: b}}})
		return marked.Rules()
	})
	pw.tick(3)
	if got := pw.under(pw.scorched); !sameCells(got, map[cell.ID]bool{a: true, b: true}) {
		t.Errorf("scorched %v, want the two marked cells %d and %d alone", got, a, b)
	}
}

// A unit's rule reaches the place under it by the role the cell plays: a plate under it is
// pressed, nothing elsewhere.
func TestStanding_ReachesThePlaceUnderTheUnitByItsRole(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(7, 7, boardtest.CellSize)
	middle, _ := grid.CellIndex(3, 3)
	pw := newPlaceWorld(t, grid, true, func(pw *placeWorld) []rule.Rule {
		plate := rule.Role("plate")
		pw.brd.CellKinds().Create(cell.Kind{Name: cell.Named("plate"), Cost: 1, Allows: cell.Land})
		pw.brd.Plays("plate", plate)
		pw.brd.Seed(board.Layout{Cells: []cell.Entry{{Kind: "plate", Cell: middle}}})
		return []rule.Rule{rule.Then[unit.Standing]("press", rule.All, rule.Here(rule.Playing(plate, rule.Apply(pw.scorched))))}
	})
	pw.tick(3)
	if got := pw.under(pw.scorched); len(got) != 1 || !got[middle] {
		t.Errorf("pressed %v, want the plate %d under the unit alone", got, middle)
	}
}
