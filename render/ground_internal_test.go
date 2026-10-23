package render

import (
	"image"
	"math"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
)

// the ground's height along a line of world points U.Step apart, as 16 bits in red and green over
// U.Most
var groundProbe = NewMeshShaderWith("ground probe", []byte(`
struct Out { @builtin(position) clip: vec4<f32>, @location(0) px: vec2<f32> }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(3.0, -1.0),
        vec2<f32>(-1.0, 3.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], 0.0, 1.0);
    o.px = vec2<f32>(corners[vid].x + 1.0, 1.0 - corners[vid].y) * 0.5 * U.Pixels;
    return o;
}
@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> {
    let h = groundHeightAt(vec2<f32>(floor(o.px.x) * U.Step, 13.0));
    if h == groundNone {
        return vec4<f32>(1.0, 1.0, 1.0, 0.0);
    }
    let v = clamp(h / U.Most, 0.0, 1.0) * 65535.0;
    return vec4<f32>(floor(v / 256.0) / 255.0, (v % 256.0) / 255.0, 0.0, 1.0);
}
`), []Uniform{{Name: "Pixels", Size: 2}, {Name: "Step", Size: 1}, {Name: "Most", Size: 1}})

type heighted struct{ g *GroundHeights }

func (heighted) Init(*goke.SysInit) {}
func (h heighted) Compose(f *Frame, _ camera.Camera) {
	if h.g != nil {
		f.GroundHeights(*h.g)
	}
}

func TestGroundHeights_ShadersReadTheGroundBlendedBetweenItsCorners(t *testing.T) {
	needGPU(t)
	// a ridge across x on a lattice 4 by 3, 10 world units a cell, its heights 100 up to 500,
	// laid from (2, 1) in a bigger picture
	heights := [4]float32{100, 500, 300, 100}
	pic := NewImage(8, 6)
	pix := make([]byte, 4*4*3)
	for y := range 3 {
		for x := range 4 {
			v := uint32((heights[x]-100)/400*65535 + 0.5)
			copy(pix[4*(y*4+x):], []byte{byte(v >> 8), byte(v), 0, 255})
		}
	}
	pic.SubImage(image.Rect(2, 1, 6, 4)).WritePixels(pix)
	g := &GroundHeights{Image: pic.SubImage(image.Rect(2, 1, 6, 4)), Corners: [2]int{4, 3}, Cell: 10,
		Low: 100, Span: 400}
	const w = 40
	read := func(g *GroundHeights) []byte {
		c := NewComposer(heighted{g})
		c.DrawWorld(nil, topDown())
		u := map[string]any{"Pixels": []float32{w, 1}, "Step": []float32{1}, "Most": []float32{600}}
		for name, v := range c.Uniforms() {
			u[name] = v
		}
		if img, ok := c.boxed["GroundHeights"]; ok {
			u["GroundHeights"] = img
		}
		screen := NewImage(w, 1)
		defer screen.Deallocate()
		screen.DrawMesh(nil, groundProbe, &DrawMeshOptions{Vertices: 3, Uniforms: u})
		out := make([]byte, 4*w)
		screen.ReadPixels(out)
		return out
	}
	out := read(g)
	for x := range w {
		f := float64(x) / 10
		i := min(int(f), 2)
		want := float64(heights[i]) + (f-float64(i))*float64(heights[i+1]-heights[i])
		if x >= 30 {
			want = float64(heights[3]) // past the last corner, the edge's
		}
		got := float64(uint32(out[4*x])<<8|uint32(out[4*x+1])) / 65535 * 600
		if math.Abs(got-want) > 0.5 {
			t.Errorf("at %d the ground stands %.1f up, want %.1f", x, got, want)
		}
	}
	if out := read(nil); out[3] != 0 {
		t.Errorf("a frame without the ground's heights reads %v, want none", out[:4])
	}
}
