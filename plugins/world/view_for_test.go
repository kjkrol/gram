package world_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
)

func TestNewView_IsTheCamerasOwnViewMadeOnceAndReadAfter(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	one, other := cameratest.NewFromSpace(1000, 1000, 0), cameratest.NewFromSpace(1000, 1000, 0)
	if w.Read(0).ViewFor(one) != nil {
		t.Fatal("a camera has a View before one was made")
	}
	v := w.Write().NewView(one)
	if v == nil || w.Write().NewView(other) == v {
		t.Fatal("two cameras share a View")
	}
	if again := w.Write().NewView(one); again != v {
		t.Error("a second call made another View for the same camera")
	}
	if read := w.Read(0).ViewFor(one); read != v {
		t.Error("the View read is not the one made")
	}
}
