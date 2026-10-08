package render

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
)

// Layer is a part of the picture initialised once, at registration: a Renderer drawing on the
// screen, or a Picture drawing the world through a camera.
type Layer interface {
	Init(*goke.SysInit)
}

// Renderer is a layer drawn once a frame on the whole screen, in screen pixels: a background, a
// menu, a telemetry line.
type Renderer interface {
	Layer
	Draw(screen *Image)
}

// Picture is what is in the world before a camera: drawn through the camera it is handed, onto an
// image the size the camera draws — a Feed's picture.
type Picture interface {
	Layer
	DrawWorld(screen *Image, cam camera.Camera)
}
