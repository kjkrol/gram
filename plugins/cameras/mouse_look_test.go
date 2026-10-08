package cameras_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
)

// looker is a camera that may look round with the mouse.
type looker struct {
	camera.Camera
	on bool
}

func (l *looker) MouseLook() bool      { return l.on }
func (l *looker) SetMouseLook(on bool) { l.on = on }

// The key of MouseLook switches looking round with the mouse on the camera the player looks
// through, and off again.
func TestMouseLook_TheKeySwitchesItOnAndOff(t *testing.T) {
	c := newCamp(t, func(*camp, kind.Of[float64]) []kind.Entry { return nil })
	cam := &looker{Camera: c.cam}
	c.look(cam)
	if err := c.one.Bind(cameras.MouseLookKey(control.KeyO)); err != nil {
		t.Fatal(err)
	}
	press := func() {
		ev := &control.InputEvents{}
		ev.AddKeyEvent(control.KeyO, control.ActionPress)
		ev.AddKeyEvent(control.KeyO, control.ActionRelease)
		c.players.EventHandler().HandleEvents(ev)
		c.tick()
	}
	if camera.MouseLooks(cam) {
		t.Fatal("the camera looks round with the mouse before anyone asked")
	}
	press()
	if !camera.MouseLooks(cam) {
		t.Error("O did not switch looking round with the mouse on")
	}
	press()
	if camera.MouseLooks(cam) {
		t.Error("O again did not switch it off")
	}
}
