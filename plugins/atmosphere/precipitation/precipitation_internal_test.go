package precipitation

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

func TestPrecipitation_FallsAsMuchAsTheWeatherSaysAndNotAtAllWhenDry(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	weather := air.Weather{}
	p := New(func() sky.Sun { return sky.DefaultSun }, func() air.Weather { return weather })
	if p.Tier() != render.Air {
		t.Errorf("what falls comes on tier %v, want the Air", p.Tier())
	}
	if f := p.fall(cam); f.Drops+f.Flakes != 0 {
		t.Errorf("a dry sky lets %d drops and %d flakes fall", f.Drops, f.Flakes)
	}
	weather = air.Weather{Rain: 0.5}
	half := p.fall(cam).Drops
	weather = air.Weather{Rain: 1}
	full := p.fall(cam).Drops
	if half == 0 || full < 2*half-1 || full > 2*half+1 {
		t.Errorf("half a rain lets %d drops fall, a full one %d; want some, twice as many", half, full)
	}
	weather = air.Weather{Snow: 1}
	if f := p.fall(cam); f.Flakes == 0 || f.Drops != 0 {
		t.Errorf("a snowfall lets %d flakes and %d drops fall, want flakes alone", f.Flakes, f.Drops)
	}
}

// flung is a camera through which the world's origin — far from where it looks — is drawn millions
// of pixels off, as a perspective draws a point behind the eye; round the middle of the screen it
// draws a world unit as a pixel.
type flung struct{ camera.Camera }

func (flung) Viewport() (float32, float32)                   { return 400, 300 }
func (flung) Unproject(sx, sy, _ float32) (float32, float32) { return 500 + sx, 500 + sy }
func (flung) Project(x, y, _ float32) (float32, float32) {
	if x < 100 && y < 100 {
		return 3e6 * (x + 1), 2e6
	}
	return x - 500, y - 500
}

// Rain slants as the wind carries it where the middle of the screen looks, however the camera
// draws points elsewhere, and never flatter than it falls: no streak runs across the screen.
func TestPrecipitation_RainSlantsNoFurtherThanItFalls(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	weather := air.Weather{Rain: 1, Wind: [2]float32{10, 0}}
	p := New(func() sky.Sun { return sky.DefaultSun }, func() air.Weather { return weather })
	if f := p.fall(flung{cam}); f.Drift != 10*rainSlant {
		t.Errorf("the rain drifts %v pixels a second, want the wind's 10 across the middle, %v times over", f.Drift, rainSlant)
	}
	weather.Wind = [2]float32{1000, 0}
	if f := p.fall(flung{cam}); f.Drift != rainSpeed {
		t.Errorf("in a gale the rain drifts %v pixels a second, want no more than it falls, %v", f.Drift, rainSpeed)
	}
}

// Drawn on the GPU the rain leaves streaks on a clear screen, and a dry sky nothing.
func TestPrecipitation_DrawsTheRainOnTheGPU(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	weather := air.Weather{Rain: 1}
	p := New(func() sky.Sun { return sky.DefaultSun }, func() air.Weather { return weather })
	screen := render.NewImage(640, 480)
	pix := make([]byte, 4*640*480)
	lit := func() int {
		screen.Clear()
		p.Draw(render.Target{Screen: screen}, cam, render.UniformsOf(map[string]any{"Clock": []float32{3}}))
		screen.ReadPixels(pix)
		n := 0
		for i := 3; i < len(pix); i += 4 {
			if pix[i] > 0 {
				n++
			}
		}
		return n
	}
	if n := lit(); n < 640*480/900*10 {
		t.Errorf("a full rain covers %d pixels, want its streaks, some %d pixels each", n, rainDrop)
	}
	weather = air.Weather{}
	if n := lit(); n != 0 {
		t.Errorf("a dry sky covers %d pixels", n)
	}
}

// needGPU readies a device without a window; a machine without one skips.
func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}
