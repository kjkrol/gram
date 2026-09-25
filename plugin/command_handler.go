package plugin

import "github.com/kjkrol/gram/control"

// CommandHandler is a plugin, or a game, that defines commands and carries them out: it lists the
// queues they land in and the bindings it suggests for them — possibly none. A command type has
// one handler; the players plugin is built over them.
type CommandHandler interface {
	Queues() []control.CommandQueue
	DefaultBindings() []control.Binding
}
