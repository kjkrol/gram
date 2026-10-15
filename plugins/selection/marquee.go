package selection

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

// MarqueeColor is the outline of a selection box being dragged.
var MarqueeColor = color.RGBA{R: 255, G: 140, B: 0, A: 255}

type marquees struct{ boxes map[camera.Camera]geom.AABB }

func (m *marquees) show(cam camera.Camera, box geom.AABB) {
	if m.boxes == nil {
		m.boxes = map[camera.Camera]geom.AABB{}
	}
	m.boxes[cam] = box
}

func (m *marquees) hide(cam camera.Camera) { delete(m.boxes, cam) }

func (m *marquees) compose(f *render.Frame, cam camera.Camera) {
	b, ok := m.boxes[cam]
	if !ok {
		return
	}
	x0, y0, x1, y1 := float32(b.TopLeft.X), float32(b.TopLeft.Y), float32(b.BottomRight.X), float32(b.BottomRight.Y)
	box := [4][2]float32{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	outline(f, render.OnTop(marqueeLevel), box, 1, MarqueeColor)
}
