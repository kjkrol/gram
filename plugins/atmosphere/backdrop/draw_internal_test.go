package backdrop

import (
	"os"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/light"
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
	cam := cameras.NewPlugin(w, cameras.Config{}).New(cameras.FromAbove, camera.Config{})
	sun := light.Distant{Dir: eyedF, Strength: 0.7, AmbientColor: render.Light{0.5, 0.7, 1}}
	b := New(w.Res.Config.Space, func() light.Distant { return sun },
		func() air.Weather { return air.Weather{} })
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

func TestBackdrop_DrawsTheLayersFlatOrHeapedOrNone(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w, cameras.Config{}).New(cameras.FromAbove, camera.Config{})
	sun := light.Distant{Dir: eyedF, Strength: 0.7, AmbientColor: render.Light{0.5, 0.7, 1}}
	bases := air.Bases(world.Scale{})
	weather := air.Weather{Bases: bases}
	var look Clouds
	b := New(w.Res.Config.Space, func() light.Distant { return sun },
		func() air.Weather { return weather }).WithClouds(func() Clouds { return look })
	draw := func(eye float32) []byte {
		u := map[string]any{"CloudTile": air.Tile()}
		c := render.NewComposer(b)
		c.DrawWorld(nil, cam)
		for name, v := range c.Uniforms() {
			u[name] = v
		}
		screen := render.NewImage(100, 100)
		defer screen.Deallocate()
		b.Draw(render.Target{Screen: screen}, raying{height: eye}, render.UniformsOf(u))
		pix := make([]byte, 4*100*100)
		screen.ReadPixels(pix)
		return pix
	}
	// over the clouds, both layers
	over := bases[air.High] + 300
	for _, c := range []struct {
		billow float32
		one    bool
	}{{0, false}, {1, false}, {1, true}} {
		billow := c.billow
		// the sky as grey, its clouds both under the eye
		weather = air.Weather{Clouds: 0.9, Billow: billow, Bases: [2]float32{1, 2}, OneLayer: c.one}
		clear := map[float32][]byte{30: draw(30), over: draw(over)}
		weather.Bases = bases
		looks := []Clouds{{}, {Bodies: [2]bool{true, false}}, {Bodies: [2]bool{true, true}},
			{Hidden: true}}
		for _, look = range looks {
			for _, eye := range []float32{30, over} {
				pix := draw(eye)
				clouded := 0
				for i := 0; i < len(pix); i += 4 {
					if pix[i+3] != 255 {
						t.Fatalf("%+v: the sky's pixel %d is see-through, %v", look, i/4,
							pix[i:i+4])
					}
					c := clear[eye]
					if pix[i] != c[i] || pix[i+1] != c[i+1] || pix[i+2] != c[i+2] {
						clouded++
					}
				}
				want := eye < over && !look.Hidden
				if got := clouded >= 100*100/5; got != want || look.Hidden && clouded > 0 {
					t.Errorf("billow %v, one layer %v, %+v, from %v up: clouds over %d of the "+
						"sky's pixels, want a heavy cover's worth %v", billow, c.one, look, eye,
						clouded, want)
				}
			}
		}
	}
}

// ground standing as high as Span everywhere, handed to the frame
type standing struct{ render.GroundHeights }

func (standing) Init(*goke.SysInit)                         {}
func (s standing) Compose(f *render.Frame, _ camera.Camera) { f.GroundHeights(s.GroundHeights) }

func TestBackdrop_LeavesTheHeapsBeforeTheGroundToAnother(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 200, Height: 200},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w, cameras.Config{}).New(cameras.FromAbove, camera.Config{})
	sun := light.Distant{Dir: eyedF, Strength: 0.7, AmbientColor: render.Light{0.5, 0.7, 1}}
	weather := air.Weather{Clouds: 0.9, Billow: 1, Bases: air.Bases(world.Scale{})}
	img := render.NewImage(2, 2)
	defer img.Deallocate()
	img.WritePixels([]byte{255, 255, 0, 255, 255, 255, 0, 255, 255, 255, 0, 255, 255, 255, 0,
		255})
	draw := func(look Clouds, ground float32) []byte {
		b := New(w.Res.Config.Space, func() light.Distant { return sun },
			func() air.Weather { return weather }).WithClouds(func() Clouds { return look })
		u := map[string]any{"CloudTile": air.Tile()}
		wall := standing{render.GroundHeights{Image: img, Corners: [2]int{2, 2}, Cell: 200,
			Span: ground}}
		c := render.NewComposer(b, wall)
		c.DrawWorld(nil, cam)
		for name, v := range c.Uniforms() {
			u[name] = v
		}
		u["GroundHeights"] = img
		screen := render.NewImage(100, 100)
		defer screen.Deallocate()
		b.Draw(render.Target{Screen: screen}, raying{height: 30}, render.UniformsOf(u))
		pix := make([]byte, 4*100*100)
		screen.ReadPixels(pix)
		return pix
	}
	differ := func(a, b []byte) int {
		n := 0
		for i := 0; i < len(a); i += 4 {
			if a[i] != b[i] || a[i+1] != b[i+1] || a[i+2] != b[i+2] {
				n++
			}
		}
		return n
	}
	heaped := Clouds{Bodies: [2]bool{true, true}}
	before := Clouds{Bodies: [2]bool{true, true}, Before: true}
	// ground higher than the heaps' lumps rise, then lower than their base
	high, low := 2*weather.Top(air.Low), weather.Base(air.Low)/2
	if n := differ(draw(heaped, high), draw(before, high)); n < 100*100/5 {
		t.Errorf("under ground higher than the heaps rise, those before it another's: the sky "+
			"changes over %d pixels, want the heaps gone off a heavy cover's worth", n)
	}
	if n := differ(draw(heaped, low), draw(before, low)); n > 0 {
		t.Errorf("under ground lower than the heaps' base, those before it another's: the sky "+
			"changes over %d pixels, want none", n)
	}
}
