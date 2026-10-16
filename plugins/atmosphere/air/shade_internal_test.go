package air

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/render"
)

// the clouds' shadow along a line of world points U.Step apart, the ground U.Ground up
var shadeProbe = render.NewMeshShaderWith("shade probe", []byte(`
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
    let p = U.From + vec2<f32>(floor(o.px.x) * U.Step, 0.0);
    return vec4<f32>(cloudsShadeAt(vec3<f32>(p, U.Ground), U.Step), 0.0, 0.0, 1.0);
}
`), []render.Uniform{{Name: "Pixels", Size: 2}, {Name: "From", Size: 2}, {Name: "Step", Size: 1},
	{Name: "Ground", Size: 1}})

func TestShade_TheShadersShadeAsTheCPUDoes(t *testing.T) {
	needGPU(t)
	const w = 2000
	sun := [3]float32{0.48, -0.36, 0.8}
	for _, c := range []struct {
		weather Weather
		ground  float32
	}{
		{Weather{Clouds: 0.6, Billow: 0.5, Bases: [2]float32{150, 600}, Drift: [2]float32{70, -30},
			HighShade: true}, 0},
		{Weather{Clouds: 0.8, Billow: 0.9, Bases: [2]float32{150, 600}}, 40},
		{Weather{Clouds: 0.5, Billow: 0.2, Bases: [2]float32{150, 600}, HighShade: true}, 200},
		{Weather{Clouds: 0.8, Billow: 0.6, Bases: [2]float32{150, 600}}, 0},
		{Weather{Clouds: 0.7, Billow: 0.6, Bases: [2]float32{150, 600}, OneLayer: true}, 0},
	} {
		u := map[string]any{
			"Pixels":          []float32{w, 1},
			"From":            []float32{-3000, 777},
			"Step":            []float32{3},
			"Ground":          []float32{c.ground},
			"Sun":             sun[:],
			"CloudBases":      []float32{c.weather.Base(Low), c.weather.Base(High)},
			"CloudsOneLayer":  []float32{flag(c.weather.OneLayer)},
			"CloudsHighShade": []float32{flag(c.weather.HighShade)},
			"Cover":           []float32{c.weather.Clouds},
			"Billow":          []float32{c.weather.Billow},
			"Drift":           c.weather.Drift[:],
			"CloudTile":       Tile(),
		}
		screen := render.NewImage(w, 1)
		screen.DrawMesh(nil, shadeProbe, &render.DrawMeshOptions{Vertices: 3, Uniforms: u})
		pix := make([]byte, 4*w)
		screen.ReadPixels(pix)
		screen.Deallocate()
		worst, sum, shaded := 0.0, 0.0, 0
		for x := range w {
			want := c.weather.Shade(-3000+float32(x)*3, 777, c.ground, sun)
			e := math.Abs(float64(pix[4*x])/255 - float64(want))
			worst, sum = max(worst, e), sum+e
			if want > 0.5 {
				shaded++
			}
		}
		// the tile holds the noise averaged over its pixels: alike on the whole, apart a little
		// along the clouds' sharpest edges
		if mean := sum / w; mean > 2.5/255 || worst > 0.15 || shaded < w/20 {
			t.Errorf("%+v over ground %v up: the shaders' shade differs from the CPU's by "+
				"%.2f/255 on the whole, %.2f/255 at most, %d points shaded; want within 2.5/255 "+
				"on the whole, 0.15 at most, and some shade", c.weather, c.ground, mean*255,
				worst*255, shaded)
		}
	}
}

func flag(on bool) float32 {
	if on {
		return 1
	}
	return 0
}
