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
	var fr render.Frame
	fr.Reset(cam)
	p.Compose(&fr, cam)
	fr.Places(func(pass render.Pass, at render.At) {
		if pass != render.SeeThrough || at != render.Nearest {
			t.Errorf("what falls is drawn in pass %v at %v, want before the whole world", pass, at)
		}
	})
	if f := p.fall(cam); f.Drops+f.Flakes != 0 {
		t.Errorf("a dry sky lets %d drops and %d flakes fall", f.Drops, f.Flakes)
	}
	weather = air.Weather{Rain: 0.5, Clouds: 1, Temperature: 10}
	half := p.fall(cam).Drops
	weather = air.Weather{Rain: 1, Clouds: 1, Temperature: 10}
	full := p.fall(cam).Drops
	if half == 0 || full < 2*half-1 || full > 2*half+1 {
		t.Errorf("half a rain lets %d drops fall, a full one %d; want some, twice as many", half, full)
	}
	weather = air.Weather{Snow: 1, Clouds: 1, Temperature: -5}
	if f := p.fall(cam); f.Flakes == 0 || f.Drops != 0 {
		t.Errorf("a snowfall lets %d flakes and %d drops fall, want flakes alone", f.Flakes, f.Drops)
	}
}

// draws the origin millions of pixels off, as a perspective draws a point behind the eye
type flung struct{ camera.Camera }

func (flung) Viewport() (float32, float32)                   { return 400, 300 }
func (flung) Unproject(sx, sy, _ float32) (float32, float32) { return 500 + sx, 500 + sy }
func (flung) Project(x, y, _ float32) (float32, float32) {
	if x < 100 && y < 100 {
		return 3e6 * (x + 1), 2e6
	}
	return x - 500, y - 500
}

func TestPrecipitation_RainSlantsNoFurtherThanItFalls(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	weather := air.Weather{Rain: 1, Clouds: 1, Temperature: 10, Wind: [2]float32{10, 0}}
	p := New(func() sky.Sun { return sky.DefaultSun }, func() air.Weather { return weather })
	if f := p.fall(flung{cam}); f.Drift != 10*rainSlant {
		t.Errorf("the rain drifts %v pixels a second, want the wind's 10 across the middle, %v times over", f.Drift, rainSlant)
	}
	weather.Wind = [2]float32{1000, 0}
	if f := p.fall(flung{cam}); f.Drift != rainSpeed {
		t.Errorf("in a gale the rain drifts %v pixels a second, want no more than it falls, %v", f.Drift, rainSpeed)
	}
}

func TestPrecipitation_DrawsTheRainOnTheGPU(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	weather := air.Weather{Rain: 1, Clouds: 1, Temperature: 10}
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

func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}

func TestPrecipitation_RainFallsOnlyUnderTheRainingMass(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	vw, _ := cam.Viewport()
	// a raining mass over the left of the view
	weather := air.Weather{Dome: domeOf(func(x, _ float32) air.Mass {
		if x < vw/2-air.BoardCell {
			return air.Mass{Cover: 1, Falls: 1}
		}
		return air.Mass{}
	}), Temperature: 10}
	p := New(func() sky.Sun { return sky.DefaultSun }, func() air.Weather { return weather })
	f := p.fall(cam)
	var left, right float32
	for j := range fallRows {
		for i := range fallCols {
			x, _ := cam.Unproject((float32(i)+0.5)/fallCols*vw, 0, 0)
			m := f.Mask[j*fallCols+i]
			if x < vw/4 {
				left += m
			} else if x > 3*vw/4 {
				right += m
			}
		}
	}
	if left <= 0 || right != 0 {
		t.Errorf("the rain's mask holds %v under the raining mass and %v beyond it, want rain on "+
			"its side alone", left, right)
	}
	weather = air.Weather{Rain: 1, Clouds: 1, Temperature: 10}
	if all := p.fall(cam).Drops; f.Drops < all*9/10 || f.Drops > all {
		t.Errorf("rain on part of the view lets %d drops fall, all over it %d; want as thick "+
			"where it falls", f.Drops, all)
	}
	weather = air.Weather{Rain: 1, Clouds: 0.01, Temperature: 10}
	if dry := p.fall(cam); dry.Drops != 0 {
		t.Errorf("rain from almost no clouds lets %d drops fall, want none from their gaps",
			dry.Drops)
	}
}

func domeOf(at func(x, y float32) air.Mass) *air.Dome {
	d := air.NewDome(640, 480, 2000)
	d.Own()
	d.Carry([2]float32{}, func(x, y float32) (air.Mass, int16) { return at(x, y), 0 }, true)
	return d
}
