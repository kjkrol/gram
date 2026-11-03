package camera_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
)

type fronted struct{ camera.Camera }

func (fronted) InFront(_, y, _ float32) bool { return y > 0 }

func TestInFront_IsTheCamerasOwnElseEveryPoint(t *testing.T) {
	if camera.InFront(fronted{}, 0, -1, 0) || !camera.InFront(fronted{}, 0, 1, 0) {
		t.Error("a fronted camera's own answer is not taken")
	}
	if !camera.InFront(focused{}, 0, -1, 0) {
		t.Error("through a camera without an eye a point lies behind it")
	}
}
