package bench_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// seamRow is a thing standing over a box: one of its own, a Whole's handle or a piece of one
type seamRow struct{ x, y, w, h float32 }

// halfScaled is a camera seeing the world west of middle at the scale west and the rest at east
type halfScaled struct {
	camera.Camera
	middle, west, east float32
}

func (c halfScaled) ScaleAt(x, _, _ float32) float32 {
	if x < c.middle {
		return c.west
	}
	return c.east
}

// observing is a world's one observer
type observing struct{ cam camera.Camera }

func (o observing) Observers(each func(camera.Camera)) { each(o.cam) }

// the seams' world: 4096 a side cut into 16 by 16 sectors 256 a side, so small is 64
const (
	seamsSide   = 4096
	seamsSector = 256
)

// seamsWorld is the seams' world with heights, a camera seeing all of it at L3 west of its middle
// and at L4 east of it, and 1000 things fixed across the sectors' seams: 800 of 40 a side, every
// other one 120 by 20 where long, and 200 pieces of 32 a side of 50 bridges, four in a row across
// a seam under their handle; its sheets photographed
func seamsWorld(b *testing.B, long bool) *headless {
	ctx := newHeadless()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: seamsSide, Height: seamsSide},
		Entities: world.EntitiesCfg{MaxCount: 1050, MinSize: 1, MaxSize: 128},
		Heights:  true,
		Sectors:  world.SectorsCfg{Buckets: 1},
		LOD:      lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 8, 4, 0.5}},
	})
	w.Sheets()
	spec := func(more ...comp.Comp) kind.Spec {
		return append(kind.Spec{
			comp.Load(func(r seamRow) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x, r.y), r.w, r.h)}
			}),
			comp.Const(world.Velocity{}),
			comp.Const(world.Fixed{}),
			comp.Const(world.Z{Altitude: 10, Height: 30}),
		}, more...)
	}
	kind.Define[seamRow](w.Kinds(), "standing", spec())
	kind.Define[seamRow](w.Kinds(), "bridge", spec(comp.Const(world.Whole{})))
	standing := kind.Named[seamRow](w.Kinds(), "standing")
	bridge := kind.Named[seamRow](w.Kinds(), "bridge")
	atlas := w.NewAtlas()
	atlas.Add(standing, 16, render.Solid(color.RGBA{R: 200, G: 180, B: 90, A: 255}))
	atlas.Close()
	ctx.look()
	seams := seamsSide/seamsSector - 1
	entries := make([]kind.Entry, 0, 800)
	for i := range 800 {
		x, y := float32(1+i%seams)*seamsSector, float32(40+i/seams*75)
		r := seamRow{x - 20, y - 20, 40, 40}
		if long && i%2 == 1 {
			r = seamRow{x - 60, y - 10, 120, 20}
		}
		entries = append(entries, standing.Entry(r))
	}
	w.Write().Seed(entries...)
	for h := range 50 {
		x, y := float32(1+h*7%seams)*seamsSector, float32(60+h*80)
		pieces := make([]kind.Entry, 0, 4)
		for k := range 4 {
			pieces = append(pieces, standing.Entry(seamRow{x - 64 + float32(k)*32, y - 16, 32, 32}))
		}
		w.Write().SeedWhole(bridge.Entry(seamRow{x - 64, y - 16, 128, 32}), pieces...)
	}
	cam := halfScaled{
		Camera: cameratest.NewFromSpace(seamsSide, seamsSide, 0),
		middle: seamsSide / 2,
		west:   5,
		east:   1,
	}
	w.Write().Observe(observing{cam})
	v := w.Write().NewView(cam)
	ctx.pending = append(ctx.pending, ctx.bound)
	ecs := ctx.start(b, func(rc goke.RunCtx, d time.Duration) { w.RunPlan(rc, d) })
	ecs.Tick(step)
	ecs.Tick(step)
	if west, east := v.Level(1000, 2000), v.Level(3000, 2000); west != lod.L3 || east != lod.L4 {
		b.Fatalf("the camera sees the west at %v, the east at %v; want L3, L4", west, east)
	}
	return ctx
}

// Benchmark_World_Seams is the entity renderer's publishing of the seams' world of small things and
// pieces, and with half the small things long: those by the seam of L3 and L4 drawn alive, the rest
// photographed. No frame develops the photographs, so every publish takes eight sectors anew.
func Benchmark_World_Seams(b *testing.B) {
	for _, sc := range []struct {
		name string
		long bool
	}{{"small", false}, {"long", true}} {
		b.Run(sc.name, func(b *testing.B) {
			ctx := seamsWorld(b, sc.long)
			ctx.publish()
			b.ReportAllocs()
			for b.Loop() {
				ctx.publish()
			}
		})
	}
}
