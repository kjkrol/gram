package driving

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// Ahead walks the units the hand is on the way each faces, Sprint urging them on; given every
// tick its key is held. Camera, the one it was given through, says which units the hand is on:
// the one it is fastened to, else those the player has selected.
type Ahead struct {
	Camera camera.Camera
	Sprint bool
}

// Back brakes the units the hand is on to a stop and then backs them away facing as they do.
type Back struct{ Camera camera.Camera }

// Turn turns the units the hand is on: Way -1 anticlockwise, 1 clockwise.
type Turn struct {
	Camera camera.Camera
	Way    int8
}

// Strafe walks the units the hand is on across the way each faces, facing on: Way 1 to the
// right, -1 to the left.
type Strafe struct {
	Camera camera.Camera
	Way    int8
}

// Toward walks the units the hand is on the way Way says, as the screen lies (up the screen the
// way Camera looks ahead where it has a heading, camera.Headed): they turn to it and go, Sprint
// urging them on. Several in one tick add up — up and right walk them up and right.
type Toward struct {
	Camera camera.Camera
	Way    geom.Vec
	Sprint bool
}

// Jump has each unit the hand is on leap (gravity.Leap), as high as its gravity.Body says.
type Jump struct{ Camera camera.Camera }

func (Ahead) Command()  {}
func (Back) Command()   {}
func (Turn) Command()   {}
func (Strafe) Command() {}
func (Toward) Command() {}
func (Jump) Command()   {}

var (
	_ command.Command = Ahead{}
	_ command.Command = Back{}
	_ command.Command = Turn{}
	_ command.Command = Strafe{}
	_ command.Command = Toward{}
	_ command.Command = Jump{}
)

// Queues are where the hand's commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{
		&p.hands.aheads,
		&p.hands.backs,
		&p.hands.turns,
		&p.hands.strafes,
		&p.hands.towards,
		&p.hands.jumps,
	}
}

// DefaultBindings are DefaultKeys, the keys driving the unit the camera is fastened to, and
// Space to jump riding inside it or behind it.
func (p *Plugin) DefaultBindings() []control.Binding {
	var out []control.Binding
	for _, k := range DefaultKeys() {
		out = append(out, k.Bindings()...)
	}
	return append(out, JumpKey(control.KeySpace).In(camera.Inside, camera.Behind))
}
