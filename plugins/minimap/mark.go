package minimap

import (
	"image/color"

	"github.com/kjkrol/gram/render"
)

// Mark is how a kind shows on a minimap, declared in its figure's Look with Styled: a square dot
// Size pixels across in Color. A kind without one is not shown.
type Mark struct {
	Color color.RGBA
	Size  float32
}

func (Mark) Style() {}

var _ render.Style = Mark{}
