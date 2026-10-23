package render

import (
	"image/color"

	"github.com/kjkrol/goke/v3"
)

// Drawer is what draws a part of the picture, readied once.
type Drawer interface {
	Init(*goke.SysInit)
}

// Renderer is a Drawer drawing once a frame on the whole screen, in screen pixels.
type Renderer interface {
	Drawer
	Draw(screen *Image)
}

// Reporter is a plugin's part of the telemetry panel: lines it adds under the standard ones,
// each a label and a value, asked for once a frame.
type Reporter interface {
	Drawer
	Report(line func(label, value string))
}

// SolidBackground fills the screen with one colour.
type SolidBackground struct{ Color color.RGBA }

func (b SolidBackground) Init(*goke.SysInit) {}

func (b SolidBackground) Draw(screen *Image) { screen.Fill(b.Color) }
