package driving_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
)

func holdsOnly(b control.Binding, hows camera.How) bool {
	for _, how := range []camera.How{camera.Loose, camera.Centred, camera.Behind, camera.Inside} {
		if b.Holds(how) != (how&hows != 0) {
			return false
		}
	}
	return true
}

func TestDefaultKeys_WSADDriveInsideAndBehindEachItsWayTheArrowsBehind(t *testing.T) {
	for _, keys := range driving.DefaultKeys() {
		var hows camera.How
		switch keys.(type) {
		case driving.Shooter:
			hows = camera.Inside
		case driving.Compass, driving.Tank:
			hows = camera.Behind
		}
		for _, b := range keys.Bindings() {
			if !holdsOnly(b, hows) {
				t.Errorf("%T's %v holds elsewhere than %v", keys, b.Trigger, hows)
			}
		}
	}
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	p := driving.NewPlugin(w, motion.NewPlugin(w), selection.NewPlugin(w))
	jumped := false
	for _, b := range p.DefaultBindings() {
		if b.Trigger == (control.KeyDown{Key: control.KeySpace}) {
			jumped = true
			if !holdsOnly(b, camera.Inside|camera.Behind) {
				t.Error("Space jumps elsewhere than riding and behind")
			}
		}
	}
	if !jumped {
		t.Error("no Space among the defaults")
	}
}
