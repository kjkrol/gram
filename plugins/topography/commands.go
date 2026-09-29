package topography

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// View has Camera look the next way round: from above, isometrically, in perspective when the
// game reaches it (Config.Perspective), from above again; a camera of another view stays as it is.
type View struct{ Camera camera.Camera }

// LookFrom puts Camera's eye at the world point (X, Y, Z), looking at the ground point in the
// middle of the screen still, in perspective: a camera in another view goes into the perspective
// one first, where the game reaches it, and stays as it is where not. The eye is put exactly
// there, below the pitch floor too, until the next Tilt.
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

// Look has Camera, riding in an entity, look round by a move of the mouse, Dx and Dy pixels:
// across turns the entity to face where the eye looks, and the view with it at once, up and down
// raise and lower the head; LookStep radians a pixel. A camera riding in nothing stays as it is.
type Look struct {
	Camera camera.Camera
	Dx, Dy float32
}

// LookStep is how far a pixel of the mouse turns the eye riding in an entity: a seventh of a
// degree.
const LookStep = 0.0025

// LookOut has Camera ride in the one Selected entity, first person (camera.FirstPerson): the eye
// at the entity's centre, its altitude and as high as it stands, kept there as the entity goes and
// looking the way it faces — world.Base's Vel.Dir, kept when it stops; the axis of its sight
// where the game points the sight that way — in perspective as LookFrom. Drive steers the entity
// (W, S, A and D there), Look turns it and raises and lowers the head into the sky and down to the
// feet (the mouse there) — one that flies climbing and diving along the look as it goes — Zoom
// narrows the field of view, Turn does nothing; W with Shift sprints. The eye rides at the
// entity's eye, over the top of the cell it stands on. Given again, or View, it lets go, back to
// the view the camera was in, over the entity; Follow, LookFrom and LookAt let go first too.
type LookOut struct{ Camera camera.Camera }

// Heightfield draws the ground from its heightmap on the GPU, every pixel's line of sight traced
// over it (topography/heightfield), in place of the tiles — and the tiles again, given once more;
// nothing without Config.Heightfield.
type Heightfield struct{}

// Turn turns Camera by Angle radians, the world clockwise on the screen, keeping the ground point in
// the middle of the screen where it is; a camera of another view stays as it is.
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

// Follow fastens Camera behind the one Selected entity: centred on it and turned so that the way it
// walks runs up the screen, from then on whatever else is done or selected. Given again for a
// camera fastened so, it lets go.
type Follow struct{ Camera camera.Camera }

// Drive steers the unit Camera is fastened behind by hand, this tick: Ahead 1 walks it on the way
// it faces — Sprint urging it to its steering's Sprint — -1 brakes it to a stop and then backs it
// away, facing as it does, at the speed it sets off at; Turn -1 or 1 turns it anticlockwise or
// clockwise. A camera fastened to nothing drives nothing.
type Drive struct {
	Camera      camera.Camera
	Ahead, Turn int8
	Sprint      bool
}

// TurnStep is how far Q and E turn the view a tick they are held: two degrees; TiltStep how far
// R and F tilt it: one.
const (
	TurnStep = math.Pi / 90
	TiltStep = math.Pi / 180
)

// Queues are where the view's, the heightfield's and the shaping commands land.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.turns, &p.tilts, &p.follows, &p.drives, &p.views, &p.lookFroms, &p.lookAts, &p.lookOuts, &p.looks, &p.fields, &p.shaping.raise, &p.shaping.lower, &p.shaping.level}
}

// DefaultBindings switch the player's view on Tab, turn the camera while Q or E is held, raise its
// head while R is and bow it while F is; = raises and - lowers the ground under the cursor, a left
// drag with L held levels it to where the drag began. Given the selection, V rides in the selected
// unit, first person, where the game reaches the perspective (Config.Perspective): there W walks
// the unit on, S brakes it and then backs it away, A and D turn it, the mouse looks round, Q, E, R and F do nothing, V or
// Tab leave it (bindings holding in camera.FirstPerson; the free camera's keys hold in camera.Free).
// Without the perspective, V fastens the camera behind the selected unit and lets it go, the
// arrows driving the unit.
func (p *Plugin) DefaultBindings() []control.Binding {
	at := func(c control.Context) geom.Vec { return c.World(c.Cursor) }
	turn := func(angle float32) func(control.Context) (Turn, bool) {
		return func(c control.Context) (Turn, bool) { return Turn{Camera: c.Camera, Angle: angle}, true }
	}
	tilt := func(angle float32) func(control.Context) (Tilt, bool) {
		return func(c control.Context) (Tilt, bool) { return Tilt{Camera: c.Camera, Angle: angle}, true }
	}
	views := "Switch the view: from above, isometric"
	if p.cfg.Perspective {
		views += ", in perspective"
	}
	out := []control.Binding{
		control.Command(control.KeyPress{Key: ebiten.KeyTab}, views, func(c control.Context) (View, bool) {
			return View{Camera: c.Camera}, true
		}).In(camera.Free),
		control.Command(control.KeyPress{Key: ebiten.KeyTab}, "Leave the unit", func(c control.Context) (View, bool) {
			return View{Camera: c.Camera}, true
		}).In(camera.FirstPerson),
		control.Command(control.KeyHeld{Key: ebiten.KeyQ}, "Turn the world anticlockwise", turn(-TurnStep)).In(camera.Free),
		control.Command(control.KeyHeld{Key: ebiten.KeyE}, "Turn the world clockwise", turn(TurnStep)).In(camera.Free),
		control.Command(control.KeyHeld{Key: ebiten.KeyR}, "Raise the head: look further off", tilt(-TiltStep)).In(camera.Free),
		control.Command(control.KeyHeld{Key: ebiten.KeyF}, "Bow the head: look down more steeply", tilt(TiltStep)).In(camera.Free),
		control.Command(control.KeyPress{Key: ebiten.KeyEqual}, "Raise the ground", func(c control.Context) (Raise, bool) { return Raise{At: at(c)}, true }),
		control.Command(control.KeyPress{Key: ebiten.KeyMinus}, "Lower the ground", func(c control.Context) (Lower, bool) { return Lower{At: at(c)}, true }),
		control.Command(control.Drag{Button: ebiten.MouseButtonLeft, Mods: control.Mods{}.Holding(ebiten.KeyL)}, "Level the ground",
			func(c control.Context) (Level, bool) {
				return Level{From: c.World(c.Start), To: c.World(c.Cursor)}, true
			}),
	}
	if p.cfg.Heightfield {
		out = append(out, control.Command(control.KeyPress{Key: ebiten.KeyG}, "Draw the ground from its heightmap on the GPU, or as tiles",
			func(control.Context) (Heightfield, bool) { return Heightfield{}, true }))
	}
	if p.selection != nil {
		drive := func(ahead, turn int8) func(control.Context) (Drive, bool) {
			return func(c control.Context) (Drive, bool) { return Drive{Camera: c.Camera, Ahead: ahead, Turn: turn}, true }
		}
		// on, with Shift held sprinting: the held key's context has the modifiers as they are
		on := func(c control.Context) (Drive, bool) {
			return Drive{Camera: c.Camera, Ahead: 1, Sprint: c.Mods.Shift}, true
		}
		if !p.cfg.Perspective { // no first person: V follows the unit from behind, the arrows drive it
			return append(out,
				control.Command(control.KeyPress{Key: ebiten.KeyV}, "Follow the selected unit from behind", func(c control.Context) (Follow, bool) {
					return Follow{Camera: c.Camera}, true
				}),
				control.Command(control.KeyHeld{Key: ebiten.KeyArrowUp}, "Walk the followed unit on; with Shift, sprint", on),
				control.Command(control.KeyHeld{Key: ebiten.KeyArrowDown}, "Brake the followed unit, then back it away", drive(-1, 0)),
				control.Command(control.KeyHeld{Key: ebiten.KeyArrowLeft}, "Turn the followed unit anticlockwise", drive(0, -1)),
				control.Command(control.KeyHeld{Key: ebiten.KeyArrowRight}, "Turn the followed unit clockwise", drive(0, 1)),
			)
		}
		lookOut := func(c control.Context) (LookOut, bool) { return LookOut{Camera: c.Camera}, true }
		out = append(out,
			control.Command(control.KeyPress{Key: ebiten.KeyV}, "Ride in the selected unit: first person", lookOut).In(camera.Free),
			control.Command(control.KeyPress{Key: ebiten.KeyV}, "Leave the unit", lookOut).In(camera.FirstPerson),
			control.Command(control.KeyHeld{Key: ebiten.KeyW}, "Walk on; with Shift, sprint", on).In(camera.FirstPerson),
			control.Command(control.KeyHeld{Key: ebiten.KeyS}, "Brake, then back away", drive(-1, 0)).In(camera.FirstPerson),
			control.Command(control.KeyHeld{Key: ebiten.KeyA}, "Turn left", drive(0, -1)).In(camera.FirstPerson),
			control.Command(control.KeyHeld{Key: ebiten.KeyD}, "Turn right", drive(0, 1)).In(camera.FirstPerson),
			control.Command(control.CursorMove{}, "Look round: across turns, up and down the head", func(c control.Context) (Look, bool) {
				return Look{Camera: c.Camera, Dx: float32(c.Delta.X), Dy: float32(c.Delta.Y)}, true
			}).In(camera.FirstPerson),
		)
	}
	return out
}
