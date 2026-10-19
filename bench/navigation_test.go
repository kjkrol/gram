package bench_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// the navigation scene: a 64x64 board of 16-px cells, the walkers seeded on its left half and
// sent across it, back again every navOrderGap ticks
const (
	navSide     = 64
	navCell     = 16
	navWallX    = 32 // the wall's column, crowded; its one gap at navGapY
	navGapY     = 32
	navOrderGap = 300
)

type navRow struct{ At cell.ID }

// navScene is a world of walkers under orders: in the open, each to the cell mirroring its own
// on the right half; crowded, all through the wall's gap to one cell behind it, where they
// press, step aside and plan again.
type navScene struct {
	ecs     *goke.ECS
	w       *world.Plugin
	g       grid.Grid
	query   *goke.Query
	base    goke.Comp[world.Base]
	ids     []uid.UID64
	from    []cell.ID
	crowded bool
}

func benchNavigation(b *testing.B, n int, crowded bool) *navScene {
	b.Helper()
	ctx := newHeadless()
	s := &navScene{crowded: crowded}
	s.w = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: navSide * navCell, Height: navSide * navCell},
		Entities: world.EntitiesCfg{MaxCount: n, MinSize: 1, MaxSize: navCell},
		Heights:  true,
	})
	s.g = grid.DefaultGrids{}.Square(navSide, navSide, navCell)
	m := motion.NewPlugin(s.w)
	if err := ctx.Use(m); err != nil {
		b.Fatal(err)
	}
	p := board.NewPlugin(s.g, &cell.SingleOccupancy{}, s.w).WithMotion(m)
	sel := selection.NewPlugin(s.w)
	drv := driving.NewPlugin(s.w, m, sel).WithGround(p)
	nav := navigation.NewPlugin(p, s.w, m, sel, drv)
	if err := ctx.Use(p); err != nil {
		b.Fatal(err)
	}
	if err := ctx.Use(sel); err != nil {
		b.Fatal(err)
	}
	if err := ctx.Use(drv); err != nil {
		b.Fatal(err)
	}
	if err := ctx.Use(nav); err != nil {
		b.Fatal(err)
	}
	if err := s.w.Carry(sel, drv, nav); err != nil {
		b.Fatal(err)
	}
	p.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	if crowded {
		for y := range uint32(navSide) {
			if y != navGapY {
				p.Board().Set(s.g.At(navWallX, y), cell.Kind{Cost: 1, Solid: true})
			}
		}
	}
	units := board.NewUnits(p, board.Shape{Size: 12}, func(r navRow) geom.Vec { return s.g.CellCenter(r.At) })
	units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{
		MaxSpeed: 4 * navCell, Accel: 8 * navCell, Brake: 16 * navCell, V0: 2 * navCell, TurnRate: 0.15,
	})
	var entries []kind.Entry
	for i := range n {
		entries = append(entries, units.Named("walker").Entry(navRow{
			At: s.g.At(uint32(2+i%26), uint32(2+(i/26)%60)),
		}))
	}
	s.w.Seed(entries...)
	ctx.pending = append(ctx.pending, func() []goke.System {
		return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
			s.query = si.NewQueryBuilder(&s.base).Build()
		}}}
	})
	s.ecs = ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		s.w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		p.RunPlan(rc, d)
		nav.RunPlan(rc, d)
		drv.RunPlan(rc, d)
		sel.RunPlan(rc, d)
	})
	s.ecs.Tick(step)
	for s.query.All(); s.query.Next(); {
		cur := s.query.Cursor()
		for i, b := range s.base.Slice(cur) {
			c, ok := s.g.CellAt(b.Pos.Center())
			if !ok {
				continue
			}
			s.ids = append(s.ids, cur.IDs[i])
			s.from = append(s.from, c)
		}
	}
	s.order(false)
	for range 60 {
		s.ecs.Tick(step)
	}
	return s
}

// order sends every walker across, or back where it came from.
func (s *navScene) order(back bool) {
	for i, id := range s.ids {
		var to cell.ID
		switch {
		case s.crowded && back:
			to = s.g.At(navWallX-6, navGapY)
		case s.crowded:
			to = s.g.At(navWallX+6, navGapY)
		case back:
			to = s.from[i]
		default:
			x, y, _ := s.g.Coords(s.from[i])
			to = s.g.At(navSide-1-x, y)
		}
		s.w.Carrier().PutFrom(id, navigation.MoveTo{Cell: to})
	}
}

// Benchmark_Navigation_Tick is one tick of the world, motion, board and navigation with N
// walkers under orders: in the open, nothing in their way; crowded, all through a wall's
// one-cell gap to one cell behind it. The orders turn them round every navOrderGap ticks.
func Benchmark_Navigation_Tick(b *testing.B) {
	for _, sc := range []struct {
		name    string
		crowded bool
	}{{"open", false}, {"crowded", true}} {
		for _, n := range []int{100, 1000} {
			b.Run(sc.name+","+entities(n), func(b *testing.B) {
				s := benchNavigation(b, n, sc.crowded)
				tick, back := 0, false
				b.ReportAllocs()
				for b.Loop() {
					if tick++; tick%navOrderGap == 0 {
						back = !back
						s.order(back)
					}
					s.ecs.Tick(step)
				}
			})
		}
	}
}
