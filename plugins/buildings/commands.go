package buildings

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/uid"
)

// Raise is the command a settlement gives itself under a world.Unfold to have the buildings the
// level's file stands in the stretch unfolding spawned as its parts.
type Raise struct{ in uid.UID64 }

// Aim tells the command the stretch of the settlement unfolding.
func (r *Raise) Aim(who uid.UID64) { r.in = who }

func (Raise) Command() {}

var (
	_ command.Command = Raise{}
	_ host.Aimed      = (*Raise)(nil)
)

// Queues are the plugin's command queues: Raise.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.raises}
}
