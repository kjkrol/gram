package players

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/telemetry"
)

// Give is the command by which an entity becomes To's, its alone; Nobody's for command.Nobody.
// An entity gives it itself, as it is made (kind.Entry.Told) or later.
type Give struct{ To command.Issuer }

// ShowControls opens the controls screen (the ControlsScene), every key and mouse action of the
// game. K by default.
type ShowControls struct{}

// ShowSettings opens the settings screen (the SettingsScene). F10 by default.
type ShowSettings struct{}

// KeepRunning has the game run on behind the settings screen (Settings.Running), or stand in the
// tactical pause while it is up.
type KeepRunning struct{ On bool }

func (Give) Command()         {}
func (ShowControls) Command() {}
func (ShowSettings) Command() {}
func (KeepRunning) Command()  {}

var (
	_ command.Command = Give{}
	_ command.Command = ShowControls{}
	_ command.Command = ShowSettings{}
	_ command.Command = KeepRunning{}
)

// Queues are where the players' own commands land, the telemetry panel's (it needs the
// scene's Composition, which the players' Handle has), and the tools taken and put down
// (control.Take, control.Drop).
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.gives, &p.listings, &p.openings, &p.runnings, &p.panels,
		&p.takes, &p.drops}
}

// DefaultBindings are GameBindings.
func (p *Plugin) DefaultBindings() []control.Binding { return p.GameBindings() }

// GameBindings are the keys every game has: K for the controls screen, F10 for the settings,
// ~ for the telemetry panel (the basics, everything, hidden, in turn), F5 to save, F9 to load,
// Shift+Esc to quit. A game that binds its own keys in place of the Defaults binds these beside
// them.
func (p *Plugin) GameBindings() []control.Binding {
	shift := control.Mods{Shift: true}
	return []control.Binding{
		control.Give(control.KeyPress{Key: control.KeyK}, "Controls; Esc closes them",
			ShowControls{}),
		control.Give(control.KeyPress{Key: control.KeyF10}, "Settings; Esc closes them",
			ShowSettings{}),
		control.Build(control.KeyPress{Key: control.KeyBackquote},
			"The telemetry panel: the basics, everything, hidden",
			func(control.Context) (telemetry.Panel, bool) { return p.nextPanel(), true }),
		control.Give(control.KeyPress{Key: control.KeyF5}, "Save the game", game.Save{}),
		control.Give(control.KeyPress{Key: control.KeyF9}, "Load the game saved", game.Load{}),
		control.Give(control.KeyPress{Key: control.KeyEscape, Mods: shift}, "Quit", game.Quit{}),
	}
}
