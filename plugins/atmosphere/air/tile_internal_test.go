package air

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/render"
)

// the clouds' tile read along a line of world points a unit apart: shreds in red, cores in green
var tileProbe = render.NewMeshShaderWith("tile probe", []byte(`
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
    let p = U.From + vec2<f32>(floor(o.px.x), 0.0);
    let torn = cloudTileAt(p, U.Level, cloudTile * cloudSize).r;
    let core = cloudTileAt(p, U.Level, heapTile * heapSize).g;
    return vec4<f32>(torn, core, 0.0, 1.0);
}
`), []render.Uniform{{Name: "Pixels", Size: 2}, {Name: "From", Size: 2}, {Name: "Level", Size: 1}})

// the heaps' cores rise steeply at their edges, a pixel of the tile 49 world units wide: a wrap
// read without its neighbour showed as a seam across the clouds
func TestTile_ReadsOnAcrossItsWrap(t *testing.T) {
	needGPU(t)
	const w = 400
	for _, c := range []struct {
		from, level float32
	}{{CloudTile - w/2, 0}, {CloudTile - w/2, 1.5}, {HeapTile - w/2, 0}, {HeapTile - w/2, 2}} {
		screen := render.NewImage(w, 1)
		u := map[string]any{
			"Pixels":    []float32{w, 1},
			"From":      []float32{c.from, 333},
			"Level":     []float32{c.level},
			"CloudTile": Tile(),
		}
		screen.DrawMesh(nil, tileProbe, &render.DrawMeshOptions{Vertices: 3, Uniforms: u})
		pix := make([]byte, 4*w)
		screen.ReadPixels(pix)
		screen.Deallocate()
		for k, name := range []string{"shreds", "cores"} {
			// across the wrap no steeper than the noise anywhere else on the line
			steep := 0.0
			for x := 1; x < w; x++ {
				if x < w/2-8 || x > w/2+8 {
					steep = max(steep, math.Abs(float64(pix[4*x+k])-float64(pix[4*(x-1)+k])))
				}
			}
			for x := w/2 - 8; x <= w/2+8; x++ {
				if d := math.Abs(float64(pix[4*x+k]) - float64(pix[4*(x-1)+k])); d > steep+2 {
					t.Errorf("from %v on level %v the %s step %v at %d units on, want no jump "+
						"across the wrap at %d over the %v elsewhere", c.from, c.level, name, d, x,
						w/2, steep)
				}
			}
		}
	}
}
