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

// Pause switches the tactical pause: game time stands, the cameras and the picture go on. Shift+P
// by default.
type Pause struct{}

// Faster raises the tempo of the simulation a notch. ] by default.
type Faster struct{}

// Slower lowers the tempo of the simulation a notch. [ by default.
type Slower struct{}

// SwitchBands switches the sectors' bands off, every sector in the first, or on again: a way of
// weighing what the bands spare. Ctrl+G by default, in a world cut into sectors.
type SwitchBands struct{}

func (Despawn) Command()     {}
func (Spawn) Command()       {}
func (Pause) Command()       {}
func (Faster) Command()      {}
func (Slower) Command()      {}
func (SwitchBands) Command() {}

var (
	_ command.Command = Despawn{}
	_ command.Command = Spawn{}
	_ command.Command = Pause{}
	_ command.Command = Faster{}
	_ command.Command = Slower{}
	_ command.Command = SwitchBands{}
)

// Queues are the queues of every command the world carries out.
func (p *Plugin) Queues() []command.CommandQueue {
	m := p.module
	return []command.CommandQueue{
		&m.pauses, &m.fasters, &m.slowers, &m.bandSwitches, &m.sectorSwitches, &m.spawns,
		&m.despawns, &m.effectCmds.queue, &m.effectCmds.triggers,
	}
}

// DefaultBindings are the clock's keys — Shift+P pauses, ] goes faster, [ slower — and, in a
// world cut into sectors, Shift+G switching what is drawn of them and Ctrl+G their bands.
func (p *Plugin) DefaultBindings() []control.Binding {
	keys := []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyP, Mods: control.Mods{Shift: true}}, "Pause the game", func(control.Context) (Pause, bool) { return Pause{}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketRight}, "Speed the game up", func(control.Context) (Faster, bool) { return Faster{}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketLeft}, "Slow the game down", func(control.Context) (Slower, bool) { return Slower{}, true }),
	}
	if p.module.sectors.grid == nil {
		return keys
	}
	return append(keys,
		control.Build(control.KeyPress{Key: control.KeyG, Mods: control.Mods{Shift: true}},
			"Show the sectors by their bands, by what the camera sees, or hide them",
			func(control.Context) (SwitchSectors, bool) { return SwitchSectors{}, true }),
		control.Build(control.KeyPress{Key: control.KeyG, Mods: control.Mods{Ctrl: true}},
			"Switch the sectors' bands off, the whole world alive, or on again",
			func(control.Context) (SwitchBands, bool) { return SwitchBands{}, true }))
}
