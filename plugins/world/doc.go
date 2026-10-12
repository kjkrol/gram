// Package world is the foundation for a Stage with moving, drawable entities: a Position and
// Velocity each, a shared spatial index other plugins query, and motion integrated each step,
// never further than Position.MaxStep. What an entity carries is set by its kind (entity/kind).
//
// The world runs gram's core: entity, entity/kind, entity/tag, clock and rule with rule/effect.
// Its sub-packages are steering (heading and speed reached gradually) and view (what one camera
// sees); its machinery is in plugins/world/internal.
//
// # Plugin
//
// [Plugin] is the world a Stage makes from a [Config]: the [SpaceCfg] sizes it and sets the edge
// rule per axis (closed, wrapping or open), the [EntitiesCfg] bounds how many entities it holds
// and their sizes. An entity wholly past an open edge carries [Outside]; the rules of a [Leaving]
// hear of it every tick, and with none it is despawned. Config.Heights gives the world heights:
// entities carry a [Z]; a flat world refuses one. The world knows its entities and nothing else:
// the ground is the board's, the sky the atmosphere's.
//
// # Base, Position and Velocity
//
// [Base] is the one component every entity carries: its [Position], its [Velocity] (a unit
// heading and a speed in world units a second), the kind it was spawned from and the capability
// bits the space indexes it under. [Z], [Layers], [Eye], [Drift], [Lie] and [Block] are the
// types of package entity under the world's own names. No entity moves further in a step than
// [StepReach] of its own shorter side.
//
// # Scale
//
// [Scale] says how many metres a world unit spans; with one the ground sinks under an eye's level
// far off ([Scale.Drop], [Scale.Horizon]). A game gives metres through [Scale.Units], and the
// sizes of its figures, drawn bigger than life, through [Scale.Figure].
//
// # Kinds, Seed and Populate
//
// [Plugin.Kinds] is the register kind.Define registers with; [Kinds.DefineTag] hands out the
// bits of a tag family by name and [Kinds.NewSprite] atlas slots no kind owns. A Stage's Spawn
// puts entities on the roster with [Plugin.Seed]; the engine calls [Plugin.Populate] only for a
// Stage entered afresh. [Plugin.Roster] is what the plugins ask of a unit's kind.
//
// # Roles and Commands
//
// What a game defines is its Stage's, by name: [Roles] ([Plugin.Roles]) and [Commands]
// ([Plugin.Commands]), beside the effects ([Plugin.Effects]) and the kinds. Define registers and
// hands nothing back; Named is the thing; a name defined twice or asked for unknown panics. A
// role is a tag of the family rule.Roles, saved by name. The world plays roles itself
// ([Plugin.Plays]): the rules of a clock.Moment they obey fire every step.
//
// # A plugin's own entity
//
// [Self] is the one entity a plugin has in the world, called by the plugin's name, carrying its
// knobs, the roles it plays and the effects it is under. A plugin makes it with [NewSelf] as it
// is made and embeds it, so it is whom a command may be for and learns from [Self.Changed] that
// its knobs were turned. The world's own is the clock's entity, which entity.World names.
//
// # Spawn and Despawn
//
// The command [Spawn] adds an entity of a kind to the running world at the next step of the
// simulation ([Plugin.Spawn] gives it as the game's own); one refused is logged, never a panic.
// An entity gives itself [Despawn] to go. The world carries out the commands about effects
// (rule.Cast, Lift, Toggle) for the entities named or grouped by their entity.Label and for the
// world itself, netting a step's commands of one effect on one entity.
//
// # Clock and the tick
//
// The world keeps the tactical clock ([Plugin.Clock]) and the commands [Pause], [Faster] and
// [Slower] on it. [Plugin.RunPlan] runs the tick: the clock's commands and the views at once;
// in every step of the simulation the plans, the spawns, the steering, the rules of a [Moving]
// over every entity, movement, the leavers, the commands about effects, the clock's moments and
// the effects. [Plugin.Carrier] is the Stage's one carrier of commands.
//
// # Drawing and Look
//
// How entities are drawn is declared on the world's [Atlas] ([Plugin.NewAtlas]) in a scene: a
// kind's look, its look Under an effect, Turning, Facing, Animated, Lying, Upright, Edges.
// [Plugin.WithRenderer] builds the entity renderer over it and applies what was declared. Each
// entity is laid on the screen by the world's [Look] ([Plugin.Look], [Plugin.SetLook]), flat
// from above unless a view plugin sets its own; a [DirectLook] draws the sprites itself on the
// GPU, a [LyingLook] lays some flat, an [UprightLook] stands some true.
//
// # Views
//
// [Plugin.ViewFor] is the view.View of a camera, refreshed once a tick after movement; the
// renderer draws only what the camera's View contains.
//
// # Telemetry
//
// [Telemetry] counts the population; [Resources] publishes it with the Config.
package world
