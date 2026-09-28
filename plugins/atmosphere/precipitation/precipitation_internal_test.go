package precipitation

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func TestPrecipitation_FallsAsMuchAsTheWeatherSaysAndNotAtAllWhenDry(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	weather := air.Weather{}
	p := New(func() sky.Sun { return sky.DefaultSun }, func() air.Weather { return weather })
	count := func() (n int, tier render.Tier) {
		var f render.Frame
		f.Reset(w.Camera())
		p.Compose(&f, w.Camera())
		f.Each(func(t render.Tier, _ float32, _ []ebiten.Vertex) { n, tier = n+1, t })
		return
	}
	if n, _ := count(); n != 0 {
		t.Errorf("a dry sky drew %d drops", n)
	}
	weather = air.Weather{Rain: 0.5}
	half, tier := count()
	weather = air.Weather{Rain: 1}
	full, _ := count()
	if half == 0 || tier != render.Air || full < 2*half-1 || full > 2*half+1 {
		t.Errorf("half a rain drew %d drops on tier %v, a full one %d; want some in the air, twice as many", half, tier, full)
	}
	weather = air.Weather{Snow: 1}
	if n, _ := count(); n == 0 {
		t.Error("a snowfall drew no flakes")
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
	weather := air.Weather{Rain: 1, Wind: [2]float32{10, 0}}
	p := New(func() sky.Sun { return sky.DefaultSun }, func() air.Weather { return weather })
	cam := flung{w.Camera()}
	var f render.Frame
	f.Reset(cam)
	p.Compose(&f, cam)
	n := 0
	f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) {
		n++
		across := max(v[0].DstX, v[1].DstX, v[2].DstX, v[3].DstX) - min(v[0].DstX, v[1].DstX, v[2].DstX, v[3].DstX)
		if across > 2*rainDrop {
			t.Errorf("a streak of rain spans %v pixels across, want no more than it falls, %v", across, rainDrop)
		}
	})
	if n == 0 {
		t.Fatal("the rain drew nothing")
	}
	x0, _ := cam.Project(500, 500, 0)
	x1, _ := cam.Project(510, 500, 0)
	if want := (x1 - x0) * rainSlant; want <= 0 {
		t.Fatalf("the wind carries nothing across the middle: %v", want)
	}
}
