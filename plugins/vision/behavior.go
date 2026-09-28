package vision

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// Between is a behavior for every observer carrying a and what it sees carrying b, once a tick per
// observer; plugin.Any on either side takes whatever is there. Register it with Plugin.RegisterBehavior.
func Between[FA, FB any](a plugin.Tag[FA], b plugin.Tag[FB], react func(t plugin.Tick, s Sighting)) plugin.Behavior {
	return host.Pair(a, b, react)
}

// Sighting is one observer and everything in its view carrying the behavior's second tag,
// nearest first, possibly none. Steering is nil for an observer that cannot be steered.
type Sighting struct {
	Self     uid.UID64
	Base     *world.Base
	Sight    *Sight
	Steering *steering.Steering
	Seen     []Seen
}

// Seen is one entity in an observer's view: which, where and how it moves, how far off — and
// which tags it carries, of the families the plugin's behaviors name (plugin.Carries).
type Seen struct {
	ID   uid.UID64
	Base *world.Base
	Dist float32
	plugin.Marks
}
