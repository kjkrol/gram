// Package game is what a game implements and what it is handed back: the [Game] the engine
// drives, its [Stage]s and their [Scene]s, and the [Initializer], [Runtime] and [Persistence] a
// Stage or Scene works through. Everything a third party writes against lives here, in the open;
// only the engine that drives it is internal.
//
// # Game
//
// A [Game] supplies [Props] — window title and size, and TargetTPS, the engine's one fixed step —
// and its Stages by Name, plus which starts active. Props.Resizable lets the player resize and
// maximize the window: the screen is then the window, and the world camera draws to all of it —
// more of a large world, a small one scaled up to cover it. F11 toggles fullscreen in every
// game ([Runtime].ToggleFullscreen). Every game writes its own small Game, even
// for a single Stage, since only a concrete type can supply its own Props. The engine never keeps
// a copy of the Stage set: it asks Stages() whenever it resolves a name.
//
// # Stage
//
// A [Stage] is one self-contained context the game can be in, with its own plugins and its own
// goke ECS, both built fresh when the Stage is entered. Init installs plugins through the
// Initializer, may call UseWorld once and defines the game's roles and rules; Restore resumes from a save or
// reports there is none; Spawn seeds the initial state, run only when Restore found nothing;
// Update advances the simulation one tick by running the plugins' RunPlan in the order the game
// needs. A Stage handles no input: that is a Scene's. A game seldom writes those by hand: package
// game/stage defines a Stage a section at a time — plugins, players, effects, rules, commands,
// kinds, controls, restore, spawn, scenes, update — always in that order, its scenes made once the
// world is there, loaded or spawned.
//
// # Scene, Scenes and Composition
//
// A [Scene] is one thing a Stage can show: Layers, bottom to top, built once when the Stage is
// entered — render.Renderers drawn on the screen; a scene showing the world is a ui.Scene, its
// screen composed of elements, the world among them a picture through a camera (a player's view,
// split-screen halves, a minimap); HandleEvents, this tick's input, run only while the Scene is active (the
// moves of the game go to the players plugin's EventHandler, the rest — pause, quit — stay here); and
// Focusable, whether it can ever be active. [Scenes], built by [NewStack], is the Stage's static
// registry of them by Name. Its [Composition] is the live state: which Scenes are visible, in what
// order, and Active, the topmost focusable one — so a non-focusable HUD can sit on top and never
// steal input. Composition is Serializable; a Stage hands it to Initializer.Track so visibility and
// order survive a save — tracked after a Load, it gets the saved state as it is tracked.
//
// # Initializer
//
// [Initializer] is what Init receives: Use installs a plugin (registering its Serializable and
// calling its Install), Track registers any other Serializable for saves, UseWorld builds and
// installs this Stage's world plugin from a world.Config (a second call panics), and TPS is the
// engine's measured tick counter. It embeds plugin.Installer, so a Stage may wire ECS modules and
// systems of its own the way a plugin does.
//
// A Stage hands its rules to nobody: a rule is a role's (world.Roles), a role is played — by a kind
// (rule.Plays), by a kind of cell or one cell of the Layout (cell.Kinds.Define, cell.Entry.Plays),
// by a plugin (world.Plugin.Plays) — and once
// Init returns the engine gives the rules of every role played to the plugin in use that catches
// their moment, a unit.Standing's to the board, a vision.Sighting's to vision. A rule no plugin
// in use catches fails Init with an error wrapping plugin.ErrUnhosted, naming the rule.
//
// # Runtime
//
// [Runtime] is engine-level control, one undivided interface: Paused, Pause, Resume, TogglePause,
// Quit, SwitchStage to another Stage by name, Persistence, TPS and ToggleFullscreen. The
// same value reaches a Stage and every Scene's HandleEvents; there is no cut-down scene-level
// subset. A menu's Start button calls SwitchStage directly.
//
// # Persistence
//
// [Persistence] is Save, Load and List over the active Stage's ECS and every tracked value, under
// a base path and a label ("" is the quicksave). Save takes extra resources to write alongside;
// Load restores what the snapshot holds and leaves anything absent unchanged.
package game
