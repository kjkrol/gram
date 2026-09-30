package board_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/uid"
)

const tickLen = time.Second / 60

// installWorldAndBoard installs w and brd alone, with one land unit at cell (1,1), and returns
// the ECS ticking world then board.
func installWorldAndBoard(t *testing.T, w *world.Plugin, brd *board.Plugin, grid board.Grid) *goke.ECS {
	t.Helper()
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	start, _ := grid.CellIndex(1, 1)
	w.Seed(kind.Define[mover](w.Kinds(), "unit", kind.Spec{
		comp.Load(func(m mover) world.Position { return world.Position{AABB: board.CellAABB(grid, m.cell, unitSize)} }),
		comp.Const(world.Velocity{}),
		comp.Load(func(m mover) board.Cell { return board.Cell{ID: m.cell} }),
		comp.Const(board.Mover{Domain: board.Land}),
	}).Entry(mover{cell: start}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	})
	return ctx.ecs
}

// footing records the last Standing of every entity a trigger saw, and whether it fell.
type footing struct {
	last map[uid.UID64]board.Standing
	fell map[uid.UID64]bool
}

func newFooting() *footing {
	return &footing{last: map[uid.UID64]board.Standing{}, fell: map[uid.UID64]bool{}}
}

func (f *footing) react(_ plugin.Tick, st board.Standing) {
	f.last[st.ID] = st
	f.fell[st.ID] = st.Fallen()
}

// pitBoard is grass with a pit of kind pit down column 3.
func pitBoard(grid board.Grid, pit board.CellKind) func(*board.Board) {
	return func(brd *board.Board) {
		brd.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
		for y := uint32(1); y <= 14; y++ {
			c, _ := grid.CellIndex(3, y)
			brd.Set(c, pit)
		}
	}
}

func TestStanding_ALandUnitDrivenIntoAHoleFellAndKeepsFalling(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	start, _ := grid.CellIndex(1, 7)
	f := newFooting()
	bw := newGroundWorld(t, grid, 6*cellSize, 16*cellSize, pitBoard(grid, board.CellKind{Name: board.Named("hole"), Cost: 1}),
		[]mover{{cell: start, heading: east}}, act.Trigger[board.Standing]("react").Runs(f.react))
	if walls := bw.solid(world.Layers(board.Land)); len(walls) != 0 {
		t.Fatalf("%d solid cells, want none: a hole is not solid", len(walls))
	}

	fellAt := -1
	for tick := range 120 {
		bw.tick()
		units := bw.snapshot()
		centre := board.Center(world.Position{AABB: toPlane(units[0])})
		over, _ := grid.CellAt(centre)
		id := onlyID(f)
		switch {
		case bw.brd.Res.Logic.Board.Kind(over).Name.String() == "hole" && !f.fell[id]:
			t.Fatalf("tick %d: centre over the hole at %v, but the unit did not fall", tick, over)
		case bw.brd.Res.Logic.Board.Kind(over).Name.String() != "hole" && f.fell[id]:
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
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	start, _ := grid.CellIndex(3, 7)
	f := newFooting()
	bw := newGroundWorld(t, grid, 6*cellSize, 16*cellSize, pitBoard(grid, board.CellKind{Name: board.Named("water"), Cost: 1, Allows: board.Water}),
		[]mover{{cell: start, domain: board.Water}}, act.Trigger[board.Standing]("react").Runs(f.react))
	bw.tick()
	id := onlyID(f)
	if f.last[id].Kind.Name.String() != "water" || f.fell[id] {
		t.Errorf("a boat on %s fell=%v, want water and no fall", f.last[id].Kind.Name, f.fell[id])
	}
}

func TestStanding_ReportsEveryUnitOnTheBoard(t *testing.T) {
	f := newFooting()
	bw, _ := squareWorldWith(t, act.Trigger[board.Standing]("react").Runs(f.react), mover{})
	bw.tick()
	if units := bw.snapshot(); len(f.last) != len(units) {
		t.Fatalf("%d standings for %d units", len(f.last), len(units))
	}
	for id, st := range f.last {
		if st.Kind.Name.String() != "grass" || f.fell[id] {
			t.Errorf("unit %d stands on %s, fell=%v; want grass, no fall", id, st.Kind.Name, f.fell[id])
		}
	}
}

func TestStanding_WorksWithoutCollision(t *testing.T) {
	grid := board.DefaultGrids{}.Square(4, 4, cellSize)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4 * cellSize, Height: 4 * cellSize},
		Entities: world.EntitiesCfg{MaxCount: 2, MinSize: unitSize, MaxSize: unitSize},
	})
	brd := board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
	brd.Res.Logic.Board.SetAll(board.CellKind{Name: board.Named("hole"), Cost: 1})
	f := newFooting()
	if err := brd.Hook(act.Trigger[board.Standing]("react").Runs(f.react)); err != nil {
		t.Fatal(err)
	}
	if err := brd.Hook(act.Trigger[collision.Meeting]("hook").Runs(func(plugin.Tick, collision.Meeting) {})); !errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Between on board: %v, want ErrUnhosted", err)
	}
	if err := brd.Hook(act.Trigger[collision.Struck]("hook").RunsOn(func(plugin.Tick, *board.Mover, collision.Struck) {})); !errors.Is(err, plugin.ErrUnhosted) {
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
	if err := brd.Hook(act.Trigger[board.Standing]("react").Runs(f.react)); !errors.Is(err, plugin.ErrHostBuilt) {
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
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	start, _ := grid.CellIndex(1, 7)
	var box geom.AABB
	record := func(_ plugin.Tick, _ *board.Mover, st board.Standing) { box = st.Box }
	bw, _ := squareWorldWith(t, act.Trigger[board.Standing]("record").RunsOn(record), mover{cell: start, offset: cellSize / 2})
	bw.tick()
	var under []board.CellID
	grid.CellsUnder(box, func(c board.CellID) { under = append(under, c) })
	right, _ := grid.CellIndex(2, 7)
	if len(under) != 2 || !slices.Contains(under, start) || !slices.Contains(under, right) {
		t.Errorf("a box straddling two cells is under %v, want %v and %v", under, start, right)
	}
}
