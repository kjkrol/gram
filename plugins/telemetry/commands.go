package telemetry

import "github.com/kjkrol/ceremony/command"

// Panel shows the telemetry panel, or hides it: ~ among the players' GameBindings,
// which carry it out.
type Panel struct{}

func (Panel) Command() {}

var _ command.Command = Panel{}
