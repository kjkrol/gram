package board_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act/effect"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

// cellWorld is world + effects + board over a 4x4 grass board, with frost defined, casting from a
// hook at the start of each tick; boardFirst runs the board's pass before the effects'.
type cellWorld struct {
	ecs     *goke.ECS
	brd     *board.Plugin
	fx      *effect.Effects
	frost   effect.Effect
	target  board.CellID
	grass   board.CellKind
	snow    board.CellKind
	casting func(cb *goke.CmdBuf)

	plots *goke.Query
	plot  goke.Comp[board.Plot]
	marks goke.OptComp[tag.Tags[effect.States]]
}

const cellTick = time.Second / 10

func newCellWorld(t *testing.T, boardFirst bool) *cellWorld {
	t.Helper()
	cw := &cellWorld{}
	grid := board.DefaultGrids{}.Square(4, 4, cellSize)
	cw.target, _ = grid.CellIndex(2, 2)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4 * cellSize, Height: 4 * cellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: unitSize, MaxSize: unitSize},
	})
	cw.fx = w.Effects()
	cw.brd = board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
	cw.grass = board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land}
	cw.snow = board.CellKind{Name: board.Named("snow"), Cost: 3, Allows: board.Land}
	cw.brd.Res.Logic.Board.SetAll(cw.grass)
	snow := cw.snow
	cw.frost = cw.fx.Define("frost", effect.Spec{effect.Lasts(2 * cellTick), effect.Alter(func(g *board.Ground) { g.Kind = snow })})

	ctx := &installCtx{ecs: goke.New()}
	for _, install := range []func() error{
		func() error { return w.Install(ctx) }, func() error { return cw.brd.Install(ctx) },
	} {
		if err := install(); err != nil {
			t.Fatal(err)
		}
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		cw.plots = si.NewQueryBuilder(&cw.plot).Optional(&cw.marks).Build()
	}})
	ctx.ecs.Setup(systems...)
	caster := ctx.ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		if cw.casting != nil {
			cw.casting(cb)
			cw.casting = nil
		}
	}})
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(caster, d)
		rc.Sync()
		w.RunPlan(rc, d) // the effects run with the world, the board after them
		cw.brd.RunPlan(rc, d)
		w.Clock().Replay(rc, d)
		rc.Sync()
	})
	cw.ecs = ctx.ecs
	return cw
}

// castFrost queues a frost on the target's cell entity for the next tick and returns that entity.
func (cw *cellWorld) castFrost() uid.UID64 {
	id, _ := cw.brd.CellEntity(cw.target)
	cw.casting = func(cb *goke.CmdBuf) { cw.fx.Cast(cb, id, cw.frost) }
	return id
}

func (cw *cellWorld) board() *board.Board { return cw.brd.Res.Logic.Board }

func (cw *cellWorld) kind() board.CellKind { return cw.board().Kind(cw.target) }

// cells counts the cell entities and how many of them still have Idle on.
func (cw *cellWorld) cells() (n, idle int) {
	for cw.plots.All(); cw.plots.Next(); {
		cur := cw.plots.Cursor()
		n += len(cur.IDs)
		for _, m := range cw.marks.Slice(cur) {
			if m.Has(effect.Idle) {
				idle++
			}
		}
	}
	return n, idle
}

func TestCells_EveryCellIsAnEntityForGood(t *testing.T) {
	cw := newCellWorld(t, false)
	if n, _ := cw.cells(); n != 16 {
		t.Fatalf("%d cell entities, want one per cell, 16", n)
	}
	seen := map[uid.UID64]board.CellID{}
	cw.board().EachCell(func(c board.CellID) {
		id, ok := cw.brd.CellEntity(c)
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
	cw.board().Set(cw.target, cw.snow)
	if got := cw.kind(); got != cw.snow {
		t.Errorf("terrain is %q right after Set, want snow", got.Name)
	}
	if cw.board().Version() == before {
		t.Error("Set left the Version as it was")
	}
	at := cw.board().Version()
	cw.board().Set(cw.target, cw.snow)
	cw.ecs.Tick(cellTick)
	if cw.board().Version() != at {
		t.Error("setting what is there already, then a quiet tick, moved the Version")
	}
}

// An effect on a cell's entity is an effect on the terrain, whichever plugin's pass runs first:
// the Version moves when it lands and when it ends, the entity stays, and no Idle is left behind.
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
			if again, _ := cw.brd.CellEntity(cw.target); again != first {
				t.Errorf("the cell entity changed from %d to %d; it is the cell's for good", first, again)
			}
			if n, idle := cw.cells(); n != 16 || idle != 0 {
				t.Errorf("%d cell entities, %d of them Idle, want 16 and none", n, idle)
			}
		})
	}
}

// A cell's version grows with every change to it — its kind, its way, its heights, an effect on it
// — and a change to another cell leaves it be.
func TestBoard_CellVersionCountsTheChangesToOneCell(t *testing.T) {
	cw := newCellWorld(t, false)
	brd := cw.board()
	far, _ := brd.CellIndex(0, 0)
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
	step("kind", func() { brd.Set(cw.target, cw.snow) })
	step("way", func() { brd.SetWay(cw.target, board.Way{Kind: cw.grass, Width: 3, Links: 1}) })
	step("heights, shaped beyond the board", func() { brd.Touch(cw.target) })
	brd.Set(cw.target, cw.grass)
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
