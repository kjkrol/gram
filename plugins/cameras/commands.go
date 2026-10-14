package cameras

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

// Pan moves Camera by screen pixels, the same at any zoom, letting go of whatever it was fastened
// to.
type Pan struct {
	Camera camera.Camera
	Dx, Dy float32
	Edge   bool // by the cursor at the window's edge: a camera.Scroller scrolls its own way
	Corner bool // in a corner of it
}

// Zoom scales Camera about the world point At: a Factor above 1 zooms in, below 1 out.
type Zoom struct {
	Camera camera.Camera
	Factor float32
	At     geom.Vec
}

// Follow fastens Camera over Entity, kept in the middle of the screen as it goes
// (camera.Centred); with On false, or the camera fastened already, it lets go. Given by an entity
// for itself (kind.Entry.Told, act.Do), Camera is fastened over that entity.
type Follow struct {
	Camera camera.Camera
	Entity uid.UID64
	On     bool
}

// LookAt moves Camera, once, to have Entity in the middle of the screen at its altitude,
// fastening it to nothing.
type LookAt struct {
	Camera camera.Camera
	Entity uid.UID64
}

func (Pan) Command()    {}
func (Zoom) Command()   {}
func (Follow) Command() {}
func (LookAt) Command() {}

var (
	_ command.Command = Pan{}
	_ command.Command = Zoom{}
	_ command.Command = Follow{}
	_ command.Command = LookAt{}
)

// Queues are where Pan, Zoom, Follow, LookAt and Zones land.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.pans, &p.zooms, &p.follows, &p.lookAts, &p.zones}
}

// DefaultBindings are DefaultKeys and Shift+H showing or hiding the scroll zones.
func (p *Plugin) DefaultBindings() []control.Binding {
	return append(DefaultKeys().Bindings(),
		control.Build(control.KeyPress{Key: control.KeyH, Mods: control.Mods{Shift: true}},
			"Show the scroll zones at the picture's edges, or hide them",
			func(control.Context) (Zones, bool) { return Zones{}, true }))
}
