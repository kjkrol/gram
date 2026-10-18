package minimap

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Switch shows the minimaps or hides them. Shift+M by default.
type Switch struct{}

// Turn has the minimaps turn with their cameras, or keep north up. Ctrl+M by default.
type Turn struct{}

func (Switch) Command() {}
func (Turn) Command()   {}

var (
	_ command.Command = Switch{}
	_ command.Command = Turn{}
)

// Queues are where the minimaps' commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.switches, &p.turns}
}

// DefaultBindings are Shift+M showing or hiding the minimaps and Ctrl+M turning them.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyM, Mods: control.Mods{Shift: true}},
			"Show the minimap or hide it",
			func(control.Context) (Switch, bool) { return Switch{}, true }),
		control.Build(control.KeyPress{Key: control.KeyM, Mods: control.Mods{Ctrl: true}},
			"Turn the minimap with the camera, or keep north up",
			func(control.Context) (Turn, bool) { return Turn{}, true }),
	}
}
