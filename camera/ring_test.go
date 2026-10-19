package camera_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
)

type ringed struct {
	camera.Camera
	rings []camera.Ring
}

func (c ringed) Rings() []camera.Ring   { return c.rings }
func (c ringed) SetRings([]camera.Ring) {}
func (ringed) Crop(_, _, _ float32)     {}
func (ringed) Uncrop()                  {}
func (c ringed) Cropped() bool          { return len(c.rings) > 1 }

func TestRingsOf_AreTheCamerasNoneForOneWithout(t *testing.T) {
	if rings := camera.RingsOf(zoomed{}); rings != nil {
		t.Errorf("a camera without rings has %v, want none", rings)
	}
	if camera.IsCropped(zoomed{}) {
		t.Error("a camera that cannot be cropped is cropped")
	}
	c := ringed{rings: []camera.Ring{{Share: 1, Resolution: 0.5}, {Share: 0.5, Resolution: 1}}}
	if rings := camera.RingsOf(c); len(rings) != 2 || rings[1].Share != 0.5 {
		t.Errorf("the rings are %v, want the camera's two", rings)
	}
	if !camera.IsCropped(c) {
		t.Error("a camera cropped to a ring is not cropped")
	}
}
