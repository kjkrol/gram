package driving

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// Keys are a set of keys driving a player's units, bound as one and in chords, sprinting with
// Shift: Tank, Shooter or Compass.
type Keys interface {
	Bindings() []control.Binding
}

// Tank are four keys held driving the units as one sits in them: Ahead walks on (with Shift,
// sprints), Back brakes and backs away, Left and Right turn. In says in which fastenings of the
// camera they hold (camera.Inside riding in the unit, camera.Behind following it, camera.Outside
// looking from anywhere; zero, every one); KeyUnknown leaves one unbound.
type Tank struct {
	Ahead, Back, Left, Right control.Key
	In                       camera.How
}

// Shooter are four keys held driving the units as a shooter does: Ahead walks on (with Shift,
// sprints), Back brakes and backs away, Left and Right strafe, facing on — the mouse turns. In
// says in which fastenings of the camera they hold (zero, every one); KeyUnknown leaves one
// unbound.
type Shooter struct {
	Ahead, Back, Left, Right control.Key
	In                       camera.How
}

// Compass are four keys held walking the units the way of the screen: up, down, left, right, as
// the camera lays it (up the way it looks ahead, camera.Headed), the units turning to face it;
// with Shift, sprinting. In says in which fastenings of the camera they hold (zero, every one);
// KeyUnknown leaves one unbound.
type Compass struct {
	Up, Down, Left, Right control.Key
	In                    camera.How
}

// DefaultKeys drive the unit the camera is fastened to: W, S, A and D riding inside it as a
// shooter's, behind it as a compass's (the way the screen lies, the mouse turning the camera);
// the arrows a tank's behind it.
func DefaultKeys() []Keys {
	return []Keys{
		Shooter{Ahead: control.KeyW, Back: control.KeyS, Left: control.KeyA, Right: control.KeyD, In: camera.Inside},
		Compass{
			Up:    control.KeyW,
			Down:  control.KeyS,
			Left:  control.KeyA,
			Right: control.KeyD,
			In:    camera.Behind,
		},
		Tank{
			Ahead: control.KeyArrowUp,
			Back:  control.KeyArrowDown,
			Left:  control.KeyArrowLeft,
			Right: control.KeyArrowRight,
			In:    camera.Behind,
		},
	}
}

// Bindings are the four keys held into Ahead, Back and Turn of the camera the player looks through.
func (k Tank) Bindings() []control.Binding {
	var out []control.Binding
	held := func(key control.Key, b control.Binding) {
		if key != control.KeyUnknown {
			out = append(out, b.In(k.In).InChords())
		}
	}
	// the held key's context has the modifiers as they are
	held(k.Ahead, control.Build(control.KeyHeld{Key: k.Ahead}, "Walk on; with Shift, sprint", func(c control.Context) (Ahead, bool) {
		return Ahead{Camera: c.Camera, Sprint: c.Mods.Shift}, true
	}))
	held(k.Back, control.Build(control.KeyHeld{Key: k.Back}, "Brake, then back away", func(c control.Context) (Back, bool) {
		return Back{Camera: c.Camera}, true
	}))
	held(k.Left, control.Build(control.KeyHeld{Key: k.Left}, "Turn left", turn(-1)))
	held(k.Right, control.Build(control.KeyHeld{Key: k.Right}, "Turn right", turn(1)))
	return out
}

// Bindings are the four keys held into Ahead, Back and Strafe of the camera the player looks
// through.
func (k Shooter) Bindings() []control.Binding {
	var out []control.Binding
	held := func(key control.Key, b control.Binding) {
		if key != control.KeyUnknown {
			out = append(out, b.In(k.In).InChords())
		}
	}
	held(k.Ahead, control.Build(control.KeyHeld{Key: k.Ahead}, "Walk on; with Shift, sprint", func(c control.Context) (Ahead, bool) {
		return Ahead{Camera: c.Camera, Sprint: c.Mods.Shift}, true
	}))
	held(k.Back, control.Build(control.KeyHeld{Key: k.Back}, "Brake, then back away", func(c control.Context) (Back, bool) {
		return Back{Camera: c.Camera}, true
	}))
	held(k.Left, control.Build(control.KeyHeld{Key: k.Left}, "Strafe left", strafe(-1)))
	held(k.Right, control.Build(control.KeyHeld{Key: k.Right}, "Strafe right", strafe(1)))
	return out
}

// Bindings are the four keys held into Toward of the camera the player looks through.
func (k Compass) Bindings() []control.Binding {
	var out []control.Binding
	held := func(key control.Key, label string, dx, dy float32) {
		if key != control.KeyUnknown {
			way := geom.NewVec(dx, dy)
			out = append(out, control.Build(control.KeyHeld{Key: key}, label, func(c control.Context) (Toward, bool) {
				return Toward{Camera: c.Camera, Way: way, Sprint: c.Mods.Shift}, true
			}).In(k.In).InChords())
		}
	}
	held(k.Up, "Walk up the screen; with Shift, sprint", 0, -1)
	held(k.Down, "Walk down the screen", 0, 1)
	held(k.Left, "Walk left", -1, 0)
	held(k.Right, "Walk right", 1, 0)
	return out
}

func turn(way int8) func(control.Context) (Turn, bool) {
	return func(c control.Context) (Turn, bool) { return Turn{Camera: c.Camera, Way: way}, true }
}

func strafe(way int8) func(control.Context) (Strafe, bool) {
	return func(c control.Context) (Strafe, bool) { return Strafe{Camera: c.Camera, Way: way}, true }
}

// JumpKey is key pressed, whatever else is held, into a Jump of the camera the player looks
// through: sprinting with Shift, one still jumps.
func JumpKey(key control.Key) control.Binding {
	return control.Build(control.KeyDown{Key: key}, "Jump", func(c control.Context) (Jump, bool) {
		return Jump{Camera: c.Camera}, true
	}).InChords()
}
