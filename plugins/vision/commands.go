package vision

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Cones shows every view drawn — the cones of sight and the ground out of sight — or hides them
// (Settings.Cones). A look at the world, like the camera's turn: at once, in the pause too.
// The settings screen alone.
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

// DefaultBindings are none: the cones are the settings screen's.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }

// Settings is the one setting, the views drawn (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	return []control.Setting{control.Switch{
		Label: "Cones of sight",
		Get:   p.read.ConesShown,
		Set:   func(on bool) command.Command { return Cones{Shown: on} },
	}}
}
