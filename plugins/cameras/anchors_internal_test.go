package cameras

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
)

func TestAnchors_AreWhatTheCamerasObserveButAMinimap(t *testing.T) {
	p := NewPlugin(world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}), Config{})
	main := p.write.New(FromAbove, camera.Config{})
	main.SetViewport(200, 200)
	main.CenterOn(300, 400, 0)
	p.write.New(FromAbove, camera.Config{Whole: true}).SetViewport(100, 100)
	var got [][2]float32
	anchors{p}.Anchors(func(x, y float32) { got = append(got, [2]float32{x, y}) })
	if len(got) != 1 || got[0] != [2]float32{300, 400} {
		t.Errorf("anchors %v, want the main camera's middle (300, 400) alone", got)
	}
}
