package overcast

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
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

// Drawn on the GPU over a flat world under clouds, the clouds' shadows darken the ground in
// patches, as much as the thickest cloud takes of the sun and no more, and leave it clear between
// them; with no clouds nothing is drawn.
func TestClouds_ShadeAFlatWorldOnTheGPU(t *testing.T) {
	needGPU(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 4096, Height: 4096}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w).New(cameras.TopDown(), camera.Config{})
	cam.SetViewport(512, 384)
	weather := air.Weather{Clouds: 0.7, Drift: [2]float32{130, -40}}
	c := New(func() sky.Sun {
		return sky.Sun{Dir: [3]float32{0.3, 0.2, 1}, Strength: 1, Sky: render.Light{0.5, 0.7, 1}}
	}, func() air.Weather { return weather })
	screen := render.NewImage(512, 384)
	pix := make([]byte, 4*512*384)
	draw := func() {
		screen.Fill(color.RGBA{R: 200, G: 200, B: 200, A: 255})
		render.NewComposer(c).DrawWorld(screen, cam)
		screen.ReadPixels(pix)
	}
	draw()
	if dir := os.Getenv("GRAM_SHOTS"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "clouds.png"))
		if err == nil {
			png.Encode(f, &image.RGBA{Pix: pix, Stride: 4 * 512, Rect: image.Rect(0, 0, 512, 384)})
			f.Close()
		}
	}
	darkest, clear := 255, 0
	for i := 0; i < len(pix); i += 4 {
		darkest = min(darkest, int(pix[i]))
		if pix[i] == 200 {
			clear++
		}
	}
	// the thickest cloud takes half the sun: 200 down to 100 at most
	if darkest > 150 || darkest < 99 || clear == 0 {
		t.Errorf("under clouds the grey 200 is %d at its darkest with %d pixels clear, want patches down towards 100 and clear sky between", darkest, clear)
	}
	weather.Clouds = 0
	draw()
	for i := 0; i < len(pix); i += 4 {
		if pix[i] != 200 {
			t.Fatalf("with no clouds pixel %d is %d, want the grey untouched", i/4, pix[i])
		}
	}
}
