package light_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
)

// the world's one observer, a camera from above looking at (x, y)
type eye struct{ cam camera.Camera }

var _ sector.Observers = eye{}

func lookingAt(w, h uint32, x, y float32) eye {
	cam := cameratest.NewFromSpace(w, h, 0)
	cam.SetViewport(2, 2)
	cam.CenterOn(x, y, 0)
	return eye{cam}
}

func (e eye) Observers(each func(cam camera.Camera)) { each(e.cam) }

func TestPlugin_ALampAsleepAtL4LightsNothingAndTheSunShinesWhateverTheSectors(t *testing.T) {
	cfg := levelWorld
	cfg.Sectors = world.SectorsCfg{Buckets: 1}
	cfg.LOD = lod.Ladder{Sight: 300}
	at := lookingAt(1000, 1000, 100, 500)
	sun := light.Distant{Dir: [3]float32{0, 0, 1}, Strength: 0.5, Ambient: 0.25,
		Color: render.Light{1, 1, 1}}
	_, p, tick := litWorldIn(t, cfg, light.Config{}, func(w *world.Plugin, p *light.Plugin) {
		w.Write().Observe(at)
		torch := lampKind(w, "torch", light.Source{Strength: 1, Reach: 50})
		w.Write().Seed(torch.Entry(spot{x: 150, y: 500}), torch.Entry(spot{x: 900, y: 500}))
		p.WithDistant(sun)
	})
	for range 2 {
		tick()
	}
	if lamps := p.Read(0).Lamps(); len(lamps) != 1 || lamps[0].X != 150 {
		t.Errorf("lamps %+v, want the torch at L0 alone", lamps)
	}
	if got := p.Read(0).Distant(); got != sun {
		t.Errorf("the distant light is %+v, want the scene's sun %+v", got, sun)
	}
	at.cam.CenterOn(900, 500, 0)
	for range 2 {
		tick()
	}
	if lamps := p.Read(0).Lamps(); len(lamps) != 1 || lamps[0].X != 900 {
		t.Errorf("looking at the other, lamps %+v, want the torch woken alone", lamps)
	}
}
