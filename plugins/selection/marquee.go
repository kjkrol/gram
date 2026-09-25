package selection

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
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

// draw outlines the box being dragged in cam's view, if there is one.
func (m *marquees) draw(screen *ebiten.Image, cam camera.Camera) {
	b, ok := m.boxes[cam]
	if !ok {
		return
	}
	size := b.BottomRight.Sub(b.TopLeft)
	vector.StrokeRect(screen, float32(b.TopLeft.X), float32(b.TopLeft.Y), float32(size.X), float32(size.Y), 1, MarqueeColor, true)
}
