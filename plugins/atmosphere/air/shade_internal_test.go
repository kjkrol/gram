package air

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
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
	gputest.Need(t)
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
		{banded(Weather{Clouds: 0.8, Billow: 0.7, Drift: [2]float32{900, -400}}, 200,
			[3]float32{0.3, 0.4, 0.3}), 0},
		{banded(Weather{Clouds: 0.6, Billow: 0.9, HighShade: true}, 200,
			[3]float32{1, 0, 0}), 100},
	} {
		u := shaded(c.weather)
		u["Pixels"] = []float32{w, 1}
		u["From"] = []float32{-3000, 777}
		u["Step"] = []float32{3}
		u["Ground"] = []float32{c.ground}
		u["LightWay"] = sun[:]
		screen := render.NewImage(w, 1)
		screen.DrawMesh(nil, shadeProbe, &render.DrawMeshOptions{Vertices: 3, Uniforms: u})
		pix := make([]byte, 4*w)
		screen.ReadPixels(pix)
		screen.Deallocate()
		apart, sum, shaded := 0, 0.0, 0
		for x := range w {
			want := c.weather.Shade(-3000+float32(x)*3, 777, c.ground, sun)
			e := math.Abs(float64(pix[4*x])/255 - float64(want))
			sum += e
			if e > 0.15 {
				apart++
			}
			if want > 0.5 {
				shaded++
			}
		}
		// the tile holds the noise averaged over its pixels, and the bases' heights lie a hair
		// apart: alike on the whole, apart along the clouds' sharpest edges alone
		if mean := sum / w; mean > 2.5/255 || apart > w/100 || shaded < w/20 {
			t.Errorf("%+v over ground %v up: the shaders' shade differs from the CPU's by "+
				"%.2f/255 on the whole, %d points by over 0.15, %d points shaded; want within "+
				"2.5/255 on the whole, under %d apart, and some shade", c.weather, c.ground,
				mean*255, apart, shaded, w/100)
		}
	}
}

// the low clouds of w hanging over a relief span world units high, their bands shared as
// ceilings say
func banded(w Weather, span float64, ceilings [3]float32) Weather {
	w.Relief = ReliefOf(world.Scale{}, 0, span)
	w.Bases = BasesOver(world.Scale{}, w.Relief)
	w.Ceilings = ceilings
	return w
}

// the uniforms the frame hands the shaders of w, as Weather.Frame does
func shaded(w Weather) map[string]any {
	c0, c1 := w.cuts()
	return map[string]any{
		"CloudBases":      []float32{w.Base(Low), w.Base(High)},
		"CloudRelief":     w.Relief[:],
		"CloudCeilings":   []float32{c0, c1},
		"CloudsOneLayer":  []float32{flag(w.OneLayer)},
		"CloudsHighShade": []float32{flag(w.HighShade)},
		"Cover":           []float32{w.Clouds},
		"Billow":          []float32{w.Billow},
		"Drift":           w.Drift[:],
		"CloudCeilingDrift": []float32{
			float32(wrap(float64(ceilingLag*w.Drift[0]), CloudTile)),
			float32(wrap(float64(ceilingLag*w.Drift[1]), CloudTile)),
		},
		"CloudTile": Tile(),
	}
}

// the low clouds' band along a line of world points U.Step apart, as shares of U.Span over the
// relief's low ground: their base in red, the top of their deck in green
var bandProbe = render.NewMeshShaderWith("band probe", []byte(`
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
    let b = (cloudLowBand(p) - U.CloudRelief.x) / U.Span;
    return vec4<f32>(b, 0.0, 1.0);
}
`), []render.Uniform{{Name: "Pixels", Size: 2}, {Name: "From", Size: 2}, {Name: "Step", Size: 1},
	{Name: "Span", Size: 1}})

func TestLowBand_TheShadersHangTheLowCloudsAsTheCPUDoes(t *testing.T) {
	gputest.Need(t)
	const w = 2000
	for _, ceilings := range [][3]float32{{0.3, 0.15, 0.55}, {0, 1, 0}, {0.5, 0, 0.5}} {
		weather := banded(Weather{Drift: [2]float32{5000, 1200}}, 1860, ceilings)
		u := shaded(weather)
		span := 1.5 * weather.Relief[1]
		u["Pixels"] = []float32{w, 1}
		u["From"] = []float32{-9000, 4321}
		u["Step"] = []float32{11}
		u["Span"] = []float32{span}
		screen := render.NewImage(w, 1)
		screen.DrawMesh(nil, bandProbe, &render.DrawMeshOptions{Vertices: 3, Uniforms: u})
		pix := make([]byte, 4*w)
		screen.ReadPixels(pix)
		screen.Deallocate()
		worst := 0.0
		for x := range w {
			base, top := weather.LowBand(-9000+float32(x)*11, 4321)
			worst = max(worst,
				math.Abs(float64(pix[4*x])/255-float64(base/span)),
				math.Abs(float64(pix[4*x+1])/255-float64(top/span)))
		}
		if worst > 1.5/255 {
			t.Errorf("ceilings %v: the shaders' low band differs from the CPU's by %.2f/255 at "+
				"most, want within 1.5/255", ceilings, worst*255)
		}
	}
}

func flag(on bool) float32 {
	if on {
		return 1
	}
	return 0
}
