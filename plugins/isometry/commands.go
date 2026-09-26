package isometry

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
)

var _ plugin.CommandHandler = (*Plugin)(nil)

// Turn turns Camera by Angle radians, the world clockwise on the screen, keeping the ground point in
// the middle of the screen where it is; a camera of another view stays as it is.
type Turn struct {
	Camera camera.Camera
	Angle  float32
}

// Tilt has Camera look down by Angle radians more steeply, less for a negative one, between ten
// degrees over the ground and straight down; a camera of another view stays as it is.
type Tilt struct {
	Camera camera.Camera
	Angle  float32
}

// Follow fastens Camera behind the one Selected entity: centred on it and turned so that the way it
// walks runs up the screen, from then on whatever else is done or selected. Given again for a
// camera fastened so, it lets go.
type Follow struct{ Camera camera.Camera }

// Drive steers the unit Camera is fastened behind by hand, this tick: Ahead 1 walks it on the way
// it faces, -1 stops it; Turn -1 or 1 turns it anticlockwise or clockwise. A camera fastened to
// nothing drives nothing.
type Drive struct {
	Camera      camera.Camera
	Ahead, Turn int8
}

// TurnStep is how far Q and E turn the view a tick they are held: two degrees; TiltStep how far
// PageUp and PageDown tilt it: one.
const (
	TurnStep = math.Pi / 90
	TiltStep = math.Pi / 180
)

// Queues are where Turn and Follow land.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.turns, &p.tilts, &p.follows, &p.drives}
}

// DefaultBindings turn the player's camera while Q or E is held, tilt it while PageUp or PageDown
// is and, given the selection, fasten it behind the selected unit or let it go on V, the arrows
// driving the unit it is fastened to.
func (p *Plugin) DefaultBindings() []control.Binding {
	out := []control.Binding{
		control.Command(control.KeyHeld{Key: ebiten.KeyQ}, "Turn the world anticlockwise", func(c control.Context) (Turn, bool) {
			return Turn{Camera: c.Camera, Angle: -TurnStep}, true
		}),
		control.Command(control.KeyHeld{Key: ebiten.KeyE}, "Turn the world clockwise", func(c control.Context) (Turn, bool) {
			return Turn{Camera: c.Camera, Angle: TurnStep}, true
		}),
		control.Command(control.KeyHeld{Key: ebiten.KeyPageUp}, "Look down more steeply", func(c control.Context) (Tilt, bool) {
			return Tilt{Camera: c.Camera, Angle: TiltStep}, true
		}),
		control.Command(control.KeyHeld{Key: ebiten.KeyPageDown}, "Look along the ground", func(c control.Context) (Tilt, bool) {
			return Tilt{Camera: c.Camera, Angle: -TiltStep}, true
		}),
	}
	if p.selection != nil {
		drive := func(ahead, turn int8) func(control.Context) (Drive, bool) {
			return func(c control.Context) (Drive, bool) { return Drive{Camera: c.Camera, Ahead: ahead, Turn: turn}, true }
		}
		out = append(out,
			control.Command(control.KeyPress{Key: ebiten.KeyV}, "Follow the selected unit from behind", func(c control.Context) (Follow, bool) {
				return Follow{Camera: c.Camera}, true
			}),
			control.Command(control.KeyHeld{Key: ebiten.KeyArrowUp}, "Walk the followed unit on", drive(1, 0)),
			control.Command(control.KeyHeld{Key: ebiten.KeyArrowDown}, "Stop the followed unit", drive(-1, 0)),
			control.Command(control.KeyHeld{Key: ebiten.KeyArrowLeft}, "Turn the followed unit anticlockwise", drive(0, -1)),
			control.Command(control.KeyHeld{Key: ebiten.KeyArrowRight}, "Turn the followed unit clockwise", drive(0, 1)),
		)
	}
	return out
}
