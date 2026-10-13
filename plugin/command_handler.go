package plugin

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// CommandHandler is a plugin, or a game, that defines commands and carries them out: the queues
// they land in and the bindings it suggests for them. A command type has one handler.
type CommandHandler interface {
	Queues() []command.CommandQueue
	DefaultBindings() []control.Binding
}
