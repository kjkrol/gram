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

// Glass is a Source drawing on the glass over its picture: DrawGlass draws onto dst, the picture
// as it is shown, in its pixels, after everything the scene lays over the picture, so nothing
// covers what it draws (the scroll zones).
type Glass interface {
	DrawGlass(dst *Image, cam camera.Camera)
}
