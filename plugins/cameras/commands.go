package cameras

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

// Pan moves Camera by screen pixels, the same at any zoom, letting go of whatever it was fastened
// to.
type Pan struct {
	Camera camera.Camera
	Dx, Dy float32
}

// Zoom scales Camera about the world point At: a Factor above 1 zooms in, below 1 out.
type Zoom struct {
	Camera camera.Camera
	Factor float32
	At     geom.Vec
}

// Follow fastens Camera over Entity, kept in the middle of the screen as it goes
// (camera.Centred); with On false, or the camera fastened already, it lets go. Given by an entity
// for itself (kind.Entry.Told, rule.Order), Camera is fastened over that entity.
type Follow struct {
	Camera camera.Camera
	Entity uid.UID64
	On     bool
}

// MouseLook switches whether Camera, riding in an entity (camera.Inside), looks round with the
// mouse — the cursor captured — or not (camera.MouseLooker; camera.Config.MouseLook at the start).
type MouseLook struct{ Camera camera.Camera }

// Queues are where Pan, Zoom, Follow and MouseLook land.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.pans, &p.zooms, &p.follows, &p.looks}
}

// MouseLookKey is key switching looking round with the mouse, in first person, for the camera the
// player looks through.
func MouseLookKey(key control.Key) control.Binding {
	return control.Command(control.KeyPress{Key: key}, "Look round with the mouse in first person, or not", func(c control.Context) (MouseLook, bool) {
		return MouseLook{Camera: c.Camera}, true
	})
}

// DefaultBindings are DefaultKeys.
func (p *Plugin) DefaultBindings() []control.Binding { return DefaultKeys().Bindings() }
