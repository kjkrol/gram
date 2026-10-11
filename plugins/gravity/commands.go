package gravity

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/uid"
)

// Leap is the command an entity gives itself (rule.Order, driving.Jump) to leap as high as its
// Body says; one in the air does not.
type Leap struct{}

// Away is the command an entity gives itself to be thrown off at Speed across, rising Rise high
// first — knocked off by a blast. It is plugin.Aimed: given on a moment with a Subject (a
// bullet.Blast's shot), it is thrown away from that one; else back from the way it faces.
type Away struct {
	Speed float64
	Rise  float64

	from  uid.UID64
	aimed bool
}

var _ plugin.Aimed = (*Away)(nil)

// Aim tells the command whom to be thrown away from.
func (a *Away) Aim(who uid.UID64) { a.from, a.aimed = who, true }

func (Leap) Command() {}
func (Away) Command() {}

var (
	_ control.Command = Leap{}
	_ control.Command = Away{}
)

var _ plugin.CommandHandler = (*Plugin)(nil)

// Queues are where Leap and Away land — the entities' own.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.leaps, &p.aways}
}

// DefaultBindings are none: a Leap is an entity's own, a key's through driving.Jump.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }
