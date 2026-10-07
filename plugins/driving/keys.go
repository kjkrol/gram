package driving

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// Keys are a set of keys driving a player's units, bound as one: Tank or Compass.
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

// Compass are four keys held walking the units the way of the screen: up, down, left, right.
// In says in which fastenings of the camera they hold (zero, every one); KeyUnknown leaves one
// unbound.
type Compass struct {
	Up, Down, Left, Right control.Key
	In                    camera.How
}

// DefaultKeys drive the unit the camera is fastened to: W, S, A and D riding inside it, the arrows
// following behind it.
func DefaultKeys() []Keys {
	return []Keys{
		Tank{Ahead: control.KeyW, Back: control.KeyS, Left: control.KeyA, Right: control.KeyD, In: camera.Inside},
		Tank{Ahead: control.KeyArrowUp, Back: control.KeyArrowDown, Left: control.KeyArrowLeft, Right: control.KeyArrowRight, In: camera.Behind},
	}
}

// Bindings are the four keys held into Ahead, Back and Turn of the camera the player looks through.
func (k Tank) Bindings() []control.Binding {
	var out []control.Binding
	held := func(key control.Key, b control.Binding) {
		if key != control.KeyUnknown {
			out = append(out, b.In(k.In))
		}
	}
	// the held key's context has the modifiers as they are
	held(k.Ahead, control.Command(control.KeyHeld{Key: k.Ahead}, "Walk on; with Shift, sprint", func(c control.Context) (Ahead, bool) {
		return Ahead{Camera: c.Camera, Sprint: c.Mods.Shift}, true
	}))
	held(k.Back, control.Command(control.KeyHeld{Key: k.Back}, "Brake, then back away", func(c control.Context) (Back, bool) {
		return Back{Camera: c.Camera}, true
	}))
	held(k.Left, control.Command(control.KeyHeld{Key: k.Left}, "Turn left", turn(-1)))
	held(k.Right, control.Command(control.KeyHeld{Key: k.Right}, "Turn right", turn(1)))
	return out
}

// Bindings are the four keys held into Toward of the camera the player looks through.
func (k Compass) Bindings() []control.Binding {
	var out []control.Binding
	held := func(key control.Key, label string, dx, dy float64) {
		if key != control.KeyUnknown {
			out = append(out, control.Command(control.KeyHeld{Key: key}, label, func(c control.Context) (Toward, bool) {
				return Toward{Camera: c.Camera, Way: geom.NewVec(dx, dy)}, true
			}).In(k.In))
		}
	}
	held(k.Up, "Drive up", 0, -1)
	held(k.Down, "Drive down", 0, 1)
	held(k.Left, "Drive left", -1, 0)
	held(k.Right, "Drive right", 1, 0)
	return out
}

func turn(way int8) func(control.Context) (Turn, bool) {
	return func(c control.Context) (Turn, bool) { return Turn{Camera: c.Camera, Way: way}, true }
}
