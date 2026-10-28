package terrain_test

import (
	"testing"
	"time"

	cgoke "github.com/kjkrol/ceremony/goke"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type cellWorld struct {
	ecs     *goke.ECS
	brd     *board.Plugin
	fx      *cgoke.Effects
	frost   effect.Effect
	target  cell.ID
	grass   cell.Kind
	snow    cell.Kind
	casting func(cb *goke.CmdBuf)

	plots *goke.Query
	plot  goke.Comp[cell.Plot]
	marks goke.OptComp[tag.Tags[effect.States]]
}

const cellTick = time.Second / 10

func newCellWorld(t *testing.T, boardFirst bool) *cellWorld {
	t.Helper()
	cw := &cellWorld{}
	grid := grid.DefaultGrids{}.Square(4, 4, boardtest.CellSize)
	cw.target = grid.At(2, 2)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4 * boardtest.CellSize, Height: 4 * boardtest.CellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: boardtest.UnitSize, MaxSize: boardtest.UnitSize},
		Heights:  true,
	})
	cw.fx = w.Write().Effects()
	cw.brd = board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	cw.grass = cell.Kind{Name: cell.Named("grass"), Pace: 1, Allows: cell.Land}
	cw.snow = cell.Kind{Name: cell.Named("snow"), Pace: 1.0 / 3, Allows: cell.Land}
	cw.brd.Write().Board().SetAll(cw.grass)
	snow := cw.snow
	cw.fx.Define("frost", effect.Spec{effect.Lasts(2 * cellTick), effect.Alter(func(g *cell.Ground) { g.Kind = snow })})
	cw.frost = cw.fx.Named("frost")

	ctx := boardtest.NewInstallCtx()
	for _, install := range []func() error{
		func() error { return w.Install(ctx) }, func() error { return cw.brd.Install(ctx) },
	} {
		if err := install(); err != nil {
			t.Fatal(err)
		}
	}
	systems := ctx.Systems()
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		cw.plots = si.NewQueryBuilder(&cw.plot).Optional(&cw.marks).Build()
	}})
	ctx.ECS().Setup(systems...)
	caster := ctx.ECS().RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		if cw.casting != nil {
			cw.casting(cb)
			cw.casting = nil
		}
	}})
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(caster, d)
		rc.Sync()
		w.RunPlan(rc, d) // the effects run with the world, the board after them
		cw.brd.RunPlan(rc, d)
		w.Write().Clock().Replay(rc, d)
		rc.Sync()
	})
	cw.ecs = ctx.ECS()
	return cw
}

func (cw *cellWorld) castFrost() uid.UID64 {
	id, _ := cw.brd.Read(0).Board().CellEntity(cw.target)
	cw.casting = func(cb *goke.CmdBuf) { cw.fx.Cast(id, cw.frost) }
	return id
}

func (cw *cellWorld) board() *board.BoardRead { return cw.brd.Read(0).Board() }

func (cw *cellWorld) write() *board.BoardWrite { return cw.brd.Write().Board() }

func (cw *cellWorld) kind() cell.Kind { return cw.board().Kind(cw.target) }

func (cw *cellWorld) cells() (n, changed int) {
	for cw.plots.All(); cw.plots.Next(); {
		cur := cw.plots.Cursor()
		n += len(cur.IDs)
		for _, m := range cw.marks.Slice(cur) {
			if m.Has(effect.Changed) {
				changed++
			}
		}
	}
	return n, changed
}

func TestCells_EveryCellIsAnEntityForGood(t *testing.T) {
	cw := newCellWorld(t, false)
	if n, _ := cw.cells(); n != 16 {
		t.Fatalf("%d cell entities, want one per cell, 16", n)
	}
	seen := map[uid.UID64]cell.ID{}
	cw.board().EachCell(func(c cell.ID) {
		id, ok := cw.brd.Read(0).Board().CellEntity(c)
		if !ok {
			t.Errorf("cell %d has no entity", c)
		}
		if other, dup := seen[id]; dup {
			t.Errorf("cells %d and %d share entity %d", other, c, id)
		}
		seen[id] = c
	})
	if got := cw.kind(); got != cw.grass {
		t.Errorf("the cell entity holds %q, want grass from the seed", got.Name)
	}
}

func TestCells_SetWritesTheEntityAtOnceAndCountsAChange(t *testing.T) {
	cw := newCellWorld(t, false)
	before := cw.board().Version()
	cw.write().Set(cw.target, cw.snow)
	if got := cw.kind(); got != cw.snow {
		t.Errorf("terrain is %q right after Set, want snow", got.Name)
	}
	if cw.board().Version() == before {
		t.Error("Set left the Version as it was")
	}
	at := cw.board().Version()
	cw.write().Set(cw.target, cw.snow)
	cw.ecs.Tick(cellTick)
	if cw.board().Version() != at {
		t.Error("setting what is there already, then a quiet tick, moved the Version")
	}
}

func TestCells_AnEffectOnTheEntityChangesTheTerrainAndIsCounted(t *testing.T) {
	for name, boardFirst := range map[string]bool{"effects then board": false, "board then effects": true} {
		t.Run(name, func(t *testing.T) {
			cw := newCellWorld(t, boardFirst)
			first := cw.castFrost()
			v0 := cw.board().Version()
			cw.ecs.Tick(cellTick) // lands and begins; with the board first, it counts a tick later
			cw.ecs.Tick(cellTick)
			if got := cw.kind(); got != cw.snow {
				t.Fatalf("terrain is %q with the effect on, want snow", got.Name)
			}
			v1 := cw.board().Version()
			if v1 == v0 {
				t.Error("the frost landing left the Version as it was")
			}
			for range 4 {
				cw.ecs.Tick(cellTick)
			}
			if got := cw.kind(); got != cw.grass {
				t.Errorf("terrain is %q after the effect, want grass back", got.Name)
			}
			if cw.board().Version() == v1 {
				t.Error("the frost ending left the Version as it was")
			}
			if again, _ := cw.brd.Read(0).Board().CellEntity(cw.target); again != first {
				t.Errorf("the cell entity changed from %d to %d; it is the cell's for good", first, again)
			}
			if n, changed := cw.cells(); n != 16 || changed != 0 {
				t.Errorf("%d cell entities, %d of them Changed, want 16 and none", n, changed)
			}
		})
	}
}

func TestBoard_CellVersionCountsTheChangesToOneCell(t *testing.T) {
	cw := newCellWorld(t, false)
	brd, write := cw.board(), cw.write()
	far := brd.At(0, 0)
	v, other := brd.CellVersion(cw.target), brd.CellVersion(far)
	step := func(what string, change func()) {
		t.Helper()
		change()
		if now := brd.CellVersion(cw.target); now <= v {
			t.Errorf("%s: version %d, want more than %d", what, now, v)
		} else {
			v = now
		}
	}
	step("kind", func() { write.Set(cw.target, cw.snow) })
	step("way", func() { write.SetWay(cw.target, cell.Way{Kind: cw.grass, Width: 3, Links: 1}) })
	step("heights, shaped beyond the board", func() { write.Touch(cw.target) })
	write.Set(cw.target, cw.grass)
	v = brd.CellVersion(cw.target)
	step("effect", func() {
		cw.castFrost()
		cw.ecs.Tick(cellTick)
		cw.ecs.Tick(cellTick)
	})
	if brd.CellVersion(far) != other {
		t.Errorf("a cell far off went from version %d to %d", other, brd.CellVersion(far))
	}
}
