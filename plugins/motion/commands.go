package motion

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Queues are the queues of the steering's commands, an entity's own (steering.Away, Toward,
// Turn).
func (p *Plugin) Queues() []command.CommandQueue { return p.module.steer.Queues() }

// DefaultBindings is nil: an entity steers itself in its rules.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }
