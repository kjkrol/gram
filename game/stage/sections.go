package stage

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/world"
)

// New begins the Stage named name: go on with its sections, each once and in their order — any
// may be left out — and end with Update, which hands back the game.Stage.
func New(name string) Start {
	d := &def{name: name}
	return Start{AfterWorld{AfterPlugins{AfterPlayers{AfterEffects{AfterRules{AfterCommands{AfterKinds{AfterControls{AfterSpawn{AfterScenes{AfterShows{d}}}}}}}}}}}}
}

// Start is a Stage of which nothing is defined yet. Each type on is the Stage after a section:
// it has the sections that come later, never one that came before.
type Start struct{ AfterWorld }

// World makes the Stage's world from cfg, anew each time the Stage is entered: what its plugins
// are made over.
func (s Start) World(cfg world.Config) AfterWorld {
	s.d.add(section.Plugins, func(ctx game.Initializer) error {
		s.d.world = ctx.UseWorld(cfg)
		return nil
	})
	return s.AfterWorld
}

// Plugins has plugins make the plugins of a Stage without a world — each adding itself to made as
// it is made — and uses them in the order made.
func (s Start) Plugins(plugins func(made *plugin.Made)) AfterPlugins {
	s.d.add(section.Plugins, func(ctx game.Initializer) error {
		var made plugin.Made
		plugins(&made)
		return s.d.use(ctx, made.All())
	})
	return s.AfterPlugins
}

// AfterWorld is a Stage with a world and no plugins yet.
type AfterWorld struct{ AfterPlugins }

// Plugins has plugins make the plugins over the Stage's world, anew each time the Stage is
// entered — each telling the world of itself as it is made (world.NewSelf) — and uses them in the
// order made: a game's arena keeps them as it makes them.
func (s AfterWorld) Plugins(plugins func(w *world.Plugin)) AfterPlugins {
	s.d.add(section.Plugins, func(ctx game.Initializer) error {
		plugins(s.d.world)
		return s.d.use(ctx, s.d.world.Made().All())
	})
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

type AfterControls struct{ AfterSpawn }

// Spawn seeds a fresh game's world in steps run in their order — its board, its units:
// Spawn(s.spawnCells, s.spawnUnits). A game loaded (game.Load) skips them.
func (s AfterControls) Spawn[F SpawnStep](steps ...F) AfterSpawn {
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

// With writes plugins' sections into the Stage where it stands — each between the parts its
// plugin says, or it panics as the chain is built: With(dialog.Conversations(s.defineConversations))
// after the rules, before the kinds. Each runs with the plugin it is of, among those World or
// Plugins made.
func (s AfterPlugins) With(sections ...section.Section) AfterPlugins {
	s.d.with(sections)
	return s
}

// With writes plugins' sections into the Stage here, as AfterPlugins.With does.
func (s AfterPlayers) With(sections ...section.Section) AfterPlayers {
	s.d.with(sections)
	return s
}

// With writes plugins' sections into the Stage here, as AfterPlugins.With does.
func (s AfterEffects) With(sections ...section.Section) AfterEffects {
	s.d.with(sections)
	return s
}

// With writes plugins' sections into the Stage here, as AfterPlugins.With does.
func (s AfterRules) With(sections ...section.Section) AfterRules {
	s.d.with(sections)
	return s
}

// With writes plugins' sections into the Stage here, as AfterPlugins.With does.
func (s AfterCommands) With(sections ...section.Section) AfterCommands {
	s.d.with(sections)
	return s
}

// With writes plugins' sections into the Stage here, as AfterPlugins.With does.
func (s AfterKinds) With(sections ...section.Section) AfterKinds {
	s.d.with(sections)
	return s
}

// With writes plugins' sections into the Stage here, as AfterPlugins.With does.
func (s AfterControls) With(sections ...section.Section) AfterControls {
	s.d.with(sections)
	return s
}
