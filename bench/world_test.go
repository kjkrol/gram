package bench_test

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type mover struct{ x, y float64 }

func benchWorld(b *testing.B, ctx *headless, n int) *goke.ECS {
	return benchWorldViewed(b, ctx, n, 30, 0, nil)
}

// view 0: the whole world
func benchWorldViewed(b *testing.B, ctx *headless, n int, spacing int, view uint32, arrange func(*world.Plugin, kind.Of[mover])) *goke.ECS {
	b.Helper()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 4000, Height: 4000, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: n, MinSize: 1, MaxSize: 100},
	})
	kind.Define[mover](w.Kinds(), "mover", kind.Spec{
		comp.Load(func(m mover) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(m.x, m.y), 20, 20)}
		}),
		comp.Const(world.Velocity{Dir: geom.NewVec(1, 0), Value: 60}),
	})
	movers := kind.Named[mover](w.Kinds(), "mover")
	side := 1
	for side*side < n {
		side++
	}
	entries := make([]kind.Entry, 0, n)
	for i := range n {
		entries = append(entries, movers.Entry(mover{float64(10 + (i%side)*spacing), float64(10 + (i/side)*spacing)}))
	}
	w.Seed(entries...)
	ctx.cam = ctx.camera(cameras.TopDown(), float32(view), float32(view))
	w.ViewFor(ctx.cam) // its View kept from the first tick
	if arrange != nil {
		arrange(w, movers)
	}
	return ctx.start(b, w.RunPlan)
}

// Benchmark_World_Draw composes one frame of the entity renderer over 5000 boxes with the camera
// viewing all of the world, a quarter or a twentieth.
func Benchmark_World_Draw(b *testing.B) {
	for _, v := range []struct {
		name string
		view uint32
	}{{"view=100%", 4000}, {"view=25%", 2000}, {"view=5%", 900}} {
		b.Run(v.name, func(b *testing.B) {
			ctx := newHeadless()
			var r *render.Composer
			ecs := benchWorldViewed(b, ctx, 5000, 56, v.view, func(w *world.Plugin, movers kind.Of[mover]) {
				atlas := render.NewAtlas()
				atlas.Add(movers.SpriteID(), 20, render.Solid(color.RGBA{R: 90, G: 200, B: 110, A: 255}))
				atlas.Close()
				w.SetAtlas(atlas)
				r = render.NewComposer(w.Renderer())
				ctx.pending = append(ctx.pending, func() []goke.System {
					return []goke.System{goke.SystemFn{OnInit: r.Init}}
				})
			})
			ecs.Tick(step)
			cam := ctx.cam
			b.ReportAllocs()
			for b.Loop() {
				r.DrawWorld(nil, cam)
			}
		})
	}
}

// Benchmark_World_Tick is one tick of the world plugin alone.
func Benchmark_World_Tick(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		b.Run(entities(n), func(b *testing.B) {
			ecs := benchWorld(b, newHeadless(), n)
			b.ReportAllocs()
			for b.Loop() {
				ecs.Tick(step)
			}
		})
	}
}

// Benchmark_World_PositionScan reads every entity's Base through a goke query, the floor under
// any system that walks the population.
func Benchmark_World_PositionScan(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		b.Run(entities(n), func(b *testing.B) {
			var base goke.Comp[world.Base]
			var query *goke.Query
			ctx := newHeadless()
			ctx.pending = append(ctx.pending, func() []goke.System {
				return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) { query = si.NewQueryBuilder(&base).Build() }}}
			})
			benchWorld(b, ctx, n)
			var sink float64
			b.ReportAllocs()
			for b.Loop() {
				query.All()
				for query.Next() {
					for _, e := range base.Slice(query.Cursor()) {
						sink += e.Pos.TopLeft.X
					}
				}
			}
			_ = sink
		})
	}
}
