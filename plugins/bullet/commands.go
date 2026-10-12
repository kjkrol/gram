package bullet

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/uid"
)

// Shoot is the command to fire a shot of Ammo: by a player from every selected unit it owns, by
// an entity from itself. It goes towards At when Targeted, at the moment's Subject when aimed,
// else the way the shooter faces.
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

// Burst is the command a landed shot gives itself to burst: every entity within Radius of its
// centre is told a Blast, and the shot is gone.
type Burst struct{ Radius float64 }

func (Shoot) Command() {}
func (Burst) Command() {}

var (
	_ control.Command = Shoot{}
	_ control.Command = Burst{}
)

var _ plugin.CommandHandler = (*Plugin)(nil)

// Queues are the plugin's command queues, Shoot and Burst.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.shoots, &p.bursts}
}

// DefaultBindings are none: a Shoot names its Ammo, which is the game's to bind.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }
