package atmosphere

import (
	"testing"

	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func TestPlugin_TheLowCloudsLookAsTheSettingsPublished(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	a := NewPlugin(w, nil, Config{})
	ctx := &drawsCtx{}
	if err := a.Install(ctx); err != nil {
		t.Fatal(err)
	}
	c := render.NewComposer(ctx.drawn[1])
	cam := cameratest.NewFromSpace(128, 128, 0)
	a.write.ShowClouds(LowBodies)
	a.Publish()
	a.Swap()
	a.write.ShowClouds(ShadowsAlone)
	c.DrawWorld(nil, cam)
	if !a.aloft.Shown() || a.skyClouds().Hidden {
		t.Errorf("hidden after the frame was published, the clouds are seen from above %v, on the "+
			"sky %v; want both till the next", a.aloft.Shown(), !a.skyClouds().Hidden)
	}
	a.Publish()
	a.Swap()
	c.DrawWorld(nil, cam)
	if a.aloft.Shown() || !a.skyClouds().Hidden {
		t.Errorf("published hidden, the clouds are seen from above %v, on the sky %v; want neither",
			a.aloft.Shown(), !a.skyClouds().Hidden)
	}
}
