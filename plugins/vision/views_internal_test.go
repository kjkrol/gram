package vision

import (
	"image/color"
	"math"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

type ridge struct{}

func (ridge) At(p geom.Vec) float64 {
	if p.X >= 100 && p.X < 120 {
		return 30
	}
	return 0
}
func (ridge) Step() float64   { return 4 }
func (ridge) Version() uint64 { return 7 }

var level = render.NewMeshShaderWith("test level", []byte(`
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    var corners = array<vec2<f32>, 6>(vec2<f32>(0.0, 0.0), vec2<f32>(256.0, 0.0), vec2<f32>(0.0, 256.0), vec2<f32>(256.0, 0.0), vec2<f32>(256.0, 256.0), vec2<f32>(0.0, 256.0));
    return U.ViewProj * vec4<f32>(corners[vid], 0.0, 1.0);
}
@fragment
fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(1.0); }
`), []render.Uniform{{Name: "ViewProj", Size: 16}})

func TestViews_VeilTheGroundOutOfSightOnTheGPU(t *testing.T) {
	gputest.Need(t)
	cam := cameratest.NewFromSpace(256, 256, 0)
	v := newViews()
	screen, depth, ground := render.NewImage(256, 256), render.NewDepth(), render.NewDepth().
		Readable()
	screen.Fill(color.Black)
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
	v.look(cam, observer{X: 40, Y: 128, Eye: 2, Reach: 200, Half: math.Pi / 6, Color: unitColor(DefaultCone.Color)})
	target := render.Target{Screen: screen, Depth: depth, Ground: ground}
	v.draw(target, cam, ridge{}, nil, 256, 256, 4, [2]bool{}, 0, DefaultShadow)
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

type wall struct{}

func (wall) Walk(
	origin, dir geom.Vec, length float64, _ world.Layers,
	visit func(near, far, bottom, top, tau float64) bool,
) {
	if dir.X > 0 && origin.X <= 100 && origin.X+length >= 120 {
		visit(100-origin.X, 120-origin.X, math.Inf(-1), math.Inf(1), 0)
	}
}
func (wall) Version() uint64 { return 3 }

func TestViews_OverAFlatWorldOnTheGPU(t *testing.T) {
	gputest.Need(t)
	cam := cameratest.NewFromSpace(256, 256, aabbworld.Torus)
	v := newViews()
	screen, depth := render.NewImage(256, 256), render.NewDepth()
	screen.Fill(color.White)
	screen.ClearDepth(depth)
	v.look(cam, observer{X: 40, Y: 128, Eye: 2, Reach: 200, Half: math.Pi / 6, Color: unitColor(DefaultCone.Color)})
	v.look(cam, observer{X: 240, Y: 40, Eye: 2, Reach: 60, Half: math.Pi / 6, Color: unitColor(DefaultCone.Color)})
	var walled coverCopy
	walled.update(nil, wall{}, 256, 256, 4, [2]bool{true, true})
	v.draw(render.Target{Screen: screen, Depth: depth}, cam, nil, &walled, 256, 256, 4,
		[2]bool{true, true}, 0, DefaultShadow)
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

func TestViews_AnEyeLookingUpVeilsNoGround(t *testing.T) {
	gputest.Need(t)
	cam := cameratest.NewFromSpace(256, 256, 0)
	v := newViews()
	screen, depth, ground := render.NewImage(256, 256), render.NewDepth(), render.NewDepth().
		Readable()
	screen.Fill(color.Black)
	screen.ClearDepth(ground)
	f, _ := cam.(camera.Rays).Rays()
	tr, _ := camera.SceneTransform(f, 256, 256)
	screen.DrawMesh(nil, level, &render.DrawMeshOptions{
		Depth: ground, WriteDepth: true, Vertices: 6, Uniforms: map[string]any{"ViewProj": tr.M[:]},
	})
	screen.CopyDepth(ground, depth)
	v.look(cam, observer{
		X: 40, Y: 128, Eye: 2, Reach: 200, Half: math.Pi / 6, Color: unitColor(DefaultCone.Color),
		Tilt: math.Pi / 3, VHalf: math.Pi / 8,
	})
	target := render.Target{Screen: screen, Depth: depth, Ground: ground}
	v.draw(target, cam, ridge{}, nil, 256, 256, 4, [2]bool{}, 0, DefaultShadow)
	pix := make([]byte, 4*256*256)
	screen.ReadPixels(pix)
	at := func(x, y int) int {
		i := 4 * (y*256 + x)
		return int(pix[i]) + int(pix[i+1]) + int(pix[i+2])
	}
	for _, x := range []int{80, 200} {
		if c := at(x, 128); c < 3*250 {
			t.Errorf("looking up, the ground at (%d, 128) is %d bright, want it left alone", x, c)
		}
	}
}

// counted is a cover over everything, τ tau, counting its walks
type counted struct {
	version uint64
	tau     float64
	walks   int
}

func (c *counted) Walk(
	_, _ geom.Vec, length float64, _ world.Layers,
	visit func(near, far, bottom, top, tau float64) bool,
) {
	c.walks++
	if c.tau > 0 {
		visit(0, length, 0, 10, c.tau)
	}
}
func (c *counted) Version() uint64 { return c.version }

func TestCoverCopy_IsWalkedAgainOnlyWhenTheCoverChanges(t *testing.T) {
	var c coverCopy
	cover := &counted{version: 1}
	wrap := [2]bool{true, true}
	c.update(nil, cover, 64, 64, 4, wrap)
	if c.ny == 0 || cover.walks != c.ny {
		t.Fatalf("%d walks over %d rows, want one a row", cover.walks, c.ny)
	}
	c.update(nil, cover, 64, 64, 4, wrap)
	if cover.walks != c.ny {
		t.Errorf("%d walks after a second publish of the same cover, want %d: the first kept",
			cover.walks, c.ny)
	}
	cover.version++
	c.update(nil, cover, 64, 64, 4, wrap)
	if cover.walks != 2*c.ny {
		t.Errorf("%d walks after the cover changed, want %d: walked anew", cover.walks, 2*c.ny)
	}
}

func TestViews_DrawTheCoverAsPublished(t *testing.T) {
	var c coverCopy
	cover := &counted{version: 1, tau: 0.5}
	wrap := [2]bool{true, true}
	c.update(nil, cover, 64, 64, 4, wrap)
	cover.version, cover.tau = 2, 0
	walks := cover.walks
	v := newViews()
	v.copy(nil, &c, 64, 64, 4, wrap)
	if cover.walks != walks {
		t.Errorf("drawing walked the cover %d times, want none: it draws the copy published",
			cover.walks-walks)
	}
	if got := v.pix[2]; got != 128 {
		t.Errorf("the ground's copy holds τ %d, want 128: the cover as it was published", got)
	}
	v.copy(nil, &c, 64, 64, 4, wrap)
	if got := v.pix[2]; got != 128 {
		t.Errorf("drawn again, the ground's copy holds τ %d, want 128", got)
	}
}
