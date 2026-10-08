package stage

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin/section"
)

// New begins the Stage named name: go on with its sections, each once and in their order — any
// may be left out — and end with Update, which hands back the game.Stage.
func New(name string) Start {
	d := &def{name: name}
	return Start{AfterPlugins{AfterPlayers{AfterEffects{AfterRules{AfterCommands{AfterKinds{AfterControls{AfterRestore{AfterSpawn{AfterScenes{AfterShows{d}}}}}}}}}}}}
}

// Start is a Stage of which nothing is defined yet. Each type on is the Stage after a section:
// it has the sections that come later, never one that came before.
type Start struct{ AfterPlugins }

// Plugins makes and uses the Stage's plugins: ctx.UseWorld, ctx.Use.
func (s Start) Plugins[F Step](f F) AfterPlugins {
	s.d.add(section.Plugins, stepOf(f))
	return s.AfterPlugins
}

type AfterPlugins struct{ AfterPlayers }

// Players makes the players and binds the plugins' default keys.
func (s AfterPlugins) Players[F Step](f F) AfterPlayers {
	s.d.add(section.Players, stepOf(f))
	return s.AfterPlayers
}

type AfterPlayers struct{ AfterEffects }

// Effects defines the states.
func (s AfterPlayers) Effects[F Step](f F) AfterEffects {
	s.d.add(section.Effects, stepOf(f))
	return s.AfterEffects
}

type AfterEffects struct{ AfterRules }

// Rules defines the roles with the rules they obey, the plans, and who plays what beyond the
// kinds: a plugin (world.Plugin.Plays).
func (s AfterEffects) Rules[F Step](f F) AfterRules {
	s.d.add(section.Rules, stepOf(f))
	return s.AfterRules
}

type AfterRules struct{ AfterCommands }

// Commands names what can be asked for (rule.Cast, Lift, Toggle; the plugins' commands) — for
// whom, a role's players among them — and hands the Initializer those an entity sets off.
func (s AfterRules) Commands[F Step](f F) AfterCommands {
	s.d.add(section.Commands, stepOf(f))
	return s.AfterCommands
}

type AfterCommands struct{ AfterKinds }

// Kinds defines the kinds — of cells, each with the roles its cells play, and of units — in steps
// run in their order: Kinds(s.defineCells, s.defineKinds).
func (s AfterCommands) Kinds[F Step](steps ...F) AfterKinds {
	for _, f := range steps {
		s.d.add(section.Kinds, stepOf(f))
	}
	return s.AfterKinds
}

type AfterKinds struct{ AfterControls }

// Controls binds the game's own keys, each to a command.
func (s AfterKinds) Controls[F Step](f F) AfterControls {
	s.d.add(section.Controls, stepOf(f))
	return s.AfterControls
}

type AfterControls struct{ AfterRestore }

// Restore resumes the Stage from a save, or reports false: a Stage without it starts fresh.
func (s AfterControls) Restore(f func(game.Persistence) (bool, error)) AfterRestore {
	s.d.restore = f
	return s.AfterRestore
}

type AfterRestore struct{ AfterSpawn }

// Spawn seeds a fresh game's world in steps run in their order — its board, its units:
// Spawn(s.spawnCells, s.spawnUnits). A loaded game skips them.
func (s AfterRestore) Spawn[F SpawnStep](steps ...F) AfterSpawn {
	for _, f := range steps {
		s.d.spawn = append(s.d.spawn, spawnOf(f))
	}
	return s.AfterSpawn
}

type AfterSpawn struct{ AfterScenes }

// Scenes makes the Stage's scenes once the world is there — fresh or loaded: the first is shown,
// unless Shows says which; their stack and its Composition, tracked for the saves, are the
// Stage's own doing.
func (s AfterSpawn) Scenes[F ScenesStep](f F) AfterScenes {
	switch g := any(f).(type) {
	case func() []game.Scene:
		s.d.scenes = func(game.Initializer) ([]game.Scene, error) { return g(), nil }
	case func() ([]game.Scene, error):
		s.d.scenes = func(game.Initializer) ([]game.Scene, error) { return g() }
	case func(game.Initializer) []game.Scene:
		s.d.scenes = func(ctx game.Initializer) ([]game.Scene, error) { return g(ctx), nil }
	case func(game.Initializer) ([]game.Scene, error):
		s.d.scenes = g
	}
	return s.AfterScenes
}

type AfterScenes struct{ AfterShows }

// Shows names the scenes shown as the Stage starts, bottom first, in place of the first alone; a
// loaded game shows those shown when it was saved.
func (s AfterScenes) Shows(names ...string) AfterShows {
	s.d.shows = append([]string{}, names...)
	return s.AfterShows
}

// AfterShows is a Stage wanting only its Update.
type AfterShows struct{ d *def }

// Update is the Stage's tick — the plugins' RunPlans in their order — and ends the chain: the
// game.Stage so defined.
func (s AfterShows) Update(f func(ctx goke.RunCtx, d time.Duration)) game.Stage {
	s.d.update = f
	return built{s.d}
}
