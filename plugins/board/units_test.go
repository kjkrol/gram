package board_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
)

type recruit struct{ start cell.ID }

func unitsWorld(t *testing.T, define func(units *board.Units[recruit]) kind.Of[recruit]) (*goke.ECS, *world.Plugin, grid.Grid) {
	t.Helper()
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 20, MaxSize: 20},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	brd := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w).
		WithMotion(m, cell.DefaultHiking).WithCollision(c)
	brd.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land | cell.Water})
	units := board.NewUnits[recruit](brd, board.Shape{Size: 20}, func(r recruit) geom.Vec { return grid.CellCenter(r.start) })
	k := define(units)

	ctx := boardtest.NewInstallCtx()
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	start := grid.At(2, 1)
	w.Write().Seed(k.Entry(recruit{start: start}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	systems := ctx.Systems()
	ctx.ECS().Setup(systems...)
	return ctx.ECS(), w, grid
}

func TestUnits_DeriveThePositionAndTheCellFromOnePoint(t *testing.T) {
	ecs, _, grid := unitsWorld(t, func(units *board.Units[recruit]) kind.Of[recruit] {
		units.Define("recruit", unit.Mover{Domain: cell.Water}, steering.Steering{MaxSpeed: 10})
		return units.Named("recruit")
	})
	var base goke.Comp[world.Base]
	var at goke.Comp[unit.At]
	var mover goke.Comp[unit.Mover]
	var layers goke.Comp[world.Layers]
	var steer goke.Comp[steering.Steering]
	var collider goke.Comp[collision.Collider]
	var q *goke.Query
	ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base, &at, &mover, &layers, &steer, &collider).Build()
	}})

	found := 0
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i := range cur.IDs {
			found++
			want := grid.At(2, 1)
			if at.Slice(cur)[i].Cell != want {
				t.Errorf("cell = %v, want the cell under the position, %v", at.Slice(cur)[i].Cell, want)
			}
			if c := base.Slice(cur)[i].Pos.Center(); c != grid.CellCenter(want) {
				t.Errorf("position centre = %v, want the cell's centre %v", c, grid.CellCenter(want))
			}
			if mover.Slice(cur)[i].Domain != cell.Water || layers.Slice(cur)[i] != world.Layers(cell.Water) {
				t.Errorf("mover %v, layers %08b, want both from Water", mover.Slice(cur)[i], layers.Slice(cur)[i])
			}
			if steer.Slice(cur)[i].MaxSpeed != 10 {
				t.Errorf("steering %+v, want the profile given", steer.Slice(cur)[i])
			}
		}
	}
	if found != 1 {
		t.Errorf("found %d units carrying every derived component and the collision defaults, want 1", found)
	}
}

func TestUnits_AUnitOffTheBoardPanicsWhenSpawned(t *testing.T) {
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "off the board") {
			t.Errorf("panic %q, want one about standing off the board", msg)
		}
	}()
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 20, MaxSize: 20},
		Heights:  true,
	})
	brd := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w).
		WithMotion(motion.NewPlugin(w), cell.DefaultHiking)
	units := board.NewUnits[recruit](brd, board.Shape{Size: 20}, func(recruit) geom.Vec { return geom.NewVec(-50, -50) })
	units.Define("stray", unit.Mover{Domain: cell.Land}, steering.Steering{})
	k := units.Named("stray")
	ctx := boardtest.NewInstallCtx()
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	w.Write().Seed(k.Entry(recruit{}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	systems := ctx.Systems()
	ctx.ECS().Setup(systems...) // Setup runs the queued spawn, where the At is read off the position
	t.Error("a unit off the board spawned")
}

func TestUnits_CastAsTheirKindSays(t *testing.T) {
	for _, c := range []struct {
		extra []comp.Comp
		want  []light.Casting
	}{
		{nil, nil}, // a Patch is the light plugin's default, which the board asks nothing of
		{[]comp.Comp{comp.Const(light.Shadow{Cast: light.Silhouette})},
			[]light.Casting{light.Silhouette}},
	} {
		ecs, _, _ := unitsWorld(t, func(units *board.Units[recruit]) kind.Of[recruit] {
			units.Define("recruit", unit.Mover{Domain: cell.Land}, steering.Steering{}, c.extra...)
			return units.Named("recruit")
		})
		var shadow goke.Comp[light.Shadow]
		var q *goke.Query
		ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&shadow).Build()
		}})
		var got []light.Casting
		for q.All(); q.Next(); {
			for _, s := range shadow.Slice(q.Cursor()) {
				got = append(got, s.Cast)
			}
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("a recruit given %d components casts %v, want %v", len(c.extra), got, c.want)
		}
	}
}

// the kinds of unit defined over the board are listed in the order defined, with what their units
// are made of: the box and the place in height a unit of each carries
func TestUnits_TheKindsOfUnitAreListedAsTheirUnitsAreMade(t *testing.T) {
	ecs, w, _ := unitsWorld(t, func(units *board.Units[recruit]) kind.Of[recruit] {
		units.Define("recruit", unit.Mover{Domain: cell.Land, Weight: 80}, steering.Steering{})
		units.Define("swimmer", unit.Mover{Domain: cell.Water}, steering.Steering{})
		return units.Named("recruit")
	})
	var brd *board.Plugin
	for _, p := range w.Made().All() {
		if b, ok := p.(*board.Plugin); ok {
			brd = b
		}
	}
	got := brd.UnitKinds()
	want := []board.UnitKind{
		{Name: "recruit", Mover: unit.Mover{Domain: cell.Land, Weight: 80},
			Shape: board.Shape{Size: 20}},
		{Name: "swimmer", Mover: unit.Mover{Domain: cell.Water}, Shape: board.Shape{Size: 20}},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("the kinds of unit %v, want %v", got, want)
	}
	var base goke.Comp[world.Base]
	var z goke.Comp[world.Z]
	var q *goke.Query
	ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base, &z).Build()
	}})
	found := 0
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i := range cur.IDs {
			found++
			size, place := base.Slice(cur)[i].Pos.Size, z.Slice(cur)[i]
			place.Altitude = 0
			if size != got[0].Shape.Box(geom.Vec{}).Size || place != got[0].Z() {
				t.Errorf("a recruit of %v in %+v, want its kind's box and place in height %+v",
					size, place, got[0].Z())
			}
		}
	}
	if found != 1 {
		t.Errorf("%d recruits found, want the one seeded", found)
	}
}

// a box whose footprint takes no more cells a side than the board's footprint is taken, one that
// takes more panics, naming the footprint and the room it holds a box with, whichever of its
// sides is the longer
func TestUnits_ABoxOfAFootprintPastTheBoardsPanics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cells   int // the board's footprint, its default where 0
		shape   board.Shape
		refused bool
	}{
		{"a cell", 0, board.Shape{Size: 32}, false},
		{"just past a cell, two cells a side", 0, board.Shape{Size: 32.5}, false},
		{"just under three cells, three a side", 0, board.Shape{Size: 95.5}, false},
		{"three cells, four a side", 0, board.Shape{Size: 96}, true},
		{"three cells across", 0, board.Shape{Size: 20, Width: 96}, true},
		{"a cell on a footprint of one", 1, board.Shape{Size: 32}, false},
		{"just past a cell on a footprint of one", 1, board.Shape{Size: 32.5}, true},
		{"just under two cells on a footprint of two", 2, board.Shape{Size: 63.9}, false},
		{"two cells on a footprint of two", 2, board.Shape{Size: 64}, true},
		{"three cells on a footprint of four", 4, board.Shape{Size: 96}, false},
		{"four cells across on a footprint of four", 4, board.Shape{Size: 20, Width: 128},
			true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := world.NewPlugin(world.Config{
				Space:    world.SpaceCfg{Width: 128, Height: 128},
				Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 20, MaxSize: 20},
				Heights:  true,
			})
			brd := board.NewPlugin(grid.DefaultGrids{}.Square(4, 4, 32),
				&cell.MultipleOccupancy{}, w).WithMotion(motion.NewPlugin(w), cell.DefaultHiking)
			if tc.cells > 0 {
				brd.WithFootprint(tc.cells)
			}
			defer func() {
				msg, _ := recover().(string)
				if refused := msg != ""; refused != tc.refused || refused &&
					(!strings.Contains(msg, "footprint") || !strings.Contains(msg, "spare")) {
					t.Errorf("panic %q; want one naming the footprint and its room %v", msg,
						tc.refused)
				}
			}()
			board.NewUnits(brd, tc.shape, func(recruit) geom.Vec { return geom.Vec{} })
		})
	}
}

// a unit of an even footprint standing about the corner its middle cells share is At the cell its
// footprint is laid from, the board keeping it there and holding it for the unit, a step after
// another
func TestUnits_AnEvenFootprintIsAtTheCellItIsLaidFrom(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, 32)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 192, Height: 192},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 20, MaxSize: 64},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	occ := &cell.SingleOccupancy{}
	brd := board.NewPlugin(g, occ, w).WithMotion(m, cell.DefaultHiking)
	brd.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	laid := g.At(2, 2)
	mid := g.CellCenter(laid)
	// past the corner rightwards, short of it downwards: the cell under it is laid's right one
	corner := geom.NewVec(mid.X+16.5, mid.Y+15.5)
	units := board.NewUnits[recruit](brd, board.Shape{Size: 60},
		func(recruit) geom.Vec { return corner })
	units.Define("giant", unit.Mover{Domain: cell.Land}, steering.Steering{})
	ctx := boardtest.NewInstallCtx()
	for _, install := range []func() error{
		func() error { return w.Install(ctx) },
		func() error { return m.Install(ctx) },
		func() error { return brd.Install(ctx) },
	} {
		if err := install(); err != nil {
			t.Fatal(err)
		}
	}
	w.Write().Seed(units.Named("giant").Entry(recruit{}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx.ECS().Setup(ctx.Systems()...)
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	})
	var at goke.Comp[unit.At]
	var q *goke.Query
	ctx.ECS().RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&at).Build()
	}})
	for step := range 3 {
		ctx.ECS().Tick(time.Second / 60)
		for q.All(); q.Next(); {
			for _, a := range at.Slice(q.Cursor()) {
				if a.Cell != laid || occ.CanEnter(laid, false, 99, cell.Land) {
					t.Errorf("step %d: At %v, its cell %v held %v; want At it and holding it",
						step, a.Cell, laid, !occ.CanEnter(laid, false, 99, cell.Land))
				}
			}
		}
	}
}
