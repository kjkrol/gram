package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
)

// Direct is a Source that draws a part of the picture itself, with a shader of its own, straight on
// the screen — a heightfield traced per pixel — at its Tier: after every piece the frame orders
// before that tier and before all the rest. Compose may hand the frame pieces too, or nothing.
// Draw is handed the screen a test leaves nil: it draws nothing then.
type Direct interface {
	Source
	Tier() Tier
	Draw(screen *ebiten.Image, cam camera.Camera)
}
