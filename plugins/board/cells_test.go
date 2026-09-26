package board_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/effects"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// cellWorld is world + effects + board over a 4x4 grass board, with frost defined, casting from a
// hook at the start of each tick; boardFirst runs the board's pass before the effects'. With
// quasi3D the world has heights and the board shapes the ground by shape.
type cellWorld struct {
	ecs     *goke.ECS
	brd     *board.Plugin
	fx      *effects.Plugin
	frost   effects.ID
	mound   effects.ID
	target  board.CellID
	grass   board.CellKind
	snow    board.CellKind
	casting func(cb *goke.CmdBuf)

	plots *goke.Query
	plot  goke.Comp[board.Plot]
	idle  goke.OptComp[effects.Idle]
}

const cellTick = time.Second / 10

func newCellWorld(t *testing.T, boardFirst bool) *cellWorld {
	return newShapedWorld(t, boardFirst, false, nil, board.Shaping{})
}

func newShapedWorld(t *testing.T, boardFirst, quasi3D bool, grid board.Grid, shape board.Shaping) *cellWorld {
	t.Helper()
	cw := &cellWorld{}
	if grid == nil {
		grid = board.DefaultGrids{}.Square(4, 4, cellSize)
	}
	cw.target, _ = grid.CellIndex(2, 2)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4 * cellSize, Height: 4 * cellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: unitSize, MaxSize: unitSize},
		Quasi3D:  quasi3D,
	})
	cw.fx = effects.NewPlugin(w)
	cw.brd = board.NewPlugin(grid, &board.MultipleOccupancy{}, w).WithShaping(shape)
	cw.grass = board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land}
	cw.snow = board.CellKind{Name: board.Named("snow"), Cost: 3, Allows: board.Land}
	cw.brd.Res.Logic.Board.SetAll(cw.grass)
	snow := cw.snow
	cw.frost = cw.fx.Define("frost", effects.Spec{effects.Lasts(2 * cellTick), effects.Alter(func(g *board.Ground) { g.Kind = snow })})
	cw.mound = cw.fx.Define("mound", effects.Spec{effects.Lasts(2 * cellTick), effects.Alter(func(p *board.Plot) {
		p.Relief = board.Relief{Corners: [4]float32{6, 6, 6, 6}}
	})})

	ctx := &installCtx{ecs: goke.New()}
	for _, install := range []func() error{
		func() error { return w.Install(ctx) }, func() error { return cw.fx.Install(ctx) }, func() error { return cw.brd.Install(ctx) },
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
		cw.plots = si.NewQueryBuilder(&cw.plot).Optional(&cw.idle).Build()
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
		w.RunPlan(rc, d)
		if boardFirst {
			cw.brd.RunPlan(rc, d)
			cw.fx.RunPlan(rc, d)
		} else {
			cw.fx.RunPlan(rc, d)
			cw.brd.RunPlan(rc, d)
		}
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

// cells counts the cell entities and how many of them still carry Idle.
func (cw *cellWorld) cells() (n, idle int) {
	for cw.plots.All(); cw.plots.Next(); {
		cur := cw.plots.Cursor()
		n += len(cur.IDs)
		if cw.idle.Present(cur) {
			idle += len(cur.IDs)
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

// The kind and the heights are apart: a frost ending puts back the grass and leaves the ground
// raised while it lay.
func TestCells_AnEffectEndingKeepsTheGroundShapedMeanwhile(t *testing.T) {
	cw := newShapedWorld(t, false, true, nil, board.Shaping{Step: 5})
	cw.castFrost()
	cw.ecs.Tick(cellTick)
	cw.ecs.Tick(cellTick)
	if got := cw.kind(); got != cw.snow {
		t.Fatalf("terrain is %q with the effect on, want snow", got.Name)
	}
	raised := board.Relief{Corners: [4]float32{3, 3, 3, 3}}
	cw.board().SetRelief(cw.target, raised)
	for range 4 {
		cw.ecs.Tick(cellTick)
	}
	if got := cw.kind(); got != cw.grass {
		t.Errorf("terrain is %q after the effect, want grass back", got.Name)
	}
	if got := cw.board().Relief(cw.target); got != raised {
		t.Errorf("the cell's corners after the effect = %v, want %v as raised under the frost", got, raised)
	}
}

// heights reads the target cell's corners.
func (cw *cellWorld) heights() [4]float32 { return cw.board().Relief(cw.target).Corners }

func TestShaping_RaiseLiftsTheNearestCornerAndTheSlopeFollows(t *testing.T) {
	cw := newShapedWorld(t, false, true, nil, board.Shaping{Step: 5, MaxStep: 5})
	b := cw.board()
	corner := b.CellCenter(cw.target).Sub(geom.NewVec(cellSize/2, cellSize/2)) // the target's top-left
	v := b.Version()
	for range 3 {
		cw.brd.Queues()[0].Put(control.Nobody, board.Raise{At: corner.Add(geom.NewVec(2, 3))})
	}
	cw.ecs.Tick(cellTick)
	if got := cw.heights(); got[0] != 15 {
		t.Errorf("the target's corners = %v, want its top-left raised three times to 15", got)
	}
	if b.Version() == v {
		t.Error("shaping left the Version as it was")
	}
	up, _ := b.CellIndex(2, 1) // the corner is this cell's bottom-left: one height, however many cells
	if got := b.Relief(up).Corners[2]; got != 15 {
		t.Errorf("the cell above shares the corner at %v, want 15", got)
	}
	// Every corner an edge away is within MaxStep of the next: 15, 10, 5 going out.
	if got := cw.heights(); got[1] != 10 || got[2] != 10 || got[3] != 5 {
		t.Errorf("the target's corners = %v, want the slope 15, 10, 10, 5", got)
	}
	far, _ := b.CellIndex(0, 0)
	if got := b.Relief(far).Corners; got != [4]float32{0, 0, 0, 5} {
		t.Errorf("a far cell's corners = %v, want only its bottom-right 3 edges out at 5", got)
	}

	cw.brd.Queues()[1].Put(control.Nobody, board.Lower{At: corner})
	cw.ecs.Tick(cellTick)
	if got := cw.heights()[0]; got != 10 {
		t.Errorf("after a Lower the corner stands at %v, want 10", got)
	}
}

func TestShaping_LevelBringsAnAreaToTheHeightWhereItBegan(t *testing.T) {
	cw := newShapedWorld(t, false, true, nil, board.Shaping{Step: 5})
	b := cw.board()
	b.SetHeights(func(p geom.Vec) float64 { return p.X / 4 }) // a ramp rising east
	from, to := b.CellCenter(cw.target), geom.NewVec(0, 0)
	cw.brd.Queues()[2].Put(control.Nobody, board.Level{From: from, To: to})
	cw.ecs.Tick(cellTick)
	want := float32(b.CellCenter(cw.target).X+cellSize/2) / 4 // the corner nearest where it began: the target's bottom-right
	for _, c := range []board.CellID{cw.target, 0} {
		for k, h := range b.Relief(c).Corners { // both cells lie wholly between the two corners
			if h != want {
				t.Errorf("cell %d corner %d at %v, want %v", c, k, h, want)
			}
		}
	}
}

func TestShaping_OnAHexGridACellIsItsOwnLevel(t *testing.T) {
	grid := board.DefaultGrids{}.Hex(4, 4, cellSize/2)
	cw := newShapedWorld(t, false, true, grid, board.Shaping{Step: 4, MaxStep: 2})
	b := cw.board()
	cw.brd.Queues()[0].Put(control.Nobody, board.Raise{At: b.CellCenter(cw.target)})
	cw.ecs.Tick(cellTick)
	if got := cw.heights(); got != [4]float32{4, 4, 4, 4} {
		t.Errorf("the raised hex = %v, want level at 4", got)
	}
	for _, n := range b.Neighbors(cw.target) {
		if got := b.Altitude(n); got != 2 {
			t.Errorf("neighbour %d stands at %v, want 2, MaxStep below", n, got)
		}
	}
}

func TestShaping_AFlatWorldHasNoShapingCommands(t *testing.T) {
	cw := newCellWorld(t, false)
	if len(cw.brd.Queues()) != 0 || len(cw.brd.DefaultBindings()) != 0 {
		t.Error("a flat world's board offers shaping commands")
	}
}

// An effect raising one cell's Plot alone raises its neighbours' corners where they meet it, and
// lowers them again when it ends: the ground has no vertical walls.
func TestCells_AnEffectOnOneCellsReliefCarriesItsNeighboursCorners(t *testing.T) {
	cw := newCellWorld(t, false)
	id, _ := cw.brd.CellEntity(cw.target)
	cw.casting = func(cb *goke.CmdBuf) { cw.fx.Cast(cb, id, cw.mound) }
	cw.ecs.Tick(cellTick)
	cw.ecs.Tick(cellTick)
	corner := func() float32 { return cw.board().Relief(neighbour(t, cw)).Corners[3] }
	if got := corner(); got != 6 {
		t.Fatalf("the neighbour's corner at the raised cell stands at %v, want 6", got)
	}
	for range 4 {
		cw.ecs.Tick(cellTick)
	}
	if got, own := corner(), cw.heights(); got != 0 || own != [4]float32{} {
		t.Errorf("after the effect the cell's corners %v, the neighbour's %v; want all back at 0", own, got)
	}
}

// neighbour is the cell up and left of the target, meeting it at its top-left corner.
func neighbour(t *testing.T, cw *cellWorld) board.CellID {
	t.Helper()
	x, y, _ := cw.board().Coords(cw.target)
	c, ok := cw.board().CellIndex(x-1, y-1)
	if !ok {
		t.Fatal("no cell up and left of the target")
	}
	return c
}
