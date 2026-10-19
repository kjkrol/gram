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

// Grounded is a Picture drawing its ground apart from what lies over it: DrawGround draws every
// source but the Overlying ones through cam into ground, keeping the ground's depth; DrawOver
// draws the Overlying ones through cam onto screen, ground laid over it already, each tested
// against that depth spread to screen's size.
type Grounded interface {
	DrawGround(ground *Image, cam camera.Camera)
	DrawOver(screen, ground *Image, cam camera.Camera)
}
