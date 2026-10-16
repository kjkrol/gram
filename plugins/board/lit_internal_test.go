package board

import (
	"image/color"
	"os"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
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

type heldSky struct {
	sun     sky.Sun
	weather *air.Weather
}

func (s heldSky) Sun() sky.Sun     { return s.sun }
func (s heldSky) Air() air.Weather { return *s.weather }

// the atmosphere's sky handing the frame its sun and weather, and a strip lying on the ground at
// a material's lift
type skyAndStrip struct {
	heldSky
	grey color.RGBA
}

func (skyAndStrip) Init(*goke.SysInit) {}
func (s skyAndStrip) Compose(f *render.Frame, cam camera.Camera) {
	s.sun.Frame(f)
	s.weather.Frame(f, s.sun, 0, 0)
	x0, y0 := cam.FromScreen(0, 0)
	x1, y1 := cam.FromScreen(100, 384)
	at := f.Lifted((x0+x1)/2, (y0+y1)/2, 0, 0.05) // a board material's lift
	strip := render.Corners{{0, 0}, {100, 0}, {0, 384}, {100, 384}}
	f.Quad(at, strip, [4]color.RGBA{s.grey, s.grey, s.grey, s.grey})
}

func TestAtmosphere_TheCloudsShadowsLieOnTheBoardsGroundUnderWhatLiesOnIt(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4096, Height: 4096},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8},
		Heights:  true,
	})
	p := NewPlugin(grid.DefaultGrids{}.Square(128, 128, 32), &cell.SingleOccupancy{}, w)
	p.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land, SpriteID: 1})
	grey := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	tiles := render.NewAtlas()
	tiles.Add(render.SpriteID(1), 8, render.Solid(grey))
	tiles.Close()
	p.SetAtlas(tiles)
	p.ShowGrid(false)
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	cam.SetViewport(512, 384)
	weather := air.Weather{Clouds: 0.7, Drift: [2]float32{130, -40}}
	held := heldSky{sun: sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 1}, weather: &weather}
	p.WithAtmosphere(held)
	screen := render.NewImage(512, 384)
	pix := make([]byte, 4*512*384)
	draw := func() {
		render.NewComposer(p.renderer, skyAndStrip{held, grey}).DrawWorld(screen, cam)
		screen.ReadPixels(pix)
	}
	draw()
	darkest, clear := 255, 0
	for i := 0; i < len(pix); i += 4 {
		if x := i / 4 % 512; x < 100 {
			if pix[i] != 200 {
				t.Fatalf("what lies on the ground is %d at (%d, %d), want it untouched by the "+
					"clouds", pix[i], x, i/4/512)
			}
			continue
		}
		darkest = min(darkest, int(pix[i]))
		if pix[i] == 200 {
			clear++
		}
	}
	// the thickest cloud takes half the sun: 200 down to 100 at most
	if darkest > 150 || darkest < 99 || clear == 0 {
		t.Errorf("under clouds the ground's grey 200 is %d at its darkest with %d pixels clear, "+
			"want patches down towards 100 and clear ground between", darkest, clear)
	}
	weather.Clouds = 0
	draw()
	for i := 0; i < len(pix); i += 4 {
		if pix[i] != 200 {
			t.Fatalf("with no clouds pixel %d is %d, want the grey untouched", i/4, pix[i])
		}
	}
}
