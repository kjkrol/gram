package render

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
)

// Layer is a part of the picture, initialised once.
type Layer interface {
	Init(*goke.SysInit)
}

// Renderer is a layer drawn once a frame on the whole screen, in screen pixels.
type Renderer interface {
	Layer
	Draw(screen *Image)
}

// Picture is what is in the world before a camera, drawn through the camera it is handed.
type Picture interface {
	Layer
	DrawWorld(screen *Image, cam camera.Camera)
}
