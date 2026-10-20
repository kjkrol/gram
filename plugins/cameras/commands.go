package cameras

import (
	"math"
	"time"

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

// CenterAt moves Camera, once, to have the ground at the world point (X, Y) in the middle of the
// screen, fastening it to nothing: a click on a minimap.
type CenterAt struct {
	Camera camera.Camera
	X, Y   float64
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

// Sight has Camera see Factor times as far (camera.Detail.Sight); one seeing without end stays
// so.
type Sight struct {
	Camera camera.Camera
	Factor float64
}

// Tilt leans Camera's stick about its foot by Angle radians, towards straight down as it grows and
// flatter for a negative one, between Config.MinPitch and straight down; a plain camera stays.
type Tilt struct {
	Camera camera.Camera
	Angle  float32
}

// Resolution has Camera drawn at Factor times the pixels it is drawn at now, between
// LeastResolution and all of them, a ring at a time in rings, and stops the following (Still).
type Resolution struct {
	Camera camera.Camera
	Factor float32
}

// Adapt has every picture's resolution follow the frame budget (Settings.FrameBudget) as
// Strategy says, Uniform the whole picture's, coarser while the frames come slower and finer
// again while they keep up, InRings drawn in rings and stepped a ring at a time, the outer
// rings first, or stay as set, Still. Alt+A by default, in turn.
type Adapt struct{ Strategy Strategy }

// Picture has every picture but a minimap's drawn at Resolution of its pixels, the centre
// ring's in rings, LeastResolution up to all of them, and stops the following (Still).
type Picture struct{ Resolution float32 }

// SharpFigures has every picture but a minimap's draw its figures, all that lies over the
// ground, at every shown pixel while its ground is drawn at the resolution, or at the resolution
// too (Settings.SoftFigures); a picture drawn in rings is drawn as it is.
type SharpFigures struct{ On bool }

// Rings has every picture but a minimap's drawn in rings (Settings.Rings) or whole; a Strategy
// saying otherwise stops (Still).
type Rings struct{ On bool }

// RingShares has the inner ring cover Inner of the frame and the middle ring Middle, at least a
// tenth over the inner (Settings.Inner, Middle).
type RingShares struct{ Inner, Middle float32 }

// RingResolutions has the outer ring drawn at Outer of its pixels and the middle ring at Middle
// (Settings.OuterRes, MiddleRes), and stops the following (Still).
type RingResolutions struct{ Outer, Middle float32 }

// Budget sets the time a Frame may take, which the resolution follows (Settings.FrameBudget).
type Budget struct{ Frame time.Duration }

// ShowRings shows the rings' edges over every picture drawn in rings, or hides them
// (Settings.ShowRings).
type ShowRings struct{ Shown bool }

func (Pan) Command()             {}
func (Zoom) Command()            {}
func (Resolution) Command()      {}
func (Adapt) Command()           {}
func (Picture) Command()         {}
func (SharpFigures) Command()    {}
func (Rings) Command()           {}
func (RingShares) Command()      {}
func (RingResolutions) Command() {}
func (Budget) Command()          {}
func (ShowRings) Command()       {}
func (Follow) Command()          {}
func (CenterOn) Command()        {}
func (CenterAt) Command()        {}
func (View) Command()            {}
func (LookFrom) Command()        {}
func (LookAt) Command()          {}
func (Look) Command()            {}
func (Ride) Command()            {}
func (Turn) Command()            {}
func (Tilt) Command()            {}
func (Sight) Command()           {}

var (
	_ command.Command = Pan{}
	_ command.Command = Zoom{}
	_ command.Command = Follow{}
	_ command.Command = CenterOn{}
	_ command.Command = CenterAt{}
	_ command.Command = View{}
	_ command.Command = LookFrom{}
	_ command.Command = LookAt{}
	_ command.Command = Look{}
	_ command.Command = Ride{}
	_ command.Command = Turn{}
	_ command.Command = Tilt{}
	_ command.Command = Sight{}
	_ command.Command = Resolution{}
	_ command.Command = Adapt{}
	_ command.Command = Picture{}
	_ command.Command = SharpFigures{}
	_ command.Command = Rings{}
	_ command.Command = RingShares{}
	_ command.Command = RingResolutions{}
	_ command.Command = Budget{}
	_ command.Command = ShowRings{}
)

// TurnStep is how far Q and E turn the view a tick they are held, in radians, eighteen of them an
// eighth of a turn; CornerStep how far the cursor in a corner turns it; TiltStep how far R and F
// tilt it.
const (
	TurnStep   = math.Pi / 72
	CornerStep = TurnStep / 3
	TiltStep   = math.Pi / 180
)

// SightStep is the factor Ctrl+= and Ctrl+- see farther and nearer by.
const SightStep = 1.25

// ResolutionStep is the factor Alt+- draws a coarser picture by and Alt+= a finer one; the
// adaptive resolution steps by it too. LeastResolution is the coarsest a picture is drawn at.
const (
	ResolutionStep  = 0.85
	LeastResolution = 0.4
)

// Queues are where the cameras' commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return append([]command.CommandQueue{&p.pans, &p.zooms, &p.follows, &p.centerOns,
		&p.centerAts, &p.zones, &p.sights, &p.resolutions, &p.adapts, &p.pictures, &p.sharps,
		&p.rings, &p.ringShares, &p.ringRes, &p.budgets, &p.showRings}, p.orders.queues()...)
}

// DefaultBindings are DefaultKeys, their corners turning where the Config turns, Shift+H showing
// or hiding the scroll zones, Alt+A the picture's resolution still, following the frame budget
// or in rings, and the views' keys the Config gives: Q and E turning, R and F tilting, Tab
// switching the projection, V riding and the mouse looking round inside an entity.
func (p *Plugin) DefaultBindings() []control.Binding {
	keys := DefaultKeys()
	keys.CornersTurn = p.cfg.Turn
	out := append(keys.Bindings(),
		control.Build(control.KeyPress{Key: control.KeyH, Mods: control.Mods{Shift: true}},
			"Show the scroll zones at the picture's edges, or hide them",
			func(control.Context) (Zones, bool) { return Zones{Shown: !p.ZonesShown()}, true }),
		control.Build(control.KeyPress{Key: control.KeyA, Mods: control.Mods{Alt: true}},
			"The picture's resolution: still, following the frame budget, in rings",
			func(control.Context) (Adapt, bool) { return p.nextAdapt(), true }))
	return append(out, viewBindings(p.cfg)...)
}

// nextAdapt is the Adapt after the Settings: still, the whole picture, in rings, in turn
func (p *Plugin) nextAdapt() Adapt { return Adapt{Strategy: (p.Strategy() + 1) % 3} }
