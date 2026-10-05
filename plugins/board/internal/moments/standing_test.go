package moments_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

const tickLen = time.Second / 60

var east = geom.NewVec(1, 0)

// installWorldAndBoard installs w and brd alone, with one land unit at cell (1,1), and returns
// the ECS ticking world then board.
func installWorldAndBoard(t *testing.T, w *world.Plugin, brd *board.Plugin, grid grid.Grid, installed ...func(ctx *boardtest.InstallCtx)) *goke.ECS {
	t.Helper()
	ctx := boardtest.NewInstallCtx()
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	for _, f := range installed {
		f(ctx)
	}
	start, _ := grid.CellIndex(1, 1)
	w.Seed(kind.Define[boardtest.Mover](w.Kinds(), "unit", kind.Spec{
		comp.Load(func(m boardtest.Mover) world.Position {
			return world.Position{AABB: boardtest.CellBox(grid, m.Here, boardtest.UnitSize)}
		}),
		comp.Const(world.Velocity{}),
		comp.Load(func(m boardtest.Mover) unit.At { return unit.At{Cell: m.Here} }),
		comp.Const(unit.Mover{Domain: cell.Land}),
	}).Entry(boardtest.Mover{Here: start}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	systems := ctx.Systems()
	ctx.ECS().Setup(systems...)
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	})
	return ctx.ECS()
}

// heard is the command the tests' rules give: which rule.
type heard struct{ Rule string }

// heards is where the heard commands land, for the world to carry.
type heards struct{ control.Queue[heard] }

func (h *heards) Queues() []control.CommandQueue     { return []control.CommandQueue{&h.Queue} }
func (h *heards) DefaultBindings() []control.Binding { return nil }

// told is which heards each unit gave since the last time.
func (h *heards) told() map[uid.UID64]map[string]bool {
	told := map[uid.UID64]map[string]bool{}
	h.Drain(func(i control.Issued[heard]) {
		if told[i.Entity] == nil {
			told[i.Entity] = map[string]bool{}
		}
		told[i.Entity][i.Command.Rule] = true
	})
	return told
}

// carried has w carry h, failing the test when it cannot.
func carried(t *testing.T, w *world.Plugin, h *heards) *heards {
	t.Helper()
	if err := w.Carry(h); err != nil {
		t.Fatal(err)
	}
	return h
}

// footing is a rule of a Standing telling, of every unit, "fell" or "stood", and "wrong cell"
// when the cell it names is not the one under its centre.
func footing(g grid.Grid) rule.Rule {
	return rule.Then[unit.Standing]("footing", rule.All, rule.Steps(
		rule.OneOf(rule.If(unit.Standing.Fallen, rule.Order(heard{Rule: "fell"})), rule.Order(heard{Rule: "stood"})),
		rule.If(func(st unit.Standing) bool {
			under, _ := g.CellAt(world.Position{AABB: toPlane(st.Box)}.Center())
			return st.Cell != under
		}, rule.Order(heard{Rule: "wrong cell"})),
	))
}

// onKind is a rule of a Standing telling "on" the name of the kind, of every unit on one so named.
func onKind(name string) rule.Rule {
	return rule.Then[unit.Standing]("on "+name, rule.All, rule.If(func(st unit.Standing) bool { return st.Kind.Name.String() == name }, rule.Order(heard{Rule: "on " + name})))
}

// pitBoard is grass with a pit of kind pit down column 3.
func pitBoard(grid grid.Grid, pit cell.Kind) func(*board.Board) {
	return func(brd *board.Board) {
		brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
		for y := uint32(1); y <= 14; y++ {
			c, _ := grid.CellIndex(3, y)
			brd.Set(c, pit)
		}
	}
}

func TestStanding_ALandUnitDrivenIntoAHoleFellAndKeepsFalling(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	start, _ := grid.CellIndex(1, 7)
	bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, pitBoard(grid, cell.Kind{Name: cell.Named("hole"), Cost: 1}),
		[]boardtest.Mover{{Here: start, Heading: east}}, footing(grid))
	orders := carried(t, bw.World, &heards{})
	if walls := bw.Solid(world.Layers(cell.Land)); len(walls) != 0 {
		t.Fatalf("%d solid cells, want none: a hole is not solid", len(walls))
	}

	fellAt := -1
	for tick := range 120 {
		bw.Tick()
		units := bw.Snapshot()
		centre := world.Position{AABB: toPlane(units[0])}.Center()
		over, _ := grid.CellAt(centre)
		told := only(t, orders.told())
		overHole := bw.Board.Res.Logic.Board.Kind(over).Name.String() == "hole"
		switch {
		case overHole && !told["fell"]:
			t.Fatalf("tick %d: centre over the hole at %v, but the unit did not fall", tick, over)
		case !overHole && told["fell"]:
			t.Fatalf("tick %d: centre over cell %v, no hole, but the unit fell", tick, over)
		case told["fell"] && fellAt < 0:
			fellAt = tick
		}
		if told["wrong cell"] {
			t.Fatalf("tick %d: Standing names a cell other than the one under the centre, %v", tick, over)
		}
	}
	if fellAt < 0 {
		t.Fatal("the unit never reached the hole")
	}
}

func TestStanding_ABoatOnWaterHasNotFallen(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	start, _ := grid.CellIndex(3, 7)
	bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, pitBoard(grid, cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Water}),
		[]boardtest.Mover{{Here: start, Domain: cell.Water}}, footing(grid), onKind("water"))
	orders := carried(t, bw.World, &heards{})
	bw.Tick()
	if told := only(t, orders.told()); !told["on water"] || told["fell"] {
		t.Errorf("a boat told %v, want on water and no fall", told)
	}
}

func TestStanding_ReportsEveryUnitOnTheBoard(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	a, _ := grid.CellIndex(1, 7)
	b, _ := grid.CellIndex(4, 7)
	bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize,
		func(brd *board.Board) { brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land}) },
		[]boardtest.Mover{{Here: a}, {Here: b}}, footing(grid), onKind("grass"))
	orders := carried(t, bw.World, &heards{})
	bw.Tick()
	told := orders.told()
	if units := bw.Snapshot(); len(told) != len(units) {
		t.Fatalf("%d standings for %d units", len(told), len(units))
	}
	for id, rules := range told {
		if !rules["on grass"] || rules["fell"] {
			t.Errorf("unit %d told %v; want on grass, no fall", id, rules)
		}
	}
}

func TestStanding_WorksWithoutCollision(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, boardtest.CellSize)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4 * boardtest.CellSize, Height: 4 * boardtest.CellSize},
		Entities: world.EntitiesCfg{MaxCount: 2, MinSize: boardtest.UnitSize, MaxSize: boardtest.UnitSize},
	})
	orders := carried(t, w, &heards{})
	brd := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	brd.Res.Logic.Board.SetAll(cell.Kind{Name: cell.Named("hole"), Cost: 1})
	var hosts *boardtest.InstallCtx
	ecs := installWorldAndBoard(t, w, brd, grid, func(ctx *boardtest.InstallCtx) {
		hosts = ctx
		if err := ctx.Deliver(footing(grid)); err != nil {
			t.Fatal(err)
		}
		met := rule.Then[collision.Meeting]("met", rule.Between(tag.Any, tag.Any), rule.Order(heard{}))
		if err := ctx.Deliver(met); !errors.Is(err, plugin.ErrUnhosted) {
			t.Errorf("Between on board: %v, want ErrUnhosted", err)
		}
		struck := rule.Then[collision.Struck]("struck", rule.Having[unit.Mover](), rule.Order(heard{}))
		if err := ctx.Deliver(struck); !errors.Is(err, plugin.ErrUnhosted) {
			t.Errorf("Having of Struck on board: %v, want ErrUnhosted", err)
		}
	})
	ecs.Tick(tickLen)
	if told := only(t, orders.told()); !told["fell"] {
		t.Errorf("a land unit spawned over a hole told %v, want fell", told)
	}
	if err := hosts.Deliver(footing(grid)); !errors.Is(err, plugin.ErrHostBuilt) {
		t.Errorf("registering after Setup: %v, want ErrHostBuilt", err)
	}
}

// only is what the one unit told, failing the test when not one did.
func only(t *testing.T, told map[uid.UID64]map[string]bool) map[string]bool {
	t.Helper()
	if len(told) != 1 {
		t.Fatalf("%d units told, want one: %v", len(told), told)
	}
	for _, rules := range told {
		return rules
	}
	return nil
}

func toPlane(b geom.AABB) plane.AABB {
	size := b.BottomRight.Sub(b.TopLeft)
	return plane.NewAABB(b.TopLeft, size.X, size.Y)
}

func TestStanding_BoxNamesEveryCellTheEntityTouches(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	start, _ := grid.CellIndex(1, 7)
	right, _ := grid.CellIndex(2, 7)
	straddles := rule.Then[unit.Standing]("straddles", rule.All, rule.If(func(st unit.Standing) bool {
		var under []cell.ID
		grid.CellsUnder(st.Box, func(c cell.ID) { under = append(under, c) })
		return len(under) == 2 && slices.Contains(under, start) && slices.Contains(under, right)
	}, rule.Order(heard{Rule: "straddles"})))
	bw, _ := boardtest.SquareWorldWith(t, straddles, boardtest.Mover{Here: start, Offset: boardtest.CellSize / 2})
	orders := carried(t, bw.World, &heards{})
	bw.Tick()
	if told := only(t, orders.told()); !told["straddles"] {
		t.Errorf("a box straddling two cells is not under %v and %v alone", start, right)
	}
}
