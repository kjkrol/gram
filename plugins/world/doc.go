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
// Read(w).GroundAt is the ground's height under a point as the plugin laying the ground in relief
// gives it ([Write.SetGround], the topography's), 0 in a world without one.
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
// about effects, the clock's moments, the aggregates' and the effects, the space told of every
// entity after a spawn or a despawn. [Write.Carrier] is the Stage's one carrier of commands.
//
// # Drawing and Look
//
// How entities are drawn is declared on the world's [Atlas] by the figures' Looks, gathered once
// the Stage is set up, before its scenes: a kind's look, its look Under an effect, Turning,
// Facing, Animated, Lying, Upright, Canopy, Cutout, Edges, and Styled: how another plugin draws
// about the kind
// (an outline, a cone, a route), a render.Style of that plugin's, read off [Read.Styles].
// Closing the atlas hands it to the world ([Write.SetAtlas]), which gives it the entity renderer
// and applies what was declared; a game declaring looks by hand closes its own
// ([Plugin.NewAtlas]) before. Each
// entity is laid on the screen by the world's [Look] ([Read.Look], [Write.SetLook]), flat
// from above unless a view plugin sets its own; a [DirectLook] draws the sprites itself on the
// GPU, a [LyingLook] lays some flat, an [UprightLook] stands some true, a [FoggyLook] fades one
// into the fog before a camera's sight, a [CanopyLook] shows a unit through the canopy over it
// and a [CutoutLook] draws what is whole in the cutout pass. A kind's look declared [Slot.Canopy]
// (a tree's crown) shows a unit standing under its entity through it at three quarters, in a view
// from above alone, where the Canopies setting says ([ShowCanopies], [Read.Canopies], under the
// settings' section Game; Config.Canopies its start); one declared [Slot.Cutout] is drawn whole
// in render's cutout pass, cut at half its alpha and writing the depth, so a unit under a crown
// is hidden in a view in relief and the crowns need no order; a canopy seen through is laid among
// what is seen through instead. Each is marked by the [Lighting], the light plugin's
// ([Write.SetLighting]), and lit as the frame says (render.Frame.Lit) by the lights the picture
// is given ([Write.SetLights], [Read.Lights]), in white light without them. An entity is drawn in
// its kind's look at the level its camera sees it at: its own at L2 (lod.Standard) and the close
// levels, plainly at L3 (no edges, flat where it would stand), a blob of its own look at L4
// (render.Blob), its blob's colour in the ground at L5, or as [Slot.At] and [Slot.Nothing] declare
// from a level on, a close one to Standard at most. At L4 the renderer draws it itself as a point,
// its box as large as it shows and a pixel at least, as opaque as it covers, with no Look, lamp or
// shadow, and a camera publishes the ladder's Keep share of a level's entities by their identity,
// the same each frame, sparing the sector.Keen, a kind [Slot.Always] and one drawn in a look
// declared there. As Config.Far and the settings say ([ShowFar], [Read.FarLook]), a kind's loose
// units in a sector drawn from L4 on are a point each or, where the kind declares a look there,
// one [Banner] at their centroid ([Read.Banners]), never thinned, drawn as a badge over their
// top ([BannerStyle]: a disc or a diamond in the kind's far colour, a kind's own by
// [Slot.Styled], and a style alone has a kind bannered there); with the counts drawn
// ([ShowCounts], [Read.Counts]) the number of a banner's units from two on stands inside its
// badge, and of a kind's points two or more in a sector over their centroid, in the picture as
// deep as they stand. What stands for good where a sheet is drawn from L4 is left to the sheet
// from above, and drawn as a point of its look there or its blob through a camera with an eye,
// which sees no sheet (a photograph from above). A camera riding inside
// an entity (camera.Inside) draws none of its figure, its shadow still. [Plugin.WithFog] is the
// fog's colour in a world without an atmosphere. The entities are published once a frame, after its
// ticks: the looks' rules settle every one some camera sees, the Lighting marks it, each camera's
// view is copied, and the frame is drawn of that alone. In a world cut into sectors whose ground a
// renderer draws ([Plugin.Sheets]) what stands ([Fixed]) in a sector some camera sees at L4 or L5
// is photographed on the GPU from its L3 looks, with the parts of the fields folded there, into
// [Read.Sheet], an albedo and a relief over the whole world ([SheetImage], [SheetReliefImage], at
// L5 in half), a few sectors a frame as their content changes; once a sector's sheet is painted its
// standing entities are no record, the renderer drawing the sheet as the ground's relief, and a
// unit there is its blob. What stands is signed into its sector as its chunk comes, goes or has
// its table moved, or as [Write.Restill] says of a change in place, never walked frame by frame. A standing entity cut by a sector's edge, or beside a sector seen finer
// than L4, is drawn alive at the finer level, never thinned, and photographed nowhere.
//
// Config.Sheets ([SheetsCfg]) keeps the sheets between runs: they are read from the tiles in its
// Dir as they are made, so a sector photographed in an earlier run is photographed again only once
// what stands in it changes, and with Keep the tiles a sector was photographed in are written back
// there as they settle, a tile a second; without Keep the folder is only read. Tiles of another
// Version, texel or world are not read. What a field's folded parts look like is not in what a
// sector is signed by, so a game changing how it plants its fields changes Version.
//
// [Read.Picture] is everything the Stage's plugins draw (plugin.Installer.Draws), one picture
// for every scene; a scene shows it through a camera, ui.Image(render.NewFeed(camera, it)).
//
// # Sectors
//
// Config.Sectors cuts the world into sectors, each ticked at the fullest level Config.LOD (a
// lod.Ladder) says one of the observers sees it at ([Write.Observe]; the cameras plugin gives its
// cameras; a sector off a camera's picture at lod.Strategic or coarser, but for the sector its
// viewpoint lies in and the eight round it, whole whichever way the camera looks) or a
// sector.Keen entity in it or beside it keeps it at, and marks each entity with its
// sector's level and how many levels coarser that is than the finest any observer sees (package
// sector); what lies at L6 (lod.Omitted) is out of the space. An entity gives itself [Keen] to keep
// its sector so, [Calm] to keep it so no more. With no observer every sector is ticked at the
// ladder's Unobserved level, L4 unless set. An entity travelling the abstract graph far from every
// camera carries sector.Abstract, saved with the game. A system adds the pace of its lod.Paces
// ([Plugin.Pace]) to its query and walks it with goke.Query.Paced, in full on the finest level
// seen, so a world with no observer goes in full everywhere; the world's plans and effects go
// lod.Slowing(2, 4, 8). [Plugin.Within] keeps a query to the sectors of one scale: a system of the
// close scale's detail goes Within(lod.Close) at lod.Exact, nowhere while nobody looks so close.
// [Read.Sectors] lays them on the telemetry panel. A camera's View in such a world is of the
// sectors it sees, each at the level it sees it at (view.NewSeen), within [Read.Sight];
// [Read.Seen] lays the first camera's on the panel. [SwitchSectors] shows the sectors over the
// ground by the level they are ticked at or by how the camera sees them ([Write.ShowSectors]);
// [SwitchBands] switches the levels off, every sector ticked at Standard, to weigh what they spare
// ([Write.SetBanded]); both are the settings screen's alone.
//
// # Aggregates
//
// An [Aggregate] is a kind of entity standing for parts, a forest's trees or an army's soldiers:
// they are the truth while its sector is ticked finer than L4 (lod.Strategic), it alone from L4 on.
// It is told [Unfold] once as its sector comes down finer than L4 (the world unfolding at most
// lod.Ladder.Unfolds stretches a step, those nearest an observer first, the others waiting folded
// as they are), when its rules spawn the parts
// ([SpawnPart]), each carrying [PartOf] it, and [Fold] once its sector has lain at L4 or coarser
// for the ladder's Fold seconds, with the [Parts] it has, each part told a [Folding] to sum itself
// into it and go; the wait is the stretch's, an entity the world keeps while it is unfolded, saved
// with the game. From its first Unfold till the step after its last Fold, its parts gone, it
// carries [Unfolded], saved too: its parts are drawn for it and what moves it stands it, at any
// level, and a point aggregate stands each step at the middle of its parts, its own size kept,
// so its sector, its banner and whatever follows it go with them. Folded, it counts in its banner
// for its Strength where its kind declares one, which the world sets to the parts folded in at
// each Fold; without, for one.
// Before a point aggregate's Fold is told the world moves it to the centroid of its parts'
// centres, so what sums them finds it where they stood; with no part left it stays. What the
// parts are and what has changed of them is the game's: lod.Seed is the number a place of the
// aggregate draws from, the same at every unfold, its losses its effects.
// Two aggregates travelling the abstract graph (sector.Abstract) are told [Met] once as their
// boxes come to overlap, again only after they have parted; one standing unfolded is on no such
// graph. A unit's route is a plain value of its order, one an aggregate and the part leading it
// may hand each other as they fold and unfold (navigation.Inherit).
//
// A [Field] aggregate, a forest, stands over a box round its centre wider than an entity may be
// and goes by sector: each sector of it unfolds and folds apart, told with the Sector and the
// Box of the field within it, its parts there those whose centre lies in it, and it is found
// asleep too, its own sector at L6 stopping none of the others. A point aggregate's moments tell
// the sector of its centre and its own box. A SpawnPart given under an Unfold is laid At off the
// middle of that Box, so the plots a rule plants, picked by words over lod.Seed and the
// aggregate's effects, lie in the sector. A field is drawn by sector, asleep too, wherever a
// camera sees its part there: its look at the level of that sector stretched over the part,
// from L4 in the sector's sheet where it declares none; a sector it is unfolded in has its
// parts drawn alone, till they have gone the step after its Fold. Drawn so, it lies on the ground,
// under the figures standing over it.
//
// A field's stretch a sheet draws, of a kind declaring no look of its own at L4, with none of its
// parts kept (never unfolded, or not since the game was loaded), is previewed before its sector
// is photographed: told its Unfold as though it unfolded, it has the parts its rules give there
// photographed where they would stand, on the ground the world is given ([Write.SetGround]), and
// kept as a Fold keeps them; none is spawned, no stretch is made, nothing is saved. Its rules
// cannot tell a preview from an unfolding, so whatever else they do under Unfold is done then too.
//
// # Views
//
// [Write.NewView] makes the view.View of a camera, as the cameras plugin makes the camera,
// refreshed once a tick after movement. What publishes for a frame culls by every camera's
// ([Read.Views], [Read.InView]); what draws reads a camera's as the frame was published
// ([Read.ViewFor]), one copy a frame for all of it, and draws only what it contains.
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
// Moments, for a role's rules: [Unfold] (a stretch of an aggregate come down finer than L4, its
// subject the stretch), [Fold] (one folding; its word [Parts]), [Folding] (each part of it, its
// subject the aggregate), [Met] (a pair of aggregates on the abstract graph, host.Met). Marks:
// [Aggregate] and [Field] (on a kind), [PartOf] (on a part), [Unfolded] (on an aggregate, for the
// plugins: a rule reads no component), [Fixed] (on a kind standing for good, what the sheets of
// the strategic scale are photographed of), [Whole] (on the kind of the handle of a thing standing
// in pieces, its joint box and no sprite, despawning its pieces as it goes), [PieceOf] (on a
// piece, its handle's), [Unseen] (on a kind none of the world's looks draw, its plugin drawing it
// by a source of its own). A thing standing for good lives in a tier of sectors (sector.Grid.Fit):
// one no longer than [SectorsCfg] Small by its centre, a longer one in the lowest holding it whole.
// Knobs, for an effect's Alter:
// render.Appearance, the look's.
// Commands: [Spawn] (a player's or an entity's own); [SpawnPart] (an aggregate's own, aimed at
// the stretch of an Unfold); [SpawnWhole] (a handle and its pieces in one step, as [Spawn]);
// [Despawn], [Keen], [Calm] (an entity's own); [Pause],
// [Faster], [Slower], [Tempo], [SwitchSectors], [SwitchBands], [ShowFar], [ShowCounts] (a
// player's). Settings ([Settings], saved with the game, in [Resources]): the sectors drawn, their
// levels on, how the loose units are drawn far off and whether they are counted. The
// clock's moment and the commands about effects (ceremony.Put, Take, Toggle, Trigger) are
// carried here too.
package world
