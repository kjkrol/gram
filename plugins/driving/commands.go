package driving

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// Ahead walks the units the hand is on the way each faces, Sprint urging them on; given every
// tick its key is held (control.KeyHeld). Camera is the one it was given through — a player's,
// from the binding's context — and says which units the hand is on: the one it is fastened to,
// else those the player has selected. Given by an entity for itself (rule.Order), its own hand.
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

// Toward walks the units the hand is on the way Way says, as the screen lies: they turn to it and
// go. Several in one tick add up — up and right walk them up and right.
type Toward struct {
	Camera camera.Camera
	Way    geom.Vec
}

// Queues are where the hand's commands land.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.hands.aheads, &p.hands.backs, &p.hands.turns, &p.hands.towards}
}

// DefaultBindings are DefaultKeys: the keys driving the unit the camera is fastened to.
func (p *Plugin) DefaultBindings() []control.Binding {
	var out []control.Binding
	for _, k := range DefaultKeys() {
		out = append(out, k.Bindings()...)
	}
	return out
}
