package minimap

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Switch shows the minimaps, or hides them (Settings.Hidden). Shift+M by default, in turn.
type Switch struct{ Shown bool }

// Turn has the minimaps turn with their cameras, or keep north up (Settings.Turning). Ctrl+M by
// default, in turn.
type Turn struct{ On bool }

// Settings is what the minimaps let the players adjust, saved with the game.
type Settings struct {
	Hidden  bool
	Turning bool
}

// Resources is what the minimaps publish: their Settings.
type Resources struct {
	Settings Settings
}

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
			func(control.Context) (Switch, bool) { return Switch{Shown: !p.Shown()}, true }),
		control.Build(control.KeyPress{Key: control.KeyM, Mods: control.Mods{Ctrl: true}},
			"Turn the minimap with the camera, or keep north up",
			func(control.Context) (Turn, bool) { return Turn{On: !p.Turning()}, true }),
	}
}

// Settings are whether the minimaps are shown and whether they turn (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	return []control.Setting{
		control.Switch{
			Label: "Minimap",
			Get:   p.Shown,
			Set:   func(on bool) command.Command { return Switch{Shown: on} },
		},
		control.Switch{
			Label: "Minimap turns with the camera",
			Get:   p.Turning,
			Set:   func(on bool) command.Command { return Turn{On: on} },
		},
	}
}
