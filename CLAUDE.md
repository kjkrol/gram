# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**gram** (formerly gokebiten; module `github.com/kjkrol/gram`) is a public, modular Go game engine library: a
user-implemented `game.Game` — a named collection of `game.Stage`s, each
with its own lifecycle (`Init`/`Restore`/`Spawn`/`Update`) and its own
`game.Scene`s (`Stack`/`Composition`, the sole entry point for input) — is
driven by a `gram.Engine` that wraps
[goke](https://github.com/kjkrol/goke) (a type-safe, archetype-based ECS)
into [Ebitengine](https://ebitengine.org/)'s `Update`/`Draw`/`Layout` loop.
Everything beyond the tick loop is installed as a `plugin.Plugin`, added
from `Stage.Init` via `ctx.Use`. See "Stage / Scene" below for the model.

Since this is a library third parties `go get` and browse on pkg.go.dev,
the extension contract — `game.Game`/`Stage`/`Scene`/`Stack`/`Composition`/
`Initializer`/`Runtime`/`Persistence` and `plugin.Plugin`/`Installer`/
`Serializable`/`PostLoader` — lives in the public root packages `game` and
`plugin`, not `internal/`, so it gets full godoc treatment. Only pure
orchestration (`Engine` itself, nobody's godoc a user needs to read) lives
in `internal/engine`; root `gram` just re-exports `Engine`/`Props`/
`NewEngine` as thin aliases over it.

## Commands

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./...   # standard verification sequence
go test ./plugins/world/... -run TestName -v                     # a single test
make demo-collision                                                # go mod tidy && run examples/collision-demo
make demo-navigation                                               # go mod tidy && run examples/navigation-demo (players: bindings for select, move, camera)
make demo-navigation-hex                                           # the same on a hex board
make demo-navigation-vision                                        # board + navigation + vision: walls cut sight, forests dim it, a hawk flies over
make demo-navigation-vision-hex                                    # the same on a hex board
make demo-effect                                                   # an ice witch: frost and frozen as effects
make demo-island                                                   # a map larger than the window under a moving camera
make demo-island-25                                                # the island in Quasi3D from above: a map in relief, sight with heights
make demo-island-isometric                                         # the island in Quasi3D through an isometric camera: relief, blocks, billboards
make demo-scenes                                                  # go mod tidy && run examples/scenes-demo
make demo-vision                                                  # go mod tidy && run examples/vision-demo
make demo-minimal                                                 # the README example
make bench                                                        # every benchmark once, with allocations
make bench-save                                                   # 5 repeats into bench_results/ (ignored by git)
```

The `examples/*` programs are real Ebitengine GUI apps (open a window)
— `go test` alone can't exercise them. To sanity-check one still runs after
a change in a headless environment: build to a temp path, run under
`timeout <n>s`, treat exit 124 (still running, not crashed) as healthy.
This doesn't substitute for looking at the demo when a visual change needs
actual confirmation.

CI (`.github/workflows/go.yml`) runs
`xvfb-run -a go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...`
on Go 1.27.0.

## Architecture

### Plugin system

`plugin.Plugin` — `Name`, `Install(ctx plugin.Installer) error`, `RunPlan`,
`WithRenderer`, `Renderer`, `EventHandler`, `Serializable`,
`RegisterBehavior` — is the one extension point. A behavior is registered on
the plugin it concerns and run inside that plugin's own pass. The hosting plugin's own
constructors build them — `vision.Between(a, b, fn)` (a pair of tags), `board.Each[T](fn)`
(one entity carrying `T`), `world.Every(fn)` (every entity) — so a game never imports
`plugin/host`; the tag families join the host's queries as optional components, so a
behavior costs no query. The func's payload type — `collision.Meeting`, `collision.Struck`,
`vision.Sighting` — is what says whose it is: a host refuses one made for another (`ErrUnhostedBehavior`), so
registering in the wrong place is an error, never a silent no-op. A plugin author wraps
`host.Pair`/`host.Each`/`host.Every` from `plugin/host` in typed constructors and runs them
with `host.PairHost[P]`/`host.EachHost[P]`. Tags are bits of a family, not component types: `plugin.Tags[F]` is one component
holding up to 64 tags of family `F` (an empty type a plugin or a game names the family by:
`selection.Family`, `behavior.Family` in vision), `kinds.DefineTag[F](name)`
hands out the bits by name through `world.Kinds` (saved by name, remapped on load like `TypeID`),
`comp.Tagged(tags...)` gives them to a kind, a query over the family's `Tags` narrows to entities
carrying any of them, and flipping a bit is a value write seen the same tick. `Between(a, b, fn)`
takes tags as values (`plugin.Any` for either side); a payload's `plugin.Marks` answers
`marks.Carries(tag)` for the families the host's behaviors name. This keeps goke's
128-component budget for data. A `Stage` builds its plugins as its own struct fields
inside `Init` and installs each via `ctx.Use(p)`, which registers
`p.Serializable()` (if any) and calls `p.Install`. There is no dependency
retry mechanism: a plugin needing another plugin's *behavior* takes it as
an explicit constructor argument (e.g.
`collision.NewPlugin(hitExpires, worldPlugin)`) rather than looking it
up — the dependency's construction order in the caller's code, not `Use`
registration order, is what matters. `Install` itself only queues ECS
wiring (`ctx.UseModule`/`ctx.Setup`), flushed once via a single
`ecs.Setup()` call after `Stage.Init` returns — this is what lets
`Persistence.Load` decide fresh-spawn vs. restore before the ECS commits
to either path. The same generic `ctx.Track(s plugin.Serializable)` lets a
`Stage` register non-`Plugin` state (its `Composition`, see "Stage /
Scene") into the same save/load machinery, without it needing to be a full
`Plugin`.

Initial state follows the same optional-interface pattern as
`plugin.Restorer`/`plugin.PostLoader`: `Stage.Spawn` only declares it through
each plugin's own typed `Seed` (`world.Plugin.Seed(roster)`,
`board.Plugin.Seed(layout)`), and the engine then calls `Populate()` on every
tracked `plugin.Populator` — only when `Restore` loaded nothing. Entity kinds
are defined in `Stage.Init` with the `plugins/world/kind` package:
`prey := kind.Define[P](world.Kinds(), "prey", kind.Spec{...})` — a `Spec` is
just the list of a kind's components, each made in `kind/comp`: `comp.Const(v)` (same for all) or
`comp.Load(func(row P) T)` (read from that entity's row), `world.Position` and
`world.Velocity` among them (one of each, or `Define` panics by name; so does a
`Load` over a row type other than `P`). `Define` hands back the kind itself,
`kind.Of[P]`: `prey.Entry(row)` builds a roster entry for `world.Plugin.Seed`
— the row's type is checked by the compiler — and `prey.ID()`/`SpriteID()` say
what its entities carry and are drawn from. `kind` never imports `world`
(`world` imports it), which is why a kind's id is `kind.ID` and the registry,
`world.Kinds`, sits behind the `kind.Registry` interface; it also issues atlas
slots no kind owns (`NewSprite`) and tells `Persistence.Load` about
every component type its kinds carry (`Kinds.LoadComps`), so a game's own tags
and state (`behavior.Predator`, `behavior.HitMark`) survive a save without being registered
anywhere else; the engine lists a type a kind shares with a module once. Cell
kinds go through `board.Plugin.CellKindDict().Create`.

A `render.Atlas` sizes nothing up front: `Register(size, draw)`/`RegisterAt(id,
size, draw)` only record sprites, each at a texture size of its own (the drawn
size is the entity's box; this is resolution), and `Close()` is what lays the
sheet out and bakes it — so a slot issued late (`Kinds().NewSprite()`) is as
welcome as an early one, as long as it comes before `Close`.

Package layout: `render` (root) — `Renderer`/`AtlasSource`/`Atlas`/
`Composer`/`Frame`/`Source`/`Tier`/`CachedRenderer`/`SolidBackground`/`TelemetryRenderer`,
drawing with zero knowledge of Stages/Scenes/plugins. The world of a scene is one
`render.Composer` over the plugins' renderers, which are `render.Source`s: each hands its pieces to
a `render.Frame` in screen pixels with a `Tier` (`Ground` 100, `Objects` 200, `Overlays` 300,
`Marks` 400; a game may use the gaps) and a depth; the composer draws tiers in order, and when the
camera's projection `Sorts()` everything below `Marks` back to front by depth (ties by tier, then arrival),
so a hill hides the route and the cone behind it and the selection stays on top. One Kage shader
draws every piece, a plain colour sampling its sheet's white texel (`AtlasSource.White`), so a run on
one sheet is one call; `Frame.Line` and `Frame.Soft` fade their edges through the vertices' custom
values; `Frame.Tile`/`TileRect` outline a tile along its own edges, which is the square board's grid.
A piece lying across cells takes the depth of its nearest end, else the nearer tile covers
half of it.

How things lie on the screen is a plugin's `Look`, swappable: `world.Look` (an entity's sprite, where
it is drawn for picking, its footprint for outlines) and `board.Look` (a cell, handed as a
`board.Tile` with its box, sprite and the heights of its top and its neighbours'), both flat from
above by default. The isometric view is a plugin a game adds: `isometry.NewPlugin(world, Config{Cell,
TileW, TileH, HeightUnit, Headroom})`, made right after the world, sets the world's camera factory
(`world.SetCameras`; `camera.Config` has no projection) and its Look (billboards), and
`WithBoard(board)` the board's (blocks with the faces turned towards the eye); it refuses a
wrapping world. Its camera turns by any angle (heading saved with the camera): the projection turns
the ground frame from the 2:1 view, Depth is how far down the screen the middle of the cell lies
(every point of a cell ties with its tile), Toward follows the heading. The plugin is a
CommandHandler with a RunPlan (after the world, before players): `Turn{Camera, Angle}` (Q/E held,
`TurnStep` 2° a tick) and `Follow{Camera}` (V, bound only `WithSelection(sel)`, which hands it the
Selected tag as navigation takes it: fastens the camera behind the one selected unit — centred,
turned with an ease of `followEase` until its `Vel.Dir` runs up the screen — held through other
selections, orders, pans and turns until V again or the unit is gone), `Tilt{Camera, Angle}`
(PageUp/PageDown, 1° a tick; the projection's `Pitch` from 10° to 90°, the 2:1 view at
asin(TileH/TileW), scaling the ground down the screen by sin and heights by cos; saved with the
camera; a fastened camera pans its unit `shoulder`·cos(pitch) of the screen below the middle) and
`Drive{Camera, Ahead, Turn}` (arrows: the camera system attaches `world.Driven` on V, writes the
keys every tick, writes a stop and detaches it on letting go; navigation's `driveSystem`, after
the orders, turns `driveTurn` a tick, walks on while the cell just ahead admits the domain and the
occupancy, stops dead otherwise, removes a MoveOrder a hand touches and keeps Cell, occupancy and
CellEntered with the unit). Commands carry
`control.Context.Camera`, as `selection.Follow` does, so the plugin never knows players; which
camera is fastened to what is the camera system's state, cameras being no entities. selection
must not import isometry, even in tests (isometry imports selection). Everything that is
the isometric view and nothing else — the projection, the camera, the billboard, the blocks and
their shading — is private to the plugin; `camera` has the contract and `TopDown`, `internal/camera`
the top-down camera. `world.Z`, relief and `Quasi3D` are not the view but the world's heights: sight
over walls and hills reads them in a top-down game too (navigation-vision-demo). A world with heights
is lit by `world.Sun` (`DefaultSun`, `SetSun`; direction, strength, ambient): the board lights each
tile per corner from the ground's slope there and at its neighbours (`board.Tile.Light`, `FaceLight`
for upright faces) and both looks draw with it, so a top-down map shows its relief; pieces carry a
`render.Shade` per corner. The terrain casts shadows (`board.Plugin.WithShadows`, on by default):
per tile corner, a walk towards the sun over the tops of the cells as the frame read them, stopped
above the highest top within 16 cells of the view; worked out as cells come into sight and kept by
the renderer until `Board.Version` or the sun changes. Entities with a `Z` cast soft shadows the
world renderer lays on the ground away from the sun (tier `Ground+20`), stretched by their height
and pushed off by how far above the ground they stand. A flat world is drawn as its sprites are.
The time of day is `plugins/sky`: a `sky.Day{Time, Pace}` on the sky's own entity (made at Setup,
found after a load), moved on every tick; at every one of `Config.Steps` a day the world's sun is
set to `Config.SunAt` the hour (east at 6, south at noon, west at 18 — the whole path turned when
`Config.NoonWay` puts noon elsewhere; the isometric island's is north-west — below the horizon at night, the
strength and ambient rising and falling), so the terrain's shadows are worked out anew only per
step. A `plugin.CommandHandler`: `Pause` (P) stops the day or lets it go on (`Day.Stopped`, saved);
`Forward` (]) and `Back` ([) double and halve the pace while it goes by, move it half an hour on or
back while it stands. Both islands with heights use it.
`CellKind.Shine` (0–1) makes a kind glint, per pixel in `render/compose.kage`: after the tile a
look calls `Frame.Glint(box, t.Shine(), t.Shore())`, a quad over the tile added to it (alpha 0)
whose vertices carry the shine (shine × sun reaching the corner) in red, the world position in
green and blue, 2 + the tile's brightness in alpha and the shore in Custom0..3 (the way to it, the
distance, how near). The shader tilts the surface by seven waves moving with the composer's clock —
within `shoreReach` (3) cells of the nearest cell that does not shine, by a swell whose crests
follow the distance to it, rolling in, its phase drifting along the coast, breaking into foam
(`surfWidth`, lit by the tile's brightness, laid over with its alpha) — and throws the frame's sun (`Frame.Sun`, set by the board) towards
`camera.Projection.Toward()` (the eye; straight up from above, along the diagonal in the isometric
view). The board works the shore out per corner of a square grid (open water on any other), once
per terrain version. A shiny tile gets its glint at night too (shine 0, the foam left). An effect altering `Ground` can make a cell shiny. The islands with heights
give their water 0.9.
A plugin adds lines to the telemetry through a `render.Reporter` (`Report(line func(label, value))`,
reading its own components through its own query); a scene hands it over with
`render.NewTelemetryRenderer(...).With(p.Reporter())` — the sky's shows the time of day. The renderers keep
their data (queries, `View`, `Drawing` behaviors, the cells) and ask the Look only for geometry;
selection picks and outlines through the world's Look, navigation lays routes on the ground through
the camera. Heights (`Quasi3D`) are the model and work in either view. `plugin`
(root) — `Plugin`/`Installer`/`Serializable`/`PostLoader`/`Populator`, the extension
contract; imports `render` (`Plugin.WithRenderer(atlas
render.AtlasSource)`). `game` (root) — `Game`/`Stage`/`Scene`/`Stack`/
`Composition`/`Initializer`/`Runtime`/`Persistence`, what a `Stage`
implements and receives, all in one package (Scene needs the same
`Runtime` a Stage does, so they're never split across a layering boundary
— see "Stage / Scene"); imports `plugin` and `render`. `internal/engine`
— the concrete `Engine` driver plus the unexported `ecsHost`/
`initializer`/`persistence`/`storage`/`stageRuntime` implementing
`game.Initializer`/`game.Persistence`/the save registry/one active
`Stage`'s runtime; imports `game`, `plugin`, and `render`. Dependency
direction is one-way: `render` ← `plugin` ← `game` ← `internal/engine` ←
`gram`. Built-in plugins (`plugins/*`) import `plugin`/`render`
directly (not `gram`), exactly like a third-party plugin would.

### Module naming convention

Every package under `plugins/` that implements `goke.Module` names that
type `module` (unexported); the exported `Plugin` is a thin facade proxying
only what callers need. A `module` never also implements `goke.System`
directly — per-tick logic lives in its own dedicated type/file (e.g.
`selection.SelectionSystem`), which `module.RegSystems` constructs.

### Systems stay inside their plugin

A type implementing `goke.System` is named with the `System` suffix
(`ScanSystem`, `cellSystem`, `altitudeSystem`) and never leaks out of
its plugin: no `Plugin` method hands out a system's state, and no plugin takes a
callback from the game that reaches into another plugin's system. A system
keeps no table beside the ECS — what it knows about an entity is a component on
that entity (`vision.Transparency` on a see-through unit, not a map of id →
value), and whoever needs it reads the component through its own query.

Each `plugin.go`/`module.go` groups methods under banner comments — contract
methods first, then everything plugin/module-specific — so a file's shape
shows how much of it is boilerplate vs. real behavior.

### Built-in plugins (`plugins/`)

- **`world`** — foundation a Stage installs by calling
  `ctx.UseWorld(cfg)` in `Init`, once (a second call panics); `cfg` sizes
  the space, toroidality, entity bounds and camera, and a Stage that never
  calls it gets no world. `SpaceCfg.Edges` (`aabbworld.Edges`) sets the edge rule
  per axis — `aabbworld.Torus`, `WrapX`/`WrapY` alone, `OpenX`/`OpenY`, a closed
  axis by default: a box stops whole at a closed edge, wraps at a wrapping one,
  and may leave by an open one. An entity wholly past an open edge carries
  `world.Outside`, put on by whoever moved it there (`MoveSystem`, collision's
  solver); every tick it does, `world.Each` behaviors of a `world.Leaving`
  registered on the world hear of it, and with none it is despawned; back inside
  it loses the mark.
  `world.Roster()` is what the plugins in the game ask of a unit's kind, gathered as the plugins
  are made: `kind.Require[T](&roster.Unit, by, why)` names what the game must supply (world:
  `Position`; board: `Cell`, `Mover`; navigation: `Steering`), `roster.Unit.Default(comp.Const(v))`
  what a plugin brings itself (world: `Velocity{}`; collision: `Collider{}`, `Physics{}`, dropped
  with `comp.Without[T]()`); a game builds a unit's Spec with `roster.Unit.Spec(own...)` and a
  missing requirement panics by plugin and reason. A plugin's requirements go in its `NewPlugin`.
  `world.Layers` are the planes an entity is on, one bit each (none, or the component absent:
  every plane); collision and vision read it, so a hawk on `Air` and a walker on `Land` neither
  push nor block each other. A world without heights is a set of planes: that is the 2D model.
  `world.Config{Quasi3D: true}` gives the world heights: entities carry `world.Z{Altitude,
  Height}`, the board sets the world's `Ground`, sight follows geometry (`Sight.Eye`) while
  collision stays on planes. The dimension is the game's choice in `world.Config`; no plugin
  guesses the mode from the data, and each refuses the other mode's facts where it first meets
  them (a `Z` in a flat world, `Blockers` in a Quasi3D one).
  `Base` — the one component every entity carries, holding its `Position`,
  `Velocity`, `TypeID` and `Caps` (the `aabbworld.Capability` bits the space
  indexes it under; `collision` writes them), so a host hands it to whatever it
  hosts instead of anyone binding it twice — plus Appearance, entity spawning and
  `Despawn`, `Attach(cb, id, v)`/`Detach[T](cb, id)` — the mid-game counterparts of a
  kind's `k.Const`, for game logic that has a `plugin.Tick` and no `CompID`
  (`Declare[T]()` in `Stage.Init` tells saves about a type only ever attached) — the shared
  `*aabbworld.Space`, per-tick movement — capped per entity at half its own
  shorter side (`world.StepReach`, `Position.MaxStep`/`MaxSpeed`), so mixed
  sizes share a world without the smallest slowing the rest — and the shared `camera.Camera` (the
  root package `camera` is only the contract and the projections; the cameras, with their window
  arithmetic — wrapping on a wrapping axis, held inside the world on any other — live in
  `internal/camera`) exposed via `world.Plugin.Camera()`, more via `NewCamera()`. World hosts three payloads for `world.Each[T]`/`world.Every`, all through
  `RegisterBehavior`: a `Moving` (every entity before it moves, to scale `Base.Vel.Value`;
  board's terrain speed is one), a `Leaving` (every tick an entity is `Outside`) and a
  `Drawing` (every entity about to be drawn; `world.Draw.Overlay[T]`, `Draw.As[T]`,
  `Draw.With[T]`, `Draw.Facing` are ready-made). The space keeps no state of its own between ticks:
  `MoveSystem` moves every box under the edge rules (`Space.Move`), then hands
  the space every `Base` as an `aabbworld.Item` (`Space.Rebuild`) — `Query`,
  `Scan` and collisions read that grid until the next tick. After movement the `ViewSystem` refreshes every
  `world.View` (a rectangle plus the `EntitySet` of entities the space finds in it; `Plugin.NewView`
  over any bounds source, `Plugin.View()` is the camera's) — the entity renderer draws only what
  the camera's View contains. `doc/views.md` maps where this leads: players (a view, a
  command queue and a translator each — bindings for a person, a brain for an AI; plugins define
  the command types and ship default, labelled bindings) and a networking plugin over them; `doc/movement.md` sketches
  movement along a route through `Steering` (motion profile, lookahead point, waypoints, walls
  as solid cells, holes). `Populate` and
  `PostLoad` rebuild it too, so it is whole before the first tick; a despawned
  entity is gone from it on the next. Anything reading the space in its own pass
  sees the boxes as they were after the last rebuild.
- **`board`** — optional grid + terrain over `world`; its grids wrap per axis,
  following the world's `Edges` (`SetWrap(x, y)`). A `CellKind` says which `Domain`s it admits
  (`Land`, `Water`, `Air`, a game's own bits), whether it is `Solid` (a wall), how much it
  `Veil`s sight (a forest at 0.6) and whom it `Veils` (a forest veils `Land`, not `Air`), and
  what it costs — `Costing(domain, cost)` prices it differently per domain, and
  `CostFor(domain)` is what a unit pays in the planner and in the Moving behavior board
  registers on the world (only entities carrying `Mover` are slowed); a unit's
  `Mover` says which domains it moves in (none: `Land`) and, in a Quasi3D world, how high it
  flies (`Lift`). `board.NewUnits[Row](brd, board.Shape{Size, Height}, at)` is how a game defines
  its units: `units.Define(name, board.Mover{…}, steering, extra...)` derives `Position` and
  `Cell` from the one point `at` reads off a row, `Layers` from the domain, in a Quasi3D world a
  `world.Z{Height}` from the shape, runs the world's roster and `kind.Define`, and hands back the
  usual `kind.Of[Row]`. Every cell is an entity for good, without a `world.Base`: `Plot` (its
  cell and its `Relief`, the corner heights) and `Ground` (its kind, kept apart so an effect
  ending restores the kind alone), made by the `cellSystem` at Setup or
  found after a load; the `Board` reads and writes them, keeping only the cells' entity ids by
  ordinal, and a seed (`TerrainMap`, heights) before Setup or on a board no ECS runs. `Version`
  counts every change: writes through the board, and effects on cell entities, which the
  `cellSystem` learns from `effects.Active.Altered` and `effects.Idle` on the cells. In a Quasi3D world a `CellKind` has a `Height` (what stands on it), the ground's heights
  come from `Layout.Heights` and the shaping commands (`Raise`, `Lower`, `Level`, `Shaping`); the
  `Board` is the world's `Ground`; the `altitudeSystem` writes every `Z.Altitude` each tick from
  the ground under the entity plus its `Lift`. A flat world refuses all of it at the first sight (`CellKindDict.Create`,
  `NewUnits`, `Units.Define`, `Kinds.Register`).
  Every tick, after
  collision's `RunPlan`, `board.RunPlan` reports a `Standing` (cell under the centre and its kind) to
  `board.Each` behaviors registered on the board, naturally `board.Each[board.Mover]`;
  `Standing.Fell(domain)` is a land unit in water or in a hole, and the reaction is the game's.
  A `board.Effect` (`Tick(brd, d) alive`) is what the board does to itself over time by writing
  terrain — `Plugin.Cast`/`Dispel`, ticked first each tick; `board/effect` ships `Timed` (terrain
  that reverts), `Cycle` (phases turning kinds, seasons) and `Once`. Terrain is never an entity in
  the space: the `Board` is the world's `Field` (`Solid`: the cells under a box that are `Solid`
  and keep out one of the entity's layers, sides open towards open ground; a hex gives the boxes
  of `Grid.CellBoxes`) and `Cover` (`Walk`: the cells along a ray whose `Veils` meet the
  observer's `Blockers`, τ = 1 - `Veil`, band from the cell's ground up by `Height`), set on the
  world in `NewPlugin` and read from the cell entities whenever collision or sight asks, so a
  change counts from the next tick. `Solid` and `Veil` are independent. Depends on `world` alone.
- **`collision`** — optional collision detection over `world`'s space, one
  `CollisionSystem` system a tick. An entity collides exactly while it carries `Collider` —
  `comp.Const(collision.Collider{})`, or `Attach`/`Detach` mid-game. The `CollisionSystem`
  first settles every `Collider`'s `Base.Caps` (`CanCollide`, plus `Static` for an
  immovable `Physics`, `Sensor` for none) and rebuilds the space when any changed;
  two colliders touch only where their `world.Layers` meet — a board game uses `Domain` bits —
  and a `Collider` counts from the tick it is carried.
  The tick is then one
  `collide.Engine.Tick` (`github.com/kjkrol/aabbworld/collide` holds the contract —
  `Handler`, `Config`, `Engine`; the `CollisionSystem` builds the engine once with
  `space.CollideEngine(handler, collide.Config{Reach: world.StepReach, Iterations})`
  and is its `Handler`) over the space's items: the engine pairs up whoever carries
  `CanCollide` and may touch within a step, tests the pairs exactly, pushes the overlapping apart and reports each
  pushed box (`Moved`), which the `CollisionSystem` writes back to `Base.Pos` by `Seek` —
  whoever it pushed out through an open edge (`Left()`) is marked `world.Outside`. Every overlap first
  passes the `CollisionSystem`'s `Touch`: both sides are resolved by `Seek`, and a side
  that lost its `Collider` since the last rebuild vetoes the pair, is marked
  `Plain`, and the space is rebuilt after the tick (so it partners nobody again); then
  the plugin's `ShapeTest` (`WithShapeTest`; `BoxesTouch` by default, at no cost),
  asked once per overlapping pair with both `Contactee`s, may refuse the contact (an
  alpha mask saying the pixels miss) or refine the penetration (an SDF). An entity
  carrying `Physics` (`Mass`, `Restitution` 0–1) is pushed out of overlaps and
  bounces — the bounce is the engine's own, an infinite `Mass` is a wall; one without
  `Physics` is only ever detected (a town, a trigger). Separation is always an even
  split. With a world `Field` (the board's solid cells) the engine, built in `Init` with
  `Config.Field`, also pushes every movable collider out of the solid ground on its `Layers`; the
  `CollisionSystem` is its `FieldHandler`, bouncing off the ground as off an infinite mass and
  recording a `Contact{Terrain: true, Cell}` (no `Meeting`: `Between` is for entities).
  Reactions are behaviors hosted inside the `CollisionSystem`'s own pass:
  `collision.Between(a, b, fn)` of a `Meeting` per confirmed contact between two tags
  (`plugin.Any` as the wildcard), `collision.Each[T]` of a `Struck` per entity per
  tick, with what it struck the tick before. A strategy exports a plain function of
  the flat `collision/behavior` package (`CountContacts`, `LogContacts`, `ShowHits`) —
  the tags it runs between are named where it is registered,
  `RegisterBehavior(collision.Between(a, b, fn), ...)`. `Collider` is the plugin's one
  aggregate: what the entity struck (`Collider.Contacts()`). Depends on `world`.
- **`navigation`** — pathfinding/movement toward a `MoveOrder` across a
  `board`. A navigated unit carries a `world.Steering` profile: navigation only asks it for a
  heading (at a lookahead point, so turns start before the bend) and for its own top speed, braking
  from the profile before the goal; a waypoint is passed by projection, the goal by radius. A
  `MoveOrder` queues up to `MaxWaypoints` further goals; its `Face` is the point the unit turns
  towards on arrival — a right click on the unit's own cell (`MoveTo.At`) or S + right click
  (`LookAt`: finish the step, stop, turn). A trigger may ask for a key held besides its modifiers
  (`control.Mods{}.Holding(key)`). Built `WithCollision(c)`, a unit that struck someone or the
  solid ground (a `Struck` behavior navigation registers on `c`) stops, re-plans from where it
  stands and holds that route for `bumpInterval`, so units pushing each other on a road step
  aside instead of shoving for ever. Occupancy is seeded from `Cell` + `Mover` at Setup (no spawn
  effect needed). A `MoveTo{Cell, Append}` command orders
  every `Selected` entity; a `plugin.CommandHandler`, its `DefaultBindings()` make a right click one,
  Shift appends. Depends on `board`, `world` and `selection` (its `Selected` tag picks whom a
  command orders).
- **`effects`** — temporary changes to entities, cast from anywhere: `p.Define(name,
  Spec{Lasts, Stacking, Grant(tags...), Alter(func(*T))})`, `p.Cast`/`CastFor`/`Dispel`/`Has` by
  entity id, `Active` slots saved with the entity, originals of altered components kept by the
  plugin and saved with the game. An entity whose last effect ended carries `effects.Idle`
  for one tick, and `effects.Each` behaviors of an `effects.Idling` registered on the plugin
  hear of it once. A cast before the plugin's pass lands the same tick.
  `board.Plugin.CellEntity(c)` is a cell's own entity, carrying its `Ground` and `Plot`, so an
  `Alter[board.Ground]` is a temporary change of terrain. `Idle` goes from entities without a
  `Base` too. Depends on `world`.
- **`selection`** — a `Select` command (ids, or a world box, additive or not) → the `Selected`
  tag on `world` entities that carry `Selectable`, both bits of `selection.Family` from
  `Plugin.Tags()` (a kind's choice via `comp.Tagged`); a bit flip, seen
  the same tick. A `plugin.CommandHandler`: its `DefaultBindings()` make a left drag one (Shift adds),
  the left button held a `Marquee` (the box being dragged, drawn by its renderer in the dragging
  camera's view until the `Select` that ends it), and F a `Follow` — the third tag, `Followed`, on the one selected unit (none with several; F
  again stops), which the `FollowSystem` keeps in the middle of the camera every tick
  (`camera.Camera.CenterOn` at its altitude) until the player moves the camera by hand; zooming
  keeps it. Depends on `world`.
- **`players`** — whoever acts in the game, a carrier over `plugin.CommandHandler`s:
  `players.NewPlugin(world, s.selection, s.nav, ...)` gathers each one's `Queues()` (the
  `control.Queue[C]` it drains in its own pass) and `DefaultBindings()`; `Defaults()` is all of
  them plus `CameraBindings()` for players' own `Pan`/`Zoom`. `Local(name)` is a player at the
  keyboard over the world's camera and `View`, `Add(name)` one without (an AI, a client);
  `Issue(player, cmd)` is how any command comes in (`ErrUnknownCommand` for a type no command handler
  defines). The contract — `Queue`, `Issued`, `PlayerID`/`Nobody`, `Binding` (`Trigger`s
  `KeyPress`, `ButtonPress`, `Drag`, `Wheel`, `ButtonHeld`, `CursorAtEdge` with exact `Mods`,
  `Command[C]` built from a `Context` with `World`/`WorldBox` through the camera and, in a Quasi3D
  world, on the ground under the cursor via `Context.Ground`) — lives in
  `control`, and `plugin.CommandHandler` names what defines and carries out commands (one handler
  per command type; events have subscribers, commands a handler), so a plugin with commands never
  imports players. `Viewports(screen)` is what a Scene showing the world returns as its
  `game.Viewer`: a viewport per camera the local players look through, in equal columns; players
  draws nothing, the Scene lists every layer itself. Its `eventHandler` is the layer from input to
  commands: per local player it keeps what it has seen of keys and buttons (`input`), matches
  events against the player's bindings and issues what they build; `Player` holds only who the
  player is.
  `Player.OwnCamera()` (before Use; `world.Plugin.NewCamera`, saved by players) splits the screen
  (`Columns`, `WithLayout`); keys reach every local player, the mouse the one under it in its
  area's pixels; `control.KeyHeld` fires once a tick while its key is down (issued at the end of
  players' RunPlan, for the next tick, so a slow frame still drives every tick). Commands that depend
  on a camera carry the player's (`selection.Select.Camera`, `Follow{Camera}`). `Player.Bind` refuses two on one
  trigger, Setup refuses a command nobody defines; `WithRenderer` draws the marquee of a drag.
  The Scene hands input to `players.EventHandler()`; `players.RunPlan` runs last and empties the
  queues. Depends on `world`.
- **`vision`** — narrowed perception: a `Sight` cone scanned against `world`'s
  space each tick fills its own `Sight.Seen` (who this entity can see, nearest first), and
  `SightOutline` on an entity gets its view's shape computed and drawn. An entity carrying
  `Transparency` dims sight instead of cutting it, and the world's `Cover` (the board's veiled
  cells) is walked along every ray, dimming or cutting it the same way without being seen, a
  ray spending its radius as a budget through it; whatever the ray reaches is seen, a forest
  looked into as much as a wall. `Sight.Blockers` are the `world.Layers` that cut or dim this
  sight at all (zero: every entity): a hawk with `Blockers` of `Air` looks over walls, forests
  and walkers and still sees them; a walker with `Land` looks under the hawk. In a Quasi3D world
  sight has heights instead: the cone's eye is `Z.Altitude + Sight.Eye`, every entity spans its
  `Z`, the ground is the world's `Ground` sampled every `WithGroundStep` (default: a cell), and a
  hawk 40 up looks over the wall, the forest and the hill a walker's cone stops at; `Blockers`
  are refused there, `Eye` in a flat world. It
  hosts `vision.Between(a, b, fn)` of a `Sighting` inside the scan's own pass: once
  a tick per observer carrying `a`, with everything in view carrying `b` — a
  directed pair, grouped by observer, empty included. A behavior tells its seen
  entities apart with `seen.Carries(tag)`, and steers only
  through `Steering.Request`. Ready-made ones live in the flat `vision/behavior`
  package (`behavior.DefineTags`, `Flee.Steer`, `Chase`); a file using both plugins'
  behaviors imports them as `cbehavior`/`vbehavior` — who flees or hunts
  whom is the registration's to say. Depends on `world`.

Each package has a `doc.go` describing the gameplay capability it adds.

### Stage / Scene

A `game.Game` also supplies `Props()` (window/tick-rate config, read
once at startup by `gram.Run(g)`; `Resizable` makes the screen the window — the engine's
`Layout` follows it and hands the active world's camera `SetViewport`, whose zoom floor scales a
world smaller than the window up to cover it; Shift+F toggles fullscreen in every game,
`Runtime.ToggleFullscreen`; `TargetTPS` is the engine's own fixed
step — Ebitengine runs one `Update` per frame (`SyncWithFPS`), and a frame that
falls behind runs at most 5 steps and drops the rest, so the game slows down
instead of spiralling) alongside a named collection of
`game.Stage`s plus which one starts active (`Stages() (map[string]Stage,
string)`) — every game writes its own small `Game` implementation, even
for a single Stage, since only a concrete type can supply its own `Props`.
A `Stage` is what `Game` itself used to be:
`Init`/`Restore`/`Spawn`/`Update`, each with its **own** `*goke.ECS`, built
fresh (plus a fresh `world.Plugin` if its `Init` calls `ctx.UseWorld`; the
engine only fills an unset camera viewport from the screen size) the moment `Runtime.SwitchStage`
enters it — so a menu Stage can sit idle with zero gameplay entities until
the player actually starts the game, at which point the gameplay Stage's
`Init`/`Restore`/`Spawn`/`ecs.Setup` run for the first time. A `Stage` is
never a `plugin.Plugin` — it's the thing that *installs* plugins via
`ctx.Use`, exactly like `Game` did before. **A `Stage` has no
`HandleEvents`** — input is a `Scene`'s sole responsibility (see below); a
shortcut a game wants shared across multiple Scenes (e.g. Escape-to-quit)
is a plain helper function each of their `HandleEvents` calls, not an
engine concept.

Within one active `Stage`, `game.Scene` is what `Game.Draw`/`HandleEvents`
used to be: `Name`, `Layers() []render.Layer` (built once, on entering the Stage: `render.Renderer`s drawn on the screen, `render.WorldRenderer`s — a `render.Composer` over the plugins' `Source`s — drawn per viewport of a `game.Viewer` scene, a run of them onto an image of the viewport's area unless it covers the screen; each layer is Init once however many scenes list it), `HandleEvents`,
`Focusable`. `Stage.Stack()` is the static, `Name()`-keyed registry of
every `Scene` it can show (`game.NewStack(scenes...)`); `Stack.Composition()`
(the only way to reach it — `Stage` has no accessor of its own) is the live
per-tick state over that Stack — which scenes are visible, in
what z-order, and which one is `Active()` (the topmost with `Focusable()
== true`, so a non-focusable HUD/minimap can sit on top and still never
steal input). Every tick, the engine calls `HandleEvents` on **only** the
Scene `Composition.Active()` names — never in parallel with anything at
the `Stage` level. `Composition` embeds `plugin.Serializable` directly (no
import trickery needed — `game` already imports `plugin`) — a `Stage`
registers it once via `ctx.Track(composition)` in `Init` so visibility/
order survive `Persistence.Save`/`Load` automatically, the same name-keyed
way a `Plugin`'s own state does.

`Runtime` (`Paused`/`Pause`/`Resume`/`TogglePause`/`Quit`/`SwitchStage`/
`Persistence`/`TPS`/`Camera`) is a single, undivided interface — the exact
same value reaches both a `Stage` (via `Initializer`/at Restore time) and
every `Scene`'s `HandleEvents`; there is no cut-down "scene-level" subset.
A menu scene's "Start" button calls `runtime.SwitchStage(gameplayStage.
Name())` directly, no need to bubble a click up through anything. Anything
that's a *plugin's* capability rather than an engine primitive (camera,
selection, navigation...) reaches a `Scene` the same way it reaches a
sibling `Plugin`: constructor injection at `Stage.Init` time, as a plain
struct field — never through `Runtime`.

See `examples/scenes-demo` for a full walkthrough: a menu `Stage` with no
gameplay entities, "Start" switching (lazily building the ECS) into a
gameplay `Stage`, and a togglable modal `Scene` over the still-ticking
world plus a non-focusable HUD overlay.

### Game / persistence

`internal/engine.Engine` (aliased `gram.Engine`) owns the Ebitengine
loop and drives a user's `game.Game` one active `Stage` at a time — see
"Stage / Scene" above. `Engine` never caches its own copy of the Stage
set: it calls `game.Stages()` whenever it needs to resolve a name (once in
`Init`, and on every `Runtime.SwitchStage`) — `game.Game` is the sole
container. Its `Persistence()` returns a `game.Persistence` (interface —
implemented by the unexported `internal/engine.persistence`, see
`persistence.go` + `persist.go`) bound to the active `Stage`'s own
`*goke.ECS`, implementing `Save`/`Load`/`List`; a tracked plugin/module
implementing `plugin.Serializable`/`plugin.PostLoader` is included
automatically. Saved resources are matched by name (a `Plugin`'s `Name()`,
or the Go type name for anything tracked without one) rather than by
position, so a save survives plugins being added/removed/reordered
between game versions — see `internal/engine/persist.go`.

### Testing conventions

Internal tests (`package world`, not `world_test`) are used when a test
needs unexported module state; otherwise use the external `_test` suffix.
Prefer testing through a plugin's real `Install` path over hand-built
shortcuts when what's under test is install-order or wiring behavior —
real regressions here have only shown up through the actual
`Install` → `ecs.Setup` → `RunPlan` sequence, not lower-level unit tests
that bypass it.

## Docs, benchmarks and commits

`doc/roadmap.md` is the map: what is done and what comes next, in order, with the reasoning in
`doc/movement.md` and `doc/views.md`; keep it current when a stage lands. `doc/rendering.md` is a
proposal, not built: one composer per viewport sorting draw items by layer and depth.
Every package has a `doc.go` with `# Type` sections describing what it brings; the root `doc.go`
carries the concepts, the tick lifecycle and the layered package graph. README leads with what the
library is; its code example is `examples/minimal`, so change that program first and keep the
README in step. All benchmarks live in `bench/` (package `bench_test`), built on a headless
`game.Initializer` there; `BENCHMARKS.md` records the results and the method (this machine drifts
~10% between runs — compare a baseline copy and the working tree alternately). `CHANGELOG.md` gets
an entry per tag. The headless installer in `bench/headless_test.go` is the fifth copy of the same
helper (the others are in the demo's and plugins' tests) — a candidate for one exported test helper.

Comments in code are short: one or two sentences saying what a thing is and what it is for. No
essays, no restating the signature, no background — that belongs in `doc/*.md` or a `doc.go`.

Commit messages use the conventional prefixes (`feat`, `fix`, `docs`, `refactor`, `perf`, `test`,
`chore`) and carry no `Co-Authored-By` trailer.
