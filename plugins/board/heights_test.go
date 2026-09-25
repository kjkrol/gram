package board_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
)

var hill = board.CellKind{Name: board.Named("hill"), Cost: 1, Allows: board.Land | board.Air}

// raiseHills puts the hill cells at 12 and the rest at 0, each corner at the mean of its cells.
func raiseHills(brd *board.Board, hills ...board.CellID) {
	brd.SetHeights(board.MeanOfCells(brd, func(c board.CellID) float64 {
		for _, h := range hills {
			if h == c {
				return 12
			}
		}
		return 0
	}))
}

func TestBoard_GroundAtReadsTheReliefAndFollowsIt(t *testing.T) {
	for name, grid := range map[string]board.Grid{
		"square": board.DefaultGrids{}.Square(4, 4, 32),
		"hex":    board.DefaultGrids{}.Hex(4, 4, 16),
	} {
		t.Run(name, func(t *testing.T) {
			brd := board.NewBoard(grid, board.NewTerrainMap())
			brd.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
			c, _ := grid.CellIndex(2, 1)
			brd.Set(c, hill)
			raiseHills(brd, c)

			// A lone hill on a square grid is smoothed to its corners' mean, 3; a hex cell stays level.
			want := 12.0
			if name == "square" {
				want = 3
			}
			if got := brd.Altitude(c); got != want {
				t.Errorf("the hill's altitude = %v, want %v", got, want)
			}
			if got := brd.GroundAt(grid.CellCenter(c)); got != want {
				t.Errorf("ground at the hill's centre = %v, want %v", got, want)
			}
			other, _ := grid.CellIndex(0, 0)
			if got := brd.GroundAt(grid.CellCenter(other)); got != 0 {
				t.Errorf("ground on the grass = %v, want 0", got)
			}
			if got := brd.GroundAt(geom.NewVec(-100, -100)); got != 0 {
				t.Errorf("ground off the board = %v, want 0", got)
			}
			before := brd.Version()
			raiseHills(brd)
			if got := brd.GroundAt(grid.CellCenter(c)); got != 0 {
				t.Errorf("ground after the hill was levelled = %v, want 0", got)
			}
			if brd.Version() == before {
				t.Error("levelling the hill left the Version as it was")
			}
			if brd.Step() <= 0 {
				t.Errorf("Step = %v, want the cell's shorter side", brd.Step())
			}
		})
	}
}

func TestBoard_GroundSlopesBetweenCellsOnASquareGrid(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 6, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
	var hills []board.CellID
	for y := uint32(2); y <= 4; y++ {
		for x := uint32(2); x <= 4; x++ {
			c, _ := grid.CellIndex(x, y)
			hills = append(hills, c)
		}
	}
	raiseHills(brd, hills...)
	centre, _ := grid.CellIndex(3, 3)
	if got := brd.GroundAt(grid.CellCenter(centre)); got != 12 {
		t.Errorf("the plateau's middle stands at %v, want the full 12", got)
	}
	corner, _ := grid.CellIndex(2, 2)
	if got := brd.GroundAt(grid.CellCenter(corner)); got != 6.75 {
		t.Errorf("the plateau's corner cell stands at %v, want 6.75, the mean of its corners 3, 6, 6 and 12", got)
	}
	last := -1.0
	for x := 40.0; x <= 112; x += 8 { // walking east along row 3 up onto the plateau
		if got := brd.GroundAt(geom.NewVec(x, 112)); got < last {
			t.Errorf("the ground drops from %v to %v at x %v on the way up the slope", last, got, x)
		} else {
			last = got
		}
	}
	if hs, x, y, ok := brd.Corners(centre); !ok || x != 3 || y != 3 || hs != [4]float64{12, 12, 12, 12} {
		t.Errorf("Corners of the middle = %v at (%d, %d) ok %v, want four 12s at (3, 3)", hs, x, y, ok)
	}
}

// quasiWorld is a Quasi3D world with a board over a 4x4 square grid, a hill at (2,1), and the units
// define makes; collide adds the collision plugin and terrain bodies.
type quasiWorld struct {
	ecs  *goke.ECS
	w    *world.Plugin
	brd  *board.Plugin
	grid board.Grid
}

func newQuasiWorld(t *testing.T, collide bool, define func(units *board.Units[recruit], grid board.Grid) []kind.Entry) *quasiWorld {
	t.Helper()
	qw := &quasiWorld{grid: board.DefaultGrids{}.Square(4, 4, 32)}
	qw.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 20, MaxSize: 20},
		Quasi3D:  true,
	})
	var c *collision.Plugin
	if collide {
		c = collision.NewPlugin(qw.w)
	}
	qw.brd = board.NewPlugin(qw.grid, &board.MultipleOccupancy{}, qw.w)
	if c != nil {
		qw.brd.WithCollision(c)
	}
	qw.brd.Res.Logic.Board.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land | board.Air})
	hillCell, _ := qw.grid.CellIndex(2, 1)
	qw.brd.Res.Logic.Board.Set(hillCell, hill)
	raiseHills(qw.brd.Res.Logic.Board, hillCell)
	units := board.NewUnits[recruit](qw.brd, board.Shape{Size: 20, Height: 2}, func(r recruit) geom.Vec { return qw.grid.CellCenter(r.start) })
	entries := define(units, qw.grid)

	ctx := &installCtx{ecs: goke.New()}
	if err := qw.w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if c != nil {
		if err := c.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := qw.brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	qw.w.Seed(entries...)
	if err := qw.w.Populate(); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		qw.w.RunPlan(rc, d)
		if c != nil {
			c.RunPlan(rc, d)
		}
		qw.brd.RunPlan(rc, d)
		rc.Sync()
	})
	qw.ecs = ctx.ecs
	return qw
}

// zs lists every entity's Z with its kind.
func (qw *quasiWorld) zs() map[kind.ID][]world.Z {
	var base goke.Comp[world.Base]
	var z goke.Comp[world.Z]
	var q *goke.Query
	qw.ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base, &z).Build() }})
	out := map[kind.ID][]world.Z{}
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i := range cur.IDs {
			out[base.Slice(cur)[i].TypeID] = append(out[base.Slice(cur)[i].TypeID], z.Slice(cur)[i])
		}
	}
	return out
}

func TestAltitude_IsTheGroundUnderTheUnitPlusItsLift(t *testing.T) {
	var walker, hawk kind.Of[recruit]
	qw := newQuasiWorld(t, false, func(units *board.Units[recruit], grid board.Grid) []kind.Entry {
		walker = units.Define("walker", board.Mover{Domain: board.Land}, world.Steering{MaxSpeed: 10})
		hawk = units.Define("hawk", board.Mover{Domain: board.Air, Lift: 40}, world.Steering{MaxSpeed: 10})
		onHill, _ := grid.CellIndex(2, 1)
		onGrass, _ := grid.CellIndex(0, 3)
		return []kind.Entry{walker.Entry(recruit{start: onHill}), hawk.Entry(recruit{start: onHill}), walker.Entry(recruit{start: onGrass})}
	})
	qw.ecs.Tick(time.Second / 60)

	zs := qw.zs()
	if got := zs[walker.ID()]; len(got) != 2 || got[0].Height != 2 || got[1].Height != 2 {
		t.Fatalf("walkers' Z = %v, want two, each 2 tall", got)
	}
	alts := map[float64]bool{}
	for _, z := range zs[walker.ID()] {
		alts[z.Altitude] = true
	}
	onHill, _ := qw.grid.CellIndex(2, 1)
	hillGround := qw.brd.Res.Logic.Board.GroundAt(qw.grid.CellCenter(onHill))
	if hillGround <= 0 || !alts[hillGround] || !alts[0] {
		t.Errorf("walkers stand at %v, want one at the hill's ground %v and one at 0 on the grass", alts, hillGround)
	}
	if got := zs[hawk.ID()]; len(got) != 1 || got[0].Altitude != hillGround+40 || got[0].Height != 2 {
		t.Errorf("hawk's Z = %v, want altitude %v (the hill plus its lift) and height 2", got, hillGround+40)
	}
}

func TestBodies_CarryTheirKindsHeightsInAQuasi3DWorld(t *testing.T) {
	qw := newQuasiWorld(t, true, func(units *board.Units[recruit], grid board.Grid) []kind.Entry {
		k := units.Define("walker", board.Mover{Domain: board.Land}, world.Steering{MaxSpeed: 10})
		start, _ := grid.CellIndex(0, 3)
		return []kind.Entry{k.Entry(recruit{start: start})}
	})
	wallCell, _ := qw.grid.CellIndex(3, 3)
	qw.brd.Res.Logic.Board.Set(wallCell, board.CellKind{Name: board.Named("wall"), Cost: 1, Solid: true, Height: 10})
	qw.brd.Res.Logic.Board.SetRelief(wallCell, board.Relief{Corners: [4]float32{12, 12, 12, 12}})
	qw.ecs.Tick(time.Second / 60)

	bodies := 0
	for id, zs := range qw.zs() {
		if id == 0 { // reserved kinds come first; the walker's is the last defined
			continue
		}
		for _, z := range zs {
			if z == (world.Z{Altitude: 12, Height: 10}) {
				bodies++
			}
		}
	}
	if bodies != 1 {
		t.Errorf("found %d bodies standing 12 up and 10 tall, want the wall", bodies)
	}
}

func TestBodies_CarryNoZInAFlatWorld(t *testing.T) {
	bw, _ := squareWorld(t, mover{})
	bw.tick()
	var z goke.OptComp[world.Z]
	var base goke.Comp[world.Base]
	var q *goke.Query
	bw.ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base).Optional(&z).Build() }})
	for q.All(); q.Next(); {
		if z.Present(q.Cursor()) {
			t.Fatal("an entity of a flat world carries a Z")
		}
	}
}

func expectPanic(t *testing.T, want string, run func()) {
	t.Helper()
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, want) {
			t.Errorf("panic %q, want one mentioning %q", msg, want)
		}
	}()
	run()
	t.Errorf("no panic, want one mentioning %q", want)
}

func TestFlatWorld_RefusesHeights(t *testing.T) {
	flat := func() (*world.Plugin, *board.Plugin) {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 128, Height: 128},
			Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 20, MaxSize: 20},
		})
		return w, board.NewPlugin(board.DefaultGrids{}.Square(4, 4, 32), &board.MultipleOccupancy{}, w)
	}
	at := func(recruit) geom.Vec { return geom.NewVec(48, 48) }

	t.Run("a kind with a height", func(t *testing.T) {
		_, brd := flat()
		expectPanic(t, "Quasi3D", func() { brd.CellKindDict().Create(board.CellKind{Name: board.Named("wall"), Height: 3}) })
	})
	t.Run("a layout with heights", func(t *testing.T) {
		_, brd := flat()
		brd.Seed(board.Layout{Heights: func(geom.Vec) float64 { return 1 }})
		if err := brd.Populate(); err == nil || !strings.Contains(err.Error(), "Quasi3D") {
			t.Errorf("Populate = %v, want an error mentioning Quasi3D", err)
		}
	})
	t.Run("units with a height", func(t *testing.T) {
		_, brd := flat()
		expectPanic(t, "Quasi3D", func() { board.NewUnits[recruit](brd, board.Shape{Size: 20, Height: 2}, at) })
	})
	t.Run("a unit with a lift", func(t *testing.T) {
		_, brd := flat()
		units := board.NewUnits[recruit](brd, board.Shape{Size: 20}, at)
		expectPanic(t, "Quasi3D", func() { units.Define("hawk", board.Mover{Domain: board.Air, Lift: 40}, world.Steering{}) })
	})
	t.Run("a unit with a Z of its own", func(t *testing.T) {
		_, brd := flat()
		units := board.NewUnits[recruit](brd, board.Shape{Size: 20}, at)
		expectPanic(t, "Quasi3D", func() {
			units.Define("tower", board.Mover{Domain: board.Land}, world.Steering{}, comp.Const(world.Z{Height: 3}))
		})
	})
}
