package players

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
)

// Give is the command by which an entity becomes To's, its alone; Nobody's for control.Nobody.
// An entity gives it itself, as it is made (kind.Entry.Told) or later.
type Give struct{ To control.PlayerID }

// ShowShortcuts opens the list of every key of the game (the Shortcuts scene), or closes it.
type ShowShortcuts struct{}

func (Give) Command()          {}
func (ShowShortcuts) Command() {}

var (
	_ control.Command = Give{}
	_ control.Command = ShowShortcuts{}
)

// Queues are where the players' own commands land, and the tools taken and put down
// (control.Take, control.Drop).
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.gives, &p.listings, &p.takes, &p.drops}
}

// DefaultBindings are GameBindings.
func (p *Plugin) DefaultBindings() []control.Binding { return GameBindings() }

// GameBindings are the keys every game has: K for the list of shortcuts, F5 to save, F9 to load,
// Shift+Esc to quit. A game that binds its own keys in place of the Defaults binds these beside
// them.
func GameBindings() []control.Binding {
	shift := control.Mods{Shift: true}
	return []control.Binding{
		control.Give(control.KeyPress{Key: control.KeyK}, "Shortcuts; Esc closes them",
			ShowShortcuts{}),
		control.Give(control.KeyPress{Key: control.KeyF5}, "Save the game", game.Save{}),
		control.Give(control.KeyPress{Key: control.KeyF9}, "Load the game saved", game.Load{}),
		control.Give(control.KeyPress{Key: control.KeyEscape, Mods: shift}, "Quit", game.Quit{}),
	}
}
