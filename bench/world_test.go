package bench_test

import (
	"fmt"
	"image/color"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

type mover struct{ x, y float32 }

func benchWorld(b *testing.B, ctx *headless, n int) *goke.ECS {
	return benchWorldViewed(b, ctx, n, 30, 0, nil)
}

// view 0: the whole world
func benchWorldViewed(b *testing.B, ctx *headless, n int, spacing int, view uint32, arrange func(*world.Plugin, kind.Of[mover])) *goke.ECS {
	return benchWorldIn(b, ctx, world.Config{}, n, spacing, view, arrange)
}

// benchWorldIn is benchWorldViewed in a world of cfg's sectors and ladder
func benchWorldIn(b *testing.B, ctx *headless, cfg world.Config, n int, spacing int, view uint32,
	arrange func(*world.Plugin, kind.Of[mover])) *goke.ECS {
	b.Helper()
	cfg.Space = world.SpaceCfg{Width: 4000, Height: 4000, Edges: aabbworld.Torus}
	cfg.Entities = world.EntitiesCfg{MaxCount: n, MinSize: 1, MaxSize: 100}
	w := ctx.UseWorld(cfg)
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
		entries = append(entries, movers.Entry(mover{float32(10 + (i%side)*spacing), float32(10 + (i/side)*spacing)}))
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

// the worlds of n boxes in a grid spacing apart drawn through a camera view wide, or two side by
// side, named by how many boxes they hold in view: all of 5000, a quarter, a twentieth, a
// hundredth, a quarter and the one opposite; a twentieth and a hundredth of 50000
var worldViews = []struct {
	name       string
	n, spacing int
	view       uint32
	two        bool
}{{"5000 of 5000 in view", 5000, 56, 4000, false},
	{"1296 of 5000 in view", 5000, 56, 2000, false},
	{"256 of 5000 in view", 5000, 56, 900, false},
	{"49 of 5000 in view", 5000, 56, 400, false},
	{"1296 and 1190 of 5000 in view, 2 cameras", 5000, 56, 2000, true},
	{"2809 of 50000 in view", 50000, 17, 900, false},
	{"529 of 50000 in view", 50000, 17, 400, false}}

// worldDrawn is the world of n boxes spacing apart ticked once, its picture and the cameras it is
// drawn through
func worldDrawn(b *testing.B, n, spacing int, view uint32, two bool) (*headless,
	*render.Composer, []camera.Camera) {
	ctx := newHeadless()
	var r *render.Composer
	var cams []camera.Camera
	ecs := benchWorldViewed(b, ctx, n, spacing, view, func(w *world.Plugin, movers kind.Of[mover]) {
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
			cam.MoveTo(float32(view), float32(view))
			cams = append(cams, cam)
		}
	})
	ecs.Tick(step)
	return ctx, r, cams
}

// the worlds of worldViews cut into sectors of 1024, the camera's pixel a unit under the ladder's
// L3 at 4 and over its L4 at 0.5, or under its L4 at 2, keeping keep of what it draws at L4
var leveledViews = []struct {
	name       string
	n, spacing int
	view       uint32
	keep       float32
	level      lod.Level
}{{"5000 of 5000 in view, level=4", 5000, 56, 4000, 0, lod.L4},
	{"49 of 5000 in view, level=4", 5000, 56, 400, 0, lod.L4},
	{"5000 of 5000 in view, level=5", 5000, 56, 4000, 0, lod.L5},
	{"49 of 5000 in view, level=5", 5000, 56, 400, 0, lod.L5}}

// worldLeveled is the world of n boxes spacing apart cut into sectors and ticked once, its kind
// in the world's own atlas, its picture and the camera drawing it at level, L4 or L5, keeping
// keep at L4
func worldLeveled(b *testing.B, n, spacing int, view uint32, keep float32,
	level lod.Level) (*headless, *render.Composer, camera.Camera) {
	ctx := newHeadless()
	var r *render.Composer
	ladder := lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 8, 4, 0.5}}
	if level == lod.L5 {
		ladder.Scales[lod.L4] = 2
	}
	ladder.Keep[lod.L4] = keep
	cfg := world.Config{Sectors: world.SectorsCfg{Buckets: 4}, LOD: ladder}
	ecs := benchWorldIn(b, ctx, cfg, n, spacing, view, func(w *world.Plugin,
		movers kind.Of[mover]) {
		atlas := w.NewAtlas()
		atlas.Add(movers, 20, render.Solid(color.RGBA{R: 90, G: 200, B: 110, A: 255}))
		atlas.Close()
		ctx.look()
		r = w.Read(0).Picture().(*render.Composer)
		ctx.pending = append(ctx.pending, ctx.bound)
	})
	ecs.Tick(step)
	return ctx, r, ctx.cam
}

// Benchmark_World_Draw composes one frame of the entity renderer over the boxes published, as
// worldViews and leveledViews say.
func Benchmark_World_Draw(b *testing.B) {
	for _, v := range worldViews {
		b.Run(v.name, func(b *testing.B) {
			ctx, r, cams := worldDrawn(b, v.n, v.spacing, v.view, v.two)
			ctx.publish()
			b.ReportAllocs()
			for b.Loop() {
				for _, cam := range cams {
					r.DrawWorld(nil, cam)
				}
			}
		})
	}
	for _, v := range leveledViews {
		b.Run(v.name, func(b *testing.B) {
			ctx, r, cam := worldLeveled(b, v.n, v.spacing, v.view, v.keep, v.level)
			ctx.publish()
			b.ReportAllocs()
			for b.Loop() {
				r.DrawWorld(nil, cam)
			}
		})
	}
}

// Benchmark_World_Publish is the entity renderer's publishing of what Benchmark_World_Draw draws,
// and of its first world at L4 keeping a quarter.
func Benchmark_World_Publish(b *testing.B) {
	for _, v := range worldViews {
		b.Run(v.name, func(b *testing.B) {
			ctx, _, _ := worldDrawn(b, v.n, v.spacing, v.view, v.two)
			b.ReportAllocs()
			for b.Loop() {
				ctx.publish()
			}
		})
	}
	thinned := leveledViews[0]
	thinned.name, thinned.keep = thinned.name+", keep=0.25", 0.25
	for _, v := range append(leveledViews, thinned) {
		b.Run(v.name, func(b *testing.B) {
			ctx, _, _ := worldLeveled(b, v.n, v.spacing, v.view, v.keep, v.level)
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
			ctx, r, cams := worldDrawn(b, v.n, v.spacing, v.view, v.two)
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

// Benchmark_World_Sheet is a frame's photographs of what stands, the publishing, the composing and
// the GPU's work waited for: a sector of 64 things fixed, and 8 such sectors at once, each raised
// or lowered a frame so its sheet is photographed anew.
func Benchmark_World_Sheet(b *testing.B) {
	gputest.Need(b)
	for _, sc := range []struct {
		name     string
		clusters int
	}{{"sector,64", 1}, {"frame,8", 8}} {
		b.Run(sc.name, func(b *testing.B) {
			ctx, picture, q, z := sheetWorld(b, sc.clusters)
			albedo, _, _, _, _ := ctx.world.Read(0).Sheet()
			dst := render.NewImage(1, 1)
			px := make([]byte, 4)
			rise := float32(1)
			shoot := func() {
				for q.All(); q.Next(); {
					zs := z.Slice(q.Cursor())
					for i := range zs {
						zs[i].Altitude += rise
					}
				}
				rise = -rise
				ctx.publish()
				picture.DrawWorld(nil, ctx.cam)
				// read as a draw reads it, the sheet has the frame's photographs developed
				dst.CopyFrom(albedo)
				render.Submit()
				dst.ReadPixels(px)
			}
			shoot()
			b.ReportAllocs()
			for b.Loop() {
				shoot()
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
			var sink float32
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

// fixedRow is a thing fixed where it stands
type fixedRow struct{ x, y float32 }

// sheetWorld is a world of 4096 a side cut into sectors, a camera seeing all of it at L4 from
// above, and clusters of 64 things fixed, each 16 a side, 12 apart, in a sector of its own; its
// sheets photographed and the things' Z to change through z
func sheetWorld(b *testing.B, clusters int) (*headless, render.Picture, *goke.Query,
	*goke.Comp[world.Z]) {
	ctx := newHeadless()
	cfg := world.Config{
		Space:    world.SpaceCfg{Width: 4096, Height: 4096},
		Entities: world.EntitiesCfg{MaxCount: 64 * clusters, MinSize: 1, MaxSize: 32},
		Heights:  true,
		Sectors:  world.SectorsCfg{Buckets: 2},
		LOD:      lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 8, 4}},
	}
	w := ctx.UseWorld(cfg)
	w.Sheets()
	kind.Define[fixedRow](w.Kinds(), "fixed", kind.Spec{
		comp.Load(func(r fixedRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x, r.y), 16, 16)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Fixed{}),
		comp.Const(world.Z{Altitude: 10, Height: 30}),
	})
	fixed := kind.Named[fixedRow](w.Kinds(), "fixed")
	atlas := w.NewAtlas()
	atlas.Add(fixed, 16, render.Solid(color.RGBA{R: 200, G: 180, B: 90, A: 255}))
	atlas.Close()
	ctx.look()
	var entries []kind.Entry
	for k := range clusters {
		x0, y0 := float32(k%4)*1024+16, float32(k/4)*1024+16
		for i := range 64 {
			entries = append(entries, fixed.Entry(fixedRow{x0 + float32(i%8)*12,
				y0 + float32(i/8)*12}))
		}
	}
	w.Write().Seed(entries...)
	ctx.cam = ctx.camera(cameras.Config{}, cameras.FromAbove, 4096, 4096)
	var z goke.Comp[world.Z]
	var q *goke.Query
	ctx.pending = append(ctx.pending, ctx.bound, func() []goke.System {
		return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&z).Include(goke.Include[world.Fixed]()).Build()
		}}}
	})
	ecs := ctx.start(b, func(rc goke.RunCtx, d time.Duration) { w.RunPlan(rc, d) })
	ecs.Tick(step)
	return ctx, w.Read(0).Picture(), q, &z
}

// Benchmark_World_Preview is a field of 256 parts, never unfolded, previewed into the sheet of the
// sector it stands over, seen at L4: spawned where another went, told its Unfold, its parts
// photographed and none spawned, the sector published before and after. "declared" is the same
// round with the field's look declared at L4, nothing previewed: what the preview adds is the
// difference.
func Benchmark_World_Preview(b *testing.B) {
	for _, sc := range []struct {
		name     string
		declared bool
	}{{"field,256", false}, {"declared,256", true}} {
		b.Run(sc.name, func(b *testing.B) {
			pv := previewWorld(b, sc.declared)
			pv.again()
			b.ReportAllocs()
			for b.Loop() {
				pv.again()
			}
		})
	}
}

// fieldRow is a field, partRow a part of it
type (
	fieldRow struct{ x, y float32 }
	partRow  struct{}
)

// previewing is a world of a field and the entry of another like it
type previewing struct {
	ctx    *headless
	ecs    *goke.ECS
	fields *goke.Query
	entry  kind.Entry
}

// previewWorld is a world of 1024 a side with heights, cut into sectors 512 a side, a camera
// seeing all of it at L4 from above and a field 256 a side over the middle of the first sector,
// planting 16 by 16 parts 16 apart as it unfolds, its look declared at L4 where declared; its
// sheets photographed
func previewWorld(b *testing.B, declared bool) *previewing {
	ctx := newHeadless()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 1024, Height: 1024},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 1, MaxSize: 32},
		Heights:  true,
		Sectors:  world.SectorsCfg{Buckets: 2},
		LOD:      lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 8, 4}},
	})
	w.Sheets()
	w.Write().SetGround(func(geom.Vec) float32 { return 5 })
	kind.Define[partRow](w.Kinds(), "part", kind.Spec{
		comp.Load(func(partRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(-4, -4), 8, 8)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Fixed{}),
		comp.Const(world.Z{Height: 12}),
	})
	parts := kind.Named[partRow](w.Kinds(), "part")
	plant := make([]act.Rule, 0, 256)
	for k := range 256 {
		at := geom.NewVec(float32(k%16)*16-120, float32(k/16)*16-120)
		plant = append(plant, act.Branch[world.Unfold]{
			Name: fmt.Sprintf("plant %d", k),
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(world.SpawnPart{Entry: parts.Entry(partRow{}), At: at}),
		})
	}
	w.Roles().Define("planting", plant...)
	kind.Define[fieldRow](w.Kinds(), "field", kind.Spec{
		comp.Load(func(r fieldRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x-8, r.y-8), 16, 16)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Aggregate{}),
		comp.Const(world.Field{Size: geom.NewVec(256, 256)}),
		ceremony.Plays(w.Roles().Named("planting")),
	})
	fields := kind.Named[fieldRow](w.Kinds(), "field")
	atlas := w.NewAtlas()
	look := atlas.Add(fields, 16, render.Solid(color.RGBA{R: 40, G: 120, B: 50, A: 255}))
	if declared {
		look.At(lod.L4, render.Solid(color.RGBA{R: 30, G: 90, B: 40, A: 255}))
	}
	atlas.Add(parts, 8, render.Solid(color.RGBA{R: 50, G: 140, B: 60, A: 255}))
	atlas.Close()
	ctx.look()
	pv := &previewing{ctx: ctx, entry: fields.Entry(fieldRow{256, 256})}
	w.Write().Seed(pv.entry)
	if err := ctx.Deliver(w.Kinds().Played()...); err != nil {
		b.Fatal(err)
	}
	ctx.cam = ctx.camera(cameras.Config{}, cameras.FromAbove, 1024, 1024)
	var base goke.Comp[world.Base]
	ctx.pending = append(ctx.pending, ctx.bound, func() []goke.System {
		return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
			pv.fields = si.NewQueryBuilder(&base).Include(goke.Include[world.Field]()).Build()
		}}}
	})
	pv.ecs = ctx.start(b, func(rc goke.RunCtx, d time.Duration) { w.RunPlan(rc, d) })
	pv.ecs.Tick(step)
	return pv
}

// again has the field go and another come in its place, previewed into the sheet and published
func (pv *previewing) again() {
	w := pv.ctx.world
	for pv.fields.All(); pv.fields.Next(); {
		for _, id := range pv.fields.Cursor().IDs {
			w.Write().Carrier().PutFrom(id, world.Despawn{})
		}
	}
	w.Write().Spawn(pv.entry)
	// the field comes, the last goes; the sheet lets go of the last one's parts
	pv.ecs.Tick(step)
	pv.ecs.Tick(step)
	// the sector waits for the preview
	pv.ctx.publish()
	// the preview told its Unfold; its parts photographed and kept by the sheet
	pv.ecs.Tick(step)
	pv.ecs.Tick(step)
	pv.ctx.publish()
}
