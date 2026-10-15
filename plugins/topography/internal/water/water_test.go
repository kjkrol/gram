package water_test

import (
	"image/color"
	"os"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	_ "github.com/kjkrol/gram/plugins/atmosphere/air" // the clouds' functions the water calls
	"github.com/kjkrol/gram/plugins/topography/internal/water"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}

type sea struct {
	atlas           *render.Atlas
	horizon, zenith [3]float32
}

func (*sea) Init(*goke.SysInit) {}
func (s *sea) Compose(f *render.Frame, _ camera.Camera) {
	f.Uniform("Fog", s.horizon[:]...)
	f.Uniform("SkyOverhead", s.zenith[:]...)
	f.Uniform("Toward", 1, 0, 0.05) // the eye low over the water
	f.SpriteRect(render.At{}, s.atlas, 1, 0, 0, 64, 64, render.Even(1))
	world := render.World{{0, 0}, {64, 0}, {0, 64}, {64, 64}}
	water.Glint(f, world, [4]float32{1, 1, 1, 1}, [4]float32{}, water.Shores{})
}

func TestGlint_TheSeaReflectsTheSkysOwnColours(t *testing.T) {
	needGPU(t)
	atlas := render.NewAtlas()
	atlas.Add(render.SpriteID(1), 8, render.Solid(color.RGBA{A: 255}))
	atlas.Close()
	red, green := [3]float32{1, 0, 0}, [3]float32{0, 1, 0}
	middle := func(horizon, zenith [3]float32) [4]byte {
		screen := render.NewImage(64, 64)
		render.NewComposer(&sea{atlas: atlas, horizon: horizon, zenith: zenith}).
			DrawWorld(screen, icamera.NewFromSpace(64, 64, 0))
		pix := make([]byte, 4*64*64)
		screen.ReadPixels(pix)
		i := 4 * (32*64 + 32)
		return [4]byte(pix[i : i+4])
	}
	if p := middle(red, green); p[0] <= p[1] {
		t.Errorf("under a red horizon and a green zenith the sea seen low is %v, want it red", p)
	}
	if p := middle(green, red); p[1] <= p[0] {
		t.Errorf("under a green horizon and a red zenith the sea seen low is %v, want it green", p)
	}
}
