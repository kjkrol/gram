package topography

import (
	"math"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	icameras "github.com/kjkrol/gram/plugins/topography/internal/cameras"
)

// View has Camera look the next way round: from above, isometrically, in perspective when the
// game reaches it (Config.Perspective), from above again; a camera of another view stays as it is.
type View struct{ Camera camera.Camera }

// LookFrom puts Camera's eye at the world point (X, Y, Z) in perspective, still looking at the
// ground point in the middle of the screen; a camera of another view goes into the perspective
// first, or stays as it is where the game has none. The eye is put exactly there, below the pitch
// floor too, until the next Tilt.
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
// it (camera.Behind), inside it in first person where the game reaches the perspective
// (camera.Inside), and over it again. A camera fastened to nothing stays as it is.
type Ride struct{ Camera camera.Camera }

// Turn turns Camera by Angle radians, the world clockwise on the screen, keeping the ground
// point in the middle of the screen where it is; a camera of another view stays as it is.
type Turn struct {
	Camera camera.Camera
	Angle  float32
}

// Tilt bows Camera's head by Angle radians — it looks down more steeply — raises it for a negative
// one, between Config.MinPitch over the ground and straight down; a camera of another view stays
// as it is.
type Tilt struct {
	Camera camera.Camera
	Angle  float32
}

func (View) Command()     {}
func (LookFrom) Command() {}
func (LookAt) Command()   {}
func (Look) Command()     {}
func (Ride) Command()     {}
func (Turn) Command()     {}
func (Tilt) Command()     {}

var (
	_ control.Command = View{}
	_ control.Command = LookFrom{}
	_ control.Command = LookAt{}
	_ control.Command = Look{}
	_ control.Command = Ride{}
	_ control.Command = Turn{}
	_ control.Command = Tilt{}
)

// TurnStep is how far Q and E turn the view a tick they are held, in radians; TiltStep how far
// R and F tilt it.
const (
	TurnStep = math.Pi / 90
	TiltStep = math.Pi / 180
)

type cameraQueues struct {
	turns     control.Queue[Turn]
	tilts     control.Queue[Tilt]
	rides     control.Queue[Ride]
	views     control.Queue[View]
	lookFroms control.Queue[LookFrom]
	lookAts   control.Queue[LookAt]
	looks     control.Queue[Look]
}

var _ icameras.Orders = (*cameraQueues)(nil)

func (q *cameraQueues) queues() []control.CommandQueue {
	return []control.CommandQueue{&q.turns, &q.tilts, &q.rides, &q.views, &q.lookFroms, &q.lookAts, &q.looks}
}

func (q *cameraQueues) Views(fn func(cam camera.Camera)) {
	q.views.Drain(func(i control.Issued[View]) { fn(i.Command.Camera) })
}

func (q *cameraQueues) Turns(fn func(cam camera.Camera, angle float32)) {
	q.turns.Drain(func(i control.Issued[Turn]) { fn(i.Command.Camera, i.Command.Angle) })
}

func (q *cameraQueues) Tilts(fn func(cam camera.Camera, angle float32)) {
	q.tilts.Drain(func(i control.Issued[Tilt]) { fn(i.Command.Camera, i.Command.Angle) })
}

func (q *cameraQueues) LookFroms(fn func(cam camera.Camera, x, y, z float32)) {
	q.lookFroms.Drain(func(i control.Issued[LookFrom]) { fn(i.Command.Camera, i.Command.X, i.Command.Y, i.Command.Z) })
}

func (q *cameraQueues) LookAts(fn func(cam camera.Camera, x, y, z float32)) {
	q.lookAts.Drain(func(i control.Issued[LookAt]) { fn(i.Command.Camera, i.Command.X, i.Command.Y, i.Command.Z) })
}

func (q *cameraQueues) Rides(fn func(cam camera.Camera)) {
	q.rides.Drain(func(i control.Issued[Ride]) { fn(i.Command.Camera) })
}

func (q *cameraQueues) Looks(fn func(cam camera.Camera, dx, dy float32)) {
	q.looks.Drain(func(i control.Issued[Look]) { fn(i.Command.Camera, i.Command.Dx, i.Command.Dy) })
}

func cameraBindings(perspective bool) []control.Binding {
	turn := func(angle float32) func(control.Context) (Turn, bool) {
		return func(c control.Context) (Turn, bool) { return Turn{Camera: c.Camera, Angle: angle}, true }
	}
	tilt := func(angle float32) func(control.Context) (Tilt, bool) {
		return func(c control.Context) (Tilt, bool) { return Tilt{Camera: c.Camera, Angle: angle}, true }
	}
	views := "Switch the view: from above, isometric"
	closer := "Follow the unit from behind, or over it again"
	if perspective {
		views += ", in perspective"
		closer = "Follow the unit from behind, ride in it, or over it again"
	}
	ride := func(c control.Context) (Ride, bool) { return Ride{Camera: c.Camera}, true }
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyTab}, views, func(c control.Context) (View, bool) {
			return View{Camera: c.Camera}, true
		}).In(camera.Outside),
		control.Build(control.KeyPress{Key: control.KeyTab}, "Leave the unit", func(c control.Context) (View, bool) {
			return View{Camera: c.Camera}, true
		}).In(camera.Inside),
		control.Build(control.KeyHeld{Key: control.KeyQ}, "Turn the world anticlockwise", turn(-TurnStep)).In(camera.Outside),
		control.Build(control.KeyHeld{Key: control.KeyE}, "Turn the world clockwise", turn(TurnStep)).In(camera.Outside),
		control.Build(control.KeyHeld{Key: control.KeyR}, "Raise the head: look further off", tilt(-TiltStep)).In(camera.Outside),
		control.Build(control.KeyHeld{Key: control.KeyF}, "Bow the head: look down more steeply", tilt(TiltStep)).In(camera.Outside),
		control.Build(control.KeyPress{Key: control.KeyV}, closer, ride).In(camera.Centred, camera.Behind),
		control.Build(control.KeyPress{Key: control.KeyV}, "Leave the unit: over it again", ride).In(camera.Inside),
		control.Build(control.CursorMove{}, "Look round, where the camera looks by mouse: across turns, up and down the head", func(c control.Context) (Look, bool) {
			return Look{Camera: c.Camera, Dx: float32(c.Delta.X), Dy: float32(c.Delta.Y)}, camera.MouseLooks(c.Camera)
		}).In(camera.Inside),
	}
}
