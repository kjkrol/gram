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

func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}

func TestBackdrop_DrawsTheSkyOnTheGPU(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	sun := sky.Sun{Dir: eyedF, Strength: 0.7, Sky: render.Light{0.5, 0.7, 1}}
	b := New(w.Res.Config.Space, world.Scale{}, func() sky.Sun { return sun }, func() air.Weather { return air.Weather{} })
	screen := render.NewImage(100, 100)
	pix := make([]byte, 4*100*100)
	at := func(x, y int) [4]byte { i := 4 * (y*100 + x); return [4]byte(pix[i : i+4]) }
	b.Draw(render.Target{Screen: screen}, raying{height: 30}, framed(b, cam))
	screen.ReadPixels(pix)
	if top, bottom := at(5, 2), at(5, 97); top[0] >= bottom[0] || top[1] >= bottom[1] || top[3] != 255 || bottom[3] != 255 {
		t.Errorf("the sky is %v at the top and %v at the bottom, want it opaque and deeper overhead", top, bottom)
	}
	if c := at(50, 50); c[0] < 250 || c[1] < 250 || c[2] < 250 {
		t.Errorf("the sun's disc in the middle is %v, want white", c)
	}
	screen.Clear()
	far := shifted{Camera: cam, dx: 150}
	b.Draw(render.Target{Screen: screen}, far, framed(b, cam))
	screen.ReadPixels(pix)
	if c := at(70, 30); c != [4]byte{128, 179, 255, 255} {
		t.Errorf("past the world's edge from above the screen is %v, want the sky's colour", c)
	}
}

func TestBackdrop_HeapsTheCloudsWhileTheyAreBodies(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	sun := sky.Sun{Dir: eyedF, Strength: 0.7, Sky: render.Light{0.5, 0.7, 1}}
	weather := air.Weather{}
	bodies := false
	b := New(w.Res.Config.Space, world.Scale{}, func() sky.Sun { return sun },
		func() air.Weather { return weather }).WithBodies(func() bool { return bodies })
	draw := func() []byte {
		u := map[string]any{"CloudTile": air.Tile()}
		c := render.NewComposer(b)
		c.DrawWorld(nil, cam)
		for name, v := range c.Uniforms() {
			u[name] = v
		}
		screen := render.NewImage(100, 100)
		defer screen.Deallocate()
		b.Draw(render.Target{Screen: screen}, raying{height: 30}, render.UniformsOf(u))
		pix := make([]byte, 4*100*100)
		screen.ReadPixels(pix)
		return pix
	}
	clear := draw()
	weather = air.Weather{Clouds: 0.9, Billow: 0.8}
	for _, bodies = range []bool{false, true} {
		pix := draw()
		clouded := 0
		for i := 0; i < len(pix); i += 4 {
			if pix[i+3] != 255 {
				t.Fatalf("bodies %v: the sky's pixel %d is see-through, %v", bodies, i/4,
					pix[i:i+4])
			}
			if pix[i] != clear[i] || pix[i+1] != clear[i+1] || pix[i+2] != clear[i+2] {
				clouded++
			}
		}
		if clouded < 100*100/5 {
			t.Errorf("bodies %v: clouds over %d of the sky's pixels, want a heavy cover's worth",
				bodies, clouded)
		}
	}
}
