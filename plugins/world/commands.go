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

// SpawnWhole is the command to add a thing standing in pieces to the running world at the next
// step: the Whole's handle and every piece, each carrying PieceOf it. One refused (a handle whose
// kind carries no Whole, an entry refused as a Spawn is, or no room for them all) spawns none of
// them and is logged; a handle's box joins its pieces', bounded by no Entities.MaxSize.
type SpawnWhole struct {
	Whole  kind.Entry
	Pieces []kind.Entry
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

// ShowFar has the loose units drawn at the strategic scale as Look says (Settings.Far), AsConfig
// as the Config does. The settings screen alone, in a world cut into sectors.
type ShowFar struct{ Look FarLook }

// ShowCounts has the number of a kind's units in a sector drawn by them at the strategic scale,
// or not (Settings.Counts). The settings screen alone, in a world cut into sectors.
type ShowCounts struct{ On bool }

// ShowCanopies has a canopy (Slot.Canopy) show the unit standing under it through in a view from
// above, or stand whole (Settings.Canopies). The settings screen alone, under Game.
type ShowCanopies struct{ On bool }

func (Despawn) Command()      {}
func (Spawn) Command()        {}
func (SpawnPart) Command()    {}
func (SpawnWhole) Command()   {}
func (Keen) Command()         {}
func (Calm) Command()         {}
func (Pause) Command()        {}
func (Faster) Command()       {}
func (Slower) Command()       {}
func (Tempo) Command()        {}
func (SwitchBands) Command()  {}
func (ShowFar) Command()      {}
func (ShowCounts) Command()   {}
func (ShowCanopies) Command() {}

var (
	_ command.Command = Despawn{}
	_ command.Command = Spawn{}
	_ command.Command = SpawnPart{}
	_ host.Aimed      = (*SpawnPart)(nil)
	_ command.Command = SpawnWhole{}
	_ command.Command = Keen{}
	_ command.Command = Calm{}
	_ command.Command = Pause{}
	_ command.Command = Faster{}
	_ command.Command = Slower{}
	_ command.Command = Tempo{}
	_ command.Command = SwitchBands{}
	_ command.Command = ShowFar{}
	_ command.Command = ShowCounts{}
	_ command.Command = ShowCanopies{}
)

// Settings is what the world lets the players adjust, saved with the game: the looks of the
// sectors, whether their levels are ticked, how the loose units are drawn far off and whether a
// canopy shows the unit under it. The tempo and the pause are the clock's (clock.State).
type Settings struct {
	Sectors  SectorsShown
	Unbanded bool
	Far      FarLook
	Counts   Shown
	Canopies Shown
}

// Shown is whether a setting has something drawn, or AsConfig.
type Shown uint8

// The Showns but AsConfig.
const (
	On Shown = iota + 1
	Off
)

// Queues are the queues of every command the world carries out.
func (p *Plugin) Queues() []command.CommandQueue {
	m := p.module
	return []command.CommandQueue{
		&m.pauses, &m.fasters, &m.slowers, &m.tempos, &m.bandSwitches, &m.sectorSwitches,
		&m.canopies,
		&m.farLooks, &m.counts, &m.spawns, &m.spawnParts, &m.spawnWholes, &m.despawns, &m.keens,
		&m.calms, &m.effectCmds.queue, &m.effectCmds.triggers,
	}
}

// Settings are the tempo, with more than one to choose from, whether a canopy shows the unit
// under it (under Game) and, in a world cut into sectors, what is drawn of them, whether their
// levels are ticked and how the loose units are drawn far off (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	out := []control.Setting{control.Switch{
		Label:   "Canopies show the units under them",
		Section: control.Game,
		Get:     p.read.Canopies,
		Set:     func(on bool) command.Command { return ShowCanopies{On: on} },
	}}
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
		},
		control.Choice{
			Label:   "Far units",
			Section: control.Graphics,
			Options: []string{"points", "banners"},
			Get:     func() int { return int(p.read.FarLook()) - 1 },
			Set:     func(i int) command.Command { return ShowFar{Look: FarLook(i + 1)} },
		},
		control.Switch{
			Label:   "Counts",
			Section: control.Graphics,
			Get:     p.read.Counts,
			Set:     func(on bool) command.Command { return ShowCounts{On: on} },
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
