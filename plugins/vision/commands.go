package vision

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Cones shows every view drawn — the cones of sight and the ground out of sight — or hides them
// (Settings.Cones). A look at the world, like the camera's turn: at once, in the pause too.
// Shift+C by default, in turn.
type Cones struct{ Shown bool }

func (Cones) Command() {}

var _ command.Command = Cones{}

// Settings is what vision lets the players adjust, saved with the game.
type Settings struct {
	Cones bool
}

// Resources is what vision publishes: its Settings.
type Resources struct {
	Settings Settings
}

// Queues are where Cones lands.
func (p *Plugin) Queues() []command.CommandQueue { return []command.CommandQueue{&p.cones} }

// DefaultBindings show and hide the cones of sight on Shift+C.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyC, Mods: control.Mods{Shift: true}},
			"Show or hide the cones of sight",
			func(control.Context) (Cones, bool) { return Cones{Shown: !p.ConesShown()}, true }),
	}
}

// Settings is the one setting, the views drawn (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	return []control.Setting{control.Switch{
		Label: "Cones of sight",
		Get:   p.ConesShown,
		Set:   func(on bool) command.Command { return Cones{Shown: on} },
	}}
}
