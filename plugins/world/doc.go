// Package world is the foundation for a Stage with drawable entities: a Position and Velocity
// each and a shared spatial index other plugins query; the motion plugin moves them. What an
// entity carries is set by its kind (entity/kind).
//
// The world runs gram's core: entity and entity/kind, clock, and ceremony through its goke
// binding (the effects, the plans, the commands about effects, the roles' tags).
// Its sub-package is view (what one camera sees); its machinery is in plugins/world/internal.
//
// # Plugin
//
// [Plugin] is the world a Stage makes from a [Config]: the [SpaceCfg] sizes it and sets the edge
// rule per axis (closed, wrapping or open), the [EntitiesCfg] bounds how many entities it holds and
// their sizes; what becomes of one past an open edge is motion's. Config.Heights gives the world
// heights: entities
// carry a [Z], and the board, the topography and the atmosphere stand in no other; a flat world
// refuses a Z. The world knows its entities and nothing else: the ground is the board's, the sky
// the atmosphere's.
//
// # Read and Write
//
// The world's state is read through [Plugin.Read] and changed through [Plugin.Write]: Read(w) is
// worker w's of the game's pool, 0 the game's own goroutine, each with queries of its own, so the
// workers read the world at once; Write is the game's goroutine's. Read(w).Space() queries the
// spatial index, Read(w).Clock() reads the clock and Read(w).Effects() which effects the entities
// are under; Write() spawns, writes the space, drives the clock and casts the effects.
//
// # Base, Position and Velocity
//
// [Base] is the one component every entity carries: its [Position], its [Velocity] (a unit
// heading and a speed in world units a second), the kind it was spawned from and the capability
// bits the space indexes it under. [Z], [Layers], [Eye] and [Lie] are the types of package
// entity under the world's own names. No entity moves further in a step than [StepReach] of its
// own shorter side.
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
// puts entities on the roster with [Write.Seed]; the engine calls [Plugin.Populate] only for a
// Stage entered afresh. [Plugin.Roster] is what the plugins ask of the kinds of each template
// ([EntityTemplate] and those a plugin brings: motion.UnitTemplate, the board's), the world and
// every plugin made writing theirs in their KindTemplates as it is first read.
//
// # Roles and Commands
//
// What a game defines is its Stage's, by name: [Roles] ([Plugin.Roles]) and [Commands]
// ([Plugin.Commands]), beside the effects ([Write.Effects]) and the kinds. Define registers and
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
// simulation ([Write.Spawn] gives it as the game's own); one refused is logged, never a panic.
// An entity gives itself [Despawn] to go. The world carries out the commands about effects
// (ceremony.Put, Take, Toggle) for the entities named or grouped by their ceremony.Label and for the
// world itself, netting a step's commands of one effect on one entity.
//
// # Clock and the tick
//
// The world keeps the tactical clock ([Write.Clock]) and the commands [Pause] (P, Shift+P),
// [Faster] and [Slower] on it. [Plugin.RunPlan] runs the tick: the clock's commands, the sectors and the views
// at once; in every step of the simulation the plans, the spawns and the despawns, the commands
// about effects, the clock's moments and the effects, the space told of every entity after a
// spawn or a despawn. [Write.Carrier] is the Stage's one carrier of commands.
//
// # Drawing and Look
//
// How entities are drawn is declared on the world's [Atlas] by the figures' Looks, gathered once
// the Stage is set up, before its scenes: a kind's look, its look Under an effect, Turning,
// Facing, Animated, Lying, Upright, Edges, and Styled: how another plugin draws about the kind
// (an outline, a cone, a route), a render.Style of that plugin's, read off [Read.Styles].
// Closing the atlas hands it to the world ([Write.SetAtlas]), which gives it the entity renderer
// and applies what was declared; a game declaring looks by hand closes its own
// ([Plugin.NewAtlas]) before. Each
// entity is laid on the screen by the world's [Look] ([Read.Look], [Write.SetLook]), flat
// from above unless a view plugin sets its own; a [DirectLook] draws the sprites itself on the
// GPU, a [LyingLook] lays some flat, an [UprightLook] stands some true, a [FoggyLook] fades one
// into the fog before a camera's sight. Each is lit as the [Lighting] says, the light plugin's
// ([Write.SetLighting]), in white light without one. In a sector a camera sees coarsely an entity
// is drawn plainly: no edges, flat where it would stand. [Plugin.WithFog] is the fog's colour in a
// world without an atmosphere.
//
// [Read.Picture] is everything the Stage's plugins draw (plugin.Installer.Draws), one picture
// for every scene; a scene shows it through a camera, ui.Image(render.NewFeed(camera, it)).
//
// # Sectors
//
// Config.Sectors cuts the world into sectors in bands of pace round the anchors, the points the
// players observe ([Write.Anchor]; the cameras plugin gives its cameras'), and marks each entity
// with its sector's band (package sector); what lies beyond the last band is out of the space.
// A system adds the pace of its policy ([Plugin.Pace]) to its query and walks it with
// goke.Query.Paced; the world's plans and effects go Paced. [Read.Sectors] lays them on the
// telemetry panel. A camera's View in such a world is of the sectors it sees, each at a level
// (view.NewSeen); [Read.Seen] lays the first camera's on the panel. [SwitchSectors] shows the
// sectors over the ground by their bands or by how the camera sees them ([Write.ShowSectors]);
// [SwitchBands] switches the bands off, every sector in the first, to weigh what they spare
// ([Write.SetBanded]); both are the settings screen's alone.
//
// # Views
//
// [Write.NewView] makes the view.View of a camera, as the cameras plugin makes the camera, and
// [Read.ViewFor] reads it; refreshed once a tick after movement, the renderer draws only what
// the camera's View contains.
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
// Knobs, for an effect's Alter: render.Appearance, the look's.
// Commands: [Spawn] (a player's or an entity's own); [Despawn] (an entity's own); [Pause],
// [Faster], [Slower], [Tempo], [SwitchSectors], [SwitchBands] (a player's). Settings
// ([Settings], saved with the game, in [Resources]): the sectors drawn, their bands. The clock's
// moment and the commands about effects (ceremony.Put, Take, Toggle, Trigger) are carried here
// too.
package world
