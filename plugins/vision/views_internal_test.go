package vision

import (
	"image/color"
	"math"
	"os"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// ridge is level ground with a ridge 30 high across x from 100 to 120.
type ridge struct{}

func (ridge) At(p geom.Vec) float64 {
	if p.X >= 100 && p.X < 120 {
		return 30
	}
	return 0
}
func (ridge) Step() float64   { return 4 }
func (ridge) Version() uint64 { return 7 }

// level draws the level ground over the world, 256 a side, white, its depth written.
var level = render.NewMeshShaderWith("test level", []byte(`
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    var corners = array<vec2<f32>, 6>(vec2<f32>(0.0, 0.0), vec2<f32>(256.0, 0.0), vec2<f32>(0.0, 256.0), vec2<f32>(256.0, 0.0), vec2<f32>(256.0, 256.0), vec2<f32>(0.0, 256.0));
    return U.ViewProj * vec4<f32>(corners[vid], 0.0, 1.0);
}
@fragment
fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(1.0); }
`), []render.Uniform{{Name: "ViewProj", Size: 16}})

// Laid on the GPU over the level ground drawn from above, a walker's view eastward over the ridge
// leaves the ground before the ridge clear, veils the ground behind it, strokes the cone's edge
// and leaves what lies outside the cone alone.
func TestViews_VeilTheGroundOutOfSightOnTheGPU(t *testing.T) {
	needGPU(t)
	cam := icamera.NewFromSpace(256, 256, 0)
	v := newViews()
	screen, depth := render.NewImage(256, 256), render.NewDepth()
	screen.Fill(color.Black)
	screen.ClearDepth(depth)
	f, _ := cam.(camera.Rays).Rays()
	tr, _ := camera.SceneTransform(f, 256, 256)
	screen.DrawMesh(nil, level, &render.DrawMeshOptions{Depth: depth, WriteDepth: true, Vertices: 6, Uniforms: map[string]any{"ViewProj": tr.M[:]}})
	v.look(cam, observer{X: 40, Y: 128, Eye: 2, Reach: 200, Half: math.Pi / 6})
	v.draw(render.Target{Screen: screen, Depth: depth}, cam, ridge{}, nil, 256, 256, 4, [2]bool{}, 0, DefaultShadow)
	pix := make([]byte, 4*256*256)
	screen.ReadPixels(pix)
	at := func(x, y int) int { i := 4 * (y*256 + x); return int(pix[i]) + int(pix[i+1]) + int(pix[i+2]) }
	if c := at(80, 128); c < 3*250 {
		t.Errorf("the ground in sight before the ridge is %d bright, want it clear", c)
	}
	if c := at(200, 128); c > 3*200 {
		t.Errorf("the ground behind the ridge is %d bright, want it veiled", c)
	}
	if c := at(200, 30); c < 3*250 {
		t.Errorf("the ground outside the cone is %d bright, want it left alone", c)
	}
	// the cone's upper edge, from (40, 128) at -30°: stroked somewhere across it
	stroked := false
	for d := -2; d <= 2; d++ {
		x, y := 40+int(60*math.Cos(math.Pi/6)), 128-int(60*math.Sin(math.Pi/6))+d
		if c := at(x, y); c < 3*245 {
			stroked = true
		}
	}
	if !stroked {
		t.Error("the cone's edge is not stroked")
	}
	if len(v.observers) != 0 {
		t.Error("the views drawn are kept for the next frame")
	}
}

// wall is a flat world's cover: a wall without end across x from 100 to 120.
type wall struct{}

func (wall) Walk(origin, dir geom.Vec, length float64, _ world.Layers, visit func(near, far, bottom, top, tau float64) bool) {
	if dir.X > 0 && origin.X <= 100 && origin.X+length >= 120 {
		visit(100-origin.X, 120-origin.X, math.Inf(-1), math.Inf(1), 0)
	}
}
func (wall) Version() uint64 { return 3 }

// Over a flat world, nothing drawn but the ground's colour and no depth, the views find the level
// ground along the camera's lines of sight: a wall without end hides what lies behind it, and on a
// wrapping world a view reaching over the seam is laid on past it.
func TestViews_OverAFlatWorldOnTheGPU(t *testing.T) {
	needGPU(t)
	cam := icamera.NewFromSpace(256, 256, aabbworld.Torus)
	v := newViews()
	screen, depth := render.NewImage(256, 256), render.NewDepth()
	screen.Fill(color.White)
	screen.ClearDepth(depth)
	v.look(cam, observer{X: 40, Y: 128, Eye: 2, Reach: 200, Half: math.Pi / 6})
	v.look(cam, observer{X: 240, Y: 40, Eye: 2, Reach: 60, Half: math.Pi / 6})
	v.draw(render.Target{Screen: screen, Depth: depth}, cam, nil, wall{}, 256, 256, 4, [2]bool{true, true}, 0, DefaultShadow)
	pix := make([]byte, 4*256*256)
	screen.ReadPixels(pix)
	at := func(x, y int) int { i := 4 * (y*256 + x); return int(pix[i]) + int(pix[i+1]) + int(pix[i+2]) }
	if c := at(80, 128); c < 3*250 {
		t.Errorf("the ground in sight before the wall is %d bright, want it clear", c)
	}
	if c := at(200, 128); c > 3*200 {
		t.Errorf("the ground behind the wall is %d bright, want it veiled", c)
	}
	// the second view's rim, 60 east of (240, 40): past the seam at 44
	stroked := false
	for x := 40; x <= 48; x++ {
		if c := at(x, 40); c < 3*245 {
			stroked = true
		}
	}
	if !stroked {
		t.Error("the view reaching over the seam is not laid on past it")
	}
}

// needGPU readies a device without a window; a machine without one skips.
func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}
