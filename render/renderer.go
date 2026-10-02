package render

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
)

// Layer is one of a Scene's layers: a Renderer drawing on the screen, or a WorldRenderer drawing
// the world through each of the scene's viewports. Init runs once, at registration.
type Layer interface {
	Init(*goke.SysInit)
}

// Renderer is a layer drawn once a frame on the whole screen, in screen pixels: a background, a
// menu, a telemetry line.
type Renderer interface {
	Layer
	Draw(screen *Image)
}

// WorldRenderer is a layer showing the world, drawn once a frame per viewport through its camera
// onto an image the size of the viewport's area.
type WorldRenderer interface {
	Layer
	DrawWorld(screen *Image, cam camera.Camera)
}

// Viewport is where the world is shown: through Camera, into Area of the screen, in pixels.
type Viewport struct {
	Camera camera.Camera
	Area   geom.AABB
}

// Whole is the one viewport of cam over the whole screen, for a scene with a single view.
func Whole(cam camera.Camera, screen geom.AABB) []Viewport {
	return []Viewport{{Camera: cam, Area: screen}}
}
