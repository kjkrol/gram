package gfx

import (
	"image"
	"image/color"
	"testing"

	"github.com/kjkrol/gram/internal/gpu/gputest"
)

// a colour of its uniform Paint over all the target
var paintShader = NewMeshShaderWith("paint", []byte(`
struct Out { @builtin(position) clip: vec4<f32> }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(3.0, -1.0),
        vec2<f32>(-1.0, 3.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], 0.0, 1.0);
    return o;
}
@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> {
    return U.Paint;
}
`), []Uniform{{Name: "Paint", Size: 4}})

func pixelOf(t *testing.T, img *Image) [4]byte {
	t.Helper()
	pix := make([]byte, 4)
	img.ReadPixels(pix)
	return [4]byte(pix)
}

func TestImage_DrawMeshDarkestKeepsTheLesserOfEachChannel(t *testing.T) {
	gputest.Need(t)
	img := NewImage(1, 1)
	img.Fill(color.RGBA{R: 200, G: 40, B: 120, A: 255})
	img.DrawMesh(nil, paintShader, &DrawMeshOptions{
		Vertices: 3,
		Darkest:  true,
		Uniforms: map[string]any{"Paint": []float32{0.2, 0.8, 120.0 / 255, 0.5}},
	})
	got := pixelOf(t, img)
	want := [4]byte{51, 40, 120, 128}
	for k := range 4 {
		if d := int(got[k]) - int(want[k]); d > 1 || d < -1 {
			t.Fatalf("the darkest of (200, 40, 120, 255) and (51, 204, 120, 128) is %v, want %v",
				got, want)
		}
	}
}

func TestImage_CopyFromPutsAlphaAndAllInPlace(t *testing.T) {
	gputest.Need(t)
	src := NewImage(2, 1)
	src.WritePixels([]byte{10, 20, 30, 40, 200, 100, 50, 250})
	dst := NewImage(2, 1)
	dst.Fill(color.RGBA{R: 255, G: 255, B: 255, A: 255})
	dst.CopyFrom(src)
	pix := make([]byte, 8)
	dst.ReadPixels(pix)
	if want := []byte{10, 20, 30, 40, 200, 100, 50, 250}; string(pix) != string(want) {
		t.Errorf("copied %v, want the source's %v", pix, want)
	}
}

func TestImage_CopyFromCopiesAPartIntoAPart(t *testing.T) {
	gputest.Need(t)
	src := NewImage(1, 3)
	src.WritePixels([]byte{1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3})
	dst := NewImage(1, 3)
	dst.WritePixels([]byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9})
	dst.SubImage(image.Rect(0, 1, 1, 2)).CopyFrom(src.SubImage(image.Rect(0, 1, 1, 2)))
	pix := make([]byte, 12)
	dst.ReadPixels(pix)
	if want := []byte{9, 9, 9, 9, 2, 2, 2, 2, 9, 9, 9, 9}; string(pix) != string(want) {
		t.Errorf("the middle row copied into the middle row leaves %v, want %v", pix, want)
	}
}
