package bench_test

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// mover is the row a moving 20x20 box spawns from.
type mover struct{ x, y float64 }

// benchWorld installs, on ctx, a world of n entities, 20x20 each on a 30-unit lattice, all
// drifting right at 60 units a second across a 4000x4000 torus, and returns its ECS ready to tick
// the world plugin alone.
func benchWorld(b *testing.B, ctx *headless, n int) *goke.ECS {
	return benchWorldViewed(b, ctx, n, 30, 0, nil)
}

// benchWorldViewed is benchWorld with the lattice spacing given, the camera's viewport set to
// view x view (0 for the whole world) and arrange run over the world and its kind before the
// Stage starts.
func benchWorldViewed(b *testing.B, ctx *headless, n int, spacing int, view uint32, arrange func(*world.Plugin, kind.Of[mover])) *goke.ECS {
	b.Helper()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 4000, Height: 4000, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: n, MinSize: 1, MaxSize: 100},
		Camera:   camera.Config{ViewportWidth: view, ViewportHeight: view},
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
	if arrange != nil {
		arrange(w, movers)
	}
	return ctx.start(b, w.RunPlan)
}

// Benchmark_World_Draw composes one frame of the entity renderer — no screen, nothing drawn — over
// 5000 boxes spread evenly across the world, with the camera viewing all of it, a quarter, or a
// twentieth (so a quarter, or a twentieth, of the boxes). The world has ticked once, so its View
// of the camera is filled; the frame only reads it.
func Benchmark_World_Draw(b *testing.B) {
	for _, v := range []struct {
		name string
		view uint32
	}{{"view=100%", 4000}, {"view=25%", 2000}, {"view=5%", 900}} {
		b.Run(v.name, func(b *testing.B) {
			ctx := newHeadless()
			var r *render.Composer
			var cam camera.Camera
			ecs := benchWorldViewed(b, ctx, 5000, 56, v.view, func(w *world.Plugin, movers kind.Of[mover]) {
				atlas := render.NewAtlas()
				atlas.Add(movers.SpriteID(), 20, render.Solid(color.RGBA{R: 90, G: 200, B: 110, A: 255}))
				atlas.Close()
				w.WithRenderer(atlas)
				r, cam = render.NewComposer(w.Renderer()), w.Camera()
				ctx.pending = append(ctx.pending, func() []goke.System {
					return []goke.System{goke.SystemFn{OnInit: r.Init}}
				})
			})
			ecs.Tick(step)
			b.ReportAllocs()
			for b.Loop() {
				r.DrawWorld(nil, cam)
			}
		})
	}
}

// Benchmark_World_Tick is one tick of the world plugin alone: steering, velocity, movement under
// the edge rules, and the space rebuilt from every entity.
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

// Benchmark_World_PositionScan reads every entity's Base through a goke query, chunk by chunk:
// the floor under any system that walks the population.
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
