package rule_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

const tickLen = time.Second / 60

var east = geom.NewVec(1, 0)

// installWorldAndBoard installs w and brd alone, with one land unit at cell (1,1), and returns
// the ECS ticking world then board.
func installWorldAndBoard(t *testing.T, w *world.Plugin, brd *board.Plugin, grid grid.Grid) *goke.ECS {
	t.Helper()
	ctx := boardtest.NewInstallCtx()
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := brd.Install(ctx); err != nil {
		t.Fatal(err)
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

// footing records the last Standing of every entity a rule saw, and whether it fell.
type footing struct {
	last map[uid.UID64]unit.Standing
	fell map[uid.UID64]bool
}

func newFooting() *footing {
	return &footing{last: map[uid.UID64]unit.Standing{}, fell: map[uid.UID64]bool{}}
}

func (f *footing) react(_ plugin.Tick, st unit.Standing) {
	f.last[st.ID] = st
	f.fell[st.ID] = st.Fallen()
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
	f := newFooting()
	bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, pitBoard(grid, cell.Kind{Name: cell.Named("hole"), Cost: 1}),
		[]boardtest.Mover{{Here: start, Heading: east}}, host.Every(f.react))
	if walls := bw.Solid(world.Layers(cell.Land)); len(walls) != 0 {
		t.Fatalf("%d solid cells, want none: a hole is not solid", len(walls))
	}

	fellAt := -1
	for tick := range 120 {
		bw.Tick()
		units := bw.Snapshot()
		centre := world.Position{AABB: toPlane(units[0])}.Center()
		over, _ := grid.CellAt(centre)
		id := onlyID(f)
		switch {
		case bw.Board.Res.Logic.Board.Kind(over).Name.String() == "hole" && !f.fell[id]:
			t.Fatalf("tick %d: centre over the hole at %v, but the unit did not fall", tick, over)
		case bw.Board.Res.Logic.Board.Kind(over).Name.String() != "hole" && f.fell[id]:
			t.Fatalf("tick %d: centre over %s, but the unit fell", tick, f.last[id].Kind.Name)
		case f.fell[id] && fellAt < 0:
			fellAt = tick
		}
		if f.fell[id] && f.last[id].Cell != over {
			t.Fatalf("tick %d: Standing names cell %v, the centre is over %v", tick, f.last[id].Cell, over)
		}
	}
	if fellAt < 0 {
		t.Fatal("the unit never reached the hole")
	}
}

func TestStanding_ABoatOnWaterHasNotFallen(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	start, _ := grid.CellIndex(3, 7)
	f := newFooting()
	bw := boardtest.NewWorld(t, grid, 6*boardtest.CellSize, 16*boardtest.CellSize, pitBoard(grid, cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Water}),
		[]boardtest.Mover{{Here: start, Domain: cell.Water}}, host.Every(f.react))
	bw.Tick()
	id := onlyID(f)
	if f.last[id].Kind.Name.String() != "water" || f.fell[id] {
		t.Errorf("a boat on %s fell=%v, want water and no fall", f.last[id].Kind.Name, f.fell[id])
	}
}

func TestStanding_ReportsEveryUnitOnTheBoard(t *testing.T) {
	f := newFooting()
	bw, _ := boardtest.SquareWorldWith(t, host.Every(f.react), boardtest.Mover{})
	bw.Tick()
	if units := bw.Snapshot(); len(f.last) != len(units) {
		t.Fatalf("%d standings for %d units", len(f.last), len(units))
	}
	for id, st := range f.last {
		if st.Kind.Name.String() != "grass" || f.fell[id] {
			t.Errorf("unit %d stands on %s, fell=%v; want grass, no fall", id, st.Kind.Name, f.fell[id])
		}
	}
}

func TestStanding_WorksWithoutCollision(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, boardtest.CellSize)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4 * boardtest.CellSize, Height: 4 * boardtest.CellSize},
		Entities: world.EntitiesCfg{MaxCount: 2, MinSize: boardtest.UnitSize, MaxSize: boardtest.UnitSize},
	})
	brd := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	brd.Res.Logic.Board.SetAll(cell.Kind{Name: cell.Named("hole"), Cost: 1})
	f := newFooting()
	if err := brd.Hook(host.Every(f.react)); err != nil {
		t.Fatal(err)
	}
	if err := brd.Hook(host.Pair(tag.Any, tag.Any, func(plugin.Tick, collision.Meeting) {})); !errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Between on board: %v, want ErrUnhosted", err)
	}
	struck := host.Each(func(plugin.Tick, *unit.Mover, collision.Struck) {})
	if err := brd.Hook(struck); !errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Each of Struck on board: %v, want ErrUnhosted", err)
	}
	ecs := installWorldAndBoard(t, w, brd, grid)
	ecs.Tick(tickLen)
	if len(f.fell) != 1 {
		t.Fatalf("%d standings, want 1", len(f.fell))
	}
	for _, fell := range f.fell {
		if !fell {
			t.Error("a land unit spawned over a hole did not fall")
		}
	}
	if err := brd.Hook(host.Every(f.react)); !errors.Is(err, plugin.ErrHostBuilt) {
		t.Errorf("registering after Setup: %v, want ErrHostBuilt", err)
	}
}

// onlyID is the one entity the footing saw.
func onlyID(f *footing) uid.UID64 {
	for id := range f.last {
		return id
	}
	return 0
}

func toPlane(b geom.AABB) plane.AABB {
	size := b.BottomRight.Sub(b.TopLeft)
	return plane.NewAABB(b.TopLeft, size.X, size.Y)
}

func TestStanding_BoxNamesEveryCellTheEntityTouches(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 16, boardtest.CellSize)
	start, _ := grid.CellIndex(1, 7)
	var box geom.AABB
	record := func(_ plugin.Tick, _ *unit.Mover, st unit.Standing) { box = st.Box }
	recording := host.Each(record)
	bw, _ := boardtest.SquareWorldWith(t, recording, boardtest.Mover{Here: start, Offset: boardtest.CellSize / 2})
	bw.Tick()
	var under []cell.ID
	grid.CellsUnder(box, func(c cell.ID) { under = append(under, c) })
	right, _ := grid.CellIndex(2, 7)
	if len(under) != 2 || !slices.Contains(under, start) || !slices.Contains(under, right) {
		t.Errorf("a box straddling two cells is under %v, want %v and %v", under, start, right)
	}
}
