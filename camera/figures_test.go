package camera_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
)

type figured struct {
	camera.Camera
	sharp bool
}

func (f *figured) SetSharpFigures(on bool) { f.sharp = on }
func (f *figured) SharpFigures() bool      { return f.sharp }

func TestSharpFiguresOf_IsTheCamerasChoiceFalseForOneWithout(t *testing.T) {
	if camera.SharpFiguresOf(zoomed{}) {
		t.Error("a camera without the choice draws its figures sharp")
	}
	f := &figured{}
	f.SetSharpFigures(true)
	if !camera.SharpFiguresOf(f) {
		t.Error("the figures a camera chose to draw sharp are not")
	}
}
