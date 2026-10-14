package bullet

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/uid"
)

// Shoot is the command to fire a shot of Ammo: by a player from the unit Camera is fastened to,
// else from every selected unit it owns; by an entity from itself. It goes towards At when
// Targeted, at the moment's Subject when aimed, else the way the shooter faces, a thrown shot
// as far as Power says, a share of its Range (0 the whole).
type Shoot struct {
	Ammo     Ammo
	At       geom.Vec
	Targeted bool
	Camera   camera.Camera
	Power    float64

	target uid.UID64
	aimed  bool
}

var _ host.Aimed = (*Shoot)(nil)

// Aim tells the command whom to shoot at.
func (s *Shoot) Aim(who uid.UID64) { s.target, s.aimed = who, true }

// Burst is the command a landed shot gives itself to burst: every entity within Radius of its
// centre is told a Blast, and the shot is gone.
type Burst struct{ Radius float64 }

func (Shoot) Command() {}
func (Burst) Command() {}

var (
	_ command.Command = Shoot{}
	_ command.Command = Burst{}
)

var _ plugin.CommandHandler = (*Plugin)(nil)

// Queues are the plugin's command queues, Shoot and Burst.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.shoots, &p.bursts}
}

// DefaultBindings are none: a Shoot names its Ammo, which is the game's to bind.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }
