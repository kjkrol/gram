package selection

import (
	"image/color"
	"maps"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

// MarqueeColor is the outline of a selection box being dragged.
var MarqueeColor = color.RGBA{R: 255, G: 140, B: 0, A: 255}

// marquees are the boxes being dragged, one a camera, in the shown pixels of its frame: a
// camera drawn in layers composes its picture once a layer, in another part of the frame at
// another resolution each time.
type marquees struct{ boxes map[camera.Camera]geom.AABB }

// show keeps box, in cam's pixels as it is drawn whole, for the camera
func (m *marquees) show(cam camera.Camera, box geom.AABB) {
	if m.boxes == nil {
		m.boxes = map[camera.Camera]geom.AABB{}
	}
	k := camera.ResolutionOf(cam)
	m.boxes[cam] = geom.NewAABB(geom.NewVec(box.TopLeft.X/k, box.TopLeft.Y/k),
		geom.NewVec(box.BottomRight.X/k, box.BottomRight.Y/k))
}

func (m *marquees) hide(cam camera.Camera) { delete(m.boxes, cam) }

// copyInto has dst hold the boxes m holds, keeping its memory
func (m *marquees) copyInto(dst *marquees) {
	if dst.boxes == nil {
		dst.boxes = map[camera.Camera]geom.AABB{}
	}
	clear(dst.boxes)
	maps.Copy(dst.boxes, m.boxes)
}

func (m *marquees) compose(f *render.Frame, cam camera.Camera) {
	b, ok := m.boxes[camera.OriginOf(cam)]
	if !ok {
		return
	}
	x0, y0 := camera.InFrame(cam, b.TopLeft.X, b.TopLeft.Y)
	x1, y1 := camera.InFrame(cam, b.BottomRight.X, b.BottomRight.Y)
	box := [4][2]float32{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	outline(f, render.OnTop(marqueeLevel), box, 1, MarqueeColor)
}
