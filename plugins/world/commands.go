package world

import (
	"fmt"
	"slices"

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

// Tempo sets the tempo of the simulation to the clock's nearest X (clock.Config.Tempos).
type Tempo struct{ X float32 }

// SwitchBands switches the sectors' bands on, or off with every sector in the first
// (Settings.Unbanded): a way of weighing what the bands spare. Ctrl+G by default, in turn, in a
// world cut into sectors.
type SwitchBands struct{ On bool }

func (Despawn) Command()     {}
func (Spawn) Command()       {}
func (Pause) Command()       {}
func (Faster) Command()      {}
func (Slower) Command()      {}
func (Tempo) Command()       {}
func (SwitchBands) Command() {}

var (
	_ command.Command = Despawn{}
	_ command.Command = Spawn{}
	_ command.Command = Pause{}
	_ command.Command = Faster{}
	_ command.Command = Slower{}
	_ command.Command = Tempo{}
	_ command.Command = SwitchBands{}
)

// Settings is what the world lets the players adjust, saved with the game: the looks of the
// sectors and their bands. The tempo and the pause are the clock's (clock.State).
type Settings struct {
	Sectors  SectorsShown
	Unbanded bool
}

// Queues are the queues of every command the world carries out.
func (p *Plugin) Queues() []command.CommandQueue {
	m := p.module
	return []command.CommandQueue{
		&m.pauses, &m.fasters, &m.slowers, &m.tempos, &m.bandSwitches, &m.sectorSwitches,
		&m.spawns, &m.despawns, &m.effectCmds.queue, &m.effectCmds.triggers,
	}
}

// Settings are the tempo, with more than one to choose from, and, in a world cut into sectors,
// what is drawn of them and whether their bands are on (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	var out []control.Setting
	if tempos := p.module.clock.Tempos(); len(tempos) > 1 {
		options := make([]string, len(tempos))
		for i, t := range tempos {
			options[i] = fmt.Sprintf("%gx", t)
		}
		out = append(out, control.Choice{
			Label:   "Tempo",
			Options: options,
			Get: func() int {
				return max(0, slices.Index(tempos, p.module.clock.State().Tempo))
			},
			Set: func(i int) command.Command { return Tempo{X: tempos[i]} },
		})
	}
	if p.module.sectors.grid == nil {
		return out
	}
	return append(out,
		control.Choice{
			Label:   "Sectors drawn",
			Options: []string{"none", "by band", "as the camera sees them"},
			Get:     func() int { return int(p.Res.Settings.Sectors) },
			Set:     func(i int) command.Command { return SwitchSectors{Shown: SectorsShown(i)} },
		},
		control.Switch{
			Label: "Sectors' bands",
			Get:   p.Banded,
			Set:   func(on bool) command.Command { return SwitchBands{On: on} },
		})
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
			func(control.Context) (SwitchSectors, bool) {
				return SwitchSectors{Shown: p.SectorsShown().next()}, true
			}),
		control.Build(control.KeyPress{Key: control.KeyG, Mods: control.Mods{Ctrl: true}},
			"Switch the sectors' bands off, the whole world alive, or on again",
			func(control.Context) (SwitchBands, bool) {
				return SwitchBands{On: !p.Banded()}, true
			}))
}
