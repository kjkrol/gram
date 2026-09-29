package sky_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func TestSun_LightsBySquareOnTheSurfaceFacesIt(t *testing.T) {
	sun := sky.Sun{Dir: [3]float32{0, 0, 2}, Strength: 0.6, Ambient: 0.3}
	if got := sun.Light(0, 0, 1)[0]; !near(got, 0.9) {
		t.Errorf("ground under a sun overhead is lit %v, want ambient 0.3 plus all 0.6", got)
	}
	if got := sun.Light(1, 0, 1)[0]; !near(got, 0.3+0.6*float32(math.Sqrt(0.5))) {
		t.Errorf("a slope at 45° is lit %v, want ambient plus 0.6·cos 45°", got)
	}
	if got := sun.Light(0, 0, -1)[0]; got != 0.3 {
		t.Errorf("a surface turned away is lit %v, want the ambient alone", got)
	}
}

// The default sun gives the isometric view the look it always had: level ground at 0.92, the face
// towards +x at 0.72, the one towards +y at 0.55.
func TestDefaultSun_KeepsTheLookOfTheIsometricView(t *testing.T) {
	sun := sky.DefaultSun
	for _, c := range []struct {
		n    [3]float32
		want float32
	}{{[3]float32{0, 0, 1}, 0.92}, {[3]float32{1, 0, 0}, 0.72}, {[3]float32{0, 1, 0}, 0.55}} {
		if got := sun.Light(c.n[0], c.n[1], c.n[2]); math.Abs(float64(got[0]-c.want)) > 0.01 || got[0] != got[1] || got[1] != got[2] {
			t.Errorf("a surface facing %v is lit %v, want %v", c.n, got, c.want)
		}
	}
}

func TestSun_ColoursItsLightAndTheSkysAndHandsTheFrameThem(t *testing.T) {
	dusk := sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.5, Ambient: 0.4, Color: render.Light{1, 0.5, 0.2}, Sky: render.Light{0.25, 0.5, 1}}
	if got, want := dusk.Light(0, 0, 1), (render.Light{0.4*0.25 + 0.5, 0.4*0.5 + 0.25, 0.4 + 0.1}); !near(got[0], want[0]) || !near(got[1], want[1]) || !near(got[2], want[2]) {
		t.Errorf("level ground at dusk is lit %v, want the sky's light and the sun's, each in its colour: %v", got, want)
	}
	if got := dusk.Shaded(0, 0, 1, 0); !near(got[0], 0.1) || !near(got[2], 0.4) {
		t.Errorf("ground in shadow at dusk is lit %v, want the sky's blue light alone", got)
	}
	c := render.NewComposer(source(func(f *render.Frame) { dusk.Frame(f) }))
	c.DrawWorld(nil, icamera.NewFromSpace(64, 64, 0))
	got := c.Uniforms()
	if got["SunStrength"][0] != 0.5 || got["SunColor"][1] != 0.5 || got["SkyColor"][2] != 1 || !near(got["Ambience"][2], 0.4) || got["Sun"][2] != 1 {
		t.Errorf("the shader is handed %v, want the sun's way and strength, the colours of its light and the sky's, and the sky's light on every surface", got)
	}
	c = render.NewComposer(source(func(f *render.Frame) { sky.DefaultSun.Frame(f) }))
	c.DrawWorld(nil, icamera.NewFromSpace(64, 64, 0))
	if got := c.Uniforms(); got["SunColor"][0] != 1 || got["SkyColor"][2] != 1 {
		t.Errorf("the default sun hands the shader colours %v and %v, want white", got["SunColor"], got["SkyColor"])
	}
}

// source is a render.Source composing whatever its function says.
type source func(f *render.Frame)

func (source) Init(*goke.SysInit)                         {}
func (s source) Compose(f *render.Frame, _ camera.Camera) { s(f) }

// shadowsOf lays the shadow of a 10x10 entity at (100, 100) standing as z says under sun, from
// above over level ground, and gives the shadow pieces' middles.
func shadowsOf(z world.Z, sun sky.Sun) (middles []geom.Vec) {
	cam := icamera.NewFromSpace(1000, 1000, 0)
	var f render.Frame
	f.Reset(cam)
	sun.Shadow(&f, cam, geom.NewAABBAt(geom.NewVec(100, 100), 10, 10), z, nil)
	f.Each(func(tier render.Tier, _ float32, v []render.Vertex) {
		if tier != sky.ShadowTier {
			return
		}
		var x, y float32
		for _, p := range v {
			x, y = x+p.DstX/4, y+p.DstY/4
		}
		middles = append(middles, geom.NewVec(float64(x), float64(y)))
	})
	return middles
}

func TestSun_LaysAShadowAwayFromItPushedOffByHowHighTheEntityStands(t *testing.T) {
	west := sky.Sun{Dir: [3]float32{-1, 0, 1}, Strength: 0.6, Ambient: 0.3} // 45° up in the west
	walker := shadowsOf(world.Z{Altitude: 0, Height: 4}, west)
	hawk := shadowsOf(world.Z{Altitude: 40, Height: 4}, west)
	if len(walker) != 1 || len(hawk) != 1 {
		t.Fatalf("shadows %v and %v, want one each", walker, hawk)
	}
	// the walker's centre is at (105, 105): its shadow reaches east by its height, 4 at 45°
	if walker[0].X != 107 || walker[0].Y != 105 {
		t.Errorf("the walker's shadow lies round %v, want (107, 105): east of it by half its height", walker[0])
	}
	if hawk[0].X != 147 || hawk[0].Y != 105 {
		t.Errorf("the hawk's shadow lies round %v, want (147, 105): pushed 40 further east", hawk[0])
	}
	if down := shadowsOf(world.Z{Height: 4}, sky.Sun{Dir: [3]float32{-1, 0, -0.1}}); len(down) != 0 {
		t.Errorf("%d shadows with the sun down, want none", len(down))
	}
}

func TestSun_ItsShaderCompiles(t *testing.T) {
	if err := render.Compile(); err != nil {
		t.Fatal(err)
	}
}
