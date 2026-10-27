package control_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

type forward struct{}

func (forward) Command() {}

func TestBinding_InHoldsInItsHowsOnly(t *testing.T) {
	b := control.Build(control.KeyHeld{Key: control.KeyW}, "w", func(control.Context) (forward, bool) { return forward{}, true })
	if !b.Holds(camera.Loose) || !b.Holds(camera.Inside) {
		t.Error("a binding given no hows does not hold in every one")
	}
	riding, loose := b.In(camera.Inside), b.In(camera.Outside)
	if riding.Holds(camera.Loose) || !riding.Holds(camera.Inside) || !loose.Holds(camera.Loose) || !loose.Holds(camera.Behind) || loose.Holds(camera.Inside) {
		t.Error("a binding In one How holds in another, or not in its own")
	}
	if loose.Overlaps(riding) || !b.Overlaps(riding) || !riding.Overlaps(riding) || !b.In(camera.Loose, camera.Inside).Overlaps(riding) {
		t.Error("overlaps wrong: bindings in two hows apart overlap, or ones sharing a How do not")
	}
	free := b.In(camera.Free)
	if !free.Holds(camera.Loose) || !free.Holds(camera.Centred) || free.Holds(camera.Behind) ||
		free.Holds(camera.Inside) {
		t.Error("Free holds other than loose and centred")
	}
	if cmd, ok := riding.Build(control.Context{}); !ok || cmd != (forward{}) {
		t.Errorf("In lost the binding's command: built %v, %v", cmd, ok)
	}
}

func TestHowOf_ReadsTheCamerasFastening(t *testing.T) {
	f := &fastenable{}
	if camera.HowOf(f) != camera.Loose || camera.HowOf(f.Camera) != camera.Loose {
		t.Error("a camera fastened to nothing, or one that cannot be, is not Loose")
	}
	f.Fasten(camera.Fastening{Entity: 7, How: camera.Inside})
	if camera.HowOf(f) != camera.Inside {
		t.Error("a camera fastened Inside is not Inside")
	}
}

type fastenable struct {
	camera.Camera
	f camera.Fastening
}

func (c *fastenable) Fasten(f camera.Fastening)   { c.f = f }
func (c *fastenable) Fastening() camera.Fastening { return c.f }
