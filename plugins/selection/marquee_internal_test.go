package selection

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
)

func TestMarquees_ABoxBelongsToTheCameraItIsDraggedIn(t *testing.T) {
	var m marquees
	a, b := cameratest.NewFromSpace(100, 100, 0), cameratest.NewFromSpace(100, 100, 0)
	box := geom.NewAABBAt(geom.NewVec(1, 2), 3, 4)
	m.show(a, box)
	if got, ok := m.boxes[a]; !ok || got != box {
		t.Errorf("camera a's box = %v (%v), want %v", got, ok, box)
	}
	if _, ok := m.boxes[b]; ok {
		t.Error("a box dragged through camera a shows through camera b")
	}
	m.hide(a)
	if _, ok := m.boxes[a]; ok {
		t.Error("the box stays after hide")
	}
}
