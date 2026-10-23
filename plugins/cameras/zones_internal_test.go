package cameras

import (
	"testing"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// the zones p draws on the glass over a picture 1000 by 800 through cam: the pixels' alpha
func glassOf(t *testing.T, p *Plugin, cam camera.Camera) func(x, y int) byte {
	t.Helper()
	img := render.NewImage(1000, 800)
	t.Cleanup(img.Deallocate)
	p.zonesView.DrawGlass(img, cam)
	pix := make([]byte, 4*1000*800)
	img.ReadPixels(pix)
	return func(x, y int) byte { return pix[4*(y*1000+x)+3] }
}

func TestZones_TheCommandSwitchesTheZonesDrawn(t *testing.T) {
	gputest.Need(t)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 800},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := NewPlugin(w, Config{})
	if err := w.Carry(p); err != nil {
		t.Fatal(err)
	}
	if p.read.ZonesShown() {
		t.Error("the zones start shown")
	}
	cam := cameratest.NewFromSpace(1000, 800, 0)
	if a := glassOf(t, p, cam)(999, 400); a != 0 {
		t.Errorf("hidden, the zones drew the right edge %v opaque", a)
	}
	for i, want := range []bool{true, false} {
		if !w.Write().Carrier().Put(command.Nobody, Zones{Shown: want}) {
			t.Fatal("the world carries no cameras.Zones")
		}
		p.switchZones()
		if p.read.ZonesShown() != want {
			t.Errorf("after %d Zones commands shown is %v, want %v", i+1, p.read.ZonesShown(), want)
		}
	}
	p.write.ShowZones(true)
	alpha := glassOf(t, p, cam)
	if alpha(999, 400) == 0 || alpha(500, 799) == 0 || alpha(0, 400) == 0 || alpha(500, 0) == 0 ||
		alpha(2, 2) == 0 || alpha(997, 797) == 0 {
		t.Error("shown, the zones leave an edge or a corner bare")
	}
	if a := glassOf(t, p, p.write.New(FromAbove, camera.Config{Whole: true}))(999, 400); a != 0 {
		t.Errorf("a camera of the whole world got the zones %v opaque", a)
	}
}

type drawsCtx struct {
	plugin.Installer
	drawn []render.Source
}

func (*drawsCtx) UseModule(goke.Module)      {}
func (*drawsCtx) Hosts(...host.Host)         {}
func (*drawsCtx) ECS() *goke.ECS             { return goke.New() }
func (c *drawsCtx) Draws(s ...render.Source) { c.drawn = append(c.drawn, s...) }

func TestPlugin_DrawsTheZonesAndTheLayers(t *testing.T) {
	p := NewPlugin(world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}), Config{})
	ctx := &drawsCtx{}
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.drawn) != 2 || ctx.drawn[0] != render.Source(p.zonesView) ||
		ctx.drawn[1] != render.Source(p.layersView) {
		t.Errorf("the cameras draw %v, want the zones and the layers", ctx.drawn)
	}
}

func TestZones_FadeFromClearInsideToTheColourAtTheEdge(t *testing.T) {
	gputest.Need(t)
	p := NewPlugin(world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 800},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}), Config{})
	p.write.ShowZones(true)
	alpha := glassOf(t, p, cameratest.NewFromSpace(1000, 800, 0))
	edge, half, bound := alpha(999, 400), alpha(970, 400), alpha(941, 400)
	if edge < zoneColor.A-2 || half < edge/3 || half > 2*edge/3 || bound > 3 {
		t.Errorf("the right strip is %v opaque at the edge, %v halfway in and %v at its inner "+
			"bound, want the colour's %v fading to clear", edge, half, bound, zoneColor.A)
	}
	if a := alpha(900, 400); a != 0 {
		t.Errorf("past the strip the glass is %v opaque, want bare", a)
	}
	if corner, inner := alpha(998, 1), alpha(881, 118); corner < zoneColor.A-2 || inner > 3 {
		t.Errorf("the top-right corner is %v opaque at the edge and %v at its inner corner, want "+
			"the colour fading to clear", corner, inner)
	}
}
