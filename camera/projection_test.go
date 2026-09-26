package camera_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
)

func TestTopDown_DrawsTheMapAsItIsAndNeverSorts(t *testing.T) {
	var p camera.TopDown
	if sx, sy := p.Project(12, 34, 50); sx != 12 || sy != 34 {
		t.Errorf("Project = (%v, %v), want the world point: height is not drawn", sx, sy)
	}
	if p.Sorts() || !p.Wraps() {
		t.Errorf("Sorts %v Wraps %v, want a plain map: no sorting, wrapping allowed", p.Sorts(), p.Wraps())
	}
}
