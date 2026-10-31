package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
)

// a walker of its own size and weight, sent from the board's row 1 west to east
type limited struct{ size, weight float32 }

type barrierRow struct{ At cell.ID }

var (
	limitedGrid             = grid.DefaultGrids{}.Square(10, 7, 32)
	limitedFrom, limitedEnd = limitedGrid.At(1, 1), limitedGrid.At(8, 1)
)

// limitedWorld is a board of grass 10 cells by 7 of 32, with collision, the walker alone sent
// across it; lay stands what it builds on the cells as barrier rows of the kinds it defines
func limitedWorld(t *testing.T, lay func(brd *board.Plugin, w *world.Plugin), walker limited,
) (*goke.ECS, func() geom.Vec) {
	t.Helper()
	g := limitedGrid
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 320, Height: 224},
		Entities: world.EntitiesCfg{MaxCount: 32, MinSize: 2, MaxSize: 32},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	brd := board.NewPlugin(g, &cell.SingleOccupancy{}, w).WithMotion(m, cell.DefaultHiking).
		WithCollision(c)
	sel := selection.NewPlugin(w)
	drive := driving.NewPlugin(w, m, sel).WithGround(brd)
	nav := NewPlugin(brd, w, m, sel, drive).WithCollision(c).WithSpacing(CellSpacing)
	brd.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	lay(brd, w)
	units := board.NewUnits(brd, board.Shape{Size: walker.size, Height: 1.5},
		func(r walkerRow) geom.Vec { return r.At })
	units.Define("walker", unit.Mover{Domain: cell.Land, Weight: walker.weight},
		steering.Steering{MaxSpeed: 48, Accel: 96, Brake: 192, V0: 24, TurnRate: 0.15},
		comp.Load(func(r walkerRow) MoveOrder { return MoveOrder{Target: r.Target} }))
	w.Write().Seed(units.Named("walker").Entry(walkerRow{At: g.CellCenter(limitedFrom),
		Target: limitedEnd}))
	if err := w.Carry(nav); err != nil {
		t.Fatal(err)
	}
	ctx := &stubInstallCtx{ecs: goke.New()}
	for _, install := range []func() error{
		func() error { return w.Install(ctx) },
		func() error { return m.Install(ctx) },
		func() error { return c.Install(ctx) },
		func() error { return brd.Install(ctx) },
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
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		c.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		nav.RunPlan(rc, d)
		drive.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	})
	id, _, _, _ := w.Kinds().Lookup("walker")
	at := func() geom.Vec {
		for q.All(); q.Next(); {
			for _, b := range base.Slice(q.Cursor()) {
				if b.TypeID == id {
					return b.Pos.Center()
				}
			}
		}
		t.Fatal("the walker is gone")
		return geom.Vec{}
	}
	return ctx.ecs, at
}

// walked ticks ecs for seconds and reports whether the walker came to its target, and whether it
// stood in through on the way
func walked(ecs *goke.ECS, at func() geom.Vec, through cell.ID, seconds int) (arrived,
	crossed bool) {
	for range 60 * seconds {
		ecs.Tick(time.Second / 60)
		if c, ok := limitedGrid.CellAt(at()); ok && c == through {
			crossed = true
		}
	}
	goal, p := limitedGrid.CellCenter(limitedEnd), at()
	return f32.Hypot(p.X-goal.X, p.Y-goal.Y) < 4, crossed
}

// a wall down column 5 but its last row, a gap 16 wide in row 1: a small walker goes through it
// and a wide one round the wall's foot, collision letting each where its route leads
func TestNavigation_AGapLetsThroughWhomItFitsAndCollisionAgrees(t *testing.T) {
	g := limitedGrid
	gap := g.At(5, 1)
	walls := func(brd *board.Plugin, w *world.Plugin) {
		walls := board.NewEntities(brd, board.Shape{Size: 32},
			func(r barrierRow) cell.ID { return r.At })
		walls.Define("wall", comp.Const(cell.Wall{Keeps: cell.Land}))
		walls.Define("gap", comp.Const(cell.Wall{Keeps: cell.Land, Gap: cell.Gap{Width: 16}}))
		for y := range uint32(6) {
			name := "wall"
			if y == 1 {
				name = "gap"
			}
			w.Write().Seed(walls.Named(name).Entry(barrierRow{At: g.At(5, y)}))
		}
	}
	for _, tc := range []struct {
		size    float32
		through bool
	}{{12, true}, {16, true}, {22, false}} {
		ecs, at := limitedWorld(t, walls, limited{size: tc.size})
		arrived, crossed := walked(ecs, at, gap, 30)
		if !arrived {
			t.Errorf("%v across: stands at %v short of its target", tc.size, at())
		}
		if crossed != tc.through {
			t.Errorf("%v across: through the gap %v, want %v", tc.size, crossed, tc.through)
		}
	}
}

// a river down column 5, a rope over it in row 1 bearing 100 kg and a ford at its foot: a light
// walker crosses by the rope, a heavy one goes round by the ford
func TestNavigation_ARopeBearsTheLightAndTheHeavyGoRound(t *testing.T) {
	g := limitedGrid
	rope := g.At(5, 1)
	river := func(brd *board.Plugin, w *world.Plugin) {
		for y := range uint32(7) {
			brd.Write().Board().Set(g.At(5, y), cell.Kind{Pace: 1, Allows: cell.Water})
		}
		floors := board.NewEntities(brd, board.Shape{Size: 32},
			func(r barrierRow) cell.ID { return r.At })
		floors.Define("rope", comp.Const(cell.Floor{Allows: cell.Land, Load: 100}))
		floors.Define("ford", comp.Const(cell.Floor{Allows: cell.Land, Pace: 0.5}))
		w.Write().Seed(floors.Named("rope").Entry(barrierRow{At: rope}))
		w.Write().Seed(floors.Named("ford").Entry(barrierRow{At: g.At(5, 6)}))
	}
	for _, tc := range []struct {
		weight float32
		over   bool
	}{{0, true}, {80, true}, {1500, false}} {
		ecs, at := limitedWorld(t, river, limited{size: 12, weight: tc.weight})
		arrived, crossed := walked(ecs, at, rope, 30)
		if !arrived {
			t.Errorf("%v kg: stands at %v short of its target", tc.weight, at())
		}
		if crossed != tc.over {
			t.Errorf("%v kg: over the rope %v, want %v", tc.weight, crossed, tc.over)
		}
	}
}
