package bench_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/motion"
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
	m := motion.NewPlugin(w)
	if err := ctx.Use(m); err != nil {
		b.Fatal(err)
	}
	side := 1
	for side*side < n {
		side++
	}
	entries := make([]kind.Entry, 0, n)
	for i := range n {
		entries = append(entries, movers.Entry(mover{float64(10 + (i%side)*spacing), float64(10 + (i/side)*spacing)}))
	}
	w.Write().Seed(entries...)
	ctx.cam = ctx.camera(cameras.Config{}, cameras.FromAbove, float32(view), float32(view))
	if arrange != nil {
		arrange(w, movers)
	}
	return ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		m.RunPlan(rc, d)
	})
}

// the cameras the world of 5000 boxes is drawn through: one viewing all of it, a quarter or a
// twentieth, or two viewing a quarter each, side by side
var worldViews = []struct {
	name string
	view uint32
	two  bool
}{{"view=100%", 4000, false}, {"view=25%", 2000, false}, {"view=5%", 900, false},
	{"view=25%,2 cameras", 2000, true}}

// worldDrawn is the world of 5000 boxes ticked once, its picture and the cameras it is drawn
// through
func worldDrawn(b *testing.B, view uint32, two bool) (*headless, *render.Composer,
	[]camera.Camera) {
	ctx := newHeadless()
	var r *render.Composer
	var cams []camera.Camera
	ecs := benchWorldViewed(b, ctx, 5000, 56, view, func(w *world.Plugin, movers kind.Of[mover]) {
		atlas := render.NewAtlas()
		atlas.Add(movers.SpriteID(), 20, render.Solid(color.RGBA{R: 90, G: 200, B: 110, A: 255}))
		atlas.Close()
		w.Write().SetAtlas(atlas)
		ctx.look()
		r = w.Read(0).Picture().(*render.Composer)
		ctx.pending = append(ctx.pending, ctx.bound)
		cams = append(cams, ctx.cam)
		if two {
			cam := ctx.camera(cameras.Config{}, cameras.FromAbove, float32(view), float32(view))
			cam.MoveTo(float64(view), float64(view))
			cams = append(cams, cam)
		}
	})
	ecs.Tick(step)
	return ctx, r, cams
}

// Benchmark_World_Draw composes one frame of the entity renderer over 5000 boxes, published, as
// worldViews says.
func Benchmark_World_Draw(b *testing.B) {
	for _, v := range worldViews {
		b.Run(v.name, func(b *testing.B) {
			ctx, r, cams := worldDrawn(b, v.view, v.two)
			ctx.publish()
			b.ReportAllocs()
			for b.Loop() {
				for _, cam := range cams {
					r.DrawWorld(nil, cam)
				}
			}
		})
	}
}

// Benchmark_World_Publish is the entity renderer's publishing of what Benchmark_World_Draw draws.
func Benchmark_World_Publish(b *testing.B) {
	for _, v := range worldViews {
		b.Run(v.name, func(b *testing.B) {
			ctx, _, _ := worldDrawn(b, v.view, v.two)
			b.ReportAllocs()
			for b.Loop() {
				ctx.publish()
			}
		})
	}
}

// Benchmark_World_Frame publishes and draws a frame of Benchmark_World_Draw.
func Benchmark_World_Frame(b *testing.B) {
	for _, v := range worldViews {
		b.Run(v.name, func(b *testing.B) {
			ctx, r, cams := worldDrawn(b, v.view, v.two)
			b.ReportAllocs()
			for b.Loop() {
				ctx.publish()
				for _, cam := range cams {
					r.DrawWorld(nil, cam)
				}
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
