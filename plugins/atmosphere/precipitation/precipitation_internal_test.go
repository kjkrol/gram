package precipitation

import (
	"os"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

func TestDrops_FallUnderTheLowCloudsWhereTheyAreBigEnoughToMakeOut(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w, cameras.Config{}).New(cameras.FromAbove, camera.Config{})
	cam.SetViewport(640, 480)
	bases := air.Bases(world.Scale{})
	var weather air.Weather
	d := NewDrops(func() light.Distant { return sky.DefaultSun }, func() air.Weather { return weather })
	drawn := func() bool {
		var fr render.Frame
		fr.Reset(cam)
		d.Compose(&fr, cam)
		some := false
		fr.Places(func(render.Pass, render.At) { some = true })
		return some
	}
	weather = air.Weather{Clouds: 0.9, Billow: 1, Rain: 1, Temperature: 10, Bases: bases}
	if drawn() {
		t.Error("a drop half a world unit long is drawn a pixel a world unit, too small to see")
	}
	for range 30 {
		cam.ZoomIn(1.25, 320, 240)
	}
	if !drawn() {
		t.Error("zoomed in under the rain no drop is drawn")
	}
	for _, dry := range []air.Weather{
		{Rain: 1, Temperature: 10, Bases: bases},
		{Clouds: 0.9, Billow: 1, Temperature: 10, Bases: bases},
	} {
		weather = dry
		if drawn() {
			t.Errorf("%+v: drops are drawn, want none", dry)
		}
	}
}

func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}

func TestDrops_SetPutsItsOwnOverTheFramesWithoutAllocating(t *testing.T) {
	d := Drops{uniforms: map[string]any{}}
	d.opts.Uniforms = map[string]any{}
	d.set("DropView", 1, 2)
	d.opts.Uniforms["DropView"] = []float32{9, 9}
	n := testing.AllocsPerRun(10, func() { d.set("DropView", 3, 4) })
	if got := d.opts.Uniforms["DropView"].([]float32); n != 0 || got[0] != 3 || got[1] != 4 {
		t.Errorf("DropView set over another: %v allocations, %v; want none and [3 4]", n, got)
	}
}
