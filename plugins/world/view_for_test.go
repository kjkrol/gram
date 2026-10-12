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
	one, other := icamera.NewFromSpace(1000, 1000, 0), icamera.NewFromSpace(1000, 1000, 0)
	v := w.ViewFor(one)
	if v == nil || w.ViewFor(other) == v {
		t.Fatal("two cameras share a View")
	}
	if again := w.ViewFor(one); again != v {
		t.Error("a second call made another View for the same camera")
	}
}
