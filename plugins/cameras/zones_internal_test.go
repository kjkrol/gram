package cameras

import (
	"testing"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func TestZones_TheCommandSwitchesTheZonesDrawn(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 800},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := NewPlugin(w)
	if err := w.Carry(p); err != nil {
		t.Fatal(err)
	}
	if p.ZonesShown() {
		t.Error("the zones start shown")
	}
	cam := icamera.NewFromSpace(1000, 800, 0)
	pieces := func() int {
		var f render.Frame
		f.Reset(cam)
		p.Renderer().(render.Source).Compose(&f, cam)
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
