package topdown

import (
	"slices"
	"testing"

	contract "github.com/kjkrol/gram/camera"
)

func TestBasicCamera_ACopyDrawsAsItStoodWhileTheCameraMovesOn(t *testing.T) {
	c := NewFromSpace(1000, 1000, 0, testViewport(100, 100, 200, 200)).(*basicCamera)
	layers := []contract.Layer{{Share: 1, Resolution: 0.6}, {Share: 0.5, Resolution: 1}}
	c.SetLayers(layers, contract.Inset)
	if c.Drawn() != contract.Camera(c) || c.Origin() != contract.Camera(c) {
		t.Fatal("before any copy the camera draws itself and is its own origin")
	}
	c.Snap()
	d := c.Drawn().(*basicCamera)
	if d == c || d.Origin() != contract.Camera(c) {
		t.Fatalf("snapped, the camera draws %p of origin %v, want a copy of it", d, d.Origin())
	}
	sx, sy := c.ToScreen(300, 400)
	if dx, dy := d.ToScreen(300, 400); d.Bounds() != c.Bounds() || dx != sx || dy != sy ||
		d.Zoom() != c.Zoom() || !slices.Equal(d.Layers(), c.Layers()) {
		t.Errorf("the copy sees %v at zoom %v in layers %v, want the camera's %v at %v in %v",
			d.Bounds(), d.Zoom(), d.Layers(), c.Bounds(), c.Zoom(), c.Layers())
	}
	bounds := d.Bounds()
	c.Pan(30, 0)
	c.SetLayers(nil, contract.Inset)
	if d.Bounds() != bounds || len(d.Layers()) != 2 {
		t.Errorf("the camera moved on, the copy sees %v in %d layers, want %v in 2 as it stood",
			d.Bounds(), len(d.Layers()), bounds)
	}
	live := c.Bounds()
	d.Crop(contract.Window{W: 50, H: 50}, 0.5)
	if c.Bounds() != live || c.Cropped() {
		t.Errorf("the copy cropped, the camera sees %v cropped %v, want %v whole", c.Bounds(),
			c.Cropped(), live)
	}
	c.Snap()
	if c.Drawn() != contract.Camera(d) || d.Bounds() != c.Bounds() || d.Cropped() {
		t.Errorf("snapped again, the copy is the same, uncropped, seeing the camera's %v; got %v",
			c.Bounds(), d.Bounds())
	}
}
