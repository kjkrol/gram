package world

import (
	"testing"

	"github.com/kjkrol/gram/plugins/cameras/cameratest"
)

func TestViewFor_IsTheFramesOnceOneWasPublished(t *testing.T) {
	w := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1000, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	one, other := cameratest.NewFromSpace(1000, 1000, 0), cameratest.NewFromSpace(1000, 1000, 0)
	live := w.Write().NewView(one)
	if w.Read(0).ViewFor(one) != live {
		t.Fatal("before any frame was published, the View read is not the camera's own")
	}
	w.renderer.Publish()
	w.renderer.Swap()
	w.Write().NewView(other)
	if v := w.Read(0).ViewFor(one); v == nil || v == live {
		t.Errorf("published, the View read is %p, want the frame's copy, not the live %p", v, live)
	}
	if v := w.Read(0).ViewFor(other); v != nil {
		t.Errorf("a camera's View made after the frame was published is read %p, want none: "+
			"the frame holds none of it", v)
	}
}
