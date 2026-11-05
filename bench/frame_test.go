package bench_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
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
