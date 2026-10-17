package cameras

import (
	"testing"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func TestZones_TheCommandSwitchesTheZonesDrawn(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 800},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := NewPlugin(w, Config{})
	if err := w.Carry(p); err != nil {
		t.Fatal(err)
	}
	if p.ZonesShown() {
		t.Error("the zones start shown")
	}
	cam := cameratest.NewFromSpace(1000, 800, 0)
	pieces := func() int {
		var f render.Frame
		f.Reset(cam)
		p.zonesView.Compose(&f, cam)
		f.Each(func(at render.At, _ []render.Vertex) {
			if at != render.OnTop(zonesLevel) {
				t.Errorf("a zone lies at %v, want a mark over every other", at)
			}
		})
		return f.Len()
	}
	if n := pieces(); n != 0 {
		t.Errorf("hidden, the zones drew %d pieces", n)
	}
	for i, want := range []bool{true, false} {
		if !w.Carrier().Put(command.Nobody, Zones{}) {
			t.Fatal("the world carries no cameras.Zones")
		}
		p.switchZones()
		if p.ZonesShown() != want {
			t.Errorf("after %d Zones commands shown is %v, want %v", i+1, p.ZonesShown(), want)
		}
	}
	p.ShowZones(true)
	if n := pieces(); n != 8 {
		t.Errorf("shown, the zones drew %d pieces, want the four strips and four corners", n)
	}
	var keys []string
	for _, b := range p.DefaultBindings() {
		if b.Type().Name() == "Zones" {
			keys = append(keys, b.Trigger.(control.KeyPress).Key.String())
		}
	}
	if len(keys) != 1 || keys[0] != "H" {
		t.Errorf("Zones is bound to %v, want Shift+H", keys)
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

func TestPlugin_DrawsTheZones(t *testing.T) {
	p := NewPlugin(world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}), Config{})
	ctx := &drawsCtx{}
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.drawn) != 1 || ctx.drawn[0] != render.Source(p.zonesView) {
		t.Errorf("the cameras draw %v, want the zones", ctx.drawn)
	}
}
