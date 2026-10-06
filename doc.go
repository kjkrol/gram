// Package gram is a modular game engine for Go: a user-implemented [game.Game] — a named set of
// [game.Stage] values, each with its own entity-component world and its own [game.Scene]s —
// driven by an engine that wraps the goke ECS in a window's loop, a tick and a picture a frame,
// the picture drawn on the GPU through WebGPU (gogpu).
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
// A game defines it a section at a time with package game/stage — stage.New(name).Plugins(…).
// Players(…).Cells(…).Effects(…).Rules(…).Commands(…).Kinds(…).Controls(…).Looks(…).Scenes(…).
// Layout(…).Units(…).Update(…) — always in that order, which the compiler keeps, each plugin
// refusing what is defined out of its section.
//
// Within a Stage, a Scene is one thing it can show: its renderers (Layers, built once on entering
// the Stage) and its input handling. The Stage's [game.Scenes] is the static registry of its
// Scenes; the live [game.Composition] over it says which are visible, in what order, and which is
// active — the topmost focusable one, the only Scene whose HandleEvents runs. A Stage has no input
// handling of its own. [game.Runtime] is one undivided interface — pause, quit, switch Stage,
// persistence, the camera — that reaches a Stage and every Scene alike.
//
// # Plugins and rules
//
// A [plugin.Plugin] is installed from Stage.Init through ctx.Use. Its Install only queues ECS
// wiring; the engine flushes it all in one ecs.Setup after Init returns, which is what lets
// Restore decide fresh-spawn or restore before the ECS commits to either. A plugin needing another
// plugin takes it as a constructor argument — construction order in the game's code is the
// dependency order; there is no registry, no lookup by name and no install-order retry.
//
// Game logic that reacts to what a plugin finds is a rule (rule.Then), run inside the pass of the
// plugin whose moment it is: a rule of a collision.Meeting for every pair of entities it meets,
// one carrying tag A and the other B; of a unit.Standing for every entity on the board. The
// payload type says whose the rule is — a Meeting is collision's, a Sighting is vision's. A rule
// is a role's, and nobody hands it to a plugin: once a Stage's Init returns the engine gives the
// rules of every role somebody plays to the plugin in use that catches their moment, and one none
// catches is an error, never a silent no-op. What
// lasts over ticks is a kind's plan, of the same steps (package rule): it casts effects and orders
// commands — the same as a player's — for its entity.
//
// A role (world.Roles) is a behaviour an entity plays — mortal, hasty, a trapdoor — not a group:
// the rules it obeys fire for those playing it alone. A kind plays roles through one component
// (rule.Plays), a cell through its kind (cell.Kinds.Define) or alone (cell.Entry.Plays), the world and the atmosphere, for the
// moments of the world as a whole, through their own Plays. What somebody
// asks for is a command, and one about an effect is a sentence: rule.Cast(open).On(entity.Group(
// "trapdoors")).By(entity.Named("lever")) — put the effect on, take it off (Lift) or switch it
// (Toggle), for the entities bearing a name, those in a group, the world itself, the player's
// selected units or the one pointed at; a key, a script and a rule give it the same way, and an
// entity sets off the commands whose By names it (rule.Trigger): a lever, a plate and the
// trapdoors they open.
//
// # Kinds and spawning
//
// What an entity is comes from a kind, defined in Stage.Init with package entity/kind: a
// Spec lists the components every entity of the kind carries, each the same for all (Const) or
// read from that entity's own row (Load). Define hands back the kind; its Entry puts one entity on
// the world's roster (Seed), and the engine spawns the roster (Populate) only when nothing was
// restored. Kinds also tell save files which component types to expect, so a game's own tags and
// state survive a save without being registered anywhere else.
//
// # Tick
//
// Props.TargetTPS is the engine's one fixed step. The window's loop runs one Update per frame,
// stepping as many times as the time gone says; a frame that falls behind runs at most five steps
// and drops the rest, so the game slows down instead of spiralling. What is drawn goes by the
// clock's Shown time, which runs on between the steps, so it moves every frame. Each step calls the active Scene's HandleEvents, then Stage.Update, where the game
// runs its plugins' RunPlan in the order it needs — the world before whatever reads its space
// (collision, vision, ...), and before it only what moves entities itself (bullet, so that the
// same step's collision tests its flights), as the examples do.
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
//	Layer 1   render              — drawing: Renderer, Composer, Frame, Atlas, sprites                (→ camera)
//	          control             — the input vocabulary: InputEvents, KeyEvent, ClickEvent, EventHandler;
//	                                commands and bindings: Queue, Issued, Binding, Command, the rules   (→ camera)
//	          entity/tag          — tag families: Tags, Tag, Any; a leaf                          (→ nothing)
//	Layer 2   entity/kind         — what an entity is: Spec, Const and Load (kind/comp), Define, Of, Registry (→ render, tag)
//	          clock               — the tactical clock: time, pause, tempo, phases, Moment, At, Every (→ control, render, tag)
//	          rule/effect         — temporary changes to entities: Grant and Alter; made and run by the world (→ tag)
//	Layer 3   entity              — what every entity carries: Base, Position, Velocity, Z, Layers, Eye (→ kind)
//	          internal/steps      — the engine running the steps of rules and plans                (→ control, effect)
//	          plugin              — the extension contract: Plugin, Installer, CommandHandler, Serializable,
//	                                PostLoader, Populator, Restorer; the hosts of rules (Rules, PairRules,
//	                                StepRules), Tick, Marks, the moments' faces    (→ control, render, tag, effect)
//	          plugins/players/owner — whose a unit is: the owners' tags, Obeys, Allies; a leaf read by selection, navigation and the cameras (→ control, tag)
//	Layer 4   rule                — rules at a plugin's moments: On, Then, the filters, the steps; roles (Role, Plays)
//	                                and commands about effects (Cast, Lift, Toggle, Trigger)    (→ control, plugin, entity, steps, tag, effect, kind/comp)
//	Layer 5   rule/plan           — what an entity does over time: New, Actor, Command, asks; run by the world (→ rule, steps, effect, kind/comp)
//	          plugins/world       — the foundation: Base (Position, Velocity, Caps), the Space,
//	                                movement, kinds, Seed and Populate, Spawn, Despawn, names and groups, the carrier of commands, Camera; it runs
//	                                the core's systems: the clock's, the plans', the effects' (→ camera, control, plugin, entity, kind, clock, rule, steps, render)
//	Layer 6   game                — what a game implements and receives: Game, Stage, Scene, Scenes,
//	                                Composition, Initializer, Runtime, Persistence, Props, TPS       (→ camera, control, plugin, rule, world, render)
//	          plugins/collision   — collision over the world's Space; Collider, Physics, Meeting, Struck (→ world, …)
//	          plugins/selection   — a Select command into a Selected tag; the roles' abilities     (→ world, rule, …)
//	          plugins/vision      — a Sight cone into Sighted, Sighting, SightOutline                   (→ world, …)
//	Layer 7   plugins/board       — a grid with terrain over the world, the solid ground and cover   (→ world, …)
//	Layer 8   plugins/navigation  — MoveOrder paths across a board                                   (→ board, selection, world, …)
//	          plugins/bullet      — shots fired, flown past the step cap and swept, landing, resting and bursting (→ world, collision, selection, board/ground, …)
//	          plugins/topography  — a map in relief drawn on the GPU: the heights, the light and the water on them, the views from above, isometric and in perspective;
//	                                its parts relief, painter, water, terrain, hexes, billboards, cameras (→ world, board, selection, atmosphere/sky, …)
//	          plugins/atmosphere  — the calendar, the climate, the weather and the sky on the world's clock; the celestial sphere
//	                                (atmosphere/celestial), the clouds, what falls, the weathering (→ world, board, …)
//	          plugins/players     — a carrier over the command handlers: players, their bindings, Pan and Zoom (→ world, …)
//	Layer 9   internal/engine     — the Engine: the window's loop (gogpu), one active Stage, persistence, the handing of the
//	                                roles' rules to the plugins' hosts (→ game, plugin, rule, world, camera, control, render)
//	Layer 10  gram                — Run; the package you import                                     (→ game, internal/engine)
//
// Expressed as a directed graph (arrow = "is imported by"), showing the spine:
//
//	camera ──► render ──► plugin ──► rule ──► plugins/world ──► game ──► internal/engine ──► gram
//	control ───┘                              │  ▲
//	                                          ▼  │
//	                     plugins/{collision, selection, vision} ──► plugins/board ──► plugins/navigation, plugins/bullet, plugins/topography, plugins/atmosphere
//
// Outside the module: goke/v3 is the ECS every Stage runs on, aabbworld the space, collisions and
// line of sight under the world, gogpu (with wgpu and naga) the window, the loop and the GPU,
// astar the pathfinding, and uid the entity identifiers.
package gram
