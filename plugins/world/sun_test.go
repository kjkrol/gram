package world_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/world"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func TestSun_LightsBySquareOnTheSurfaceFacesIt(t *testing.T) {
	sun := world.Sun{Dir: [3]float32{0, 0, 2}, Strength: 0.6, Ambient: 0.3}
	if got := sun.Light(0, 0, 1); !near(got, 0.9) {
		t.Errorf("ground under a sun overhead is lit %v, want ambient 0.3 plus all 0.6", got)
	}
	if got := sun.Light(1, 0, 1); !near(got, 0.3+0.6*float32(math.Sqrt(0.5))) {
		t.Errorf("a slope at 45° is lit %v, want ambient plus 0.6·cos 45°", got)
	}
	if got := sun.Light(0, 0, -1); got != 0.3 {
		t.Errorf("a surface turned away is lit %v, want the ambient alone", got)
	}
}

// The default sun gives the isometric view the look it always had: level ground at 0.92, the face
// towards +x at 0.72, the one towards +y at 0.55.
func TestDefaultSun_KeepsTheLookOfTheIsometricView(t *testing.T) {
	sun := world.DefaultSun
	for _, c := range []struct {
		n    [3]float32
		want float32
	}{{[3]float32{0, 0, 1}, 0.92}, {[3]float32{1, 0, 0}, 0.72}, {[3]float32{0, 1, 0}, 0.55}} {
		if got := sun.Light(c.n[0], c.n[1], c.n[2]); math.Abs(float64(got-c.want)) > 0.01 {
			t.Errorf("a surface facing %v is lit %v, want %v", c.n, got, c.want)
		}
	}
}

func TestPlugin_SetSunLightsTheWorldWithIt(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 64, Height: 64}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	if w.Sun() != world.DefaultSun {
		t.Errorf("a new world's sun is %+v, want DefaultSun", w.Sun())
	}
	dusk := world.Sun{Dir: [3]float32{-1, 0, 0.2}, Strength: 0.4, Ambient: 0.2}
	w.SetSun(dusk)
	if w.Sun() != dusk {
		t.Errorf("sun after SetSun %+v, want %+v", w.Sun(), dusk)
	}
}
