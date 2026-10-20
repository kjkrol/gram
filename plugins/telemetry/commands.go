package telemetry

import "github.com/kjkrol/ceremony/command"

// Panel shows the telemetry panel, or hides it, the basics or everything (Mode): ~ among the
// players' GameBindings, which carry it out, turns it from hidden to the basics, to everything
// and to hidden again.
type Panel struct {
	Shown    bool
	Extended bool
}

// Settings is what the panel keeps of its own, saved with the game: whether it shows every line
// or the basics alone.
type Settings struct {
	Extended bool
}

// Resources is what the panel publishes: its Settings.
type Resources struct {
	Settings Settings
}

func (Panel) Command() {}

var _ command.Command = Panel{}
