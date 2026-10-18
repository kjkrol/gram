package cameras

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/cameras/internal/stick"
)

// the views' commands as the cameras in relief read them
type orders struct {
	turns     command.Queue[Turn]
	tilts     command.Queue[Tilt]
	rides     command.Queue[Ride]
	views     command.Queue[View]
	lookFroms command.Queue[LookFrom]
	lookAts   command.Queue[LookAt]
	looks     command.Queue[Look]
}

var _ stick.Orders = (*orders)(nil)

func (q *orders) queues() []command.CommandQueue {
	return []command.CommandQueue{
		&q.turns,
		&q.tilts,
		&q.rides,
		&q.views,
		&q.lookFroms,
		&q.lookAts,
		&q.looks,
	}
}

func (q *orders) Views(fn func(cam camera.Camera)) {
	q.views.Drain(func(i command.Issued[View]) { fn(i.Command.Camera) })
}

func (q *orders) Turns(fn func(cam camera.Camera, angle float32)) {
	q.turns.Drain(func(i command.Issued[Turn]) { fn(i.Command.Camera, i.Command.Angle) })
}

func (q *orders) Tilts(fn func(cam camera.Camera, angle float32)) {
	q.tilts.Drain(func(i command.Issued[Tilt]) { fn(i.Command.Camera, i.Command.Angle) })
}

func (q *orders) LookFroms(fn func(cam camera.Camera, x, y, z float32)) {
	q.lookFroms.Drain(func(i command.Issued[LookFrom]) {
		fn(i.Command.Camera, i.Command.X, i.Command.Y, i.Command.Z)
	})
}

func (q *orders) LookAts(fn func(cam camera.Camera, x, y, z float32)) {
	q.lookAts.Drain(func(i command.Issued[LookAt]) {
		fn(i.Command.Camera, i.Command.X, i.Command.Y, i.Command.Z)
	})
}

func (q *orders) Rides(fn func(cam camera.Camera)) {
	q.rides.Drain(func(i command.Issued[Ride]) { fn(i.Command.Camera) })
}

func (q *orders) Looks(fn func(cam camera.Camera, dx, dy float32)) {
	q.looks.Drain(func(i command.Issued[Look]) { fn(i.Command.Camera, i.Command.Dx, i.Command.Dy) })
}

// the views' keys the Config gives
func viewBindings(cfg Config) []control.Binding {
	turn := func(angle float32) func(control.Context) (Turn, bool) {
		return func(c control.Context) (Turn, bool) {
			return Turn{Camera: c.Camera, Angle: angle}, true
		}
	}
	tilt := func(angle float32) func(control.Context) (Tilt, bool) {
		return func(c control.Context) (Tilt, bool) {
			return Tilt{Camera: c.Camera, Angle: angle}, true
		}
	}
	look := func(c control.Context) (Look, bool) {
		return Look{Camera: c.Camera, Dx: float32(c.Delta.X), Dy: float32(c.Delta.Y)}, true
	}
	view := func(c control.Context) (View, bool) { return View{Camera: c.Camera}, true }
	ride := func(c control.Context) (Ride, bool) { return Ride{Camera: c.Camera}, true }
	tab := control.KeyPress{Key: control.KeyTab}
	v := control.KeyPress{Key: control.KeyV}
	var out []control.Binding
	if cfg.Perspective {
		out = append(out,
			control.Build(tab, "Switch between the orthographic view and the perspective", view).
				In(camera.Outside),
			control.Build(tab, "Leave the unit", view).In(camera.Inside))
	}
	if cfg.Turn {
		out = append(out,
			control.Build(control.KeyHeld{Key: control.KeyQ}, "Turn the world anticlockwise",
				turn(-TurnStep)).In(camera.Outside),
			control.Build(control.KeyHeld{Key: control.KeyE}, "Turn the world clockwise",
				turn(TurnStep)).In(camera.Outside))
	}
	if cfg.Tilt {
		out = append(out,
			control.Build(control.KeyHeld{Key: control.KeyR},
				"Look down more steeply, straight down at the most", tilt(TiltStep)).
				In(camera.Outside),
			control.Build(control.KeyHeld{Key: control.KeyF}, "Look flatter: further off",
				tilt(-TiltStep)).In(camera.Outside))
	}
	if cfg.Turn || cfg.Perspective {
		closer := "Follow the unit from behind, or over it again"
		if cfg.Perspective {
			closer = "Follow the unit from behind, ride in it, or over it again"
		}
		out = append(out, control.Build(v, closer, ride).In(camera.Centred, camera.Behind))
	}
	if cfg.Perspective {
		out = append(out,
			control.Build(v, "Leave the unit: over it again", ride).In(camera.Inside),
			control.Build(control.CursorMove{},
				"Look round with the mouse: across turns, up and down the head", look).
				In(camera.Inside))
	}
	return out
}
