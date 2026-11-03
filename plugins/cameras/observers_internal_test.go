package cameras

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
)

func TestObservers_AreTheCamerasButAMinimaps(t *testing.T) {
	p := NewPlugin(world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}), Config{})
	main := p.write.New(FromAbove, camera.Config{})
	p.write.New(FromAbove, camera.Config{Whole: true}).SetViewport(100, 100)
	var got []camera.Camera
	observers{p}.Observers(func(cam camera.Camera) { got = append(got, cam) })
	if len(got) != 1 || got[0] != main {
		t.Errorf("observers %v, want the main camera alone", got)
	}
}
