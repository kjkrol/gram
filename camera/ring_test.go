package camera_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
)

type ringed struct {
	camera.Camera
	rings []camera.Ring
}

func (c ringed) Rings() []camera.Ring       { return c.rings }
func (c ringed) SetRings([]camera.Ring)     {}
func (ringed) Crop(_, _, _ float32)         {}
func (ringed) Uncrop()                      {}
func (c ringed) Cropped() bool              { return len(c.rings) > 1 }
func (ringed) Frame() (float32, float32)    { return 400, 300 }
func (ringed) Viewport() (float32, float32) { return 100, 75 }
func (ringed) Resolution() float32          { return 0.5 }
func (ringed) SetResolution(float32)        {}

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

func TestInFrame_IsTheShownPixelInTheCropsPixelsElseAtTheResolution(t *testing.T) {
	// a 400x300 frame cropped to its central 200x150 drawn at a half: the crop begins at
	// (100, 75) of the frame and a shown pixel is half a camera pixel
	c := ringed{rings: []camera.Ring{{Share: 1, Resolution: 0.5}, {Share: 0.5, Resolution: 0.5}}}
	if x, y := camera.InFrame(c, 100, 75); x != 0 || y != 0 {
		t.Errorf("the crop's corner lies at (%v, %v) of its pixels, want (0, 0)", x, y)
	}
	if x, y := camera.InFrame(c, 300, 225); x != 100 || y != 75 {
		t.Errorf("the crop's far corner lies at (%v, %v), want (100, 75)", x, y)
	}
	if x, y := camera.InFrame(c, 0, 0); x != -50 || y != -37.5 {
		t.Errorf("the frame's corner lies at (%v, %v), want (-50, -37.5), off the crop", x, y)
	}
	whole := ringed{rings: []camera.Ring{{Share: 1, Resolution: 0.5}}}
	if x, y := camera.InFrame(whole, 100, 75); x != 50 || y != 37.5 {
		t.Errorf("uncropped, the pixel lies at (%v, %v), want it at the resolution", x, y)
	}
	if x, y := camera.InFrame(zoomed{}, 100, 75); x != 100 || y != 75 {
		t.Errorf("through a plain camera the pixel lies at (%v, %v), want itself", x, y)
	}
}
