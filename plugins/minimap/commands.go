package minimap

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Switch shows the minimaps, or hides them (Settings.Hidden). The settings screen alone.
type Switch struct{ Shown bool }

// Turn has the minimaps turn with their cameras, or keep north up (Settings.Turning). The
// settings screen alone.
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

// DefaultBindings are none: the minimaps shown and turning are the settings screen's.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }

// Settings are whether the minimaps are shown and whether they turn (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	return []control.Setting{
		control.Switch{
			Label: "Minimap",
			Get:   p.read.Shown,
			Set:   func(on bool) command.Command { return Switch{Shown: on} },
		},
		control.Switch{
			Label: "Minimap turns with the camera",
			Get:   p.read.Turning,
			Set:   func(on bool) command.Command { return Turn{On: on} },
		},
	}
}
