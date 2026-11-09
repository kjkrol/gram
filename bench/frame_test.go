package bench_test

import (
	"image/color"
	"math/rand/v2"
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
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// the screen the island's frames are drawn into: a 1440p display
const frameW, frameH = 2560, 1440

// Benchmark_Frame_Topography draws the island through the composer on a headless GPU into a
// 1440p screen, read back every frame so the GPU is waited for: in isometry and perspective,
// under a distant sun with the ground's shade baked, without the water's shine (no water
// layer), at 0.7 of the pixels straight into a screen of that size, and cut into sectors of
// 256 every one drawn at L2, L3, L4 or L5 ("level="), in layers (the whole frame at 0.5 of the
// pixels, the central 60% at 0.75, the central 35% at all of them) inset or footed, and through
// a feed at 0.7 of the pixels scaled up to the screen; in layers and through the feed the
// figures at the ground's resolution ("feed", the control, and the layered ones) or at every
// pixel ("sharp"); "readback" alone is the cost of the wait, to take off the others.
func Benchmark_Frame_Topography(b *testing.B) {
	gputest.Need(b)
	layered := []camera.Layer{{Share: 1, Resolution: 0.5}, {Share: 0.6, Resolution: 0.75},
		{Share: 0.35, Resolution: 1}}
	const (
		inset  = camera.Inset
		footed = camera.Footed
	)
	for _, sc := range []struct {
		name, view string
		shine, lit bool
		w, h       float32
		level      lod.Level // the sectors' every one, Omitted for none
		layers     []camera.Layer
		layout     camera.Layout
		res        float32 // through a feed at this share of the pixels, where set
		sharp      bool
	}{
		{"iso", "iso", true, false, frameW, frameH, lod.Omitted, nil, inset, 0, false},
		{"persp", "persp", true, false, frameW, frameH, lod.Omitted, nil, inset, 0, false},
		{"iso,lit", "iso", true, true, frameW, frameH, lod.Omitted, nil, inset, 0, false},
		{"iso,no-water", "iso", false, false, frameW, frameH, lod.Omitted, nil, inset, 0, false},
		{"iso,0.7", "iso", true, false, frameW * 0.7, frameH * 0.7, lod.Omitted, nil, inset, 0, false},
		{"iso,level=2", "iso", true, false, frameW, frameH, lod.L2, nil, inset, 0, false},
		{"iso,level=3", "iso", true, false, frameW, frameH, lod.L3, nil, inset, 0, false},
		{"iso,level=4", "iso", true, false, frameW, frameH, lod.L4, nil, inset, 0, false},
		{"iso,level=5", "iso", true, false, frameW, frameH, lod.L5, nil, inset, 0, false},
		{"iso,layered", "iso", true, false, frameW, frameH, lod.Omitted, layered, inset, 0, false},
		{"persp,layered", "persp", true, false, frameW, frameH, lod.Omitted, layered, inset, 0, false},
		{"persp,footed", "persp", true, false, frameW, frameH, lod.Omitted, layered, footed, 0, false},
		{"iso,layered,sharp", "iso", true, false, frameW, frameH, lod.Omitted, layered, inset, 0,
			true},
		{"persp,footed,sharp", "persp", true, false, frameW, frameH, lod.Omitted, layered, footed, 0,
			true},
		{"iso,0.7,feed", "iso", true, false, frameW, frameH, lod.Omitted, nil, inset, 0.7, false},
		{"iso,0.7,sharp", "iso", true, false, frameW, frameH, lod.Omitted, nil, inset, 0.7, true},
		{"persp,0.7", "persp", true, false, frameW, frameH, lod.Omitted, nil, inset, 0.7, false},
		{"persp,0.7,sharp", "persp", true, false, frameW, frameH, lod.Omitted, nil, inset, 0.7, true},
	} {
		b.Run(sc.name, func(b *testing.B) {
			ctx, _, _ := islandWith(b, sc.view, false, sc.shine, sc.lit, sc.level)
			cam := ctx.cam
			cam.SetViewport(sc.w, sc.h)
			cam.CenterOn(96*32/2, 64*32/2, 0)
			ctx.ecs.Tick(step)
			ctx.publish()
			picture := ctx.world.Read(0).Picture()
			screen := render.NewImage(int(sc.w), int(sc.h))
			buf := make([]byte, 4*int(sc.w)*int(sc.h))
			draw := func() { picture.DrawWorld(screen, cam) }
			if sc.layers != nil || sc.res > 0 {
				cam.(camera.Layered).SetLayers(sc.layers, sc.layout)
				cam.(camera.Figured).SetSharpFigures(sc.sharp)
				if sc.res > 0 {
					cam.(camera.Resolved).SetResolution(sc.res)
				}
				feed := render.NewFeed(cam, picture)
				feed.Resize(frameW, frameH)
				draw = func() { feed.DrawOn(screen) }
			}
			// the camera's view is made at the first frame and sees its sectors from the next
			// tick on; the first frame paints the ground, waited for, as a game's first does
			draw()
			render.Submit()
			screen.ReadPixels(buf)
			ctx.ecs.Tick(step)
			ctx.publish()
			b.ReportAllocs()
			for b.Loop() {
				screen.Clear()
				draw()
				render.Submit()
				screen.ReadPixels(buf)
			}
		})
	}
	b.Run("iso,level=4,sheet", func(b *testing.B) {
		ctx, _, _ := islandWith(b, "iso", false, true, false, lod.L4, standingHalf)
		cam := ctx.cam
		cam.SetViewport(frameW, frameH)
		cam.CenterOn(96*32/2, 64*32/2, 0)
		ctx.ecs.Tick(step)
		ctx.publish()
		picture := ctx.world.Read(0).Picture()
		screen := render.NewImage(frameW, frameH)
		buf := make([]byte, 4*frameW*frameH)
		frame := func() {
			screen.Clear()
			picture.DrawWorld(screen, cam)
			render.Submit()
			screen.ReadPixels(buf)
		}
		// the view sees its sectors from the second tick on, sheetPerFrame of them photographed a
		// frame: the frames measured hold every one
		for range 12 {
			frame()
			ctx.ecs.Tick(step)
			ctx.publish()
		}
		b.ReportAllocs()
		for b.Loop() {
			frame()
		}
	})
	b.Run("readback", func(b *testing.B) {
		screen := render.NewImage(frameW, frameH)
		buf := make([]byte, 4*frameW*frameH)
		for b.Loop() {
			screen.Clear()
			render.Submit()
			screen.ReadPixels(buf)
		}
	})
}

type lanternRow struct{ x, y float32 }

// standingHalf has four lanterns stand fixed in every other sector of the island's, 256 a side,
// in a look of their own, so the sheets photograph half its sectors
func standingHalf(ctx *headless) {
	w := ctx.world
	kind.Define[lanternRow](w.Kinds(), "lantern", kind.Spec{
		comp.Load(func(r lanternRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x, r.y), 24, 24)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Fixed{}),
		comp.Const(world.Z{Altitude: 100, Height: 40}),
	})
	lanterns := kind.Named[lanternRow](w.Kinds(), "lantern")
	atlas := w.NewAtlas()
	atlas.Add(lanterns, 16, render.Solid(color.RGBA{R: 230, G: 200, B: 90, A: 255}))
	atlas.Close()
	var entries []kind.Entry
	for sy := range 8 {
		for sx := range 12 {
			if (sx+sy)%2 != 0 {
				continue
			}
			for k := range 4 {
				entries = append(entries, lanterns.Entry(lanternRow{
					float32(sx*256 + 64 + (k%2)*128), float32(sy*256 + 64 + (k/2)*128)}))
			}
		}
	}
	w.Write().Seed(entries...)
}

// Benchmark_Bake_Tile bakes anew a tile of the ground's shade, 256 pixels a side, as the set of
// 2000 posts standing within it changes, over level ground 1024 a side lit by a sun 45° high,
// waited for: the shadows of the posts near the tile found and drawn over the ground's.
func Benchmark_Bake_Tile(b *testing.B) {
	gputest.Need(b)
	rng := rand.New(rand.NewPCG(0x5eed, 0xc0ffee))
	tall := render.Block{X: 100, Y: 900, UX: 1, Along: 4, Wide: 4, Top: 40}
	// within tile (2, 2), their shadows too: 20 west of a post at most, and the penumbra
	posts := make([]render.Block, 2000)
	for i := range posts {
		posts[i] = render.Block{X: 560 + rng.Float32()*192, Y: 530 + rng.Float32()*220, UX: 1,
			Along: 2, Wide: 2, Top: 10 + rng.Float32()*10}
	}
	lights := &render.Lights{
		Sun:    render.Distant{Dir: [3]float32{0.7071, 0, 0.7071}, Strength: 1, Ambient: 0.3},
		Ground: render.Lattice{Cols: 129, Rows: 129, Cell: 8},
		Casts: []render.BlockSet{
			{Box: [4]float32{96, 896, 8, 8}, Top: 40, Version: 1, Blocks: []render.Block{tall}},
			{Box: [4]float32{558, 528, 196, 224}, Top: 20, Version: 1, Blocks: posts},
		},
	}
	c := render.NewComposer()
	c.Light(func() *render.Lights { return lights })
	cam := cameratest.NewFromSpace(1024, 1024, 0)
	screen := render.NewImage(8, 8)
	buf := make([]byte, 4*8*8)
	frame := func() {
		c.DrawWorld(screen, cam)
		render.Submit()
		screen.ReadPixels(buf)
	}
	// the whole baked first, as a game's first frame does
	frame()
	b.ReportAllocs()
	for b.Loop() {
		lights.Casts[1].Version++
		frame()
	}
}

// Benchmark_Light_Publish publishes the light over a world of 800 sectors, 64 a side, where
// 30 000 entities stand with their shadows Baked, nothing changed since the last publish: the
// blocks gathered into their sectors' sets and summed to find none changed.
func Benchmark_Light_Publish(b *testing.B) {
	const n = 30_000
	ctx := newHeadless()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 2560, Height: 1280},
		Entities: world.EntitiesCfg{MaxCount: n, MinSize: 4, MaxSize: 4},
		Sectors:  world.SectorsCfg{Buckets: 4},
		Heights:  true,
	})
	sun := light.Distant{Dir: [3]float32{-0.8, 0.45, 0.3}, Strength: 0.75, Ambient: 0.3}
	l := light.NewPlugin(w, light.Config{}).WithDistant(sun)
	if err := ctx.Use(l); err != nil {
		b.Fatal(err)
	}
	kind.Define[world.Position](w.Kinds(), "tree", kind.Spec{
		comp.Load(func(p world.Position) world.Position { return p }),
		comp.Const(world.Velocity{}),
		comp.Const(world.Z{Height: 12}),
		comp.Const(light.Shadow{Cast: light.Baked}),
	})
	trees := kind.Named[world.Position](w.Kinds(), "tree")
	placement := world.NewGridPlacement(2560, 1280, 4)
	entries := make([]kind.Entry, n)
	for i := range entries {
		entries[i] = trees.Entry(placement.Place(i, n))
	}
	w.Write().Seed(entries...)
	ecs := ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		l.RunPlan(rc, d)
	})
	ecs.Tick(step)
	ecs.Tick(step)
	if got := w.Read(0).SectorsLen(); got != 800 {
		b.Fatalf("the world is cut into %d sectors, want 800", got)
	}
	for range 2 {
		l.Publish()
		l.Swap()
	}
	if lit := w.Read(0).Lights(); lit == nil || lit.Ground.Cols == 0 {
		b.Fatal("nothing published to bake")
	}
	b.ReportAllocs()
	for b.Loop() {
		l.Publish()
		l.Swap()
	}
}
