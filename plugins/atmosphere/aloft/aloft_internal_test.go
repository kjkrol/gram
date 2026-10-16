package aloft

import (
	"os"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/backdrop"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}

// heaped clouds over the west half of a world 640 by 480, the east clear
func westCloudy() air.Weather {
	d := air.NewDome(640, 480, 2000)
	d.Own()
	d.Carry([2]float32{}, func(x, _ float32) (air.Mass, int16) {
		if x < 320 {
			return air.Mass{Cover: 0.9, Billow: 0.5}, 0
		}
		return air.Mass{}, 0
	}, true)
	return air.Weather{Dome: d, Temperature: 10}
}

// a view from above whose lines of sight start under the clouds, over the highest ground
type low struct{ camera.Camera }

func (l low) Rays() (camera.RayField, bool) {
	f, ok := l.Camera.(camera.Rays).Rays()
	f.Origin[2] = 50
	return f, ok
}

type sunk struct {
	camera.Camera
	z float32
}

// a perspective eye at height z, looking down
func (s sunk) Rays() (camera.RayField, bool) {
	return camera.RayField{Origin: [3]float32{320, 240, s.z}, Dir: [3]float32{0, 0, -1},
		DDX: [3]float32{0.01, 0, 0}, DDY: [3]float32{0, 0.01, 0}}, true
}

func TestRenderer_DrawsTheCloudsFromAboveOverTheirMassAlone(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	cam.SetViewport(640, 480)
	weather := westCloudy()
	sun := func() sky.Sun { return sky.DefaultSun }
	weatherNow := func() air.Weather { return weather }
	vault := backdrop.New(w.Res.Config.Space, world.Scale{}, sun, weatherNow)
	r := New(world.Scale{}, weatherNow)
	if _, ok := r.field(cam); ok {
		t.Fatal("hidden, the clouds are drawn from above")
	}
	r.Show(true)
	c := render.NewComposer(vault, r)
	c.DrawWorld(nil, cam)
	u := map[string]any{}
	for name, v := range c.Uniforms() {
		u[name] = v
	}
	// the frame's images, as the sky hands them
	var masses air.DomeImage
	u["AirMasses"], u["CloudTile"] = masses.Of(weather.Dome), air.Tile()
	cover := func(cam camera.Camera, x0, x1 int) float64 {
		screen := render.NewImage(640, 480)
		defer screen.Deallocate()
		r.Draw(render.Target{Screen: screen}, cam, render.UniformsOf(u))
		pix := make([]byte, 4*640*480)
		screen.ReadPixels(pix)
		sum := 0.0
		for y := range 480 {
			for x := x0; x < x1; x++ {
				sum += float64(pix[4*(y*640+x)+3])
			}
		}
		return sum / float64(255*480*(x1-x0))
	}
	// the noise leaves gaps: the west half covered on the whole, the east bare
	for _, through := range []camera.Camera{cam, low{cam}} {
		west, east := cover(through, 0, 280), cover(through, 360, 640)
		if west < 0.2 || east > 0.01 {
			t.Errorf("seen from above through %T the west is covered %.2f and the east %.2f, want "+
				"the clouds over the west alone", through, west, east)
		}
	}
	if _, ok := r.field(sunk{Camera: cam, z: 100}); ok {
		t.Error("an eye under the clouds sees them from above, want them left to the sky")
	}
	if _, ok := r.field(sunk{Camera: cam, z: 9000}); !ok {
		t.Error("an eye over the clouds does not see them from above")
	}
}
