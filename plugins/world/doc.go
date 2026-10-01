// Package world is the foundation for a Stage with moving, drawable entities: a Position and
// Velocity each, a shared spatial index other plugins query, and motion integrated each tick,
// never further than Position.MaxStep. What an entity carries is set by its kind — see kind.
//
// The world has sub-packages: entity (what every entity carries; the world names its types
// itself, see below), entity/kind (what an entity is) and entity/tag (tag families), steering
// (heading and speed asked for and reached gradually), view (what one pair of eyes sees), clock
// (game time) and rule (how entities behave: rules, plans, commands, facts; rule/effect the
// temporary changes to entities). The world makes and runs their systems; none of them imports
// the world.
//
// # Plugin
//
// [Plugin] is installed by a Stage's Init through ctx.UseWorld, once, from a [Config]: the
// [SpaceCfg] sizes the world and sets the edge rule per axis (aabbworld.Torus, WrapX or WrapY
// alone, OpenX or OpenY, a closed axis by default — a box stops whole at a closed edge, wraps at
// a wrapping one, may leave by an open one); the [EntitiesCfg] bounds how many entities the world
// holds and the sizes they spawn with; the camera.Config sizes the camera. An entity wholly past
// an open edge carries [Outside] — put on by whoever moved it there, the MoveSystem or collision's
// solver — and every tick it does, the rules of a [Leaving] hooked with [Plugin.Hook] hear of
// it; with none hooked it is despawned. Put back inside, it
// loses the mark. [Plugin.Roster] is what the plugins in the game ask of a unit's kind — see
// package kind; world requires a Position and brings a Velocity. [Layers] are the planes an
// entity is on, one bit each, read by collision and
// sight: two entities meet only where they share a bit, and one carrying none is on every plane.
// Config.Heights gives the world heights: entities carry a [Z] (bottom and rise), which the board
// in relief (plugins/topography) writes from its ground; a flat world refuses a Z. The world knows
// its entities and nothing else: the ground is the board's, the sky the atmosphere's.
// The Plugin exposes the shared [aabbworld.Space] ([Plugin.Space]) and the shared camera
// ([Plugin.Camera]; the players plugin moves it through Pan and Zoom commands).
//
// # Base, Position and Velocity
//
// [Base] is the one component every entity carries: its [Position] (a plane.AABB), its
// [Velocity] (a unit heading and a speed in world units a second), the [kind.ID] it was spawned
// from, and the aabbworld.Capability bits the space indexes it under, which collision writes.
// A host hands Base to whatever it hosts instead of anyone binding it twice. No entity moves
// further in a tick than [StepReach] of its own shorter side ([Position.MaxStep],
// [Position.MaxSpeed]), so mixed sizes share a world without the smallest slowing the rest.
// [Eye] is where an entity looks from and how wide — its cone of sight (plugins/vision) and a
// camera riding in it (plugins/topography) read the one Eye. These, with [Z] and [Layers], are
// the types of package entity under the world's own names (type aliases: one type, so a
// component is the same wherever it is named and saves do not care); the world's sub-packages
// read them from entity, everyone else from here.
//
// # Steering
//
// Package steering is how an entity's wants become motion: a steering.Steering carries the heading
// asked for and a motion profile, and the steering.System, run in every step before movement,
// turns the entity's heading by at most its TurnRate a tick and writes its base speed from the
// profile. steering.Driven marks an entity steered by hand, carried out by the plugin that moves
// entities over the ground (navigation).
//
// # Scale
//
// [Config].Scale ([Scale]) says how many metres a world unit spans, across and up alike. Without
// one the world is a board: flat as far as the eye goes. With one it is a stretch of the Earth's
// surface: a line of sight bends over it — the ground d off sinks [Scale.Drop],
// (1 − [Refraction])·d²/(2·[EarthRadius]), under an eye's level, so level ground past
// [Scale.Horizon] is out of sight. A game gives heights, sizes and reaches in metres through
// [Scale.Units]; sight (plugins/vision) and the topography's perspective read the rest, and the
// atmosphere works out of it how far the air lets one see (plugins/atmosphere/air).
//
// # Kinds, Seed and Populate
//
// [Plugin.Kinds] is the registry kind.Define registers with; [Kinds] also issues atlas slots no
// kind owns ([Kinds.NewSprite]) and tells saves every component type its kinds carry. A Stage's
// Spawn puts entities on the roster with [Plugin.Seed]; the engine calls [Plugin.Populate] only
// when nothing was restored. [GridPlacement] arranges a population on a regular grid.
//
// # Tags
//
// A tag is a bit of a family: [tag.Tags] is the family's component, an empty type of the
// plugin's or the game's names the family, and [Kinds.DefineTag] hands out the bits by name — saved by name, so a build defining them in another order still loads. A
// kind gives its entities tags with [comp.Tagged]; a query over the family's Tags narrows to
// the entities carrying any of them, and setting or clearing a bit is a value write, seen the
// same tick. Rules name tags with the filters Self and Between (plugins/world/rule); the marker
// components of old are gone.
//
// # Bodies
//
// [Bodies] spawns entities of a kind reserved with [Kinds.Reserve]: a Base and the caller's own
// columns, no Appearance and no size bounds, counted against MaxCount. It is how a plugin
// materializes geometry of its own from inside a system, at any tick. Terrain on a grid needs none:
// it is the board's solid ground and cover (board.Plugin.WithCollision, board.Plugin.Cover), which
// collision and sight read cell by cell.
//
// # Attach, Detach and Declare
//
// [Plugin.Attach] and [Plugin.Detach] are the mid-game counterparts of a kind's Const, for game
// logic that has a plugin.Tick and no component id; [Plugin.Despawn] removes an entity at the end
// of the tick, and an entity gives itself the command [Despawn] to go (Order in a plan or a
// rule).
// [Plugin.Declare] tells saves about a type only ever attached; call it in Init.
//
// # Commands the entities give themselves
//
// The world keeps a stage's one carrier of commands, [Plugin.Commands]: the players plugin gives
// it the players' commands, and a plan or a rule (Order) the commands its entities give
// themselves, each taken to the queue of the plugin that handles it. [Plugin.Carry] takes a
// plugin.CommandHandler's queues; the engine carries every one a stage uses, and a host's
// plugin.Tick hands the carrier to its rules. Nothing is dropped: a command waits for its
// handler's pass — given after it, for the next frame's.
//
// # Clock, Systems and Effects
//
// The world keeps the tactical clock (plugins/world/clock, [Plugin.Clock]): game time is the sum
// of the simulation's steps, Space is the tactical pause and ] and [ the tempo — the players carry
// its commands ([Plugin.Queues], [Plugin.DefaultBindings]) — the entities' trees
// (plugins/world/rule), run after the decision systems, and the effects
// (plugins/world/rule/effect, [Plugin.Effects]), which last in game time and fire the rules
// of the clock's moments (clock.Moment) every step.
//
// [Plugin.RunPlan] runs the tick: at once, the clock's commands and the cameras' views; then, as
// the simulation the clock replays as many times as the tempo says and not at all in the pause,
// every registered [Behavior] (a decision system, see
// [Plugin.Hook]), then the steering.System carries out steering.Steering requests
// (heading, and base speed for an entity with a motion profile), [VelocitySystem] runs the
// rules of a [Moving] over every entity so they may scale that speed, then [MoveSystem] moves every box under the
// edge rules and hands the space every Base as an aabbworld.Item — Space.Rebuild. The space keeps
// no state of its own between ticks: Populate and PostLoad rebuild it too, so it is whole before
// the first tick, and a despawned entity is gone from it on the next. Anything reading the space
// in its own pass sees the boxes as they stand after the last rebuild — and, after a collision
// tick, as the engine pushed them. The leavers and the effects close every step.
//
// # Appearance and Renderer
//
// [Appearance] is the sprite an entity is drawn from; [Plugin.WithRenderer] builds the entity
// [Renderer] over an atlas, and the rules of a [Drawing] registered on the plugin settle
// each entity's layers in order — [Draw].Overlay, Draw.As, Draw.With and Draw.Facing are the
// ready-made ones. The Renderer, a render.Source for a scene's render.Composer, hands it the
// entities in the camera's view.View and nothing else, each laid on the screen by the world's [Look]
// with its box and its [Z] — where it stands and how tall — from above its box, unless a view
// plugin ([Plugin.SetLook], plugins/topography) stands it up as a billboard as tall as its Z says.
// The Renderer asks the Look for every entity in white light, swaying as its Appearance says: the
// world knows no sun and no wind; the Look — a view plugin's, or the atmosphere's over a flat board
// (atmosphere.Plugin.WithBoard) — lights the entity, leans it and lays its shadow. Picking and
// outlines ask the same Look. A [DirectLook] draws the sprites itself on the GPU: the Renderer, a
// render.Direct at render.Objects, readies it every frame, hands it the sprites and has it draw
// them — the world's own flat look as instances (render.Sprites), the topography's as billboards
// against the ground's depth. A view plugin also makes the world's cameras ([Plugin.SetCameras],
// [Cameras]).
//
// # Views
//
// A view.View is what one pair of eyes sees: a rectangle of the world and the entities the Space
// finds in it, as a view.EntitySet — a set of the world's entities by index. The world keeps any
// number of Views ([Plugin.NewView] over a source of bounds, [Plugin.DropView]) and the view.System
// refreshes each of them once a tick, right after movement has rebuilt the Space; a View whose
// bounds cover the whole world is not queried and simply sees everything, as does the zero View a
// Stage has before its first tick. [Plugin.View] is the camera's, made by the plugin itself, and
// [Plugin.ViewFor] the View of any camera of the world's; the entity renderer reads them.
//
// # Telemetry
//
// [Telemetry] counts the population; Resources publishes it for a renderer to show.
package world
