package bench_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

// a ground with no profile: sight samples it every step
type sampled struct{ ground.Heights }

// Benchmark_Vision_Relief is a tick of 37 observers on a board of rolling hills in relief, the
// ground sampled or read along its profile.
func Benchmark_Vision_Relief(b *testing.B) {
	for _, c := range []struct {
		name  string
		step  float64 // metres; 0, the relief's profile
		every float64
	}{
		{"step=50m", 50, 0},
		{"step=cell", 333, 0},
		{"profile", 0, 0},
		{"profile,15Hz", 0, 15},
	} {
		b.Run(c.name, func(b *testing.B) {
			const cols, rows, size = 192, 96, 32
			scale := world.Scale{Metres: 333.0 / size}
			ctx := newHeadless()
			w := ctx.UseWorld(world.Config{
				Scale:    scale,
				Space:    world.SpaceCfg{Width: cols * size, Height: rows * size},
				Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 1, MaxSize: size},
				Heights:  true,
			})
			g := grid.DefaultGrids{}.Square(cols, rows, size)
			p := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
			topo := topography.NewPlugin(w, p, topography.Config{Cell: size})
			v := vision.NewPlugin(w).WithBoard(p).WithFrequency(c.every)
			if c.step > 0 {
				v.WithGroundStep(scale.Units(c.step)).
					WithHeights(func() ground.Heights { return sampled{p.Heights()} })
			}
			if err := ctx.Use(p); err != nil {
				b.Fatal(err)
			}
			if err := ctx.Use(topo); err != nil {
				b.Fatal(err)
			}
			if err := ctx.Use(v); err != nil {
				b.Fatal(err)
			}
			p.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
			topo.Relief().SetHeights(func(q geom.Vec) float64 {
				return scale.Units(600 + 500*math.Sin(q.X/150)*math.Cos(q.Y/110))
			})
			units := board.NewUnits(p, board.Shape{Size: 3, Height: 9},
				func(r walkerRow) geom.Vec { return g.CellCenter(r.At) })
			units.Define("watcher", unit.Mover{Domain: cell.Land}, steering.Steering{},
				comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: 30 * size}),
				comp.Const(world.Eye{Angle: 72 * math.Pi / 180}),
				comp.Const(vision.Sighted{}))
			var entries []kind.Entry
			for i := range 37 {
				entries = append(entries, units.Named("watcher").Entry(walkerRow{
					At: g.At(uint32(20+(i%8)*20), uint32(10+(i/8)*18)),
				}))
			}
			w.Seed(entries...)
			ecs := ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
				w.RunPlan(rc, d)
				p.RunPlan(rc, d)
				v.RunPlan(rc, d)
				topo.RunPlan(rc, d)
			})
			ecs.Tick(step)
			for b.Loop() {
				ecs.Tick(step)
			}
		})
	}
}
