// Package gram is a modular 2D game engine for Go: a user-implemented [game.Game] — a named set
// of [game.Stage] values, each with its own entity-component world and its own [game.Scene]s —
// driven through Ebitengine's Update/Draw/Layout loop by an engine that wraps the goke ECS.
// Everything beyond the tick loop is a [plugin.Plugin]: the built-in ones give a Stage a world of
// moving boxes, collisions, sight, a board with terrain, pathfinding and mouse selection; a game
// adds its own the same way. [Run] is the whole public surface of this package.
//
// # Game, Stage and Scene
//
// A Game supplies [game.Props] (window, tick rate) and its Stages by name, plus which one starts.
// A Stage is one self-contained context the game can be in — a menu, the gameplay, a map screen —
// with its own goke ECS, built fresh the moment [game.Runtime.SwitchStage] enters it, so a menu
// Stage sits idle with no gameplay entities until the player starts. Its lifecycle is Init
// (install plugins through a [game.Initializer]), Restore (resume from a save, or report there is
// none), Spawn (seed the initial state, only when Restore found nothing) and Update (one tick).
//
// Within a Stage, a Scene is one thing it can show: its renderers (Layers, built once on entering
// the Stage) and its input handling. The Stage's [game.Scenes] is the static registry of its
// Scenes; the live [game.Composition] over it says which are visible, in what order, and which is
// active — the topmost focusable one, the only Scene whose HandleEvents runs. A Stage has no input
// handling of its own. [game.Runtime] is one undivided interface — pause, quit, switch Stage,
// persistence, the camera — that reaches a Stage and every Scene alike.
//
// # Plugins and behaviors
//
// A [plugin.Plugin] is installed from Stage.Init through ctx.Use. Its Install only queues ECS
// wiring; the engine flushes it all in one ecs.Setup after Init returns, which is what lets
// Restore decide fresh-spawn or restore before the ECS commits to either. A plugin needing another
// plugin takes it as a constructor argument — construction order in the game's code is the
// dependency order; there is no registry, no lookup by name and no install-order retry.
//
// Game logic that reacts to what a plugin finds is a behavior, registered on the plugin it
// concerns and run inside that plugin's own pass, built with that plugin's constructors:
// collision.Between of a Meeting for every pair of entities it meets, one carrying tag A and the
// other B; board.Each of a Standing for every entity carrying T. The payload type says whose the behavior is — a Meeting is collision's,
// a Sighting is vision's — and a plugin refuses one made for another, so registering in the wrong
// place is an error, never a silent no-op.
//
// # Kinds and spawning
//
// What an entity is comes from a kind, defined in Stage.Init with package plugins/world/kind: a
// Spec lists the components every entity of the kind carries, each the same for all (Const) or
// read from that entity's own row (Load). Define hands back the kind; its Entry puts one entity on
// the world's roster (Seed), and the engine spawns the roster (Populate) only when nothing was
// restored. Kinds also tell save files which component types to expect, so a game's own tags and
// state survive a save without being registered anywhere else.
//
// # Tick
//
// Props.TargetTPS is the engine's one fixed step. Ebitengine runs one Update per frame; a frame
// that falls behind runs at most five steps and drops the rest, so the game slows down instead of
// spiralling. Each step calls the active Scene's HandleEvents, then Stage.Update, where the game
// runs its plugins' RunPlan in the order it needs — world first, then whatever reads the world's
// space (collision, vision, ...), as the examples do.
//
// # Persistence
//
// [game.Persistence] saves and loads the active Stage's ECS together with every tracked value's
// state: a plugin's Serializable, or anything a Stage handed to ctx.Track, such as its Scene
// Composition. Resources are matched by name — a plugin's Name, or the Go type name of anything
// else — never by position, so a save survives plugins being added, removed or reordered between
// game versions. PostLoader and Restorer are the hooks run after a load.
//
// # Package dependencies
//
// The packages form a strict acyclic graph; each imports only layers below it:
//
//	Layer 0   camera              — a Camera over a world: screen conversion, culling, move and zoom
//	Layer 1   render              — drawing primitives: Renderer, Atlas, QuadBatch, sprites          (→ camera)
//	          control             — the input vocabulary: InputEvents, KeyEvent, ClickEvent, EventHandler;
//	                                commands and bindings: Queue, Issued, Binding, Command, the triggers   (→ camera)
//	Layer 2   plugin              — the extension contract: Plugin, Installer, Tick, Between and Each,
//	                                PairHost and EachHost, Serializable, PostLoader, Populator      (→ control, render)
//	Layer 3   plugins/world/kind  — what an entity is: Spec, Const and Load, Define, Of, Registry    (→ render)
//	Layer 4   plugins/world       — the foundation: Base (Position, Velocity, Caps), the Space,
//	                                movement, kinds, Seed and Populate, Attach and Detach, Camera   (→ camera, control, plugin, kind, render)
//	Layer 5   game                — what a game implements and receives: Game, Stage, Scene, Scenes,
//	                                Composition, Initializer, Runtime, Persistence, Props, TPS       (→ camera, control, plugin, world, render)
//	          plugins/collision   — the CollisionSystem over the world's Space; Collider, Physics, Meeting, Struck (→ world, …)
//	          plugins/selection   — a Select command into a Selected tag                           (→ world, …)
//	          plugins/vision      — a Sight cone into Seen, Sighting, SightOutline                   (→ world, …)
//	          plugins/effects     — temporary changes to entities: Grant and Alter, cast anywhere    (→ world, …)
//	Layer 6   plugins/board       — a grid with terrain over the world, walls as bodies              (→ world, collision, …)
//	          plugins/collision/behavior, plugins/vision/behavior — ready-made reactions              (→ their plugin, world, plugin)
//	Layer 7   plugins/navigation  — MoveOrder paths across a board                                   (→ board, selection, world, …)
//	          plugins/players     — a carrier over the command handlers: players, their bindings, Pan and Zoom (→ world, …)
//	Layer 8   internal/engine     — the Engine: the Ebitengine loop, one active Stage, persistence   (→ game, plugin, world, camera, control, render)
//	Layer 9   gram                — Run; the package you import                                     (→ game, internal/engine)
//
// Expressed as a directed graph (arrow = "is imported by"), showing the spine:
//
//	camera ──► render ──► plugin ──► plugins/world/kind ──► plugins/world ──► game ──► internal/engine ──► gram
//	control ───┘                                              │  ▲
//	                                                          ▼  │
//	                     plugins/{collision, selection, vision, effects} ──► plugins/board ──► plugins/navigation, plugins/*/behavior
//
// Outside the module: goke/v3 is the ECS every Stage runs on, aabbworld the space, collisions and
// line of sight under the world, ebiten/v2 the loop and the drawing, astar the pathfinding, and
// uid the entity identifiers.
package gram
