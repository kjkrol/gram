package world_test

import (
	"testing"

	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/world"
)

func TestViewFor_IsTheCamerasOwnViewMadeOnce(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	if w.ViewFor(w.Camera()) != w.View() {
		t.Error("the world's camera has a View other than View()")
	}
	other := icamera.NewFromSpace(1000, 1000, 0)
	v := w.ViewFor(other)
	if v == nil || v == w.View() {
		t.Fatal("a second camera shares the world camera's View")
	}
	if again := w.ViewFor(other); again != v {
		t.Error("a second call made another View for the same camera")
	}
}
