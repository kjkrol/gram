package bullet

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/uid"
)

// Shoot is the command to fire a shot of Ammo: by a player, from every selected unit it owns; by
// an entity (Order in a rule or a plan), from itself. At, when Targeted, is where to — a thrown
// shot comes down there; else the shot goes the way the shooter faces. It is plugin.Aimed: given on
// a moment with a Subject, a Sighting's nearest seen, it goes at that one.
type Shoot struct {
	Ammo     Ammo
	At       geom.Vec
	Targeted bool

	target uid.UID64
	aimed  bool
}

var _ plugin.Aimed = (*Shoot)(nil)

// Aim tells the command whom to shoot at.
func (s *Shoot) Aim(who uid.UID64) { s.target, s.aimed = who, true }

// Burst is the command a landed shot gives itself (Order in a rule of its Resting) to burst:
// every entity within Radius of its centre is told a Blast, and the shot is gone.
type Burst struct{ Radius float64 }

var _ plugin.CommandHandler = (*Plugin)(nil)

// Queues are where Shoot and Burst land — for the players plugin and the entities' own.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.shoots, &p.bursts}
}

// DefaultBindings are none: a Shoot names its Ammo, which is the game's to bind.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }
