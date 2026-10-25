package bench_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

type watcher struct{ x, y float32 }

// workers 0: every CPU
func benchVision(b *testing.B, n int, outlines bool, workers int) *goke.ECS {
	b.Helper()
	ctx := newHeadless()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 4000, Height: 4000},
		Entities: world.EntitiesCfg{MaxCount: n, MinSize: 1, MaxSize: 100},
	})
	m := motion.NewPlugin(w)
	if err := ctx.Use(m); err != nil {
		b.Fatal(err)
	}
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
	side := int(f32.Ceil(f32.Sqrt(float32(n))))
	entries := make([]kind.Entry, 0, n)
	for i := range n {
		entries = append(entries, watchers.Entry(watcher{float32(100 + (i%side)*120), float32(100 + (i/side)*120)}))
	}
	w.Write().Seed(entries...)
	return ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		m.RunPlan(rc, d)
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

// Benchmark_Vision_Scan is one tick of the vision plugin, on every CPU and (serial) on one.
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

// Benchmark_Vision_ScanWithOutlines is Benchmark_Vision_Scan with every observer carrying a
// SightOutline.
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
