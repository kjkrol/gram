package routes

import (
	"image/color"
	"os"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// the level ground over the world, 256 a side, black, its depth written
var level = render.NewMeshShaderWith("test level", []byte(`
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    var corners = array<vec2<f32>, 6>(vec2<f32>(0.0, 0.0), vec2<f32>(256.0, 0.0), vec2<f32>(0.0, 256.0), vec2<f32>(256.0, 0.0), vec2<f32>(256.0, 256.0), vec2<f32>(0.0, 256.0));
    return U.ViewProj * vec4<f32>(corners[vid], 0.0, 1.0);
}
@fragment
fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(0.0, 0.0, 0.0, 1.0); }
`), []render.Uniform{{Name: "ViewProj", Size: 16}})

func TestRoutes_LayAStretchOverTheGround(t *testing.T) {
	needGPU(t)
	cam := cameratest.NewFromSpace(256, 256, 0)
	screen, depth, ground := render.NewImage(256, 256), render.NewDepth(), render.NewDepth().
		Readable()
	screen.ClearDepth(ground)
	f, _ := cam.(camera.Rays).Rays()
	tr, _ := camera.SceneTransform(f, 256, 256)
	screen.DrawMesh(nil, level, &render.DrawMeshOptions{
		Depth:      ground,
		WriteDepth: true,
		Vertices:   6,
		Uniforms:   map[string]any{"ViewProj": tr.M[:]},
	})
	screen.CopyDepth(ground, depth)
	g := New()
	flat := func(geom.Vec) float32 { return 0 }
	g.Add(cam, geom.NewVec(40, 100), geom.NewVec(200, 100), flat, 32, 3, color.RGBA{R: 255, A: 255})
	if len(g.stretches) != 16 {
		t.Fatalf("a stretch is %d floats, want 16: ends, rect, colour, width", len(g.stretches))
	}
	g.Draw(render.Target{Screen: screen, Depth: depth, Ground: ground}, cam)
	pix := make([]byte, 4*256*256)
	screen.ReadPixels(pix)
	red := func(x, y int) byte { return pix[4*(y*256+x)] }
	for _, x := range []int{60, 120, 190} {
		if v := red(x, 100); v < 200 {
			t.Errorf("on the stretch at (%d, 100) red is %d, want the line", x, v)
		}
	}
	for _, p := range [][2]int{{120, 104}, {120, 96}, {20, 100}, {220, 100}} {
		if v := red(p[0], p[1]); v > 10 {
			t.Errorf("off the stretch at %v red is %d, want none", p, v)
		}
	}
	if len(g.stretches) != 0 {
		t.Error("the stretches drawn are kept for the next frame")
	}
}

func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}
