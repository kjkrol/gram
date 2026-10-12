package navigation

import (
	"cmp"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

type walkerRow struct {
	At     geom.Vec
	Target cell.ID
	Level  Level
}

type spanRow struct {
	At   cell.ID
	Head bool
}

// valleyBridge is an 8 x 8 board of cells 32 wide in relief, 10 up but for a valley running
// south-west to north-east across the middle, bridged slantwise from a head on (1, 1) to a head on
// (5, 5), a deck 10 up, 12 wide, railed; a walker 3 wide at at ordered to target, kept apart by
// its body as the island's walkers are.
func valleyBridge(t *testing.T, at geom.Vec, target cell.ID, extra ...goke.System) (*goke.ECS,
	*goke.Query, *goke.Comp[world.Base], *world.Plugin) {
	return valleyBridgeTo(t, at, target, EitherLevel, extra...)
}

// valleyBridgeTo is valleyBridge, the walker ordered to the level of target.
func valleyBridgeTo(t *testing.T, at geom.Vec, target cell.ID, level Level,
	extra ...goke.System) (*goke.ECS, *goke.Query, *goke.Comp[world.Base], *world.Plugin) {
	t.Helper()
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	slantwise := bridgeSite{
		heights: func(p geom.Vec) float64 {
			if math.Abs((p.X+p.Y)/32-7) < 2 {
				return 0
			}
			return 10
		},
		from: g.At(1, 1),
		to:   g.At(5, 5),
	}
	return bridged(t, slantwise, at, target, level, extra...)
}

// bridgeSite is a board 8 x 8 of cells 32 wide in relief, its heights, the cells of it under
// water, and a bridge running straight from a head on from to a head on to, a deck 10 up; its
// walkers kept a cell each where cells says, and others besides the one bridged places.
type bridgeSite struct {
	heights  func(geom.Vec) float64
	wet      []cell.ID
	from, to cell.ID
	cells    bool
	others   []walkerRow
}

// bridged is the site, its bridge's deck 12 wide and railed, a walker 3 wide at at ordered to
// the level of target, kept apart by its body as the island's walkers are, unless the site keeps
// them a cell each.
func bridged(t *testing.T, site bridgeSite, at geom.Vec, target cell.ID, level Level,
	extra ...goke.System) (*goke.ECS, *goke.Query, *goke.Comp[world.Base], *world.Plugin) {
	t.Helper()
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 2, MaxSize: 32},
		Heights:  true,
	})
	c := collision.NewPlugin(w)
	var occupancy cell.Occupancy = &cell.MultipleOccupancy{}
	spacing := BodySpacing
	if site.cells {
		occupancy, spacing = &cell.SingleOccupancy{}, CellSpacing
	}
	brd := board.NewPlugin(g, occupancy, w).WithCollision(c)
	topo := topography.NewPlugin(w, brd, topography.Config{Cell: 32})
	sel := selection.NewPlugin(w)
	drive := driving.NewPlugin(w, sel).WithGround(brd)
	nav := NewPlugin(brd, w, sel, drive).WithCollision(c).WithSpacing(spacing)
	brd.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	for _, c := range site.wet {
		brd.Board().Set(c, cell.Kind{Cost: 1, Allows: cell.Water})
	}
	topo.Relief().SetHeights(site.heights)
	x0, y0, _ := g.Coords(site.from)
	x1, y1, _ := g.Coords(site.to)
	dx, dy := cmp.Compare(x1, x0), cmp.Compare(y1, y0)
	ahead, _ := grid.Direction(g, dx, dy)
	back := g.Opposite(ahead)
	spans := board.NewEntities(brd, board.Shape{Size: 32}, func(r spanRow) cell.ID { return r.At })
	spans.Define("bridge", comp.Load(func(r spanRow) cell.Floor {
		links := cell.Links(1<<ahead | 1<<back)
		switch {
		case r.Head && r.At == site.from:
			links = 1 << ahead
		case r.Head:
			links = 1 << back
		}
		return cell.Floor{
			Allows:    cell.Land,
			Height:    10 - topo.Relief().Altitude(r.At),
			Thickness: 0.5,
			Cost:      1,
			Graded:    true,
			Links:     links,
			Width:     12,
		}
	}), comp.Const(cell.Rail{Keeps: cell.Land, Height: 1}))
	for x, y := int(x0), int(y0); ; x, y = x+dx, y+dy {
		at := g.At(uint32(x), uint32(y))
		w.Seed(spans.Named("bridge").Entry(spanRow{At: at, Head: at == site.from || at == site.to}))
		if at == site.to {
			break
		}
	}
	units := board.NewUnits(brd, board.Shape{Size: 3, Height: 3},
		func(r walkerRow) geom.Vec { return r.At })
	units.Define("walker", unit.Mover{Domain: cell.Land},
		steering.Steering{MaxSpeed: 24, Accel: 48, Brake: 96, V0: 12, TurnRate: 0.15},
		comp.Load(func(r walkerRow) MoveOrder {
			return MoveOrder{Target: r.Target, Level: r.Level}
		}))
	w.Seed(units.Named("walker").Entry(walkerRow{At: at, Target: target, Level: level}))
	for _, r := range site.others {
		w.Seed(units.Named("walker").Entry(r))
	}
	if err := w.Carry(nav); err != nil {
		t.Fatal(err)
	}
	ctx := &stubInstallCtx{ecs: goke.New()}
	for _, install := range []func() error{
		func() error { return w.Install(ctx) },
		func() error { return c.Install(ctx) },
		func() error { return brd.Install(ctx) },
		func() error { return topo.Install(ctx) },
		func() error { return sel.Install(ctx) },
		func() error { return drive.Install(ctx) },
		func() error { return nav.Install(ctx) },
	} {
		if err := install(); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	var base goke.Comp[world.Base]
	var q *goke.Query
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base).Build()
	}})
	systems = append(systems, extra...)
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		c.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		topo.RunPlan(rc, d)
		nav.RunPlan(rc, d)
		drive.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	})
	return ctx.ecs, q, &base, w
}

// A walker standing anywhere by the way onto a slantwise bridge — on its head, beside the corner
// its deck cuts — crosses it to the other side.
func TestNavigation_ByASlantwiseBridgesCornerItCrossesWithoutCircling(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	for _, at := range []geom.Vec{
		geom.NewVec(68, 60), // beside the way from the head, near the corner
		geom.NewVec(60, 68),
		geom.NewVec(48, 80), // in the middles of the cells beside the corner
		geom.NewVec(80, 48),
		geom.NewVec(61, 61), // on the head by the corner
		geom.NewVec(30, 48), // off the head
		geom.NewVec(48, 20),
	} {
		ecs, q, base, _ := valleyBridge(t, at, g.At(6, 6))
		goal := g.CellCenter(g.At(6, 6))
		var c geom.Vec
		for range 60 * 20 {
			ecs.Tick(time.Second / 60)
		}
		for q.All(); q.Next(); {
			for _, b := range base.Slice(q.Cursor()) {
				if b.Pos.Size.X == 3 {
					c = b.Pos.Center()
				}
			}
		}
		if d := math.Hypot(c.X-goal.X, c.Y-goal.Y); d > 4 {
			t.Errorf("from %v: after 20 s the walker stands at %v, %.1f off the goal across",
				at, c, d)
		}
	}
}

// Sent to a cell the bridge runs over, a walker ends on the level asked for: on the deck, or down
// in the valley under it.
func TestNavigation_ToACellUnderABridgeOnTheLevelAsked(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	for level, want := range map[Level]float64{OnDeck: 10, OnGround: 0} {
		var z goke.Comp[world.Z]
		var base goke.Comp[world.Base]
		var q *goke.Query
		ecs, _, _, _ := valleyBridgeTo(t, geom.NewVec(48, 20), g.At(3, 3), level,
			goke.SystemFn{OnInit: func(si *goke.SysInit) {
				q = si.NewQueryBuilder(&base, &z).Build()
			}})
		for range 60 * 30 {
			ecs.Tick(time.Second / 60)
		}
		for q.All(); q.Next(); {
			for i, b := range base.Slice(q.Cursor()) {
				if b.Pos.Size.X != 3 {
					continue
				}
				if alt := z.Slice(q.Cursor())[i].Altitude; math.Abs(alt-want) > 0.5 {
					t.Errorf("sent to level %v, the walker stands %.1f up at %v, want %v", level,
						alt, b.Pos.Center(), want)
				}
				if c, ok := g.CellAt(b.Pos.Center()); !ok || c != g.At(3, 3) {
					t.Errorf("sent to level %v, the walker stands at %v, off its goal", level,
						b.Pos.Center())
				}
			}
		}
	}
}

// Standing on a bridge over a valley, a walker sent down under a cell of it goes off the bridge
// and comes in under the deck from beside it, ending on the valley's floor — under the cell it
// stands on, of a slantwise bridge and of a straight one whose heads lie flat at its deck; under
// the next, the way down by the near head under water, over the deck of its goal to the far one.
func TestNavigation_FromABridgeDownUnderIt(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	valley := func(p geom.Vec) float64 {
		if p.X <= 64 || p.X >= 160 {
			return 10
		}
		return 0
	}
	straight := bridgeSite{heights: valley, from: g.At(1, 3), to: g.At(5, 3)}
	moat := straight
	for _, c := range [][2]uint32{{0, 2}, {1, 2}, {2, 2}, {0, 3}, {0, 4}, {1, 4}, {2, 4}} {
		moat.wet = append(moat.wet, g.At(c[0], c[1]))
	}
	for name, tc := range map[string]struct {
		bridge      func(extra goke.System) (*goke.ECS, *world.Plugin)
		stand, goal cell.ID
	}{
		"slantwise": {func(extra goke.System) (*goke.ECS, *world.Plugin) {
			ecs, _, _, w := valleyBridgeTo(t, geom.NewVec(48, 48), g.At(3, 3), OnDeck, extra)
			return ecs, w
		}, g.At(3, 3), g.At(3, 3)},
		"straight": {func(extra goke.System) (*goke.ECS, *world.Plugin) {
			ecs, _, _, w := bridged(t, straight, geom.NewVec(48, 112), g.At(3, 3), OnDeck, extra)
			return ecs, w
		}, g.At(3, 3), g.At(3, 3)},
		"over its goal": {func(extra goke.System) (*goke.ECS, *world.Plugin) {
			ecs, _, _, w := bridged(t, moat, geom.NewVec(48, 112), g.At(2, 3), OnDeck, extra)
			return ecs, w
		}, g.At(2, 3), g.At(3, 3)},
	} {
		var z goke.Comp[world.Z]
		var base goke.Comp[world.Base]
		var q *goke.Query
		ecs, w := tc.bridge(goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&base, &z).Build()
		}})
		walker := func() (uid.UID64, geom.Vec, float64) {
			for q.All(); q.Next(); {
				for i, b := range base.Slice(q.Cursor()) {
					if b.Pos.Size.X == 3 {
						return q.Cursor().IDs[i], b.Pos.Center(), z.Slice(q.Cursor())[i].Altitude
					}
				}
			}
			t.Fatal("no walker")
			return 0, geom.Vec{}, 0
		}
		for range 60 * 20 {
			ecs.Tick(time.Second / 60)
		}
		id, at, alt := walker()
		if c, _ := g.CellAt(at); c != tc.stand || math.Abs(alt-10) > 0.5 {
			t.Fatalf("%s: the walker stands %.1f up at %v, want on the deck over %v", name, alt,
				at, tc.stand)
		}
		order := MoveTo{Cell: tc.goal, At: g.CellCenter(tc.goal), Level: OnGround}
		if !w.Carrier().PutFrom(id, order) {
			t.Fatal("nobody takes MoveTo")
		}
		for range 60 * 40 {
			ecs.Tick(time.Second / 60)
		}
		if _, at, alt = walker(); math.Abs(alt) > 0.5 {
			t.Errorf("%s: sent down under the bridge, the walker stands %.1f up at %v, want on "+
				"the valley's floor", name, alt, at)
		}
		if c, _ := g.CellAt(at); c != tc.goal {
			t.Errorf("%s: sent down under the bridge, the walker stands at %v, off its goal",
				name, at)
		}
	}
}

// Kept a cell each, a walker standing on a bridge's deck and one sent to the ground under it share
// the cell: each holds its own level of it.
func TestNavigation_KeptACellEachUnderAWalkerOnADeck(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	goal := g.At(3, 3)
	site := bridgeSite{
		heights: func(p geom.Vec) float64 {
			if p.X <= 64 || p.X >= 160 {
				return 10
			}
			return 0
		},
		from:   g.At(1, 3),
		to:     g.At(5, 3),
		cells:  true,
		others: []walkerRow{{At: g.CellCenter(g.At(3, 0)), Target: goal, Level: OnGround}},
	}
	var z goke.Comp[world.Z]
	var base goke.Comp[world.Base]
	var q *goke.Query
	ecs, _, _, _ := bridged(t, site, geom.NewVec(48, 112), goal, OnDeck,
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&base, &z).Build()
		}})
	for range 60 * 20 {
		ecs.Tick(time.Second / 60)
	}
	var alts []float64
	for q.All(); q.Next(); {
		for i, b := range base.Slice(q.Cursor()) {
			if b.Pos.Size.X != 3 {
				continue
			}
			alt := z.Slice(q.Cursor())[i].Altitude
			if c, _ := g.CellAt(b.Pos.Center()); c != goal {
				t.Errorf("a walker stands %.1f up at %v, off the goal", alt, b.Pos.Center())
			}
			alts = append(alts, alt)
		}
	}
	slices.Sort(alts)
	if len(alts) != 2 || math.Abs(alts[0]) > 0.5 || math.Abs(alts[1]-10) > 0.5 {
		t.Errorf("the walkers stand %v up, want one under the deck and one on it", alts)
	}
}

// A step along a deck through a corner passes over the corner's cells — entering one is no step
// off the way — but holds none of them: the deck carries the unit over.
func TestLeg_AlongADeckPassesOverItsCornersHoldingNone(t *testing.T) {
	leg := Leg{From: 1, To: 2, C1: 3, C2: 4, Diagonal: true, Decked: true, Active: true}
	if got := leg.cells(); len(got) != 4 {
		t.Errorf("a decked leg passes over %v, want its corners too", got)
	}
	if got := leg.held(); len(got) != 2 {
		t.Errorf("a decked leg holds %v, want From and To alone", got)
	}
	leg.Decked = false
	if got := leg.held(); len(got) != 4 {
		t.Errorf("a leg cutting a corner over the ground holds %v, want its corners too", got)
	}
}
