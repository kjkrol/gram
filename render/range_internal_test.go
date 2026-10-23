package render

import (
	"image/color"
	"testing"

	"github.com/kjkrol/gram/internal/gpu/gputest"
)

// two triangles, vertices 0 to 2 over the left half of the target, 3 to 5 over the right
var halvesShader = NewMeshShaderWith("halves", []byte(`
struct Out { @builtin(position) clip: vec4<f32> }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 6>(
        vec2<f32>(-1.0, -3.0), vec2<f32>(-1.0, 3.0), vec2<f32>(0.0, 0.0),
        vec2<f32>(1.0, -3.0), vec2<f32>(1.0, 3.0), vec2<f32>(0.0, 0.0),
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

func TestImage_DrawMeshDrawsTheRangesOfItsIndicesAlone(t *testing.T) {
	gputest.Need(t)
	indices := NewIndices([]uint32{0, 1, 2, 3, 4, 5})
	defer indices.Release()
	for _, c := range []struct {
		ranges      []IndexRange
		left, right byte
	}{
		{[]IndexRange{{3, 3}}, 0, 255},
		{[]IndexRange{{0, 3}}, 255, 0},
		{[]IndexRange{{0, 3}, {3, 3}}, 255, 255},
		{nil, 255, 255},
		{[]IndexRange{{6, 3}}, 0, 0},
	} {
		img := NewImage(2, 1)
		img.Fill(color.RGBA{A: 255})
		img.DrawMesh(indices, halvesShader, &DrawMeshOptions{
			Ranges:   c.ranges,
			Uniforms: map[string]any{"Paint": []float32{1, 1, 1, 1}},
		})
		pix := make([]byte, 8)
		img.ReadPixels(pix)
		if pix[0] != c.left || pix[4] != c.right {
			t.Errorf("ranges %v: the left and right pixels are %d and %d, want %d and %d",
				c.ranges, pix[0], pix[4], c.left, c.right)
		}
	}
}
