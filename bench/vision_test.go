package bench_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

// watcher is the row an observer spawns from.
type watcher struct{ x, y float64 }

// benchVision installs n observers on a 120-unit lattice of a 4000x4000 world, each with a 60°
// cone of radius 200 facing right, and ticks only the vision plugin, scanning on workers
// goroutines at most (0: every CPU, 1: one).
func benchVision(b *testing.B, n int, outlines bool, workers int) *goke.ECS {
	b.Helper()
	ctx := newHeadless()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 4000, Height: 4000},
		Entities: world.EntitiesCfg{MaxCount: n, MinSize: 1, MaxSize: 100},
	})
	v := vision.NewPlugin(w).WithWorkers(workers)
	if err := ctx.Use(v); err != nil {
		b.Fatal(err)
	}
	spec := kind.Spec{
		comp.Load(func(d watcher) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(d.x, d.y), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(vision.Sight{Facing: geom.NewVec(1.0, 0.0), Radius: 200}), comp.Const(vision.Sighted{}),
		comp.Const(world.Eye{Angle: math.Pi / 3}),
	}
	if outlines {
		spec = append(spec, comp.Const(vision.SightOutline{}))
	}
	kind.Define[watcher](w.Kinds(), "watcher", spec)
	watchers := kind.Named[watcher](w.Kinds(), "watcher")
	side := int(math.Ceil(math.Sqrt(float64(n))))
	entries := make([]kind.Entry, 0, n)
	for i := range n {
		entries = append(entries, watchers.Entry(watcher{float64(100 + (i%side)*120), float64(100 + (i/side)*120)}))
	}
	w.Seed(entries...)
	return ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d) // the world's clock runs the scan's step
		v.RunPlan(rc, d)
		rc.Sync()
	})
}

func observers(n, workers int) string {
	if workers == 1 {
		return fmt.Sprintf("observers=%d,serial", n)
	}
	return fmt.Sprintf("observers=%d", n)
}

// Benchmark_Vision_Scan is one tick of the vision plugin: every observer's cone scanned against
// the shared space and its Seen filled — on every CPU at once, and (serial) on one goroutine.
func Benchmark_Vision_Scan(b *testing.B) {
	for _, workers := range []int{0, 1} {
		for _, n := range []int{100, 500} {
			b.Run(observers(n, workers), func(b *testing.B) {
				ecs := benchVision(b, n, false, workers)
				b.ReportAllocs()
				for b.Loop() {
					ecs.Tick(step)
				}
			})
		}
	}
}

// Benchmark_Vision_ScanWithOutlines is the same tick with every observer also carrying a
// SightOutline, so each view's shape is computed for drawing.
func Benchmark_Vision_ScanWithOutlines(b *testing.B) {
	for _, workers := range []int{0, 1} {
		for _, n := range []int{100, 500} {
			b.Run(observers(n, workers), func(b *testing.B) {
				ecs := benchVision(b, n, true, workers)
				b.ReportAllocs()
				for b.Loop() {
					ecs.Tick(step)
				}
			})
		}
	}
}
