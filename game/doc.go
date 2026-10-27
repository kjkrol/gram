// Package game is what a game implements and what it is handed back: the [Game] the engine
// drives, its [Stage]s and their [Scene]s, and the [Initializer], [Runtime] and [Persistence] a
// Stage or Scene works through. Everything a third party writes against lives here, in the open;
// only the engine that drives it is internal.
//
// # Game
//
// A [Game] supplies its [Props] (the window, a Description for the game's own menu, TargetTPS
// the engine's fixed step) and its Stages by Name, plus which starts active. [New] is
// the Game of props and stages a game keeping nothing else of its own needs, the first stage
// starting unless [Defined.Starting] names another; a game wanting more writes its own.
//
// # Stage
//
// A [Stage] is one self-contained context the game can be in, with its own plugins and its own
// goke ECS, both built fresh when the Stage is entered. Init installs plugins through the
// Initializer; Spawn seeds the initial state of a Stage entered afresh, Loaded is told in its place
// when it was entered from a save; Update advances the simulation one tick. A Stage handles no
// input: that is a Scene's. A game seldom writes a Stage by hand: package game/stage defines one
// a section at a time.
//
// # Saving and loading
//
// Saving and loading are the engine's, asked for by commands it carries out: [Save] the active
// Stage in a slot, [Load] a Stage from its save, from any Stage (a menu's Continue), [Enter] a
// Stage afresh, [Quit]. They are given like any command: a key's binding, a ui button, a rule's
// Order, [Runtime].Issue. A save is the Stage's, a file a slot in [Props].Saves; a Stage entered
// from one is built anew and its world loaded in place of Spawn.
//
// # Scene, Scenes and Composition
//
// A [Scene] is one thing a Stage can show: Drawers, bottom to top, render.Renderers drawn on the
// screen (the world among them through a ui.Image of a render.Feed); HandleEvents, this tick's
// input, run only while the Scene is active; and Focusable, whether it can ever be active.
// [Scenes], built by [NewStack], is the Stage's registry of them by Name. Its [Composition] is the
// live state: which Scenes are visible, in what order, and Active, the topmost focusable one, so a
// non-focusable HUD can sit on top and never steal input. Composition is Serializable; a Stage
// hands it to Initializer.Track so visibility and order survive a save.
//
// # Initializer
//
// [Initializer] is what Init receives: Use installs a plugin, Track registers any other
// Serializable for saves, UseWorld builds this Stage's world plugin from a world.Config, and TPS is
// the engine's measured tick counter. It embeds plugin.Installer, so a Stage may wire ECS modules
// and systems of its own the way a plugin does.
//
// A Stage hands its rules to nobody: a rule is a role's (world.Roles), a role is played by a kind,
// a cell or a plugin, and once Init returns the engine gives the rules of every role played to the
// plugin in use that catches their moment. A rule no plugin in use catches fails Init with an error
// wrapping host.ErrUnhosted, naming the rule.
//
// # Runtime
//
// [Runtime] is engine-level control, one undivided interface: the pause, Quit, SwitchStage, Save,
// Load, Issue, Persistence, TPS and ToggleFullscreen. The same value reaches a Stage and every
// Scene's HandleEvents.
//
// # Persistence
//
// [Persistence] is Save, Load and List over the active Stage's ECS and every tracked value, under
// a base path and a label ("" is the quicksave). Load restores what the snapshot holds and leaves
// anything absent unchanged.
package game
