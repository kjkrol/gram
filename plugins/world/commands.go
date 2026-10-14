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

// Boxes shows every entity's box as a red outline over the picture, or hides them: a look for
// finding out why things touch or miss, not saved. Shift+X by default.
type Boxes struct{}

func (Despawn) Command() {}
func (Spawn) Command()   {}
func (Pause) Command()   {}
func (Faster) Command()  {}
func (Slower) Command()  {}
func (Boxes) Command()   {}

var (
	_ command.Command = Despawn{}
	_ command.Command = Spawn{}
	_ command.Command = Pause{}
	_ command.Command = Faster{}
	_ command.Command = Slower{}
	_ command.Command = Boxes{}
)

// Queues are the queues of every command the world carries out.
func (p *Plugin) Queues() []command.CommandQueue {
	m := p.module
	q := []command.CommandQueue{
		&m.pauses, &m.fasters, &m.slowers, &m.spawns, &m.despawns,
		&m.effectCmds.queue, &m.effectCmds.triggers, &p.boxes,
	}
	return append(q, m.steer.Queues()...)
}

// DefaultBindings are the clock's keys — Shift+P pauses, ] goes faster, [ slower — and Shift+X
// showing the entities' boxes.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyP, Mods: control.Mods{Shift: true}}, "Pause the game", func(control.Context) (Pause, bool) { return Pause{}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketRight}, "Speed the game up", func(control.Context) (Faster, bool) { return Faster{}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketLeft}, "Slow the game down", func(control.Context) (Slower, bool) { return Slower{}, true }),
		control.Build(control.KeyPress{Key: control.KeyX, Mods: control.Mods{Shift: true}},
			"Show every entity's box, or hide them",
			func(control.Context) (Boxes, bool) { return Boxes{}, true }),
	}
}
