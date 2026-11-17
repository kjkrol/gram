package air

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// the gust along a line of world points U.Step apart in red, each standing for U.Span world
// units, and the lean of what sways wholly there in green and blue, half a unit of slope a half
var gustProbe = render.NewMeshShaderWith("gust probe", []byte(`
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
    let p = U.From + U.Way * (floor(o.px.x) * U.Step);
    return vec4<f32>(gustAt(p, U.Span), windLean(1.0, p) + 0.5, 1.0);
}
`), []render.Uniform{{Name: "Pixels", Size: 2}, {Name: "From", Size: 2}, {Name: "Way", Size: 2},
	{Name: "Step", Size: 1}, {Name: "Span", Size: 1}})

func TestGust_TheShadersGustAndLeanAsTheCPUDoes(t *testing.T) {
	gputest.Need(t)
	const w = 2000
	const clock = 37.25
	for _, c := range []struct {
		weather Weather
		span    float32
	}{
		{Weather{Wind: [2]float32{30, 0}, Gusts: 64}, 0},
		{Weather{Wind: [2]float32{-14, 20}, Gusts: 76.8, Drift: [2]float32{90321, -4417}}, 0},
		{Weather{Wind: [2]float32{-14, 20}, Gusts: 76.8, Drift: [2]float32{90321, -4417}}, 32},
		{Weather{Wind: [2]float32{3, -50}, Gusts: 16, Drift: [2]float32{-700000, 1.5e6}}, 0},
	} {
		from, way := [2]float32{-900, 1300}, [2]float32{0.8, 0.6}
		screen := render.NewImage(w, 1)
		uniforms := map[string]any{
			"Pixels":   []float32{w, 1},
			"From":     from[:],
			"Way":      way[:],
			"Step":     []float32{7},
			"Span":     []float32{c.span},
			"Clock":    []float32{clock},
			"Wind":     c.weather.Wind[:],
			"GustCell": []float32{gustSide(c.weather.Gusts)},
			"Blown": []float32{
				float32(wrap(float64(c.weather.Drift[0]), blownSpan)),
				float32(wrap(float64(c.weather.Drift[1]), blownSpan)),
			},
		}
		screen.DrawMesh(nil, gustProbe, &render.DrawMeshOptions{Vertices: 3, Uniforms: uniforms})
		pix := make([]byte, 4*w)
		screen.ReadPixels(pix)
		screen.Deallocate()
		apart, blown := 0, 0
		for i := range w {
			x, y := from[0]+way[0]*float32(i)*7, from[1]+way[1]*float32(i)*7
			gust := c.weather.gustOver(x, y, c.span)
			lx, ly := c.weather.Sway(clock, x, y, 1)
			if math.Abs(float64(pix[4*i])/255-float64(gust)) > 2.0/255 ||
				math.Abs(float64(pix[4*i+1])/255-0.5-float64(lx)) > 2.0/255 ||
				math.Abs(float64(pix[4*i+2])/255-0.5-float64(ly)) > 2.0/255 {
				apart++
			}
			if gust > 0.5 {
				blown++
			}
		}
		// a point on a square's edge may fall either side of it by a float's last bit
		if apart > w/200 || blown < w/50 {
			t.Errorf("%+v, a point %v wide: the shaders' gust or lean differs from the CPU's at "+
				"%d points of %d, %d under a gust; want under %d apart and some gusts", c.weather,
				c.span, apart, w, blown, w/200)
		}
	}
}

func TestGust_BlowsInBandsAcrossTheWindCarriedAlongByIt(t *testing.T) {
	still := Weather{Gusts: 64}
	if g := still.Gust(100, 100); g != 0 {
		t.Errorf("in the calm a gust blows %v, want none", g)
	}
	steady := Weather{Wind: [2]float32{20, 0}}
	if g := steady.Gust(100, 100); g != 0 {
		t.Errorf("a wind without gusts gusts %v, want steady", g)
	}
	w := Weather{Wind: [2]float32{20, 0}, Gusts: 64}
	// the bands lie across the wind: along it a gust is short, across it long
	along, across := runs(w, 1, 0), runs(w, 0, 1)
	if along <= 0 || across < 2.5*along {
		t.Errorf("a gust runs %v world units along the wind and %v across it; want it lying "+
			"across, some three times as long as wide at the least", along, across)
	}
	if along < 64*0.6 || along > 64*2 {
		t.Errorf("a gust is %v world units wide along the wind, want about its band's 64", along)
	}
	// carried: the same gusts stand gustPace times the clouds' drift further on
	moved := w
	moved.Drift = [2]float32{500, 120}
	for _, p := range [][2]float32{{310, 4000}, {5000, 77}, {12345, 6789}} {
		was := w.Gust(p[0], p[1])
		is := moved.Gust(p[0]+gustPace*500, p[1]+gustPace*120)
		if math.Abs(float64(was-is)) > 1e-3 {
			t.Errorf("the gust at %v was %v and is %v where the wind has carried it; want it "+
				"carried whole", p, was, is)
		}
	}
	// the drift wrapped, the gusts stand where they stood
	wrapped := w
	wrapped.Drift = [2]float32{blownSpan, 3 * blownSpan}
	for _, p := range [][2]float32{{310, 4000}, {5000, 77}, {12345, 6789}} {
		if a, b := w.Gust(p[0], p[1]), wrapped.Gust(p[0], p[1]); math.Abs(float64(a-b)) > 1e-3 {
			t.Errorf("the gust at %v is %v, and %v with the drift gone round; want the same", p,
				a, b)
		}
	}
}

// runs is the mean length of the stretches a gust blows over along the way (dx, dy), world units
func runs(w Weather, dx, dy float32) float32 {
	var total, count float32
	for line := range 40 {
		ox, oy := float32(line)*397*dy, float32(line)*397*dx
		in := false
		for i := range 6000 {
			if g := w.Gust(ox+dx*float32(i)*4, oy+dy*float32(i)*4); g > 0.3 {
				total += 4
				if !in {
					count++
				}
				in = true
			} else {
				in = false
			}
		}
	}
	if count == 0 {
		return 0
	}
	return total / count
}

func TestGustBand_IsAsWideAsTheFiguresAreDrawn(t *testing.T) {
	if got := GustBand(world.Scale{}); got != unscaledGust {
		t.Errorf("a world without a scale has gusts %v wide, want %v", got, float32(unscaledGust))
	}
	madeira := world.Scale{Metres: 3.125, Figures: 15}
	got, want := GustBand(madeira), float32(GustWide*15/3.125)
	if math.Abs(float64(got-want)) > 1e-3 {
		t.Errorf("gusts over figures 15 times life are %v world units wide, want %v", got, want)
	}
}
