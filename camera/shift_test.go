package camera_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
)

type shifted struct {
	camera.Camera
	n lod.Level
}

func (c *shifted) SetLevelShift(n lod.Level) { c.n = n }
func (c *shifted) LevelShift() lod.Level     { return c.n }

func TestLevelShiftOf_IsTheCamerasOwnElseNone(t *testing.T) {
	cam := &shifted{}
	cam.SetLevelShift(2)
	if got := camera.LevelShiftOf(cam); got != 2 {
		t.Errorf("a shifted camera draws %v levels coarser, want 2", got)
	}
	if got := camera.LevelShiftOf(focused{}); got != 0 {
		t.Errorf("a camera without a shift draws %v levels coarser, want none", got)
	}
}
