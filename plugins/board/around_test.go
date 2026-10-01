package board_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/rule"
	"github.com/kjkrol/gram/plugins/world/rule/effect"
)

// placeWorld is world + effects + board over a 7x7 grid of grass, with one effect, scorched, a
// unit standing in the middle when asked, and the rules hook writes hooked on the board.
type placeWorld struct {
	w        *world.Plugin
	ecs      *goke.ECS
	brd      *board.Plugin
	fx       *effect.Effects
	scorched effect.Effect
	middle   board.CellID
	casting  func(cb *goke.CmdBuf)
}

func newPlaceWorld(t *testing.T, grid board.Grid, unit bool, hook func(pw *placeWorld) []plugin.Rule) *placeWorld {
	t.Helper()
	pw := &placeWorld{}
	pw.middle, _ = grid.CellIndex(3, 3)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 8 * cellSize, Height: 8 * cellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 8, MaxSize: 8},
	})
	pw.w, pw.fx = w, w.Effects()
	pw.scorched = pw.fx.Define("scorched", effect.Spec{})
	pw.brd = board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
	pw.brd.Res.Logic.Board.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
	if err := pw.brd.Hook(hook(pw)...); err != nil {
		t.Fatal(err)
	}
	if err := pw.brd.Populate(); err != nil {
		t.Fatal(err)
	}
	if unit {
		k := kind.Define[struct{}](w.Kinds(), "unit", kind.Spec{
			comp.Const(world.Position{AABB: board.CellAABB(grid, pw.middle, 8)}),
			comp.Const(world.Velocity{}),
			comp.Const(board.At{Cell: pw.middle}),
			comp.Const(board.Mover{Domain: board.Land}),
		})
		w.Seed(k.Entry(struct{}{}))
		if err := w.Populate(); err != nil {
			t.Fatal(err)
		}
	}
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pw.brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
	caster := ctx.ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		if pw.casting != nil {
			pw.casting(cb)
			pw.casting = nil
		}
	}})
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(caster, d)
		rc.Sync()
		w.RunPlan(rc, d)
		pw.brd.RunPlan(rc, d)
		w.Clock().Replay(rc, d)
		rc.Sync()
	})
	pw.ecs = ctx.ecs
	return pw
}

func (pw *placeWorld) tick(n int) {
	for range n {
		pw.ecs.Tick(time.Second / 10)
	}
}

// under is the set of cells whose entity is under e.
func (pw *placeWorld) under(e effect.Effect) map[board.CellID]bool {
	out := map[board.CellID]bool{}
	pw.brd.Res.Logic.Board.EachCell(func(c board.CellID) {
		if id, ok := pw.brd.CellEntity(c); ok && pw.fx.Has(id, e) {
			out[c] = true
		}
	})
	return out
}

// within is every cell n neighbours or fewer from the seed cells, by a walk of the grid's own.
func within(grid board.Grid, seed []board.CellID, n int) map[board.CellID]bool {
	out := map[board.CellID]bool{}
	edge := seed
	for _, c := range seed {
		out[c] = true
	}
	for range n {
		var next []board.CellID
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

func sameCells(a, b map[board.CellID]bool) bool {
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

func placeGrids() map[string]board.Grid {
	return map[string]board.Grid{
		"square": board.DefaultGrids{}.Square(7, 7, cellSize),
		"hex":    board.DefaultGrids{}.Hex(7, 7, cellSize/2),
	}
}

// Here acts on the cells under the unit, Around on those and the rings of neighbours round them —
// each cell once, on square and hex grids alike.
func TestAround_ReachesTheRingsRoundTheCellsUnderTheUnit(t *testing.T) {
	for name, grid := range placeGrids() {
		for _, rings := range []int{0, 1, 2} {
			pw := newPlaceWorld(t, grid, true, func(pw *placeWorld) []plugin.Rule {
				return []plugin.Rule{rule.On("scorch", rule.All, func(m *rule.Moment[board.Standing]) rule.Step {
					if rings == 0 {
						return m.Here(m.Apply(pw.scorched))
					}
					return m.Around(rings, m.Apply(pw.scorched))
				})}
			})
			pw.tick(3)
			var seed []board.CellID
			grid.CellsUnder(board.CellAABB(grid, pw.middle, 8).AABB, func(c board.CellID) { seed = append(seed, c) })
			if got, want := pw.under(pw.scorched), within(grid, seed, rings); !sameCells(got, want) {
				t.Errorf("%s, %d rings: scorched %v, want %v", name, rings, got, want)
			}
		}
	}
}

// A rule of a Cell fires for every cell of the board.
func TestCell_FiresForEveryCell(t *testing.T) {
	for name, grid := range placeGrids() {
		pw := newPlaceWorld(t, grid, false, func(pw *placeWorld) []plugin.Rule {
			return []plugin.Rule{rule.On("scorch all", rule.All, func(m *rule.Moment[board.Cell]) rule.Step {
				return m.Apply(pw.scorched)
			})}
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
		pw := newPlaceWorld(t, grid, false, func(pw *placeWorld) []plugin.Rule {
			return []plugin.Rule{rule.On("fire spreads", rule.Self(pw.scorched.Mark()), func(m *rule.Moment[board.Cell]) rule.Step {
				return m.Around(1, m.Apply(pw.scorched))
			})}
		})
		start, _ := pw.brd.CellEntity(pw.middle)
		pw.casting = func(cb *goke.CmdBuf) { pw.scorched.Cast(cb, start) }
		grown := 0
		for tick := 1; tick <= 40; tick++ {
			pw.tick(1)
			burning := pw.under(pw.scorched)
			for c := range burning {
				if !within(grid, []board.CellID{pw.middle}, tick)[c] {
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

// A cell carries the game's tags of places the Layout gives it, and a rule of a Cell filtered by one
// fires for those cells alone; a CellEntry without a Kind keeps the Default.
func TestPlaces_ARuleOfACellFiltersCellsByTheirTags(t *testing.T) {
	grid := board.DefaultGrids{}.Square(7, 7, cellSize)
	a, _ := grid.CellIndex(1, 1)
	b, _ := grid.CellIndex(5, 2)
	pw := newPlaceWorld(t, grid, false, func(pw *placeWorld) []plugin.Rule {
		marked := pw.w.Kinds().DefineTag[board.Places]("marked")
		tags := tag.Tags[board.Places](0).With(marked)
		pw.brd.Seed(board.Layout{Cells: []board.CellEntry{{Cell: a, Tags: tags}, {Cell: b, Tags: tags}}})
		return []plugin.Rule{rule.On("scorch the marked", rule.Self(marked), func(m *rule.Moment[board.Cell]) rule.Step {
			return m.Apply(pw.scorched)
		})}
	})
	pw.tick(3)
	if got := pw.under(pw.scorched); !sameCells(got, map[board.CellID]bool{a: true, b: true}) {
		t.Errorf("scorched %v, want the two marked cells %d and %d alone", got, a, b)
	}
	if k := pw.brd.Res.Logic.Board.Kind(a); k.Name.String() != "grass" {
		t.Errorf("a marked cell is %q, want the grass it was: a CellEntry without a Kind keeps it", k.Name)
	}
}

// A unit standing on a cell carries, in its Standing, the game's tags of the cell's place: a
// plate under it, nothing elsewhere.
func TestStanding_TellsTheTagsOfThePlaceUnderTheUnit(t *testing.T) {
	grid := board.DefaultGrids{}.Square(7, 7, cellSize)
	middle, _ := grid.CellIndex(3, 3)
	var plate tag.Tag[board.Places]
	pw := newPlaceWorld(t, grid, true, func(pw *placeWorld) []plugin.Rule {
		plate = pw.w.Kinds().DefineTag[board.Places]("plate")
		pw.brd.Seed(board.Layout{Cells: []board.CellEntry{{Cell: middle, Tags: tag.Tags[board.Places](0).With(plate)}}})
		return []plugin.Rule{rule.On("press", rule.All, func(m *rule.Moment[board.Standing]) rule.Step {
			return m.If(func(st board.Standing) bool { return st.Places.Has(plate) }, m.Here(m.Apply(pw.scorched)))
		})}
	})
	pw.tick(3)
	if got := pw.under(pw.scorched); !got[middle] {
		t.Errorf("pressed %v, want the plate %d under the unit", got, middle)
	}
}
