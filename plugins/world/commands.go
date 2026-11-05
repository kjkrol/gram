package world

import (
	"fmt"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/uid"
)

// Despawn is the command an entity gives itself to leave the world.
type Despawn struct{}

// Spawn is the command to add an entity of a kind to the running world, carried out at the next
// step of the simulation. One refused (an unknown kind, a wrong row, a full world, a bad size, a
// box wholly past an open edge) is logged, never a panic.
type Spawn struct{ Entry kind.Entry }

// SpawnPart is the command an aggregate gives itself to spawn, at the next step, a part of it
// carrying PartOf it, the Entry's entity laid At off the centre of the stretch its Unfold tells of,
// or of the aggregate given under none. One refused, given by no aggregate or as a Spawn is, is
// logged.
type SpawnPart struct {
	Entry kind.Entry
	// At is where the part's row's box is laid from, off that centre: zero at it.
	At    geom.Vec
	in    uid.UID64
	aimed bool
}

// Aim tells the command the stretch of the aggregate it lies in.
func (s *SpawnPart) Aim(who uid.UID64) { s.in, s.aimed = who, true }

// Keen is the command an entity gives itself to keep its sector and the eight round it ticked at
// Level at most (sector.Keen) till it gives Calm: a battle its rules would see fought out unseen.
type Keen struct{ Level lod.Level }

// Calm is the command an entity gives itself to take off the sector.Keen it carries.
type Calm struct{}

// Pause switches the tactical pause: game time stands, the cameras and the picture go on. P and
// Shift+P by default.
type Pause struct{}

// Faster raises the tempo of the simulation a notch. ] by default.
type Faster struct{}

// Slower lowers the tempo of the simulation a notch. [ by default.
type Slower struct{}

// Tempo sets the tempo of the simulation to the clock's nearest X (clock.Config.Tempos).
type Tempo struct{ X float32 }

// SwitchBands switches the sectors' levels on, or off with every sector ticked at Standard
// (Settings.Unbanded): a way of weighing what the levels spare. The settings screen alone, in a
// world cut into sectors.
type SwitchBands struct{ On bool }

func (Despawn) Command()     {}
func (Spawn) Command()       {}
func (SpawnPart) Command()   {}
func (Keen) Command()        {}
func (Calm) Command()        {}
func (Pause) Command()       {}
func (Faster) Command()      {}
func (Slower) Command()      {}
func (Tempo) Command()       {}
func (SwitchBands) Command() {}

var (
	_ command.Command = Despawn{}
	_ command.Command = Spawn{}
	_ command.Command = SpawnPart{}
	_ host.Aimed      = (*SpawnPart)(nil)
	_ command.Command = Keen{}
	_ command.Command = Calm{}
	_ command.Command = Pause{}
	_ command.Command = Faster{}
	_ command.Command = Slower{}
	_ command.Command = Tempo{}
	_ command.Command = SwitchBands{}
)

// Settings is what the world lets the players adjust, saved with the game: the looks of the
// sectors and whether their levels are ticked. The tempo and the pause are the clock's
// (clock.State).
type Settings struct {
	Sectors  SectorsShown
	Unbanded bool
}

// Queues are the queues of every command the world carries out.
func (p *Plugin) Queues() []command.CommandQueue {
	m := p.module
	return []command.CommandQueue{
		&m.pauses, &m.fasters, &m.slowers, &m.tempos, &m.bandSwitches, &m.sectorSwitches,
		&m.spawns, &m.spawnParts, &m.despawns, &m.keens, &m.calms, &m.effectCmds.queue,
		&m.effectCmds.triggers,
	}
}

// Settings are the tempo, with more than one to choose from, and, in a world cut into sectors,
// what is drawn of them and whether their levels are ticked (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	var out []control.Setting
	if tempos := p.module.clock.Read().Tempos(); len(tempos) > 1 {
		options := make([]string, len(tempos))
		for i, t := range tempos {
			options[i] = fmt.Sprintf("%gx", t)
		}
		out = append(out, control.Choice{
			Label:   "Tempo",
			Options: options,
			Get: func() int {
				return max(0, slices.Index(tempos, p.module.clock.Read().State().Tempo))
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
			Options: []string{"none", "by the level ticked", "as the camera sees them"},
			Get:     func() int { return int(p.Res.Settings.Sectors) },
			Set:     func(i int) command.Command { return SwitchSectors{Shown: SectorsShown(i)} },
		},
		control.Switch{
			Label: "Sectors' levels",
			Get:   p.read.Banded,
			Set:   func(on bool) command.Command { return SwitchBands{On: on} },
		})
}

// DefaultBindings are the clock's keys: P and Shift+P pause, ] goes faster, [ slower. The
// sectors drawn and their levels are the settings screen's.
func (p *Plugin) DefaultBindings() []control.Binding {
	pause := func(control.Context) (Pause, bool) { return Pause{}, true }
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyP}, "Pause the game", pause),
		control.Build(control.KeyPress{Key: control.KeyP, Mods: control.Mods{Shift: true}}, "Pause the game", pause),
		control.Build(control.KeyPress{Key: control.KeyBracketRight}, "Speed the game up", func(control.Context) (Faster, bool) { return Faster{}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketLeft}, "Slow the game down", func(control.Context) (Slower, bool) { return Slower{}, true }),
	}
}
