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

func TestSun_GlintsOnlyWhereTheSurfaceFacesHalfwayBetweenSunAndEye(t *testing.T) {
	overhead := world.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.6, Ambient: 0.3}
	above := [3]float32{0, 0, 1}
	if g := overhead.Glint(0, 0, 1, above, 1, 1); !near(g, 0.6) {
		t.Errorf("a level mirror under the sun seen from above throws back %v, want the sun's whole 0.6", g)
	}
	if g := overhead.Glint(0.5, 0, 1, above, 1, 1); g > 0.01 {
		t.Errorf("a surface tilted away from the reflection throws back %v, want next to nothing", g)
	}
	if g := overhead.Glint(0, 0, 1, above, 0.5, 1); !near(g, 0.3) {
		t.Errorf("half the shine throws back %v, want half the glint", g)
	}
	if g := overhead.Glint(0, 0, 1, above, 1, 0); g != 0 {
		t.Errorf("a surface in shadow throws back %v, want none", g)
	}
	// a low sun in the east and an eye above: the surface that throws it back leans east
	east := world.Sun{Dir: [3]float32{1, 0, 1}, Strength: 0.6}
	if lean, level := east.Glint(0.4, 0, 1, above, 1, 1), east.Glint(0, 0, 1, above, 1, 1); lean <= level {
		t.Errorf("a surface leaning towards the sun throws back %v, a level one %v; want the leaning more", lean, level)
	}
}
