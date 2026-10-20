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

// Laid is a Picture drawn in layers from one composition: Lay composes every source (the
// ground's alone when apart) through cam once, in cam's pixels; DrawLayer draws what was
// composed into layer, cam cropped to the frame's window at, the pieces placed by the window's
// scale and offset, the Directs drawing through cam; DrawOver composes the Overlying sources and
// draws them onto screen over the layers laid since Lay, tested against their depths placed at
// screen's size, the last laid over the first. A nil layer or screen only composes.
type Laid interface {
	Lay(cam camera.Camera, apart bool)
	DrawLayer(layer *Image, cam camera.Camera, at camera.Window)
	DrawOver(screen *Image, cam camera.Camera)
}

// Glass is a Source drawing on the glass over its picture: DrawGlass draws onto dst, the picture
// as it is shown, in its pixels, after everything the scene lays over the picture, so nothing
// covers what it draws (the scroll zones).
type Glass interface {
	DrawGlass(dst *Image, cam camera.Camera)
}
