package backdrop

import (
	"os"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// needGPU readies a device without a window; a machine without one skips.
func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}

// Drawn on the GPU through a perspective the sky is deeper at the top of the screen than at the
// bottom and the sun's disc shows where it stands; seen from above past the world's edge the
// screen is filled in the sky's colour.
func TestBackdrop_DrawsTheSkyOnTheGPU(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w, cameras.TopDown(), camera.Config{}).Main()
	sun := sky.Sun{Dir: eyedF, Strength: 0.7, Sky: render.Light{0.5, 0.7, 1}}
	b := New(w.Res.Config.Space, world.Scale{}, func() sky.Sun { return sun }, func() air.Weather { return air.Weather{} })
	screen := render.NewImage(100, 100)
	pix := make([]byte, 4*100*100)
	at := func(x, y int) [4]byte { i := 4 * (y*100 + x); return [4]byte(pix[i : i+4]) }
	b.Draw(render.Target{Screen: screen}, raying{height: 30}, render.UniformsOf(map[string]any{}))
	screen.ReadPixels(pix)
	if top, bottom := at(5, 2), at(5, 97); top[0] >= bottom[0] || top[1] >= bottom[1] || top[3] != 255 || bottom[3] != 255 {
		t.Errorf("the sky is %v at the top and %v at the bottom, want it opaque and deeper overhead", top, bottom)
	}
	if c := at(50, 50); c[0] < 250 || c[1] < 250 || c[2] < 250 {
		t.Errorf("the sun's disc in the middle is %v, want white", c)
	}
	screen.Clear()
	b.Draw(render.Target{Screen: screen}, shifted{Camera: cam, dx: 150}, render.UniformsOf(map[string]any{}))
	screen.ReadPixels(pix)
	if c := at(70, 30); c != [4]byte{128, 179, 255, 255} {
		t.Errorf("past the world's edge from above the screen is %v, want the sky's colour", c)
	}
}
