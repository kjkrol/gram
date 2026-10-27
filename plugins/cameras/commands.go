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
	X, Y   float32
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
	Factor float32
}

// Tilt leans Camera's stick about its foot by Angle radians, towards straight down as it grows and
// flatter for a negative one, between Config.MinPitch and straight down; a plain camera stays.
type Tilt struct {
	Camera camera.Camera
	Angle  float32
}

// Resolution has Camera drawn at Factor times the pixels it is drawn at now, between
// LeastResolution and all of them, a layer at a time in layers, and stops the following of its
// profile (Still).
type Resolution struct {
	Camera camera.Camera
	Factor float32
}

// Adapt has the resolution of Profile's pictures follow the frame budget (Settings.FrameBudget)
// as Strategy says, Uniform the whole picture's, coarser while the frames come slower and finer
// again while they keep up, InLayers drawn in layers and stepped a layer at a time, the outer
// layers first, or stay as set, Still. The settings screen alone.
type Adapt struct {
	Profile  Profile
	Strategy Strategy
}

// Picture has Profile's pictures drawn at Resolution of their pixels, the centre layer's in
// layers, LeastResolution up to all of them, and stops the following (Still).
type Picture struct {
	Profile    Profile
	Resolution float32
}

// SharpFigures has Profile's pictures draw their figures, all that lies over the ground, at
// every shown pixel while the ground is drawn at the resolution, or at the resolution too
// (Knobs.SoftFigures); a picture drawn in layers is drawn as it is.
type SharpFigures struct {
	Profile Profile
	On      bool
}

// Layers has Profile's pictures drawn in layers (Knobs.Layers) or whole; a Strategy saying
// otherwise stops (Still).
type Layers struct {
	Profile Profile
	On      bool
}

// LayerShares has the inner layer of Profile's pictures cover Inner of the frame and the middle
// layer Middle, at least a tenth over the inner (Knobs.Inner, Middle).
type LayerShares struct {
	Profile       Profile
	Inner, Middle float32
}

// LayerResolutions has the outer layer of Profile's pictures drawn at Outer of its pixels and the
// middle layer at Middle (Knobs.OuterRes, MiddleRes), and stops the following (Still).
type LayerResolutions struct {
	Profile       Profile
	Outer, Middle float32
}

// Budget sets the time a Frame may take, which the resolution follows (Settings.FrameBudget).
type Budget struct{ Frame time.Duration }

// Sampling sets how long the frame rate is averaged Over before the resolution follows it
// (Settings.Sampling), LeastSampling up to MostSampling.
type Sampling struct{ Over time.Duration }

// Thresholds sets the shares of the budget's rate the following steps at: coarser Under the
// first, finer over the second (Settings.Under, Settings.Met).
type Thresholds struct{ Under, Met float32 }

// Lever is what else the frame budget turns, given the cameras in Config.Levers: a step coarser
// as the rate falls under the budget, before the pictures' resolution, Coarser taking it and
// saying whether it could; a step finer as the rate stays over, after the pictures', Finer the
// same; one not Following is left alone.
type Lever interface {
	Following() bool
	Coarser() bool
	Finer() bool
}

// ShowLayers shows the layers' edges over every picture a Profile draws in layers, or hides
// them (Knobs.ShowLayers).
type ShowLayers struct {
	Profile Profile
	Shown   bool
}

func (Pan) Command()              {}
func (Zoom) Command()             {}
func (Resolution) Command()       {}
func (Adapt) Command()            {}
func (Picture) Command()          {}
func (SharpFigures) Command()     {}
func (Layers) Command()           {}
func (LayerShares) Command()      {}
func (LayerResolutions) Command() {}
func (Budget) Command()           {}
func (Sampling) Command()         {}
func (Thresholds) Command()       {}
func (ShowLayers) Command()       {}
func (Follow) Command()           {}
func (CenterOn) Command()         {}
func (CenterAt) Command()         {}
func (View) Command()             {}
func (LookFrom) Command()         {}
func (LookAt) Command()           {}
func (Look) Command()             {}
func (Ride) Command()             {}
func (Turn) Command()             {}
func (Tilt) Command()             {}
func (Sight) Command()            {}

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
	_ command.Command = Layers{}
	_ command.Command = LayerShares{}
	_ command.Command = LayerResolutions{}
	_ command.Command = Budget{}
	_ command.Command = Sampling{}
	_ command.Command = Thresholds{}
	_ command.Command = ShowLayers{}
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

// ResolutionStep is the factor the adaptive resolution steps a picture coarser by, and finer by
// its inverse. LeastResolution is the coarsest a picture is drawn at.
const (
	ResolutionStep  = 0.85
	LeastResolution = 0.4
)

// Queues are where the cameras' commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return append([]command.CommandQueue{&p.pans, &p.zooms, &p.follows, &p.centerOns,
		&p.centerAts, &p.zones, &p.edgeScrolls, &p.edgeSpeeds, &p.sights, &p.resolutions, &p.adapts, &p.pictures, &p.sharps,
		&p.layers, &p.layerShares, &p.layerRes, &p.budgets, &p.samplings, &p.thresholds, &p.showLayers}, p.orders.queues()...)
}

// DefaultBindings are DefaultKeys, their corners turning where the Config turns, the world grabbed
// with the right button where it says so and their edge as the settings say (EdgesScroll,
// EdgeSpeed), and the views' keys the Config gives: Q and E turning, R and F tilting, Tab
// switching the projection, V riding and the mouse looking round inside an entity. The scroll
// zones and the pictures' resolution are the settings screen's.
func (p *Plugin) DefaultBindings() []control.Binding {
	keys := DefaultKeys()
	keys.CornersTurn = p.cfg.Turn
	keys.Grab = p.cfg.Grab
	return append(keys.bindings(p), viewBindings(p.cfg)...)
}
