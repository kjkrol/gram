package selection

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

// MarqueeColor is the outline of a selection box being dragged.
var MarqueeColor = color.RGBA{R: 255, G: 140, B: 0, A: 255}

// marquees are the selection boxes being dragged, one per camera they are dragged in.
type marquees struct{ boxes map[camera.Camera]geom.AABB }

func (m *marquees) show(cam camera.Camera, box geom.AABB) {
	if m.boxes == nil {
		m.boxes = map[camera.Camera]geom.AABB{}
	}
	m.boxes[cam] = box
}

func (m *marquees) hide(cam camera.Camera) { delete(m.boxes, cam) }

// compose outlines the box being dragged in cam's view, if there is one.
func (m *marquees) compose(f *render.Frame, cam camera.Camera) {
	b, ok := m.boxes[cam]
	if !ok {
		return
	}
	x0, y0, x1, y1 := float32(b.TopLeft.X), float32(b.TopLeft.Y), float32(b.BottomRight.X), float32(b.BottomRight.Y)
	outline(f, [4][2]float32{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}, 1, MarqueeColor)
}
