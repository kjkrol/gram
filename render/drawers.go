package render

import "image/color"

// Renderer draws once a frame on the whole screen, in screen pixels.
type Renderer interface {
	Draw(screen *Image)
}

// Reporter is a plugin's part of the telemetry panel: lines it adds under the standard ones,
// each a label and a value, asked for once a frame.
type Reporter interface {
	Report(line func(label, value string))
}

// SolidBackground fills the screen with one colour.
type SolidBackground struct{ Color color.RGBA }

func (b SolidBackground) Draw(screen *Image) { screen.Fill(b.Color) }
