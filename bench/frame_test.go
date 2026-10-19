package bench_test

import (
	"testing"

	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// the screen the island's frames are drawn into: a 1440p display
const frameW, frameH = 2560, 1440

// Benchmark_Frame_Topography draws the island through the composer on a headless GPU into a
// 1440p screen, read back every frame so the GPU is waited for: in isometry and perspective,
// under a distant sun with the ground's shade baked, without the water's shine (no water
// layer), and at 0.7 of the pixels; "readback" alone is the cost of the wait, to take off the
// others.
func Benchmark_Frame_Topography(b *testing.B) {
	headlessGPU(b)
	for _, sc := range []struct {
		name, view string
		shine, lit bool
		w, h       float32
	}{
		{"iso", "iso", true, false, frameW, frameH},
		{"persp", "persp", true, false, frameW, frameH},
		{"iso,lit", "iso", true, true, frameW, frameH},
		{"iso,no-water", "iso", false, false, frameW, frameH},
		{"iso,0.7", "iso", true, false, frameW * 0.7, frameH * 0.7},
	} {
		b.Run(sc.name, func(b *testing.B) {
			ctx, _, _ := islandWith(b, sc.view, false, 0, sc.shine, sc.lit)
			cam := ctx.cam
			cam.SetViewport(sc.w, sc.h)
			cam.CenterOn(96*32/2, 64*32/2, 0)
			ctx.ecs.Tick(step)
			picture := ctx.world.Picture()
			screen := render.NewImage(int(sc.w), int(sc.h))
			buf := make([]byte, 4*int(sc.w)*int(sc.h))
			b.ReportAllocs()
			for b.Loop() {
				screen.Clear()
				picture.DrawWorld(screen, cam)
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
