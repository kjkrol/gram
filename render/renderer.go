package render

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
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

// Picture is what is in the world before a camera, drawn through the camera it is handed.
type Picture interface {
	Drawer
	DrawWorld(screen *Image, cam camera.Camera)
}
