package aloft

import (
	"testing"

	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// a white cloud in the pixel U.ProbeAt of the image drawn on, lying U.ProbeDepth deep, none in
// the others; with U.ProbeAt left of the image, the whole of it that deep and no colour: a ground
var layProbe = render.NewMeshShaderWith("lay probe", []byte(`
struct Out { @builtin(position) clip: vec4<f32>, @location(0) px: vec2<f32> }
struct Seen { @location(0) color: vec4<f32>, @builtin(frag_depth) depth: f32 }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(3.0, -1.0),
        vec2<f32>(-1.0, 3.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], 0.0, 1.0);
    o.px = vec2<f32>(corners[vid].x + 1.0, 1.0 - corners[vid].y) * 0.5 * U.ProbeSize;
    return o;
}
@fragment
fn fs_main(o: Out) -> Seen {
    if U.ProbeAt.x < 0.0 {
        return Seen(vec4<f32>(0.0), U.ProbeDepth);
    }
    let at = floor(o.px);
    if at.x == U.ProbeAt.x && at.y == U.ProbeAt.y {
        return Seen(vec4<f32>(1.0), U.ProbeDepth);
    }
    return Seen(vec4<f32>(0.0), 0.0);
}
`), []render.Uniform{{Name: "ProbeSize", Size: 2}, {Name: "ProbeAt", Size: 2},
	{Name: "ProbeDepth", Size: 1}})

// laid lays a cloud in the pixel (2, 2) of an image 6 by 6, half-way deep, over a screen 24 by 24
// whose ground lies ground deep, and gives the screen's pixels
func laid(ground float32) []byte {
	const coarse, by = 6, 4
	cloud, depth := render.NewImage(coarse, coarse), render.NewDepth().Readable()
	defer cloud.Deallocate()
	defer depth.Release()
	cloud.DrawMesh(nil, layProbe, &render.DrawMeshOptions{Vertices: 3, Depth: depth,
		ClearDepth: true, WriteDepth: true, Uniforms: map[string]any{
			"ProbeSize":  []float32{coarse, coarse},
			"ProbeAt":    []float32{2, 2},
			"ProbeDepth": []float32{0.5},
		}})
	screen, under := render.NewImage(coarse*by, coarse*by), render.NewDepth()
	defer screen.Deallocate()
	defer under.Release()
	screen.DrawMesh(nil, layProbe, &render.DrawMeshOptions{Vertices: 3, Depth: under,
		ClearDepth: true, WriteDepth: true, Uniforms: map[string]any{
			"ProbeSize":  []float32{coarse * by, coarse * by},
			"ProbeAt":    []float32{-1, -1},
			"ProbeDepth": []float32{ground},
		}})
	opts := render.DrawMeshOptions{Vertices: 3, Depth: under, ReadDepth: depth,
		Uniforms: map[string]any{
			"LayView":   []float32{coarse * by, coarse * by},
			"LayCoarse": []float32{by, by},
		}}
	opts.Images[0] = cloud
	screen.DrawMesh(nil, layShader, &opts)
	pix := make([]byte, 4*coarse*by*coarse*by)
	screen.ReadPixels(pix)
	return pix
}

func TestLay_ACloudsEdgeFadesOverThePixelsBesideItUncut(t *testing.T) {
	gputest.Need(t)
	pix := laid(0.2) // the ground behind the cloud
	at := func(x, y int) int { return int(pix[4*(y*24+x)+3]) }
	// along the row through the cloud's middle, from inside it out past its east edge at x 12:
	// it fades evenly, no step where its own coarse pixel ends
	in, edge, past, far := at(10, 10), at(12, 10), at(14, 10), at(19, 10)
	if in < 50 || edge < in/2 || edge > in || past < in/8 || past >= edge || far != 0 {
		t.Errorf("a cloud laid over a ground behind it shows %d inside, %d at its coarse pixel's "+
			"edge, %d two pixels past it and %d far off; want it fading evenly to nothing", in,
			edge, past, far)
	}
	for y := range 24 {
		for x := 1; x < 24; x++ {
			if d := at(x, y) - at(x-1, y); d > in/3 || d < -in/3 {
				t.Fatalf("the cloud steps by %d between the pixels %d and %d of row %d; want no "+
					"edge of a coarse pixel showing", d, x-1, x, y)
			}
		}
	}
}

func TestLay_AGroundNearerThanTheCloudHidesAllOfIt(t *testing.T) {
	gputest.Need(t)
	for i, a := range laid(0.8) {
		if i%4 == 3 && a != 0 {
			t.Fatalf("a cloud behind the ground shows %d at pixel %d, want it hidden", a, i/4)
		}
	}
}
