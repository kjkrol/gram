package bench_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
)

type walkerRow struct{ At cell.ID }

// Benchmark_Board_Altitude is walkers standing still over a 64x64 board of hills with a deck
// over the fourth row, their heights written every step.
func Benchmark_Board_Altitude(b *testing.B) { standing(b, false, false) }

// Benchmark_Gravity_Standing is Benchmark_Board_Altitude with the walkers under gravity.
func Benchmark_Gravity_Standing(b *testing.B) { standing(b, true, false) }

// Benchmark_Board_AltitudeBands is Benchmark_Board_Altitude with the deck a railed bridge.
func Benchmark_Board_AltitudeBands(b *testing.B) { standing(b, false, true) }

// Benchmark_Gravity_StandingBands is Benchmark_Gravity_Standing over a railed bridge.
func Benchmark_Gravity_StandingBands(b *testing.B) { standing(b, true, true) }

func standing(b *testing.B, bodies, bands bool) {
	const side, size = 64, 16
	for _, n := range []int{100, 1000} {
		b.Run(entities(n), func(b *testing.B) {
			ctx := newHeadless()
			ctx.UseWorld(world.Config{
				Space:    world.SpaceCfg{Width: side * size, Height: side * size},
				Entities: world.EntitiesCfg{MaxCount: n + side, MinSize: 1, MaxSize: size},
				Heights:  true,
			})
			g := grid.DefaultGrids{}.Square(side, side, size)
			m := motion.NewPlugin(ctx.world)
			if err := ctx.Use(m); err != nil {
				b.Fatal(err)
			}
			p := board.NewPlugin(g, &cell.MultipleOccupancy{}, ctx.world).WithMotion(m)
			topo := topography.NewPlugin(ctx.world, p, nil, topography.Config{})
			if err := ctx.Use(p); err != nil {
				b.Fatal(err)
			}
			if err := ctx.Use(topo); err != nil {
				b.Fatal(err)
			}
			var fall *gravity.Plugin
			var extra []comp.Comp
			if bodies {
				fall = gravity.NewPlugin(ctx.world, m, gravity.Config{}).WithSurfaces(p.Read(0).Footing)
				if err := ctx.Use(fall); err != nil {
					b.Fatal(err)
				}
				extra = append(extra, comp.Const(gravity.Body{}))
			}
			p.Write().Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
			topo.Write().Relief().SetHeights(ground.MeanOfCells(g, func(c cell.ID) float64 {
				if x, y, _ := g.Coords(c); (x/4+y/4)%2 == 0 {
					return 12
				}
				return 0
			}))
			at := func(r walkerRow) cell.ID { return r.At }
			units := board.NewUnits(p, board.Shape{Size: 2, Height: 3},
				func(r walkerRow) geom.Vec { return g.CellCenter(r.At) })
			units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 1},
				extra...)
			decks := board.NewEntities(p, board.Shape{Size: size}, at)
			deck := comp.Const(cell.Floor{Allows: cell.Land, Height: 20})
			if bands {
				west, _ := grid.Direction(g, -1, 0)
				east, _ := grid.Direction(g, 1, 0)
				deck = comp.Load(func(r walkerRow) cell.Floor {
					return cell.Floor{
						Allows: cell.Land,
						Height: 32 - topo.Write().Relief().Altitude(r.At),
						Links:  1<<west | 1<<east,
						Width:  size / 2,
					}
				})
			}
			decks.Define("deck", deck, comp.Const(cell.Rail{Keeps: cell.Land, Height: 1}))
			var entries []kind.Entry
			for i := range n {
				entries = append(entries, units.Named("walker").Entry(walkerRow{
					At: g.At(uint32(i%side), uint32(i/side%side)),
				}))
			}
			for x := range uint32(side) {
				entries = append(entries, decks.Named("deck").Entry(walkerRow{At: g.At(x, 3)}))
			}
			ctx.world.Write().Seed(entries...)
			ecs := ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
				ctx.world.RunPlan(rc, d)
				m.RunPlan(rc, d)
				if fall != nil {
					fall.RunPlan(rc, d)
				}
				p.RunPlan(rc, d)
				topo.RunPlan(rc, d)
			})
			ecs.Tick(step)
			for b.Loop() {
				ecs.Tick(step)
			}
		})
	}
}
