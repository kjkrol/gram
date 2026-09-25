package players

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

// MarqueeColor is the outline of a drag in progress.
var MarqueeColor = color.RGBA{R: 255, G: 140, B: 0, A: 255}

// Renderer draws the marquee of a local player's drag in progress, in the pixels of the viewport
// the player looks through.
type Renderer struct{ p *Plugin }

var _ render.WorldRenderer = (*Renderer)(nil)

func (r *Renderer) Init(*goke.SysInit) {}

// DrawWorld draws the drags of the local players looking through cam.
func (r *Renderer) DrawWorld(screen *ebiten.Image, cam camera.Camera) {
	for _, pl := range r.p.Locals() {
		if pl.Camera != cam {
			continue
		}
		start, current, dragging := pl.DragBox()
		if !dragging {
			continue
		}
		x0, y0 := float32(start.X), float32(start.Y)
		x1, y1 := float32(current.X), float32(current.Y)
		if x1 < x0 {
			x0, x1 = x1, x0
		}
		if y1 < y0 {
			y0, y1 = y1, y0
		}
		vector.StrokeRect(screen, x0, y0, x1-x0, y1-y0, 1, MarqueeColor, true)
	}
}
