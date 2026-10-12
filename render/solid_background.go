package render

import (
	"image/color"

	"github.com/kjkrol/goke/v3"
)

// SolidBackground fills the screen with one colour.
type SolidBackground struct{ Color color.RGBA }

func (b SolidBackground) Init(*goke.SysInit) {}

func (b SolidBackground) Draw(screen *Image) { screen.Fill(b.Color) }
