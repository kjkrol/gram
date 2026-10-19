package telemetry

import "github.com/kjkrol/ceremony/command"

// Panel shows the telemetry panel, or hides it: ~ among the players' GameBindings, which carry
// it out, shows it when hidden and hides it when shown.
type Panel struct{ Shown bool }

func (Panel) Command() {}

var _ command.Command = Panel{}
