package light_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

func near(a, b float32) bool { return f32.Abs(float32(a-b)) < 1e-3 }

func TestDefaultSun_KeepsTheLookOfTheIsometricView(t *testing.T) {
	sun := sky.DefaultSun
	for _, c := range []struct {
		n    [3]float32
		want float32
	}{{[3]float32{0, 0, 1}, 0.92}, {[3]float32{1, 0, 0}, 0.72}, {[3]float32{0, 1, 0}, 0.55}} {
		got := sun.Light(c.n[0], c.n[1], c.n[2])
		if f32.Abs(float32(got[0]-c.want)) > 0.01 || got[0] != got[1] || got[1] != got[2] {
			t.Errorf("a surface facing %v is lit %v, want %v", c.n, got, c.want)
		}
	}
}

func TestDistant_ColoursItsLightAndTheSkysAndHandsTheFrameThem(t *testing.T) {
	dusk := light.Distant{
		Dir:          [3]float32{0, 0, 1},
		Strength:     0.5,
		Ambient:      0.4,
		Color:        render.Light{1, 0.5, 0.2},
		AmbientColor: render.Light{0.25, 0.5, 1},
	}
	got, want := dusk.Light(0, 0, 1), render.Light{0.4*0.25 + 0.5, 0.4*0.5 + 0.25, 0.4 + 0.1}
	if !near(got[0], want[0]) || !near(got[1], want[1]) || !near(got[2], want[2]) {
		t.Errorf("level ground at dusk is lit %v, want the ambient light and the direct, each in "+
			"its colour: %v", got, want)
	}
	if got := dusk.Shaded(0, 0, 1, 0); !near(got[0], 0.1) || !near(got[2], 0.4) {
		t.Errorf("ground in shadow at dusk is lit %v, want the sky's blue light alone", got)
	}
	c := render.NewComposer(source(func(f *render.Frame) { dusk.Frame(f) }))
	c.DrawWorld(nil, cameratest.NewFromSpace(64, 64, 0))
	u := c.Uniforms()
	if u["LightStrength"][0] != 0.5 || u["LightColor"][1] != 0.5 || u["AmbientColor"][2] != 1 ||
		!near(u["Ambience"][2], 0.4) || u["LightWay"][2] != 1 {
		t.Errorf("the shader is handed %v, want the light's way and strength, the colours of its "+
			"light and of the ambient, and the ambient light on every surface", u)
	}
	c = render.NewComposer(source(func(f *render.Frame) { sky.DefaultSun.Frame(f) }))
	c.DrawWorld(nil, cameratest.NewFromSpace(64, 64, 0))
	if u := c.Uniforms(); u["LightColor"][0] != 1 || u["AmbientColor"][2] != 1 {
		t.Errorf("the default sun hands the shader colours %v and %v, want white",
			u["LightColor"], u["AmbientColor"])
	}
}

type source func(f *render.Frame)

func (s source) Compose(f *render.Frame, _ camera.Camera) { s(f) }

func TestDistant_ItsShaderCompiles(t *testing.T) {
	gputest.Need(t)
	if err := render.Compile(); err != nil {
		t.Fatal(err)
	}
}
