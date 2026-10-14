// Package world is the foundation for a Stage with moving, drawable entities: a Position and
// Velocity each, a shared spatial index other plugins query, and motion integrated each step,
// never further than Position.MaxStep. What an entity carries is set by its kind (entity/kind).
//
// The world runs gram's core: entity and entity/kind, clock, and ceremony through its goke
// binding (the effects, the plans, the commands about effects, the roles' tags).
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
// role is a tag of the family cond.Roles, saved by name. The world plays roles itself
// ([Plugin.Plays]): the rules of a clock.Moment they obey fire every step.
//
// # A plugin's own entity
//
// [Self] is the one entity a plugin has in the world, called by the plugin's name, carrying its
// knobs, the roles it plays and the effects it is under. A plugin makes it with [NewSelf] as it
// is made and embeds it, so it is whom a command may be for and learns from [Self.Changed] that
// its knobs were turned. The world's own is the clock's entity, which ceremony.World names.
//
// # Spawn and Despawn
//
// The command [Spawn] adds an entity of a kind to the running world at the next step of the
// simulation ([Plugin.Spawn] gives it as the game's own); one refused is logged, never a panic.
// An entity gives itself [Despawn] to go. The world carries out the commands about effects
// (ceremony.Put, Take, Toggle) for the entities named or grouped by their ceremony.Label and for the
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
// How entities are drawn is declared on the world's [Atlas] by the figures' Looks, gathered as
// [Plugin.Renderer] is first asked for: a kind's look, its look Under an effect, Turning,
// Facing, Animated, Lying, Upright, Edges, and Styled: how another plugin draws about the kind
// (an outline, a cone, a route), a render.Style of that plugin's, read off [Plugin.Styles].
// Closing the atlas hands it to the world ([Plugin.SetAtlas]), which builds the entity renderer
// over it and applies what was declared; a game declaring looks by hand starts its own
// ([Plugin.NewAtlas]). Each
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
//
// # Figures
//
// A [Figure] is a kind of entity in one piece: Define says what it is (kind.Define with the
// roster, the roles it plays, its plan), Look how it looks on the world's [Atlas]; one with
// states of its own is [Stateful] (Effects), one with roles and rules of its own [Ruled]
// (Rules). [Plugin.Figures] notes them in the Stage's Figures link, and the Stage runs each
// part in its section after the game's own functions ([Plugin.FiguresIn]); [Atlas.Looks]
// declares every look in a scene. What several kinds share — a role many play, a rule of the
// whole — stays the game's, in its sections. A figure standing on a board is the board's
// (board.Plugin.Figures), defined over it and looked here.
//
// # Vocabulary
//
// Moments, for a role's rules: [Moving] (every entity before it moves; its words [Still],
// [Braking]),
// [Leaving] (every tick an entity is past an open edge). Knobs, for an effect's Alter:
// render.Appearance, the look's. Commands: [Spawn] (a player's or an entity's own); [Despawn] (an
// entity's own); [Pause], [Faster], [Slower] (a player's). The clock's moment and the
// commands about effects (ceremony.Put, Take, Toggle, Trigger) are carried here too.
package world
