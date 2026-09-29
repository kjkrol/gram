package control_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// A binding holds in every camera mode unless In says which, and two bindings overlap where they
// hold in one mode both.
func TestBinding_InHoldsInItsModesOnly(t *testing.T) {
	b := control.Command(control.KeyHeld{Key: control.KeyW}, "w", func(control.Context) (int, bool) { return 1, true })
	if !b.Holds(camera.Free) || !b.Holds(camera.FirstPerson) {
		t.Error("a binding given no modes does not hold in every one")
	}
	riding, free := b.In(camera.FirstPerson), b.In(camera.Free)
	if riding.Holds(camera.Free) || !riding.Holds(camera.FirstPerson) || !free.Holds(camera.Free) || free.Holds(camera.FirstPerson) {
		t.Error("a binding In one mode holds in the other, or not in its own")
	}
	if free.Overlaps(riding) || !b.Overlaps(riding) || !riding.Overlaps(riding) || !b.In(camera.Free, camera.FirstPerson).Overlaps(riding) {
		t.Error("overlaps wrong: bindings in two modes apart overlap, or ones sharing a mode do not")
	}
	if cmd, ok := riding.Build(control.Context{}); !ok || cmd != 1 {
		t.Errorf("In lost the binding's command: built %v, %v", cmd, ok)
	}
}

// A camera is FirstPerson while it says it rides in an entity, Free otherwise.
func TestModeOf_IsFirstPersonWhileTheCameraRides(t *testing.T) {
	r := &rider{}
	if camera.ModeOf(r) != camera.Free {
		t.Error("a camera not riding is not Free")
	}
	r.on = true
	if camera.ModeOf(r) != camera.FirstPerson {
		t.Error("a camera riding is not FirstPerson")
	}
}

type rider struct {
	camera.Camera
	on bool
}

func (r *rider) FirstPerson() bool { return r.on }
