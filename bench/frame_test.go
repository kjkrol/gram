package bench_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// the screen the island's frames are drawn into: a 1440p display
const frameW, frameH = 2560, 1440

// Benchmark_Frame_Topography draws the island through the composer on a headless GPU into a
// 1440p screen, read back every frame so the GPU is waited for: in isometry and perspective,
// under a distant sun with the ground's shade baked, without the water's shine (no water
// layer), at 0.7 of the pixels straight into a screen of that size, and cut into sectors of
// 256 shaded with full care everywhere, within two of the screen's middle and within one, in
// rings (the whole frame at 0.5 of the pixels, the central 60% at 0.75, the central 35% at all
// of them), and through a feed at 0.7 of the pixels scaled up to the screen, the figures at the
// ground's resolution ("feed", the control) or at every pixel ("sharp"); "readback" alone is
// the cost of the wait, to take off the others.
func Benchmark_Frame_Topography(b *testing.B) {
	headlessGPU(b)
	ringed := []camera.Ring{{Share: 1, Resolution: 0.5}, {Share: 0.6, Resolution: 0.75},
		{Share: 0.35, Resolution: 1}}
	for _, sc := range []struct {
		name, view string
		shine, lit bool
		w, h       float32
		sectored   bool
		care       int
		rings      []camera.Ring
		res        float32 // through a feed at this share of the pixels, where set
		sharp      bool
	}{
		{"iso", "iso", true, false, frameW, frameH, false, 0, nil, 0, false},
		{"persp", "persp", true, false, frameW, frameH, false, 0, nil, 0, false},
		{"iso,lit", "iso", true, true, frameW, frameH, false, 0, nil, 0, false},
		{"iso,no-water", "iso", false, false, frameW, frameH, false, 0, nil, 0, false},
		{"iso,0.7", "iso", true, false, frameW * 0.7, frameH * 0.7, false, 0, nil, 0, false},
		{"iso,care=all", "iso", true, false, frameW, frameH, true, 0, nil, 0, false},
		{"iso,care=2", "iso", true, false, frameW, frameH, true, 2, nil, 0, false},
		{"iso,care=1", "iso", true, false, frameW, frameH, true, 1, nil, 0, false},
		{"iso,ringed", "iso", true, false, frameW, frameH, false, 0, ringed, 0, false},
		{"persp,ringed", "persp", true, false, frameW, frameH, false, 0, ringed, 0, false},
		{"iso,0.7,feed", "iso", true, false, frameW, frameH, false, 0, nil, 0.7, false},
		{"iso,0.7,sharp", "iso", true, false, frameW, frameH, false, 0, nil, 0.7, true},
		{"persp,0.7", "persp", true, false, frameW, frameH, false, 0, nil, 0.7, false},
		{"persp,0.7,sharp", "persp", true, false, frameW, frameH, false, 0, nil, 0.7, true},
	} {
		b.Run(sc.name, func(b *testing.B) {
			ctx, _, _, topo := islandWith(b, sc.view, false, 0, sc.shine, sc.lit, sc.sectored)
			topo.SetCare(sc.care)
			cam := ctx.cam
			cam.SetViewport(sc.w, sc.h)
			cam.CenterOn(96*32/2, 64*32/2, 0)
			ctx.ecs.Tick(step)
			picture := ctx.world.Picture()
			screen := render.NewImage(int(sc.w), int(sc.h))
			buf := make([]byte, 4*int(sc.w)*int(sc.h))
			draw := func() { picture.DrawWorld(screen, cam) }
			if sc.rings != nil || sc.res > 0 {
				if sc.rings != nil {
					cam.(camera.Ringed).SetRings(sc.rings)
				} else {
					cam.(camera.Resolved).SetResolution(sc.res)
					cam.(camera.Figured).SetSharpFigures(sc.sharp)
				}
				feed := render.NewFeed(cam, picture)
				feed.Resize(frameW, frameH)
				draw = func() { feed.DrawOn(screen) }
			}
			// the camera's view is made at the first frame and sees its sectors from the next
			// tick on
			draw()
			ctx.ecs.Tick(step)
			b.ReportAllocs()
			for b.Loop() {
				screen.Clear()
				draw()
				gpu.Submit()
				screen.ReadPixels(buf)
			}
		})
	}
	b.Run("readback", func(b *testing.B) {
		screen := render.NewImage(frameW, frameH)
		buf := make([]byte, 4*frameW*frameH)
		for b.Loop() {
			screen.Clear()
			gpu.Submit()
			screen.ReadPixels(buf)
		}
	})
}
