package board_test

import (
	"errors"
	"maps"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
)

// installCtx is the plugin.Installer a Stage would hand over, minus the engine.
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

// mover is a unit's row: where it starts, where it keeps driving when heading is set, what it
// sees, and how it moves (zero: Land).
type mover struct {
	cell    board.CellID
	heading geom.Vec
	sight   *vision.Sight
	domain  board.Domain
	offset  float64 // shifts the box right, to straddle two cells
}

const unitSize = 22

// groundWorld is world + collision + board + vision ticked as the demo ticks them.
type groundWorld struct {
	t     *testing.T
	w     *world.Plugin
	brd   *board.Plugin
	ecs   *goke.ECS
	base  goke.Comp[world.Base]
	sight goke.OptComp[vision.Sight]
	q     *goke.Query
}

func newGroundWorld(t *testing.T, grid board.Grid, width, height uint32, terrain func(*board.Board), units []mover, behaviors ...plugin.Behavior) *groundWorld {
	t.Helper()
	bw := &groundWorld{t: t}
	bw.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: unitSize, MaxSize: unitSize},
	})
	c := collision.NewPlugin(bw.w)
	bw.brd = board.NewPlugin(grid, &board.MultipleOccupancy{}, bw.w)
	terrain(bw.brd.Res.Logic.Board)
	for _, b := range behaviors {
		err := bw.brd.RegisterBehavior(b)
		if errors.Is(err, plugin.ErrUnhostedBehavior) {
			err = c.RegisterBehavior(b)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	v := vision.NewPlugin(bw.w)

	ctx := &installCtx{ecs: goke.New()}
	if err := bw.w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := bw.brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := v.Install(ctx); err != nil {
		t.Fatal(err)
	}

	for i, u := range units {
		spec := kind.Spec{
			comp.Load(func(m mover) world.Position {
				box := board.CellAABB(grid, m.cell, unitSize)
				box.TopLeft.X += m.offset
				box.BottomRight.X += m.offset
				return world.Position{AABB: box}
			}),
			comp.Const(world.Velocity{}),
			comp.Const(collision.Collider{}),
			comp.Const(collision.Physics{}),
			comp.Load(func(m mover) board.Cell { return board.Cell{ID: m.cell} }),
			comp.Load(func(m mover) board.Mover {
				if m.domain == 0 {
					return board.Mover{Domain: board.Land}
				}
				return board.Mover{Domain: m.domain}
			}),
		}
		if u.heading != (geom.Vec{}) {
			spec = append(spec, comp.Load(func(m mover) world.Steering {
				return world.Steering{Want: m.heading, WantSpeed: 64, MaxSpeed: 64}
			}))
		}
		if u.sight != nil {
			spec = append(spec, comp.Const(*u.sight))
		}
		name := string(rune('a' + i))
		bw.w.Seed(kind.Define[mover](bw.w.Kinds(), name, spec).Entry(u))
	}
	if err := bw.w.Populate(); err != nil {
		t.Fatal(err)
	}

	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		bw.q = si.NewQueryBuilder(&bw.base).Optional(&bw.sight).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		bw.w.RunPlan(rc, d)
		c.RunPlan(rc, d)
		bw.brd.RunPlan(rc, d)
		v.RunPlan(rc, d)
		rc.Sync()
		bw.w.Clock().Replay(rc, d)
	})
	bw.ecs = ctx.ecs
	return bw
}

func (bw *groundWorld) tick() { bw.ecs.Tick(time.Second / 60) }

// snapshot lists the units' boxes by TypeID order.
func (bw *groundWorld) snapshot() []geom.AABB {
	byType := map[kind.ID]geom.AABB{}
	for bw.q.All(); bw.q.Next(); {
		cur := bw.q.Cursor()
		for _, b := range bw.base.Slice(cur) {
			byType[b.TypeID] = b.Pos.AABB.AABB
		}
	}
	units := make([]geom.AABB, 0, len(byType))
	for _, id := range slices.Sorted(maps.Keys(byType)) {
		units = append(units, byType[id])
	}
	return units
}

// solid lists the boxes the world's Field holds solid for an entity on layers, board-wide.
func (bw *groundWorld) solid(layers world.Layers) []geom.AABB {
	var out []geom.AABB
	w, h := bw.w.Res.Config.Space.Width, bw.w.Res.Config.Space.Height
	bw.w.Field().Solid(layers, geom.NewAABBAt(geom.NewVec(0, 0), float64(w), float64(h)), func(fb world.FieldBox) bool {
		out = append(out, fb.Box)
		return true
	})
	return out
}

// seen returns what the one observer saw.
func (bw *groundWorld) seen() (vision.Sighted, bool) {
	for bw.q.All(); bw.q.Next(); {
		cur := bw.q.Cursor()
		if !bw.sight.Present(cur) {
			continue
		}
		return bw.sight.Slice(cur)[0].Seen, true
	}
	return vision.Sighted{}, false
}

// overlaps reports whether a and b share interior, not just an edge.
func overlaps(a, b geom.AABB) bool {
	const eps = 1e-6
	return min(a.BottomRight.X, b.BottomRight.X)-max(a.TopLeft.X, b.TopLeft.X) > eps &&
		min(a.BottomRight.Y, b.BottomRight.Y)-max(a.TopLeft.Y, b.TopLeft.Y) > eps
}

func (bw *groundWorld) assertClear(tick int, units, walls []geom.AABB) {
	bw.t.Helper()
	for i, u := range units {
		for _, b := range walls {
			if overlaps(u, b) {
				bw.t.Fatalf("tick %d: unit %d at %v overlaps solid ground %v", tick, i, u, b)
			}
		}
	}
}

const cellSize = 32

func squareWorld(t *testing.T, units ...mover) (*groundWorld, board.CellID) {
	t.Helper()
	return squareWorldWith(t, nil, units...)
}

// squareWorldWith is squareWorld with a behavior registered on the board.
func squareWorldWith(t *testing.T, behavior plugin.Behavior, units ...mover) (*groundWorld, board.CellID) {
	t.Helper()
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	cell := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	for i := range units {
		if units[i].cell == 0 {
			units[i].cell = cell(1, 7)
		}
	}
	var behaviors []plugin.Behavior
	if behavior != nil {
		behaviors = append(behaviors, behavior)
	}
	bw := newGroundWorld(t, grid, 6*cellSize, 16*cellSize, func(brd *board.Board) {
		brd.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
		for y := uint32(1); y <= 14; y++ {
			brd.Set(cell(3, y), wall)
		}
	}, units, behaviors...)
	return bw, cell(3, 7)
}

var east = geom.NewVec(1, 0)

// wall is a solid kind that also cuts sight, as a wall of stone does.
var wall = board.CellKind{Name: board.Named("wall"), Cost: 1, Solid: true, Veil: 1}

func TestGround_AWallColumnIsSolidAndAUnitCannotEnterIt(t *testing.T) {
	bw, _ := squareWorld(t, mover{heading: east})
	walls := bw.solid(world.Layers(board.Land))
	if len(walls) != 14 {
		t.Fatalf("%d solid cells, want the 14 of the wall", len(walls))
	}
	var units []geom.AABB
	for tick := range 60 {
		bw.tick()
		units = bw.snapshot()
		bw.assertClear(tick, units, walls)
	}
	if units[0].BottomRight.X < walls[0].TopLeft.X-1 {
		t.Errorf("unit ends at %v, never reached the wall at %v", units[0], walls[0].TopLeft.X)
	}
}

func TestGround_AUnitPushedByAnotherStaysOutOfTheWall(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	cell := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	bw, _ := squareWorld(t, mover{cell: cell(2, 7)}, mover{cell: cell(1, 7), heading: east})
	walls := bw.solid(world.Layers(board.Land))
	for tick := range 120 {
		bw.tick()
		bw.assertClear(tick, bw.snapshot(), walls)
	}
	units := bw.snapshot()
	if start := float64(cellSize) + (cellSize-unitSize)/2; units[1].TopLeft.X < start+5 {
		t.Errorf("the pusher never moved: %v behind %v", units[1], units[0])
	}
}

func TestGround_AHexIsCoveredAndKeepsAUnitOut(t *testing.T) {
	grid := board.DefaultGrids{}.Hex(4, 4, cellSize)
	hex, _ := grid.CellIndex(1, 1)
	start, _ := grid.CellAt(geom.NewVec(20, grid.CellCenter(hex).Y))
	bw := newGroundWorld(t, grid, 320, 256, func(brd *board.Board) {
		brd.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
		brd.Set(hex, board.CellKind{Name: board.Named("rock"), Solid: true})
	}, []mover{{cell: start, heading: east}})
	walls := bw.solid(world.Layers(board.Land))
	if len(walls) != 2*board.HexCapStrips+1 {
		t.Fatalf("%d solid boxes for one hex, want %d", len(walls), 2*board.HexCapStrips+1)
	}
	for tick := range 90 {
		bw.tick()
		bw.assertClear(tick, bw.snapshot(), walls)
	}
	units := bw.snapshot()
	center := grid.CellCenter(hex)
	if units[0].BottomRight.X < center.X-math.Sqrt(3)/2*cellSize-1 {
		t.Errorf("unit ends at %v, never reached the hex round %v", units[0], center)
	}
}

// Knocking a cell out of the wall opens it on the next tick, and the world gains no entity.
func TestGround_AGapKnockedInTheWallLetsAUnitThroughOnTheNextTick(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	cell := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	bw, gap := squareWorld(t, mover{cell: cell(1, 7), heading: east})
	bw.tick()
	before := bw.w.Res.Telemetry.Count
	bw.brd.Res.Logic.Board.Set(gap, board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
	if got := len(bw.solid(world.Layers(board.Land))); got != 13 {
		t.Fatalf("%d solid cells after knocking one out, want 13", got)
	}
	for range 90 {
		bw.tick()
	}
	if units := bw.snapshot(); units[0].TopLeft.X < float64(4*cellSize) {
		t.Errorf("unit ends at %v, want it through the gap", units[0])
	}
	if got := bw.w.Res.Telemetry.Count; got != before || got != 1 {
		t.Errorf("telemetry counts %d entities, %d before the change; want the one unit", got, before)
	}
}

func TestGround_AStrikeOnTheWallIsAContactWithTheTerrain(t *testing.T) {
	var hits []collision.Contact
	strikes := collision.Every(func(_ plugin.Tick, s collision.Struck) { hits = append(hits, s.Contacts...) })
	bw, gap := squareWorldWith(t, strikes, mover{heading: east})
	for range 60 {
		bw.tick()
	}
	if len(hits) == 0 {
		t.Fatal("the unit drove into the wall and struck nothing")
	}
	h := hits[len(hits)-1]
	if !h.Terrain || h.Other != 0 || h.Cell != uint64(gap) || h.Normal != geom.NewVec(-1, 0) {
		t.Errorf("last contact %+v, want the terrain at cell %d, pushing west", h, gap)
	}
}

func TestGround_AWallThatVeilsCutsSight(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	cell := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	observer := mover{cell: cell(1, 7), sight: &vision.Sight{Facing: east, HalfAngle: math.Pi / 8, Radius: 300}}
	target := mover{cell: cell(5, 7)}

	bw, _ := squareWorld(t, observer, target)
	bw.tick()
	if seen, ok := bw.seen(); !ok || seen.Count != 0 {
		t.Errorf("saw %v through the wall, want nobody", seen.IDs[:seen.Count])
	}

	open := newGroundWorld(t, grid, 6*cellSize, 16*cellSize, func(brd *board.Board) {
		brd.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
	}, []mover{observer, target})
	open.tick()
	if seen, ok := open.seen(); !ok || seen.Count != 1 {
		t.Errorf("saw %d across open ground, want 1", seen.Count)
	}
}

// Solid and Veil are apart: a fence stops walkers and hides nothing.
func TestGround_ASolidCellWithNoVeilLetsSightThrough(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	cell := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	fence := func(brd *board.Board) {
		brd.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
		for y := uint32(1); y <= 14; y++ {
			brd.Set(cell(3, y), board.CellKind{Name: board.Named("fence"), Cost: 1, Solid: true})
		}
	}
	observer := mover{cell: cell(1, 7), sight: &vision.Sight{Facing: east, HalfAngle: math.Pi / 8, Radius: 300}}
	bw := newGroundWorld(t, grid, 6*cellSize, 16*cellSize, fence, []mover{observer, {cell: cell(5, 7)}})
	bw.tick()
	if seen, ok := bw.seen(); !ok || seen.Count != 1 {
		t.Errorf("saw %d across the fence, want the target", seen.Count)
	}
	if got := len(bw.solid(world.Layers(board.Land))); got != 14 {
		t.Errorf("%d solid fence cells, want 14", got)
	}
}

// forestColumn is grass with a forest down column 3, veiling sight by veil, solid when solid.
func forestColumn(grid board.Grid, veil float64, solid bool) func(*board.Board) {
	cell := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	return func(brd *board.Board) {
		brd.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
		for y := uint32(1); y <= 14; y++ {
			brd.Set(cell(3, y), board.CellKind{Name: board.Named("forest"), Cost: 1, Allows: board.Land, Solid: solid, Veil: veil, Veils: board.Land})
		}
	}
}

func TestGround_AFullyVeiledCellOnlyBlocksSight(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	cell := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	forest := forestColumn(grid, 1, false)
	observer := mover{cell: cell(1, 7), sight: &vision.Sight{Facing: east, HalfAngle: math.Pi / 8, Radius: 300}}
	bw := newGroundWorld(t, grid, 6*cellSize, 16*cellSize, forest, []mover{observer, {cell: cell(5, 7)}})
	bw.tick()
	if seen, ok := bw.seen(); !ok || seen.Count != 0 {
		t.Errorf("saw %v through the forest, want nobody", seen.IDs[:seen.Count])
	}
	if got := len(bw.solid(world.Layers(board.Land))); got != 0 {
		t.Errorf("%d solid forest cells, want none", got)
	}

	walker := newGroundWorld(t, grid, 6*cellSize, 16*cellSize, forest, []mover{{cell: cell(1, 7), heading: east}})
	for range 90 {
		walker.tick()
	}
	if units := walker.snapshot(); units[0].TopLeft.X < float64(4*cellSize) {
		t.Errorf("unit ends at %v, want it past the forest column", units[0])
	}
}

// A forest column one cell (32) thick at Veil 0.6 costs 80 of reach: the target two cells past it
// is 165 away in budget terms and 117 as the crow flies.
func TestGround_AVeilDimsSightByItsDepth(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	cell := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	forest := forestColumn(grid, 0.6, false)
	target := mover{cell: cell(5, 7)}
	look := func(radius float64, blockers world.Layers) uint8 {
		observer := mover{cell: cell(1, 7), sight: &vision.Sight{Facing: east, HalfAngle: math.Pi / 8, Radius: radius, Blockers: blockers}}
		bw := newGroundWorld(t, grid, 6*cellSize, 16*cellSize, forest, []mover{observer, target})
		bw.tick()
		seen, ok := bw.seen()
		if !ok {
			t.Fatal("no observer")
		}
		return seen.Count
	}

	if n := look(160, 0); n != 0 {
		t.Errorf("at 160 through the forest saw %d, want nobody", n)
	}
	if n := look(170, 0); n != 1 {
		t.Errorf("at 170 through the forest saw %d, want the target", n)
	}
	if n := look(160, world.Layers(board.Air)); n != 1 {
		t.Errorf("at 160 looking over the forest from Air saw %d, want the target", n)
	}
}

// A Warcraft forest is solid and veiled at once: nobody walks in, sight is dimmed; cut down, it
// lets both through on the next tick.
func TestGround_AForestThatIsSolidAndVeiledStopsAndDimsUntilItIsCut(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	cell := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	observer := mover{cell: cell(1, 3), sight: &vision.Sight{Facing: east, HalfAngle: math.Pi / 16, Radius: 160}}
	bw := newGroundWorld(t, grid, 6*cellSize, 16*cellSize, forestColumn(grid, 0.6, true),
		[]mover{observer, {cell: cell(5, 3)}, {cell: cell(1, 9), heading: east}})
	walls := bw.solid(world.Layers(board.Land))
	for tick := range 60 {
		bw.tick()
		bw.assertClear(tick, bw.snapshot(), walls)
	}
	if seen, _ := bw.seen(); seen.Count != 0 {
		t.Errorf("saw %d through the forest at 160, want nobody", seen.Count)
	}

	for y := uint32(1); y <= 14; y++ {
		bw.brd.Res.Logic.Board.Set(cell(3, y), board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
	}
	for range 90 {
		bw.tick()
	}
	if seen, _ := bw.seen(); seen.Count != 1 {
		t.Errorf("saw %d across the cut forest, want the target", seen.Count)
	}
	if units := bw.snapshot(); units[2].TopLeft.X < float64(4*cellSize) {
		t.Errorf("the walker ends at %v, want it through where the forest stood", units[2])
	}
}

// The wall veils every layer, so it cuts sight whatever the Blockers.
func TestGround_AWallVeilingEveryLayerCutsSightFromEveryLayer(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	cell := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	for _, blockers := range []world.Layers{0, world.Layers(board.Land), world.Layers(board.Air)} {
		observer := mover{cell: cell(1, 7), sight: &vision.Sight{Facing: east, HalfAngle: math.Pi / 8, Radius: 300, Blockers: blockers}}
		bw, _ := squareWorld(t, observer, mover{cell: cell(5, 7)})
		bw.tick()
		if seen, ok := bw.seen(); !ok || seen.Count != 0 {
			t.Errorf("blockers %08b: saw %v through the wall, want nobody", blockers, seen.IDs[:seen.Count])
		}
	}
}

// A solid kind admitting Air keeps Land and Water out: it is solid for every layer but Air.
func TestGround_IsSolidForTheLayersItsKindKeepsOut(t *testing.T) {
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)
	cell := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	bw := newGroundWorld(t, grid, 6*cellSize, 16*cellSize, func(brd *board.Board) {
		brd.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
		brd.Set(cell(3, 3), board.CellKind{Name: board.Named("wall"), Cost: 1, Solid: true, Allows: board.Air})
		brd.Set(cell(3, 8), board.CellKind{Name: board.Named("rock"), Cost: 1, Solid: true})
	}, []mover{{cell: cell(1, 1)}})

	for _, c := range []struct {
		layers world.Layers
		want   int
	}{
		{world.Layers(board.Land), 2},
		{world.Layers(board.Water), 2},
		{world.Layers(board.Air), 1},
		{0, 2},
	} {
		if got := len(bw.solid(c.layers)); got != c.want {
			t.Errorf("layers %08b: %d solid cells, want %d", c.layers, got, c.want)
		}
	}
}

// A side of a solid cell is open where the neighbour is not solid for the entity: along a wall
// only its faces are open, so nothing is pushed along it.
func TestGround_OpensOnlyTheSidesFacingGroundTheEntityMayStandOn(t *testing.T) {
	bw, gap := squareWorld(t, mover{})
	w := bw.w.Res.Config.Space.Width
	var mid world.FieldBox
	bw.w.Field().Solid(world.Layers(board.Land), geom.NewAABBAt(geom.NewVec(0, 7*cellSize), float64(w), cellSize), func(fb world.FieldBox) bool {
		if fb.Cell == uint64(gap) {
			mid = fb
		}
		return true
	})
	if want := collide.Left | collide.Right; mid.Open != want {
		t.Errorf("the middle of the wall opens %04b, want only its left and right faces %04b", mid.Open, want)
	}
}

func TestGround_AVeiledHexCutsSightAcrossIt(t *testing.T) {
	grid := board.DefaultGrids{}.Hex(6, 3, cellSize)
	hex, _ := grid.CellIndex(2, 1)
	from, _ := grid.CellIndex(0, 1)
	to, _ := grid.CellIndex(4, 1)
	observer := mover{cell: from, sight: &vision.Sight{Facing: east, HalfAngle: math.Pi / 32, Radius: 300}}
	look := func(veil float64) uint8 {
		bw := newGroundWorld(t, grid, 400, 200, func(brd *board.Board) {
			brd.SetAll(board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land})
			brd.Set(hex, board.CellKind{Name: board.Named("forest"), Cost: 1, Allows: board.Land, Veil: veil})
		}, []mover{observer, {cell: to}})
		bw.tick()
		seen, _ := bw.seen()
		return seen.Count
	}
	if n := look(0); n != 1 {
		t.Fatalf("saw %d across open ground, want the target", n)
	}
	if n := look(1); n != 0 {
		t.Errorf("saw %d through a fully veiled hex, want nobody", n)
	}
	if n := look(0.2); n != 1 {
		t.Errorf("saw %d through a thin veil, want the target", n)
	}
}
