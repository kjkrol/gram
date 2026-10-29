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
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

type cellWorld struct {
	ecs     *goke.ECS
	brd     *board.Plugin
	fx      *cgoke.Effects
	frost   effect.Effect
	sway    effect.Effect
	ice     effect.Effect
	target  cell.ID
	grass   cell.Kind
	snow    cell.Kind
	iced    cell.Kind
	casting func(cb *goke.CmdBuf)

	plots *goke.Query
	plot  goke.Comp[cell.Plot]
	marks goke.OptComp[tag.Tags[effect.States]]
}

const (
	cellTick = time.Second / 10
	swaying  = 0.6 // as the weathering's sway
)

func newCellWorld(t *testing.T, boardFirst bool) *cellWorld {
	t.Helper()
	return newCellWorldLaid(t, nil)
}

// laid is handed the world with its board laid in the seed, before the cells are made
func newCellWorldLaid(t *testing.T, laid func(cw *cellWorld)) *cellWorld {
	t.Helper()
	cw := &cellWorld{}
	grid := grid.DefaultGrids{}.Square(4, 4, boardtest.CellSize)
	cw.target = grid.At(2, 2)
	w := world.NewPlugin(world.Config{
		Space: world.SpaceCfg{Width: 4 * boardtest.CellSize, Height: 4 * boardtest.CellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: boardtest.UnitSize,
			MaxSize: boardtest.UnitSize},
		Heights: true,
	})
	cw.fx = w.Write().Effects()
	cw.brd = board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	cw.grass = cell.Kind{Name: cell.Named("grass"), Pace: 1, Allows: cell.Land}
	cw.snow = cell.Kind{Name: cell.Named("snow"), Pace: 1.0 / 3, Allows: cell.Land}
	cw.iced = cell.Kind{Name: cell.Named("ice"), Pace: 0.5, Allows: cell.Land | cell.Water}
	cw.brd.Write().Board().SetAll(cw.grass)
	snow, iced := cw.snow, cw.iced
	cw.fx.Define("frost", effect.Spec{
		effect.Lasts(2 * cellTick),
		effect.Alter(func(g *cell.Ground) { g.Kind = snow }),
	})
	cw.frost = cw.fx.Named("frost")
	cw.fx.Define("sway", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind.Sway = swaying })})
	cw.sway = cw.fx.Named("sway")
	cw.fx.Define("ice", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind = iced })})
	cw.ice = cw.fx.Named("ice")
	if laid != nil {
		laid(cw)
	}

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

func (cw *cellWorld) castFrost() uid.UID64 { return cw.cast(cw.frost) }

func (cw *cellWorld) cast(fx effect.Effect) uid.UID64 {
	id, _ := cw.brd.Read(0).Board().CellEntity(cw.target)
	cw.casting = func(cb *goke.CmdBuf) { cw.fx.Cast(id, fx) }
	return id
}

// castOn has fx land on the target cell and begin, its change counted
func (cw *cellWorld) castOn(fx effect.Effect) {
	cw.cast(fx)
	cw.ecs.Tick(cellTick)
	cw.ecs.Tick(cellTick)
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
				t.Errorf("the cell entity changed from %d to %d; it is the cell's for good",
					first, again)
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

// the wind's sway turns what is drawn of the ground alone: the terrain counts a change, the
// passage none
func TestCells_ASwayAltersTheVersionNotThePassage(t *testing.T) {
	cw := newCellWorld(t, false)
	brd := cw.board()
	version, cellVersion := brd.Version(), brd.CellVersion(cw.target)
	passage, cellPassage := brd.Passage(), brd.CellPassage(cw.target)
	cw.castOn(cw.sway)
	if got := cw.kind(); got.Sway != swaying {
		t.Fatalf("the cell sways %v under the effect, want %v", got.Sway, swaying)
	}
	if brd.Version() == version || brd.CellVersion(cw.target) == cellVersion {
		t.Error("the sway left the Version or the cell's as it was")
	}
	if brd.Passage() != passage || brd.CellPassage(cw.target) != cellPassage {
		t.Errorf("the sway moved the Passage %d to %d and the cell's %d to %d, want neither",
			passage, brd.Passage(), cellPassage, brd.CellPassage(cw.target))
	}
}

func TestCells_AnIceAltersBoth(t *testing.T) {
	cw := newCellWorld(t, false)
	brd := cw.board()
	far := brd.At(0, 0)
	version, passage := brd.Version(), brd.Passage()
	cellPassage, farPassage := brd.CellPassage(cw.target), brd.CellPassage(far)
	cw.castOn(cw.ice)
	if got := cw.kind(); got != cw.iced {
		t.Fatalf("the cell is %q under the effect, want ice", got.Name)
	}
	if brd.Version() == version {
		t.Error("the ice left the Version as it was")
	}
	if brd.Passage() <= passage || brd.CellPassage(cw.target) <= cellPassage {
		t.Errorf("the ice moved the Passage %d to %d and the cell's %d to %d, want both up",
			passage, brd.Passage(), cellPassage, brd.CellPassage(cw.target))
	}
	if brd.CellPassage(far) != farPassage {
		t.Errorf("a cell far off went from passage %d to %d", farPassage, brd.CellPassage(far))
	}
}

// the heights are the topography's, the slope of a step theirs: a touch always counts
func TestCells_ATouchCountsAsPassage(t *testing.T) {
	cw := newCellWorld(t, false)
	brd := cw.board()
	far := brd.At(0, 0)
	passage, cellPassage, farPassage := brd.Passage(), brd.CellPassage(cw.target),
		brd.CellPassage(far)
	cw.write().Touch(cw.target)
	if brd.Passage() <= passage || brd.CellPassage(cw.target) <= cellPassage {
		t.Errorf("a touch moved the Passage %d to %d and the cell's %d to %d, want both up",
			passage, brd.Passage(), cellPassage, brd.CellPassage(cw.target))
	}
	if brd.CellPassage(far) != farPassage {
		t.Errorf("a cell far off went from passage %d to %d", farPassage, brd.CellPassage(far))
	}
}

func TestCells_ASetDifferingInSpriteAloneIsNoPassage(t *testing.T) {
	cw := newCellWorld(t, false)
	brd := cw.board()
	version, passage := brd.Version(), brd.Passage()
	painted := cw.grass
	painted.SpriteID++
	cw.write().Set(cw.target, painted)
	if brd.Version() == version {
		t.Fatal("a Set of another sprite counted no change")
	}
	if brd.Passage() != passage {
		t.Errorf("a Set of another sprite moved the Passage %d to %d", passage, brd.Passage())
	}
}

// every part of a cell a step reads counts in its passage alone, the ground's under a way too,
// which a slantwise step beside the way crosses
func TestCells_APassageIsWhatAStepReads(t *testing.T) {
	road := cell.Kind{Name: cell.Named("road"), Pace: 1, Allows: cell.Land, Graded: true}
	laid := cell.Way{Kind: road, Width: 8, Links: 1<<0 | 1<<4}
	type change func(k *cell.Kind, w *cell.Way)
	for _, tc := range []struct {
		name   string
		way    cell.Way
		change change
		counts bool
	}{
		{"the kind's name", cell.Way{},
			func(k *cell.Kind, _ *cell.Way) { k.Name = cell.Named("meadow") }, false},
		{"its sprite", cell.Way{}, func(k *cell.Kind, _ *cell.Way) { k.SpriteID++ }, false},
		{"its veil", cell.Way{}, func(k *cell.Kind, _ *cell.Way) { k.Veil = 0.5 }, false},
		{"whom it veils", cell.Way{}, func(k *cell.Kind, _ *cell.Way) { k.Veils = cell.Air }, false},
		{"its height", cell.Way{}, func(k *cell.Kind, _ *cell.Way) { k.Height = 3 }, false},
		{"its sway", cell.Way{}, func(k *cell.Kind, _ *cell.Way) { k.Sway = swaying }, false},
		{"the shade it takes", cell.Way{},
			func(k *cell.Kind, _ *cell.Way) { k.Takes = render.Shines }, false},
		{"its pace", cell.Way{}, func(k *cell.Kind, _ *cell.Way) { k.Pace = 0.5 }, true},
		{"its pace for a domain", cell.Way{},
			func(k *cell.Kind, _ *cell.Way) { *k = k.Paced(cell.Land, 0.5) }, true},
		{"whom it admits", cell.Way{},
			func(k *cell.Kind, _ *cell.Way) { k.Allows |= cell.Water }, true},
		{"its being solid", cell.Way{}, func(k *cell.Kind, _ *cell.Way) { k.Solid = true }, true},
		{"its grading", cell.Way{}, func(k *cell.Kind, _ *cell.Way) { k.Graded = true }, true},
		{"a way laid", cell.Way{}, func(_ *cell.Kind, w *cell.Way) { *w = laid }, true},
		{"the way taken away", laid, func(_ *cell.Kind, w *cell.Way) { *w = cell.Way{} }, true},
		{"the ground's pace under a way", laid,
			func(k *cell.Kind, _ *cell.Way) { k.Pace = 0.5 }, true},
		{"whom the ground under a way admits", laid,
			func(k *cell.Kind, _ *cell.Way) { k.Allows = cell.Water }, true},
		{"the ground's name under a way", laid,
			func(k *cell.Kind, _ *cell.Way) { k.Name = cell.Named("meadow") }, false},
		{"the way's fading", laid, func(_ *cell.Kind, w *cell.Way) { w.Fade = 0.5 }, false},
		{"the way's look", laid, func(_ *cell.Kind, w *cell.Way) { w.Mix = 0.5 }, false},
		{"the way's sprite", laid, func(_ *cell.Kind, w *cell.Way) { w.Kind.SpriteID++ }, false},
		{"the way's pace", laid, func(_ *cell.Kind, w *cell.Way) { w.Kind.Pace = 0.5 }, true},
		{"whom the way admits", laid,
			func(_ *cell.Kind, w *cell.Way) { w.Kind.Allows |= cell.Water }, true},
		{"the way's grading", laid,
			func(_ *cell.Kind, w *cell.Way) { w.Kind.Graded = false }, true},
		{"the way's kind", laid,
			func(_ *cell.Kind, w *cell.Way) { w.Kind.Name = cell.Named("track") }, true},
		{"the way's width", laid, func(_ *cell.Kind, w *cell.Way) { w.Width = 4 }, true},
		{"the way's links", laid, func(_ *cell.Kind, w *cell.Way) { w.Links |= 1 << 2 }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cw := newCellWorld(t, false)
			brd, write := cw.board(), cw.write()
			k, w := cw.grass, tc.way
			write.SetWay(cw.target, w)
			version, passage, cellPassage := brd.Version(), brd.Passage(),
				brd.CellPassage(cw.target)
			tc.change(&k, &w)
			write.Set(cw.target, k)
			write.SetWay(cw.target, w)
			if brd.Version() == version {
				t.Fatal("the change counted nothing")
			}
			if counted := brd.Passage() != passage; counted != tc.counts {
				t.Errorf("the Passage moved %v, want %v", counted, tc.counts)
			}
			if counted := brd.CellPassage(cw.target) != cellPassage; counted != tc.counts {
				t.Errorf("the cell's passage moved %v, want %v", counted, tc.counts)
			}
		})
	}
}

// what the passage counted while the board was laid in its seed it carries on from as the
// cells are made, every cell's then; nothing written after takes it back
func TestCells_PassageNeverStartsOver(t *testing.T) {
	var seeded uint64
	cw := newCellWorldLaid(t, func(cw *cellWorld) {
		cw.write().Set(cw.target, cw.snow)
		seeded = cw.board().Passage()
	})
	brd := cw.board()
	if seeded == 0 {
		t.Fatal("laying the seed counted no passage")
	}
	made := brd.Passage()
	if made <= seeded {
		t.Fatalf("the Passage went from %d in the seed to %d as the cells were made, want on",
			seeded, made)
	}
	brd.EachCell(func(c cell.ID) {
		if got := brd.CellPassage(c); got != made {
			t.Errorf("cell %d's passage %d as the cells were made, want %d", c, got, made)
		}
	})
	last := made
	for _, step := range []struct {
		what  string
		write func()
	}{
		{"a Set", func() { cw.write().Set(cw.target, cw.grass) }},
		{"a touch", func() { cw.write().Touch(cw.target) }},
		{"a SetAll", func() { cw.write().SetAll(cw.snow) }},
	} {
		step.write()
		if now := brd.Passage(); now <= last {
			t.Errorf("%s: the Passage went from %d to %d, want on", step.what, last, now)
		} else {
			last = now
		}
	}
	brd.EachCell(func(c cell.ID) {
		if got := brd.CellPassage(c); got != last {
			t.Errorf("cell %d's passage %d after a SetAll, want %d", c, got, last)
		}
	})
}
