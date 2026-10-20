package camera_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
)

type layered struct {
	camera.Camera
	layers []camera.Layer
}

func (c layered) Layers() []camera.Layer     { return c.layers }
func (c layered) SetLayers([]camera.Layer)   {}
func (layered) Crop(_, _, _ float32)         {}
func (layered) Uncrop()                      {}
func (c layered) Cropped() bool              { return len(c.layers) > 1 }
func (layered) Frame() (float32, float32)    { return 400, 300 }
func (layered) Viewport() (float32, float32) { return 100, 75 }
func (layered) Resolution() float32          { return 0.5 }
func (layered) SetResolution(float32)        {}

func TestLayersOf_AreTheCamerasNoneForOneWithout(t *testing.T) {
	if layers := camera.LayersOf(zoomed{}); layers != nil {
		t.Errorf("a camera without layers has %v, want none", layers)
	}
	if camera.IsCropped(zoomed{}) {
		t.Error("a camera that cannot be cropped is cropped")
	}
	c := layered{layers: []camera.Layer{{Share: 1, Resolution: 0.5}, {Share: 0.5, Resolution: 1}}}
	if layers := camera.LayersOf(c); len(layers) != 2 || layers[1].Share != 0.5 {
		t.Errorf("the layers are %v, want the camera's two", layers)
	}
	if !camera.IsCropped(c) {
		t.Error("a camera cropped to a layer is not cropped")
	}
}

func TestInFrame_IsTheShownPixelInTheCropsPixelsElseAtTheResolution(t *testing.T) {
	// a 400x300 frame cropped to its central 200x150 drawn at a half: the crop begins at
	// (100, 75) of the frame and a shown pixel is half a camera pixel
	c := layered{layers: []camera.Layer{{Share: 1, Resolution: 0.5}, {Share: 0.5, Resolution: 0.5}}}
	if x, y := camera.InFrame(c, 100, 75); x != 0 || y != 0 {
		t.Errorf("the crop's corner lies at (%v, %v) of its pixels, want (0, 0)", x, y)
	}
	if x, y := camera.InFrame(c, 300, 225); x != 100 || y != 75 {
		t.Errorf("the crop's far corner lies at (%v, %v), want (100, 75)", x, y)
	}
	if x, y := camera.InFrame(c, 0, 0); x != -50 || y != -37.5 {
		t.Errorf("the frame's corner lies at (%v, %v), want (-50, -37.5), off the crop", x, y)
	}
	whole := layered{layers: []camera.Layer{{Share: 1, Resolution: 0.5}}}
	if x, y := camera.InFrame(whole, 100, 75); x != 50 || y != 37.5 {
		t.Errorf("uncropped, the pixel lies at (%v, %v), want it at the resolution", x, y)
	}
	if x, y := camera.InFrame(zoomed{}, 100, 75); x != 100 || y != 75 {
		t.Errorf("through a plain camera the pixel lies at (%v, %v), want itself", x, y)
	}
}
