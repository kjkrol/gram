# Changelog

## Unreleased

Saves written by v0.2.0 do not load: `Base` and the marker components changed shape.

**Movement**
- `world.Steering` holds a motion profile — `MaxSpeed`, `Accel`, `Brake`, `V0`, `TurnRate` — and
  `SteeringSystem` writes the base speed every tick; navigation steers through it: a lookahead
  point, waypoints passed by projection, braking to rest on the goal, a queue of goals
  (Shift + right click), routes previewed to every queued goal.
- A unit under orders that struck someone stops, plans again and holds that route for a while
  (`MoveOrder.Bumped`, `Cooldown`), so units head-on on a road step aside instead of pushing each
  other for ever. `board.Plugin.Collision()`.
- `Occupancy` is kept per domain (`CanEnter`/`Enter` take a `Domain`): `SingleOccupancy` lets one
  entity per domain into a cell, so a flyer and a walker share one; `MultipleOccupancy` stays a
  stack of tokens. Navigation seeds it from `Cell` + `Mover` at Setup, fresh or loaded — the
  demos' spawn effects are gone. island-demo uses `SingleOccupancy`.
- `world.Layers`, the planes an entity is on (one bit each; none, or no component: every plane),
  read by collision and by sight. Two colliders touch only where their layers meet; terrain
  bodies are on the bits of the domains their kind keeps out, so a wall admitting Air lets a flyer
  over, and the demos' hawk carries `Physics` on the Air layer. `Collider.Layers` is gone.
- `vision.Sight.Blockers` replaces `Clear`: the layers that cut or dim an observer at all (zero:
  every entity). An entity on none of them is looked over as if absent and still seen, so a hawk
  with `Blockers` of Air looks over walls, forests and walkers, and a walker with Land looks under
  the hawk. `CellKind.Veils` says whom a veil dims (zero: everyone; the demos' forests veil Land).
  Whatever a ray reaches within its budget is now seen, a forest looked into included (aabbworld
  v1.7.0).
- Routes and legs lose their footing when the terrain changes under them; a unit stuck where its
  domain may not keeps its order.
- The camera pans in screen pixels at any zoom.
- `game.Props.Resizable`: the window can be resized and maximized, the screen is the window, and
  the world camera follows it (`Camera.SetViewport`: the middle kept, a world smaller than the window
  scaled up to cover it). Shift+F toggles fullscreen in every game (`Runtime.ToggleFullscreen`).
  The world demos are resizable. `render.CachedRenderer` follows the screen's size, drawing anew
  when it changes; collision-demo and vision-demo get the players plugin, so their camera pans,
  zooms and scrolls at the edges. `Camera.CenterOn(x, y, z)` puts a world point at a
  height in the middle of the screen.
- A right click on the cell a selected unit stands on turns it towards the point clicked, and a
  right click with S held (`navigation.LookAt`) has every selected unit finish its step, stop and
  turn there: `MoveTo.At`, `MoveOrder.Face`, turned by the steering in place. A trigger may ask for
  a key held besides its modifiers (`control.Mods{}.Holding(ebiten.KeyS)`); players track the keys
  their bindings hold. The demos' sight follows the heading, turning in place included.
- F follows the one selected unit with the camera (`selection.Follow`, tag `Followed`,
  `FollowSystem`); F again, or moving the camera by hand, stops it; zooming does not.
- A unit that looks further ahead than half a cell (the hawk) keeps its leg until its centre enters
  the waypoint it passed, instead of re-planning its route every step.

**Isometric view**
- `camera.Projection`: `TopDown` (the default, unchanged) and `Isometric` (Transport Tycoon's 2:1
  diamonds, heights lifting a point); `Camera.Project/Unproject/Depth` and `Projection()`,
  `camera.Config.Projection`. An isometric camera keeps a screen window over the projected world
  and refuses a wrapping one. `Camera.Viewport` is the screen in pixels; players read it for edge
  scrolling instead of deriving it from the world bounds.
- `render.Sorted` draws several `render.Submitter`s back to front by depth as one picture; the
  board's and the world's renderers submit their quads (cells at their altitude, entities at their
  `Z.Altitude`) besides drawing them as before.
- Through an isometric camera the board's renderer draws relief: raised ground shows the faces
  towards the viewer down to its lower neighbours, tall kinds (a wall, a forest) stand as blocks.
- Through an isometric camera entities are billboards standing on their projected centre; the
  vision fan is draped over the ground (`vision.Renderer.WithGround`), path sprites lie on the
  cells' diamonds and the selection highlight rounds the diamond under the unit
  (`selection.HighlightStyle.Draw` takes the altitude). Picking follows the relief: a command
  `Context` carries the world's `Ground` and `World`/`WorldBox` land on the ground under the cursor
  (a move order on a hill goes to the hill), and a `Select` carries the `Screen` rectangle it was
  drawn in, so the selection hits entities where they are drawn — a unit on a hill, a hawk in the
  air (`NewSelectionSystem` takes the camera). `render.QuadBatch.AppendCorners`,
  `render.ProjectCorners`, `render.Billboard`.
- `island-isometric-demo`: the island in a Quasi3D world through an isometric camera — hills 20 and
  mountains 40 up with sloping sides, forests 8 tall, units upright, a hawk 40 up; `island-demo`
  gains sight cones and the hawk on the Air plane.
- Path sprites through an isometric camera lie on the tile as it is drawn, corner heights included.
- On a square grid the ground slopes between cells: `Board.Corners` (each corner the mean of the
  cells meeting there), `GroundAt` interpolated, tiles drawn tilted, faces only where a top stands
  above its neighbour's, lit from the upper left: a slope rising towards the light is brighter, one
  falling away darker. `Grid.Coords` inverts `CellIndex`.

**Heights**
- `world.Config{Quasi3D: true}` gives a world heights; the default is flat and no plugin guesses
  the mode from the data. Entities carry `world.Z{Altitude, Height}`; a flat world refuses a Z.
- Board: `CellKind.Altitude` (ground level) and `Height` (what stands on the cell), `Mover.Lift`,
  `Shape{Size, Height}` for `NewUnits`, `Units.Define(name, board.Mover{Domain, Lift}, …)`. The
  `Board` keeps a raster of altitudes (`Grid.Ordinal`, `GroundAt`) rebuilt when the terrain
  changes and is the world's `Ground`; the board writes every `Z.Altitude` each tick from the
  ground under the entity plus its `Lift`; terrain bodies carry their kind's `Z`.
- Vision: `Sight.Eye`; in a Quasi3D world the cone has heights (aabbworld v1.7.0: eye, entity
  bands, ground sampled every `Plugin.WithGroundStep`), so a hawk 40 up looks over a wall 10 tall,
  a forest and a hill a walker's cone stops at. `Blockers` are refused in a Quasi3D world, `Eye`
  in a flat one. Collision stays on `Layers` in both.
- In a Quasi3D world a `SightOutline` reaches the full `Sight.Radius` and keeps the ground out of
  sight as `Shadows` (up to `MaxShadowsPerSample` `Band`s per angle, from aabbworld v1.8.0's
  `View.Shadows`); the renderer fills them over the ground as holes in the view (`ConeShader`,
  `DefaultShadow`). A flat world keeps its reach cut at walls.
- The vision demos run in a Quasi3D world with a hill; aabbworld is taken from v1.8.0.

**Scenes and viewports**
- A Scene's `Layers()` are `render.Layer`s: a `render.Renderer` draws on the screen, a
  `render.WorldRenderer` (`DrawWorld(screen, cam)`) shows the world and is drawn once per
  `render.Viewport` — a camera and a rectangle of the screen — of a Scene that is a `game.Viewer`.
  No renderer keeps a camera: the board's, the world's, navigation's, vision's, selection's and
  players' take the viewport's at draw time; `render.Sorted`, `Submitter.Submit(sink, cam)`,
  `Overlayer.Overlay(screen, cam)`, `QuadBatch.Reset(cam)`. `plugin.Plugin.Renderer()` returns a
  `render.Layer`. The engine sizes each viewport's camera to its area and initialises a layer once
  however many scenes list it; a scene with world layers and no viewports is refused on entry.
- `players.Plugin.Viewports(screen)`: one viewport per camera the local players look through, in
  columns; `Renderers()` is gone, so a Scene lists the path and selection renderers itself and a
  command handler's renderer is never drawn twice. `world.Plugin.ViewFor(cam)` is the View of
  any camera.
- Split screen: `Player.OwnCamera()` gives a local player a camera of its own
  (`world.Plugin.NewCamera`), saved with the game; `players.Viewports` lays such players out
  (`Columns`, `WithLayout`) and each keeps its `Area`. Keys reach every local player, the mouse only
  the one whose part of the screen it is over, in that part's pixels. `control.KeyHeld` fires once a
  tick while its key is down, issued by players at the end of a tick for the next. `selection.Select.Camera` and `Follow{Camera}` carry the
  issuing player's camera; `NewSelectionSystem` and `NewFollowSystem` take none.
- `split-screen-demo`: red drives its block with WSAD, blue with the arrows, each through a camera
  of its own in its half of the screen; a minimap scene shows the whole arena through a camera of
  its own, drawn by the same board and world renderers.
- A steering profile without `V0` sets off from standing; it stood still for ever.

**Terrain in the ECS**
- Every cell is an entity for good, made at Setup or found again after a load: `board.Plot` (its
  cell and its `Relief`, the heights of its four corners) and `Ground` (its kind). They carry no
  `world.Base`, so they are not in the world's space and do not count against `MaxCount`. The
  `Board` reads and writes them and keeps only which entity is which cell's; the terrain is saved
  with the ECS and `board.Resources` persists nothing. `Plugin.CellEntity(c)` returns the cell's
  entity and a bool, and `DropCellEntity` and the spawning on demand are gone.
- `Board.Version` counts every change to kinds and heights: a write through the board, or an effect
  altering a cell's `Ground` or `Plot`, the tick it lands and the tick it ends. The board reads
  that from `effects.Active.Altered`, set on every pass that rewrote a component through an
  `Alter`, and from `effects.Idle`, and never walks all the cells. `effects` takes `Idle` off entities
  without a `Base` too.
- `CellKind.Altitude` is gone. Heights are the ground's, not a kind's: `Layout.Heights` raises
  the ground when a Stage starts fresh, `Board.SetHeights`, `Relief`, `SetRelief`, and
  `board.MeanOfCells` builds heights from one number per cell. A hex cell is level. The demos'
  hills come from their layouts, and island-isometric's rise smoothly from the fields.
- Shaping as in Transport Tycoon: `board.Raise`, `Lower` and `Level` commands, `Shaping{Step,
  MaxStep}` with the ground round about following, `Plugin.WithShaping`, `Board.Lift` and
  `Flatten`. In a Quasi3D world the board is a `plugin.CommandHandler`: = and - under the cursor, a
  left drag with L held levels. island-isometric-demo shapes its island.
- Reading the terrain from the cell entities costs less than the map and the raster did: A* across
  a 128x128 board costs the same, the ground under a point about 42% less.

**Kinds**
- Package `kind/comp` holds what names one component of a Spec — `comp.Const`, `comp.Load`,
  `comp.Tagged`, `comp.Without` — and `kind` keeps the kinds: `Spec`, `Define`, `Of`, `Roster`.
- `world.Plugin.Roster()`: what the plugins in the game ask of a unit's kind, gathered as the
  plugins are made. `kind.Require[T](role, by, why)` names what the game must supply (world:
  `Position`; board: `Cell`, `Mover`; navigation: `Steering`), `Role.Default` what a plugin brings
  (world: `Velocity{}`; collision: `Collider{}`, `Physics{}`; `comp.Without[T]()` drops one).
  `Roster().Unit.Spec(own...)` builds the Spec and panics naming every requirement left unmet, by
  plugin and reason.
- `board.NewUnits[Row](brd, size, at)` and `Units.Define(name, domain, steering, extra...)`: a
  game's units over a board are defined by where a row says they stand, the domain they move in
  and their steering profile; `Position` and `Cell` come from the one point, `Mover` and `Layers`
  from the one domain, the roster is run and `kind.Define` called. The demos define their units
  through it.

**Board**
- Terrain kinds say whom they admit (`Allows`, a bitset of `Domain`s), whether they are `Solid`,
  how much they `Veil` sight (0 clear, 1 cutting; a forest 0.6), and what they cost per domain
  (`Costing`, `CostFor`); a unit's `Mover` says how it moves. `Passable` is gone.
- `WithCollision`: solid terrain becomes immovable bodies built from `Grid.CellBoxes` — one box
  for a square, capped strips for a hex — merged up to `MaxBodyCells` a side; veiled terrain
  becomes bodies carrying a `vision.Transparency` of 1 - Veil, dimming sight only.
- `Standing`, reported every tick to `board.Each` behaviors: the cell under an entity, its kind,
  its box; `Fell(domain)` says the entity is where it may not be.
- `Grid.CellsUnder`, `CellBounds`, `CellOutline`; hex cells drawn as hexagons; `TerrainMap.Version`.
- Cell entities (`CellEntity`, `Ground`) let an effect change terrain for a while.
- `CellKind.Name` is a `board.Name`, `MaxNameLen` bytes (`board.Named("grass")`, `String()`), so a
  `Ground` component is contiguous in memory; `CellKindDict.Get` and `Layout` keep taking strings.

**Plugins**
- Commands, the other direction of behaviors, as `control`'s vocabulary: a `plugin.CommandHandler`
  keeps a `control.Queue[C]` of each command type it defines, drains it in its own pass (`Issued`
  with the `PlayerID`, `Nobody` for none) and suggests `DefaultBindings` — a `control.Binding` is
  a `Trigger` (`KeyPress`, `ButtonPress`, `Drag`, `Wheel`, `ButtonHeld`, `CursorAtEdge`, exact
  `Mods`), the command `Command[C]` builds from a `Context` (camera, cursor, drag;
  `World`/`WorldBox`) and a label. `selection.Select` and `navigation.MoveTo` are such commands.
- `plugins/players`: who acts in the game, a carrier built over the command handlers
  (`players.NewPlugin(world, selection, nav)`): a `Local` player at the keyboard over the world's
  camera and `View`, `Add` one without (an AI, a client), `Defaults()` to bind, `Issue` for any
  command, `Pan`/`Zoom` with `CameraBindings`, the marquee of a drag drawn by its renderer,
  `Viewports(screen)` for the Scene to show the world through; two
  bindings on one trigger refused at `Bind`, a command nobody defines at Setup. Gone:
  `selection.Resources`/`DefaultEventHandler`, `navigation.Resources`/`MoveCommand`/
  `DefaultCommandEventHandler`, `world.WithCameraControls`.
- Tags are bits of families: `plugin.Tags[F]` is one component per family, `Kinds.DefineTag`
  names the bits (saved by name), `comp.Tagged` gives them to a kind, `Between(a, b, fn)` takes
  them as values; `Selectable` and `Selected`, the vision behaviors' tags and terrain bodies are
  bits. `navigation.NewPlugin` takes the selection plugin.
- `plugins/effects`: temporary changes to entities — `Grant` and `Alter` in a `Spec`, `Lasts`
  or until `Dispel`, `Cast`/`CastFor`/`Dispel`/`Has` by entity id, `Active` saved with the entity.
- `plugins/world`: `Kinds.Reserve` and `Bodies` for kind-less entities; `Kinds.DefineTag`.
- `collision.Detector` is `CollisionSystem` (`NewCollisionSystem`), as every system is named.
- Leaving by an open edge is a component, `world.Outside`, not a set: whoever moves the box out
  marks it, `world.Each` behaviors of a `world.Leaving` registered on the world hear of it every
  tick it is out (despawned with none), and it is unmarked once back inside. `world.Plugin.OnExit`
  and `Tracked` are gone.
- Behaviors are built by the plugin that hosts them: `vision.Between`, `collision.Between`/`Each`/
  `Every`, `board.Each`/`Every`, `effects.Each`/`Every`, `world.Each`/`Every` (payload inferred
  from the function, one of `Moving`, `Leaving`, `Drawing`). `plugin.Between`/`Each`/`Every` and the
  hosts moved to `plugin/host`, a plugin author's package a game never imports; `plugin` keeps
  `Behavior`, `Tick`, `Tag`/`Tags`/`Any`, `Marks` (built by hosts with `MarksOf`).
- Two kinds of thing remain, behaviors and effects: `world.SpeedModifier`, `RegisterSpeedModifier`,
  `AppearanceModifier`, `AppearanceStrategy` and the renderer's `With*` are gone. Speed is a
  `world.Each`/`Every` of a `world.Moving` (board's terrain slows entities carrying `Mover`, and
  only those), drawing a `world.Each`/`Every` of a `world.Drawing` (`world.Draw.Overlay[T]`,
  `Draw.As[T]`, `Draw.With[T]`, `Draw.Facing`; `behavior.HitOverlay` is one), both registered on
  the world plugin. `Every` is `Each` without a state component.
- The end of an entity's last effect is a component, `effects.Idle`, on for one tick: `effects.Each`
  behaviors of an `effects.Idling` registered on the effects plugin hear of it once, and the board
  drops a cell entity it finds so. `effects.Plugin.OnIdle` and `board.Plugin.WithEffects` are gone.
- `vision`: sight through terrain — an entity carrying `Transparency` dims sight instead of
  cutting it (aabbworld v1.6.0: a ray spends its radius as a budget, a forest at 0.6 takes 2.5×
  its depth), `Sight.Clear` looks over the veils (a flyer), what cuts sight still cuts. A
  `Sight.Radius` above `MaxSightRadius` works; only the outline is coarser.
- Flying is a convention, not a feature: `Mover{Domain: Air}` on kinds that admit `Air`, a
  `Collider` without `Physics` so nothing pushes the flyer, `Costing(Air, 1)`, `Sight{Clear: true}`.
- `navigation`: route arrows every 15°, so hex steps draw true.
- `render`: `Hexagon`; `QuadBatch` draws in chunks under the 16-bit index limit.

**Demos**
- `navigation-hex-demo`, `navigation-vision-demo`, `navigation-vision-hex-demo`, `island-demo`,
  `effect-demo`. The two vision demos have a hawk that flies over the wall and the forest and sees
  through the forest, whose veil fades the other units' cones.

## v0.2.0 — 2026-09-22

Renamed to **gram**: the module is `github.com/kjkrol/gram`, the root package `gram` (`gram.Run`).
Plugin resource names (`gram.world`, `gram.collision`, ...) follow, so saves written by gokebiten
do not load. The API below is what settled since v0.1.1.

**Game**
- Stages and Scenes: a `game.Game` is a named set of `game.Stage` values, each with its own ECS
  built when entered, and each Stage a set of `game.Scene`s with a live `Composition` saying what
  is shown and which Scene takes input. `game.Runtime` is one interface for pause, quit, switching
  Stage, persistence and the camera.
- A launcher: `gram.Run(game)`.
- Persistence: resources matched by name; `PostLoader` and `Restorer` hooks.

**Plugins**
- `plugin.Plugin` is the one extension point; behaviors (`plugin.Between`, `plugin.Each`) are
  registered on the plugin that hosts them and refused elsewhere.
- `plugins/world`: config moved into the plugin (`ctx.UseWorld`); edge rules per axis; `OnExit`
  and `Tracked`; `Attach`, `Detach`, `Declare`; the entity renderer with appearance modifiers.
- `plugins/world/kind`: kinds defined from a `Spec` of `Const` and `Load` components; `Seed` and
  `Populate`.
- `plugins/collision`: `Collider` and `Physics`, `ShapeTest`, `Meeting` and `Struck` behaviors,
  ready-made `CountContacts`, `ShowHits`, `LogContacts`.
- `plugins/vision`: `Sight` cones scanned each tick into `Seen`, `Sighting` behaviors, drawn
  outlines; ready-made `Flee` and `Chase`.
- `plugins/board`: single-occupancy fix. `camera`: a root package with zoom limits.
- Clean architecture: `game`, `plugin` and `render` public; only `internal/engine` internal.
- On aabbworld v1.5.0: collisions through `collide.Engine`.

**Project**
- Every package has a `doc.go`; the root one carries the concepts, the tick lifecycle and the
  package graph. README rewritten, with `examples/minimal` as its example.
- All benchmarks in `bench/`; `Makefile` with `bench` and `bench-save`; results and method in
  `BENCHMARKS.md`.

## v0.1.1

Resources reorganised, camera improvements, internal logic hidden behind a public API.

## v0.1.0

First release: `Game`, `Plugin`, world, physics and camera plugins, the collision demo.
