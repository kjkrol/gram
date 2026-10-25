package selection

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render"
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

func TestMarquees_ComposeTheBoxInThePixelsOfEveryLayersPass(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 800, 0)
	cam.SetViewport(400, 300)
	var m marquees
	m.show(cam, geom.NewAABB(geom.NewVec(100, 100), geom.NewVec(200, 200)))
	var f render.Frame
	bounds := func() (minX, minY, maxX, maxY float32) {
		minX, minY, maxX, maxY = 1e9, 1e9, -1e9, -1e9
		f.Each(func(_ render.At, verts []render.Vertex) {
			for _, v := range verts {
				minX, minY = min(minX, v.DstX), min(minY, v.DstY)
				maxX, maxY = max(maxX, v.DstX), max(maxY, v.DstY)
			}
		})
		return
	}
	near := func(a, b float32) bool { return f32.Abs(float32(a-b)) <= 1 }
	f.Reset(cam)
	m.compose(&f, cam)
	if x0, y0, x1, y1 := bounds(); !near(x0, 100) || !near(y0, 100) || !near(x1, 200) ||
		!near(y1, 200) {
		t.Errorf("drawn whole, the box spans (%v, %v)-(%v, %v), want (100, 100)-(200, 200)",
			x0, y0, x1, y1)
	}
	// the central 200x150 of the 400x300 frame at a half: the box's corner (100, 100) of the
	// frame is the crop's corner, 100 shown pixels in are 50 of the crop's
	cam.(camera.Cropping).Crop(camera.Window{X: 100, Y: 75, W: 200, H: 150}, 0.5)
	f.Reset(cam)
	m.compose(&f, cam)
	if x0, y0, x1, y1 := bounds(); !near(x0, 0) || !near(y0, 12.5) || !near(x1, 50) ||
		!near(y1, 62.5) {
		t.Errorf("drawn a layer, the box spans (%v, %v)-(%v, %v), want (0, 12.5)-(50, 62.5)",
			x0, y0, x1, y1)
	}
	cam.(camera.Cropping).Uncrop()
}

func TestMarquees_ComposeTheBoxDraggedInACameraThroughItsCopy(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 800, 0)
	cam.SetViewport(400, 300)
	var m marquees
	m.show(cam, geom.NewAABB(geom.NewVec(100, 100), geom.NewVec(200, 200)))
	cam.(camera.Snapped).Snap()
	var f render.Frame
	f.Reset(camera.DrawnOf(cam))
	m.compose(&f, camera.DrawnOf(cam))
	pieces := 0
	f.Each(func(render.At, []render.Vertex) { pieces++ })
	if pieces == 0 {
		t.Error("drawn through the camera's copy, the box dragged in the camera is not composed")
	}
}
