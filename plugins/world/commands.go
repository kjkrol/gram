package world

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
)

// Despawn is the command an entity gives itself to leave the world: gone in the step it gives it.
type Despawn struct{}

// Spawn is the command to add an entity of a kind to the running world, as Seed does before the
// game: the kind's Loads read the row of its Entry, as given. A player, the game's code
// (control.Nobody, Plugin.Spawn) or an entity (Order in a rule or a plan: a building raising a
// recruit at its gate, the Entry fixed as the rule is written) gives it; it is carried out at the
// world's next step of the simulation — the same step for a plan's Order, none in the tactical
// pause — and refused with a log line, never a panic, for an unknown kind, a wrong row, a world
// that is full (Config.Entities.MaxCount), a size out of bounds or a box wholly past an open edge;
// at a closed edge the box is stopped inside, as at Populate. A Load that panics is the game's
// own bug, as it is at Populate.
type Spawn struct{ Entry kind.Entry }

// Pause holds the simulation in the tactical pause, or lets it go on: the cameras, the players'
// commands and the picture go on, game time stands. Space by default.
type Pause struct{}

// Faster raises the tempo of the simulation a notch (Config.Clock.Tempos), no further than the
// fastest. ] by default.
type Faster struct{}

// Slower lowers the tempo of the simulation a notch, no further than the slowest. [ by default.
type Slower struct{}

// Queues are the clock's, Spawn's, Despawn's, the commands' about effects (rule.Command,
// rule.Triggered) and the steering's (steering.Away, Toward, Turn) — for the players plugin, which
// carries the world's commands itself.
func (p *Plugin) Queues() []control.CommandQueue {
	m := p.module
	q := []control.CommandQueue{&m.pauses, &m.fasters, &m.slowers, &m.spawns, &m.despawns, &m.effectCmds.queue, &m.effectCmds.triggers}
	return append(q, m.steer.Queues()...)
}

// DefaultBindings are the clock's keys: Space pauses, ] goes faster, [ slower.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Command(control.KeyPress{Key: control.KeySpace}, "Pause the game", func(control.Context) (Pause, bool) { return Pause{}, true }),
		control.Command(control.KeyPress{Key: control.KeyBracketRight}, "Speed the game up", func(control.Context) (Faster, bool) { return Faster{}, true }),
		control.Command(control.KeyPress{Key: control.KeyBracketLeft}, "Slow the game down", func(control.Context) (Slower, bool) { return Slower{}, true }),
	}
}
