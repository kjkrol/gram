package world

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
)

// Despawn is the command an entity gives itself to leave the world.
type Despawn struct{}

// Spawn is the command to add an entity of a kind to the running world, carried out at the next
// step of the simulation. One refused (an unknown kind, a wrong row, a full world, a bad size, a
// box wholly past an open edge) is logged, never a panic.
type Spawn struct{ Entry kind.Entry }

// Pause switches the tactical pause: game time stands, the cameras and the picture go on. Space
// by default.
type Pause struct{}

// Faster raises the tempo of the simulation a notch. ] by default.
type Faster struct{}

// Slower lowers the tempo of the simulation a notch. [ by default.
type Slower struct{}

func (Despawn) Command() {}
func (Spawn) Command()   {}
func (Pause) Command()   {}
func (Faster) Command()  {}
func (Slower) Command()  {}

var (
	_ command.Command = Despawn{}
	_ command.Command = Spawn{}
	_ command.Command = Pause{}
	_ command.Command = Faster{}
	_ command.Command = Slower{}
)

// Queues are the queues of every command the world carries out.
func (p *Plugin) Queues() []command.CommandQueue {
	m := p.module
	q := []command.CommandQueue{&m.pauses, &m.fasters, &m.slowers, &m.spawns, &m.despawns, &m.effectCmds.queue, &m.effectCmds.triggers}
	return append(q, m.steer.Queues()...)
}

// DefaultBindings are the clock's keys: Space pauses, ] goes faster, [ slower.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeySpace}, "Pause the game", func(control.Context) (Pause, bool) { return Pause{}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketRight}, "Speed the game up", func(control.Context) (Faster, bool) { return Faster{}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketLeft}, "Slow the game down", func(control.Context) (Slower, bool) { return Slower{}, true }),
	}
}
