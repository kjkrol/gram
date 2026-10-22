package moments_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

const tickLen = time.Second / 60

var east = geom.NewVec(1, 0)

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
	start := grid.At(1, 1)
	kind.Define[boardtest.Mover](w.Kinds(), "unit", kind.Spec{
		comp.Load(func(m boardtest.Mover) world.Position {
			return world.Position{AABB: boardtest.CellBox(grid, m.Here, boardtest.UnitSize)}
		}),
		comp.Const(world.Velocity{}),
		comp.Load(func(m boardtest.Mover) unit.At { return unit.At{Cell: m.Here} }),
		comp.Const(unit.Mover{Domain: cell.Land}),
	})
	w.Seed(kind.Named[boardtest.Mover](w.Kinds(), "unit").Entry(boardtest.Mover{Here: start}))
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

type heard struct{ Rule string }

func (heard) Command() {}

type heards struct{ command.Queue[heard] }

func (h *heards) Queues() []command.CommandQueue     { return []command.CommandQueue{&h.Queue} }
func (h *heards) DefaultBindings() []control.Binding { return nil }

func (h *heards) told() map[uid.UID64]map[string]bool {
	told := map[uid.UID64]map[string]bool{}
	h.Drain(func(i command.Issued[heard]) {
		if told[i.Entity] == nil {
			told[i.Entity] = map[string]bool{}
		}
		told[i.Entity][i.Command.Rule] = true
	})
	return told
}

func carried(t *testing.T, w *world.Plugin, h *heards) *heards {
	t.Helper()
	if err := w.Carry(h); err != nil {
		t.Fatal(err)
	}
	return h
}

func footing(g grid.Grid) act.Rule {
	return act.Branch[unit.Standing]{
		Name: "footing",
		When: cond.Self.If(cond.Always),
		Then: act.Steps(
			act.OneOf(
				act.Branch[unit.Standing]{
					When: cond.Self.If(unit.Fallen()),
					Then: act.Self.Do(heard{Rule: "fell"}),
				},
				act.Self.Do(heard{Rule: "stood"}),
			),
			act.Branch[unit.Standing]{
				When: cond.Self.If(cond.NewWord("wrong cell", func(st unit.Standing) bool {
					under, _ := g.CellAt(world.Position{AABB: toPlane(st.Box)}.Center())
					return st.Cell != under
				})),
				Then: act.Self.Do(heard{Rule: "wrong cell"}),
			},
		),
	}
}

func onKind(name string) act.Rule {
	return act.Branch[unit.Standing]{
		Name: "on " + name,
		When: cond.Self.If(cond.NewWord("on "+name,
			func(st unit.Standing) bool { return st.Kind.Name.String() == name })),
		Then: act.Self.Do(heard{Rule: "on " + name}),
	}
}

func pitBoard(grid grid.Grid, pit cell.Kind) func(*board.BoardWrite) {
	return func(brd *board.BoardWrite) {
		brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
		for y := uint32(1); y <= 14; y++ {
			c := grid.At(3, y)
			brd.Set(c, pit)
		}
	}
}

func TestStanding_ALandUnitDrivenIntoAHoleFellAndKeepsFalling(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	start := grid.At(1, 7)
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
		overHole := bw.Board.Read(0).Board().Kind(over).Name.String() == "hole"
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
	start := grid.At(3, 7)
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
	a := grid.At(1, 7)
	b := grid.At(4, 7)
	bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize,
		func(brd *board.BoardWrite) {
			brd.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
		},
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
		Heights:  true,
	})
	orders := carried(t, w, &heards{})
	brd := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	brd.Write().Board().SetAll(cell.Kind{Name: cell.Named("hole"), Cost: 1})
	var hosts *boardtest.InstallCtx
	ecs := installWorldAndBoard(t, w, brd, grid, func(ctx *boardtest.InstallCtx) {
		hosts = ctx
		if err := ctx.Deliver(footing(grid)); err != nil {
			t.Fatal(err)
		}
		met := act.Branch[collision.Meeting]{Name: "met", When: cond.Self.If(cond.Always), Then: act.Self.Do(heard{})}
		if err := ctx.Deliver(met); !errors.Is(err, host.ErrUnhosted) {
			t.Errorf("Between on board: %v, want ErrUnhosted", err)
		}
		struck := act.Branch[collision.Struck]{
			Name: "struck",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(heard{}),
		}
		if err := ctx.Deliver(struck); !errors.Is(err, host.ErrUnhosted) {
			t.Errorf("Struck on board: %v, want ErrUnhosted", err)
		}
	})
	ecs.Tick(tickLen)
	if told := only(t, orders.told()); !told["fell"] {
		t.Errorf("a land unit spawned over a hole told %v, want fell", told)
	}
	if err := hosts.Deliver(footing(grid)); !errors.Is(err, host.ErrHostBuilt) {
		t.Errorf("registering after Setup: %v, want ErrHostBuilt", err)
	}
}

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
	start := grid.At(1, 7)
	right := grid.At(2, 7)
	straddling := func(st unit.Standing) bool {
		var under []cell.ID
		grid.CellsUnder(st.Box, func(c cell.ID) { under = append(under, c) })
		return len(under) == 2 && slices.Contains(under, start) && slices.Contains(under, right)
	}
	straddles := act.Branch[unit.Standing]{
		Name: "straddles",
		When: cond.Self.If(cond.NewWord("straddling", straddling)),
		Then: act.Self.Do(heard{Rule: "straddles"}),
	}
	bw, _ := boardtest.SquareWorldWith(t, straddles, boardtest.Mover{Here: start, Offset: boardtest.CellSize / 2})
	orders := carried(t, bw.World, &heards{})
	bw.Tick()
	if told := only(t, orders.told()); !told["straddles"] {
		t.Errorf("a box straddling two cells is not under %v and %v alone", start, right)
	}
}
