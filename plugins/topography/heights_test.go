package topography_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/steering"
)

var hill = board.CellKind{Name: board.Named("hill"), Cost: 1, Allows: board.Land | board.Air}

// raiseHills puts the hill cells at 12 and the rest at 0, each corner at the mean of its cells.
func raiseHills(r *topography.Relief, grid board.Grid, hills ...board.CellID) {
	r.SetHeights(topography.MeanOfCells(grid, func(c board.CellID) float64 {
		for _, h := range hills {
			if h == c {
				return 12
			}
		}
		return 0
	}))
}

func TestRelief_GroundAtReadsTheReliefAndFollowsIt(t *testing.T) {
	for name, grid := range map[string]board.Grid{
		"square": board.DefaultGrids{}.Square(4, 4, 32),
		"hex":    board.DefaultGrids{}.Hex(4, 4, 16),
	} {
		t.Run(name, func(t *testing.T) {
			brd := board.NewBoard(grid, board.NewTerrainMap())
			r := topography.NewRelief(brd)
			c, _ := grid.CellIndex(2, 1)
			raiseHills(r, grid, c)

			// A lone hill on a square grid is smoothed to its corners' mean, 3; a hex cell stays level.
			want := 12.0
			if name == "square" {
				want = 3
			}
			if got := r.Altitude(c); got != want {
				t.Errorf("the hill's altitude = %v, want %v", got, want)
			}
			if got := r.GroundAt(grid.CellCenter(c)); got != want {
				t.Errorf("ground at the hill's centre = %v, want %v", got, want)
			}
			other, _ := grid.CellIndex(0, 0)
			if got := r.GroundAt(grid.CellCenter(other)); got != 0 {
				t.Errorf("ground on the grass = %v, want 0", got)
			}
			if got := r.GroundAt(geom.NewVec(-100, -100)); got != 0 {
				t.Errorf("ground off the board = %v, want 0", got)
			}
			before, boardBefore := r.Version(), brd.Version()
			raiseHills(r, grid)
			if got := r.GroundAt(grid.CellCenter(c)); got != 0 {
				t.Errorf("ground after the hill was levelled = %v, want 0", got)
			}
			if r.Version() == before || brd.Version() == boardBefore {
				t.Error("levelling the hill left the relief's or the board's Version as it was")
			}
			if r.Step() <= 0 {
				t.Errorf("Step = %v, want the cell's shorter side", r.Step())
			}
		})
	}
}

func TestRelief_GroundSlopesBetweenCellsOnASquareGrid(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 6, 32)
	r := topography.NewRelief(board.NewBoard(grid, board.NewTerrainMap()))
	var hills []board.CellID
	for y := uint32(2); y <= 4; y++ {
		for x := uint32(2); x <= 4; x++ {
			c, _ := grid.CellIndex(x, y)
			hills = append(hills, c)
		}
	}
	raiseHills(r, grid, hills...)
	centre, _ := grid.CellIndex(3, 3)
	if got := r.GroundAt(grid.CellCenter(centre)); got != 12 {
		t.Errorf("the plateau's middle stands at %v, want the full 12", got)
	}
	corner, _ := grid.CellIndex(2, 2)
	if got := r.GroundAt(grid.CellCenter(corner)); got != 6 {
		t.Errorf("the plateau's corner cell stands at %v in its middle, want 6: its corners 3, 6, 6 and 12 are drawn split along the diagonal of the two 6s", got)
	}
	if got := r.Altitude(corner); got != 6.75 {
		t.Errorf("the plateau's corner cell's level is %v, want 6.75, the mean of its corners", got)
	}
	last := -1.0
	for x := 40.0; x <= 112; x += 8 { // walking east along row 3 up onto the plateau
		if got := r.GroundAt(geom.NewVec(x, 112)); got < last {
			t.Errorf("the ground drops from %v to %v at x %v on the way up the slope", last, got, x)
		} else {
			last = got
		}
	}
	if hs := r.Corners(centre); hs != (topography.Corners{12, 12, 12, 12}) {
		t.Errorf("Corners of the middle = %v, want four 12s", hs)
	}
}

// A wrapping board's lattice folds: the corners along the seam are one.
func TestRelief_FoldsWhereTheBoardWraps(t *testing.T) {
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	grid.(interface{ SetWrap(x, y bool) }).SetWrap(true, false)
	r := topography.NewRelief(board.NewBoard(grid, board.NewTerrainMap()))
	west, _ := grid.CellIndex(0, 1)
	east, _ := grid.CellIndex(3, 1)
	r.SetCorners(west, topography.Corners{5, 0, 5, 0})
	if got := r.Corners(east); got != (topography.Corners{0, 5, 0, 5}) {
		t.Errorf("across the seam the east cell's corners are %v, want its right ones the west cell's left, 5", got)
	}
	if got := r.GroundAt(geom.NewVec(127.9, 48)); got < 4.9 {
		t.Errorf("the ground at the seam stands at %v, want about 5", got)
	}
}

// installCtx is a plugin.Installer over a bare ECS.
type installCtx struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *installCtx) ECS() *goke.ECS                                  { return c.ecs }

type recruit struct{ start board.CellID }

// quasiWorld is a world with heights with a board over a 4x4 square grid in relief, a hill at (2,1), and
// the units define makes; collide adds the collision plugin.
type quasiWorld struct {
	ecs  *goke.ECS
	w    *world.Plugin
	brd  *board.Plugin
	topo *topography.Plugin
	grid board.Grid
}

func newQuasiWorld(t *testing.T, collide bool, define func(units *board.Units[recruit], grid board.Grid) []kind.Entry) *quasiWorld {
	t.Helper()
	qw := &quasiWorld{grid: board.DefaultGrids{}.Square(4, 4, 32)}
	qw.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 20, MaxSize: 20},
		Heights:  true,
	})
	var c *collision.Plugin
	if collide {
		c = collision.NewPlugin(qw.w)
	}
	qw.brd = board.NewPlugin(qw.grid, &board.MultipleOccupancy{}, qw.w)
	qw.topo = topography.NewPlugin(qw.w, qw.brd, topography.Config{Cell: 32})
	qw.brd.Res.Logic.Board.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land | board.Air})
	hillCell, _ := qw.grid.CellIndex(2, 1)
	qw.brd.Res.Logic.Board.Set(hillCell, hill)
	raiseHills(qw.topo.Relief(), qw.grid, hillCell)
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
	if err := qw.topo.Install(ctx); err != nil {
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
		qw.topo.RunPlan(rc, d)
		rc.Sync()
		qw.w.Clock().Replay(rc, d)
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
		walker = units.Define("walker", board.Mover{Domain: board.Land}, steering.Steering{MaxSpeed: 10})
		hawk = units.Define("hawk", board.Mover{Domain: board.Air, Lift: 40}, steering.Steering{MaxSpeed: 10})
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
	hillGround := qw.topo.Relief().GroundAt(qw.grid.CellCenter(onHill))
	if hillGround <= 0 || !alts[hillGround] || !alts[0] {
		t.Errorf("walkers stand at %v, want one at the hill's ground %v and one at 0 on the grass", alts, hillGround)
	}
	if got := zs[hawk.ID()]; len(got) != 1 || got[0].Altitude != hillGround+40 || got[0].Height != 2 {
		t.Errorf("hawk's Z = %v, want altitude %v (the hill plus its lift) and height 2", got, hillGround+40)
	}
	if qw.brd.Heights() != qw.topo.Relief() {
		t.Error("the board's heights are not the topography's relief")
	}
}

// coverAcross walks the board's Cover along row 3, west to east, listing every stretch.
func (qw *quasiWorld) coverAcross() [][5]float64 {
	var out [][5]float64
	qw.brd.Cover().Walk(geom.NewVec(1, 3*32+16), geom.NewVec(1, 0), 126, 0, func(near, far, bottom, top, tau float64) bool {
		out = append(out, [5]float64{near, far, bottom, top, tau})
		return true
	})
	return out
}

// The cover of a cell spans its ground and its kind's Height over it, and follows the ground
// when it is lifted.
func TestCover_SpansTheCellsBandAndFollowsItsGround(t *testing.T) {
	qw := newQuasiWorld(t, true, func(units *board.Units[recruit], grid board.Grid) []kind.Entry {
		k := units.Define("walker", board.Mover{Domain: board.Land}, steering.Steering{MaxSpeed: 10})
		start, _ := grid.CellIndex(0, 3)
		return []kind.Entry{k.Entry(recruit{start: start})}
	})
	brd := qw.brd.Res.Logic.Board
	wallCell, _ := qw.grid.CellIndex(3, 3)
	brd.Set(wallCell, board.CellKind{Name: board.Named("wall"), Cost: 1, Solid: true, Veil: 1, Height: 10})
	qw.topo.Relief().SetCorners(wallCell, topography.Corners{12, 12, 12, 12})
	qw.ecs.Tick(time.Second / 60)

	if got := qw.coverAcross(); len(got) != 1 || got[0] != [5]float64{95, 126, 12, 22, 0} {
		t.Errorf("cover along the row %v, want the wall from 95 on, 12 to 22, opaque", got)
	}
	qw.topo.Relief().SetCorners(wallCell, topography.Corners{20, 20, 20, 20})
	if got := qw.coverAcross(); len(got) != 1 || got[0][2] != 20 || got[0][3] != 30 {
		t.Errorf("cover after lifting the wall %v, want it from 20 to 30", got)
	}
}

// The heights ride on the topography's entity: what a save carries, and what a loaded game's
// relief takes over.
func TestHeights_LiveOnTheTopographysEntity(t *testing.T) {
	qw := newQuasiWorld(t, false, func(units *board.Units[recruit], grid board.Grid) []kind.Entry {
		k := units.Define("walker", board.Mover{Domain: board.Land}, steering.Steering{MaxSpeed: 10})
		start, _ := grid.CellIndex(0, 3)
		return []kind.Entry{k.Entry(recruit{start: start})}
	})
	qw.ecs.Tick(time.Second / 60)
	var comp goke.Comp[topography.Heights]
	var q *goke.Query
	qw.ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&comp).Build() }})
	n := 0
	for q.All(); q.Next(); {
		cur := q.Cursor()
		n += len(cur.IDs)
		if got := comp.Slice(cur)[0]; len(got.Values) != 25 {
			t.Errorf("the entity carries %d heights, want 25: a 4x4 grid's 5x5 corners", len(got.Values))
		}
	}
	if n != 1 {
		t.Errorf("%d entities carry heights, want one", n)
	}
}

func TestPlugin_RefusesAFlatWorld(t *testing.T) {
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "Heights") {
			t.Errorf("NewPlugin over a flat world: %q, want a refusal naming Heights", r)
		}
	}()
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 20}})
	topography.NewPlugin(w, board.NewPlugin(board.DefaultGrids{}.Square(4, 4, 32), &board.MultipleOccupancy{}, w), topography.Config{Cell: 32})
}

// The ground under a point is the ground drawn: the cell's top split into two flat triangles along
// the diagonal whose corners stand nearer in height, as render.Frame.Fold draws it.
func TestRelief_TheGroundIsTheGroundDrawn(t *testing.T) {
	grid := board.DefaultGrids{}.Square(2, 2, 32)
	r := topography.NewRelief(board.NewBoard(grid, board.NewTerrainMap()))
	c, _ := grid.CellIndex(0, 0)
	// corners 0, 20, 24 and 2: the 0 and the 2 stand nearer, so the top is split along 0–3
	r.SetCorners(c, topography.Corners{0, 20, 24, 2})
	for _, p := range []struct{ x, y, want float64 }{
		{16, 16, 1},    // the middle, on the diagonal of the 0 and the 2
		{24, 8, 10.5},  // on the triangle with the 20
		{8, 24, 12.5},  // on the triangle with the 24
		{28, 4, 15.25}, // near the 20
	} {
		if got := r.GroundAt(geom.NewVec(p.x, p.y)); math.Abs(got-p.want) > 1e-9 {
			t.Errorf("the ground at (%v, %v) is %v, want %v on the triangles drawn", p.x, p.y, got, p.want)
		}
	}
	// corners 10, 0, 0, 20: split along the two 0s, 1–2; the middle is 0, not the mean 7.5
	r.SetCorners(c, topography.Corners{10, 0, 0, 20})
	if got := r.GroundAt(geom.NewVec(16, 16)); got != 0 {
		t.Errorf("the ground in the middle is %v, want 0 on the diagonal of the 0s", got)
	}
	if got := r.GroundAt(geom.NewVec(4, 4)); math.Abs(got-7.5) > 1e-9 {
		t.Errorf("the ground near the 10 is %v, want 7.5", got)
	}
}
