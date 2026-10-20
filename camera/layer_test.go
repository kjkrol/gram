package camera_test

import (
	"reflect"
	"testing"

	"github.com/kjkrol/gram/camera"
)

type layered struct {
	camera.Camera
	layers []camera.Layer
}

func (c layered) Layers() []camera.Layer                  { return c.layers }
func (c layered) SetLayers([]camera.Layer, camera.Layout) {}
func (layered) Layout() camera.Layout                     { return camera.Inset }
func (layered) Crop(_, _, _ float32)                      {}
func (layered) Uncrop()                                   {}
func (c layered) Cropped() bool                           { return len(c.layers) > 1 }
func (layered) Frame() (float32, float32)                 { return 400, 300 }
func (layered) Viewport() (float32, float32)              { return 100, 75 }
func (layered) Resolution() float32                       { return 0.5 }
func (layered) SetResolution(float32)                     {}

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

func TestArrange_InsetWindowsAreCentredAndFootedOnesStandOnTheBottomEdge(t *testing.T) {
	layers := []camera.Layer{{Share: 1, Resolution: 0.6}, {Share: 0.6, Resolution: 0.8},
		{Share: 0.35, Resolution: 1}}
	inset := camera.Arrange(camera.Inset, layers, 400, 300)
	wantInset := []camera.Window{{X: 0, Y: 0, W: 400, H: 300}, {X: 80, Y: 60, W: 240, H: 180},
		{X: 130, Y: 97, W: 140, H: 106}}
	if !reflect.DeepEqual(inset, wantInset) {
		t.Errorf("inset, the windows are %v, want %v", inset, wantInset)
	}
	footed := camera.Arrange(camera.Footed, layers, 400, 300)
	wantFooted := []camera.Window{{X: 0, Y: 0, W: 400, H: 300}, {X: 80, Y: 120, W: 240, H: 180},
		{X: 130, Y: 195, W: 140, H: 105}}
	if !reflect.DeepEqual(footed, wantFooted) {
		t.Errorf("footed, the windows are %v, want %v", footed, wantFooted)
	}
}

func TestArrange_RoundsToWholePixelsWithEvenMargins(t *testing.T) {
	wins := camera.Arrange(camera.Inset, []camera.Layer{{Share: 0.5, Resolution: 1},
		{Share: 0.001, Resolution: 1}, {Share: 2, Resolution: 1}}, 101, 75)
	for i, w := range wins {
		if w.X*2+w.W != 101 || w.Y*2+w.H != 75 {
			t.Errorf("layer %d at %v is not centred in 101 by 75", i, w)
		}
		if w.W < 2 || w.H < 2 || w.W > 101 || w.H > 75 {
			t.Errorf("layer %d at %v is outside 2 to the frame", i, w)
		}
		if w.X != float32(int(w.X)) || w.W != float32(int(w.W)) {
			t.Errorf("layer %d at %v is not whole pixels", i, w)
		}
	}
	if camera.LayoutOf(zoomed{}) != camera.Inset {
		t.Error("a camera without layers has a layout other than Inset")
	}
}
