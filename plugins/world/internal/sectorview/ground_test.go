package sectorview_test

import (
	"image/color"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world/internal/sectorview"
	"github.com/kjkrol/gram/render"
)

// level ground 256 wide and 192 down, black, its depth written; the sky below it
var level = render.NewMeshShaderWith("test level", []byte(`
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    var corners = array<vec2<f32>, 6>(
        vec2<f32>(0.0, 0.0), vec2<f32>(256.0, 0.0), vec2<f32>(0.0, 192.0),
        vec2<f32>(256.0, 0.0), vec2<f32>(256.0, 192.0), vec2<f32>(0.0, 192.0),
    );
    return U.ViewProj * vec4<f32>(corners[vid], 0.0, 1.0);
}
@fragment
fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(0.0, 0.0, 0.0, 1.0); }
`), []render.Uniform{{Name: "ViewProj", Size: 16}})

func TestGround_LaysEachSectorInItsColourOverTheGroundAlone(t *testing.T) {
	gputest.Need(t)
	cam := cameratest.NewFromSpace(256, 256, 0)
	screen := render.NewImage(256, 256)
	depth := render.NewDepth()
	ground := render.NewDepth().Readable()
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
	g := sectorview.New(sectorview.Grid{Side: 128, Cols: 2, Rows: 2, Width: 256, Height: 256})
	g.Paint(cam, []color.RGBA{
		{R: 255, A: 255},
		{G: 255, A: 255},
		{B: 255, A: 255},
		{},
	})
	g.Draw(render.Target{Screen: screen, Depth: depth, Ground: ground}, cam,
		sectorview.Line{Width: 2, Inside: 0.5, Edge: 1})
	pix := make([]byte, 4*256*256)
	screen.ReadPixels(pix)
	at := func(x, y int) [3]byte {
		k := 4 * (y*256 + x)
		return [3]byte{pix[k], pix[k+1], pix[k+2]}
	}
	for _, c := range []struct {
		x, y int
		want [3]byte
		what string
	}{
		{64, 64, [3]byte{128, 0, 0}, "inside the first sector, half its red"},
		{127, 64, [3]byte{255, 0, 0}, "on the first sector's edge, its red whole"},
		{192, 64, [3]byte{0, 128, 0}, "inside the second, half its green"},
		{64, 160, [3]byte{0, 0, 128}, "inside the third on the ground, half its blue"},
		{192, 160, [3]byte{0, 0, 0}, "inside the fourth, not painted"},
		{64, 230, [3]byte{0, 0, 0}, "the sky under the third"},
	} {
		got := at(c.x, c.y)
		for k := range got {
			if d := int(got[k]) - int(c.want[k]); d < -8 || d > 8 {
				t.Errorf("(%d, %d), %s: %v, want %v", c.x, c.y, c.what, got, c.want)
				break
			}
		}
	}
}

func TestGround_LaysTheColoursPaintedLast(t *testing.T) {
	gputest.Need(t)
	cam := cameratest.NewFromSpace(256, 256, 0)
	screen := render.NewImage(256, 256)
	depth := render.NewDepth()
	ground := render.NewDepth().Readable()
	f, _ := cam.(camera.Rays).Rays()
	tr, _ := camera.SceneTransform(f, 256, 256)
	g := sectorview.New(sectorview.Grid{Side: 128, Cols: 2, Rows: 2, Width: 256, Height: 256})
	pix := make([]byte, 4*256*256)
	for _, c := range []color.RGBA{{R: 255, A: 255}, {G: 255, A: 255}} {
		screen.Fill(color.RGBA{A: 255})
		screen.ClearDepth(ground)
		screen.DrawMesh(nil, level, &render.DrawMeshOptions{
			Depth:      ground,
			WriteDepth: true,
			Vertices:   6,
			Uniforms:   map[string]any{"ViewProj": tr.M[:]},
		})
		screen.CopyDepth(ground, depth)
		g.Paint(cam, []color.RGBA{c, c, c, c})
		g.Draw(render.Target{Screen: screen, Depth: depth, Ground: ground}, cam,
			sectorview.Line{Width: 2, Inside: 1, Edge: 1})
		screen.ReadPixels(pix)
		if got := [2]byte{pix[4*(64*256+64)], pix[4*(64*256+64)+1]}; got != [2]byte{c.R, c.G} {
			t.Errorf("painted %v, the ground shows red and green %v", c, got)
		}
	}
}
