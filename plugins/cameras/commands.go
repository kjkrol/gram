package cameras

import (
	"math"

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

// CenterOn moves Camera, once, to have Entity in the middle of the screen at its altitude,
// fastening it to nothing.
type CenterOn struct {
	Camera camera.Camera
	Entity uid.UID64
}

// View has Camera switch between the orthographic view and the perspective, keeping the foot of
// its stick, its turn, pitch and scale, or leave the entity it rides in; a camera whose Config has
// no Perspective stays as it is.
type View struct{ Camera camera.Camera }

// LookFrom puts Camera's eye at the world point (X, Y, Z) in perspective, still looking at the
// ground point in the middle of the screen; a camera of another view goes into the perspective
// first, or stays as it is where the Config has none. The eye is put exactly there, below the
// pitch floor too, until the next Tilt.
type LookFrom struct {
	Camera  camera.Camera
	X, Y, Z float32
}

// LookAt has Camera's eye look at the world point (X, Y, Z) from where it is, in perspective as
// LookFrom.
type LookAt struct {
	Camera  camera.Camera
	X, Y, Z float32
}

// Look has Camera, riding in an entity, look round by a move of the mouse of Dx and Dy pixels:
// across turns the entity, up and down the head, a seventh of a degree a pixel. A camera riding
// in nothing stays as it is.
type Look struct {
	Camera camera.Camera
	Dx, Dy float32
}

// Ride takes Camera closer round the entity it is fastened to: over it (camera.Centred), behind
// it (camera.Behind), inside it in first person where the Config reaches the perspective
// (camera.Inside), and over it again. A camera fastened to nothing stays as it is.
type Ride struct{ Camera camera.Camera }

// Turn turns Camera's stick about its foot by Angle radians, the world clockwise on the screen; a
// plain camera stays as it is.
type Turn struct {
	Camera camera.Camera
	Angle  float32
}

// Tilt leans Camera's stick about its foot by Angle radians, towards straight down as it grows and
// flatter for a negative one, between Config.MinPitch and straight down; a plain camera stays.
type Tilt struct {
	Camera camera.Camera
	Angle  float32
}

func (Pan) Command()      {}
func (Zoom) Command()     {}
func (Follow) Command()   {}
func (CenterOn) Command() {}
func (View) Command()     {}
func (LookFrom) Command() {}
func (LookAt) Command()   {}
func (Look) Command()     {}
func (Ride) Command()     {}
func (Turn) Command()     {}
func (Tilt) Command()     {}

var (
	_ command.Command = Pan{}
	_ command.Command = Zoom{}
	_ command.Command = Follow{}
	_ command.Command = CenterOn{}
	_ command.Command = View{}
	_ command.Command = LookFrom{}
	_ command.Command = LookAt{}
	_ command.Command = Look{}
	_ command.Command = Ride{}
	_ command.Command = Turn{}
	_ command.Command = Tilt{}
)

// TurnStep is how far Q and E turn the view a tick they are held, in radians, eighteen of them an
// eighth of a turn; CornerStep how far the cursor in a corner turns it; TiltStep how far R and F
// tilt it.
const (
	TurnStep   = math.Pi / 72
	CornerStep = TurnStep / 3
	TiltStep   = math.Pi / 180
)

// Queues are where the cameras' commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return append([]command.CommandQueue{&p.pans, &p.zooms, &p.follows, &p.centerOns, &p.zones},
		p.orders.queues()...)
}

// DefaultBindings are DefaultKeys, their corners turning where the Config turns, Shift+H showing
// or hiding the scroll zones, and the views' keys the Config gives: Q and E turning, R and F
// tilting, Tab switching the projection, V riding and the mouse looking round inside an entity.
func (p *Plugin) DefaultBindings() []control.Binding {
	keys := DefaultKeys()
	keys.CornersTurn = p.cfg.Turn
	out := append(keys.Bindings(),
		control.Build(control.KeyPress{Key: control.KeyH, Mods: control.Mods{Shift: true}},
			"Show the scroll zones at the picture's edges, or hide them",
			func(control.Context) (Zones, bool) { return Zones{}, true }))
	return append(out, viewBindings(p.cfg)...)
}
