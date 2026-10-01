# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**gram** (formerly gokebiten; module `github.com/kjkrol/gram`) is a public, modular Go game engine library: a
user-implemented `game.Game` — a named collection of `game.Stage`s, each
with its own lifecycle (`Init`/`Restore`/`Spawn`/`Update`) and its own
`game.Scene`s (`Stack`/`Composition`, the sole entry point for input) — is
driven by a `gram.Engine` that wraps
[goke](https://github.com/kjkrol/goke) (a type-safe, archetype-based ECS)
in a window's loop — a tick and a picture a frame — drawing everything on the GPU through WebGPU
([gogpu](https://github.com/gogpu/gogpu), pure Go over Vulkan; Ebitengine is gone since 2026-09-29).
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
make demo-board                                                    # the island on the simple map: a flat board drawn from its kinds' colours, plain bands, a flat day
make demo-board-topography                                         # the island in relief: heights, light, water, isometric or from above (Tab), the weather on the ground
make demo-board-atlas                                              # a small flat board drawn from the game's own atlas of drawn sprites
make demo-scenes                                                  # go mod tidy && run examples/scenes-demo
make demo-vision                                                  # go mod tidy && run examples/vision-demo
make demo-minimal                                                 # the README example
make bench                                                        # every benchmark once, with allocations
make bench-save                                                   # 5 repeats into bench_results/ (ignored by git)
```

Only `render/gpu` and `internal/engine` import gogpu (and wgpu, gputypes); plugins see `render`
and `control`. Tests that draw ready a headless device (`gpu.Headless`; `GRAM_GPU=software` for
the software rasteriser, which draws wrong) and skip without a GPU. A running game logs its frame
rate and where a frame's time goes every second with `GRAM_FPS_LOG=1`, starts fullscreen with
`GRAM_FULLSCREEN=1` (F11 switches) and runs unpaced with `GRAM_VSYNC=off`; headless probes of a
demo's frame (temporary `zz_*_test.go` files) include about 10 ms of waiting for the GPU's
readback at 2560x1440, so compare them with each other, not with a frame rate.

The `examples/*` programs are real GUI apps (open a gogpu window)
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
`Hook` — is the one extension point. A rule (`rule.On(name, filter, func(m *rule.Moment[P])
rule.Step {…})`, `plugins/world/rule`, a `plugin.Rule`) is hooked on the plugin it concerns and run
inside that plugin's own pass; its filter, the second argument, says whom it fires for:
`rule.All`, `rule.Self(a)` one carrying a tag, `rule.Between(a, b)` a pair (a moment that is
`rule.Met`), `rule.Having[T]()` one carrying `T` — so a game never imports `plugin/host`; a
plugin's own hooks and the ready-made ones in its `hooks` package are written straight on
`plugin/host` (`host.Each`, `host.Every`, `host.Pair`), Go code a game never writes; the tag
families join the host's queries as optional components, so a rule costs no query, and rules
over one component share its column (`host.Own` shares the host's own). The moment's type —
`collision.Meeting`, `collision.Struck`, `vision.Sighting`, `unit.Standing`, `cell.Now`,
`world.Moving`, `clock.Moment` — is what says whose it is: a
host refuses one made for another (`plugin.ErrUnhosted`), so hooking in the wrong place is an
error, never a silent no-op. A plugin author runs them with `host.PairHost[P]`/`host.EachHost[P]`/
`host.ListHost[P]` from `plugin/host`. Tags are bits of a family, not component types
(`plugins/world/entity/tag`, a leaf): `tag.Tags[F]` is one component holding up to 64 tags of
family `F` (an empty type a plugin or a game names the family by: `selection.Family`,
`hooks.Family` in vision), `kinds.DefineTag[F](name)` hands out the bits by name through
`world.Kinds` (saved by name, remapped on load like `TypeID`), `comp.Tagged(tags...)` gives them
to a kind, a query over the family's `Tags` narrows to entities carrying any of them, and
flipping a bit is a value write seen the same tick. A payload's `plugin.Marks` answers
`marks.Carries(tag)` for the families the host's rules name. This keeps goke's 128-component
budget for data. The bits serve two ways (`entity/tag` doc): **tags** are groups a kind gives
(`comp.Tagged`); **markers** are states switched on and off — a plugin's family `States`, carried
for good (`comp.Marks[F]()` in a kind, `Roster().Unit.Default` for every unit, attached once where
missing), each marker a constant bit defined by name like the owners (`effect.Changed`, every
effect's own `Effect.Mark()` "effect.<name>", `navigation.Entered`). Putting a component on or off moves the
entity in memory (~200 ns, `Benchmark_Marker_*`) against 1–2 ns for a bit: a state that changes
often or lasts a step is a marker; one that lasts, on few entities, walked alone (`MoveOrder`,
`Mind`, the facts, `world.Outside`) keeps its own component. Data that comes and goes keeps its
component, empty when off (`effect.Active`). A `Stage` builds its plugins as its own struct fields
inside `Init` and installs each via `ctx.Use(p)`, which registers
`p.Serializable()` (if any) and calls `p.Install`. There is no dependency
retry mechanism: a plugin needing another plugin's *logic* takes it as
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
are defined in `Stage.Init` with the `plugins/world/entity/kind` package:
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
and state (`hooks.Predator`, a game's own component) survive a save without being registered
anywhere else; the engine lists a type a kind shares with a module once. Cell
kinds go through `board.Plugin.CellKinds().Create`.

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
so a hill hides the route and the cone behind it and the selection stays on top. One WGSL shader
(`render/shaders/compose.wgsl` over the library and every registered material) draws every piece,
a plain colour sampling its sheet's white texel (`AtlasSource.White`), so a run on one sheet is one
call; `Frame.Line` and `Frame.Soft` fade their edges through the vertices' custom
values; `Frame.Tile`/`TileRect` outline a tile along its own edges, which is the square board's grid.
A piece lying across cells takes the depth of its nearest end, else the nearer tile covers
half of it. A `render.Direct` source draws a part of the picture itself with a WGSL shader of its
own (`render.NewMeshShaderWith`: its own vertex stage, `Instanced(n)` vec4s an instance, depth
test and write, `ReadDepth` to lay a decal over what the frame drew) at its tier, handed a
`render.Target{Screen, Depth}` — the frame's shared depth buffer, reversed (1 nearest), cleared
as the frame begins — and the composer's uniforms; the terrain, the sky, the rain, the world's
sprites, the views of sight and the routes are Direct. `render.Sprites` draws a Direct source's
sprites as instances, piece for piece what `Frame.SpriteRectUV` lays (the world's flat look). The
per-frame CPU work of the island at 2560x1440 is under 0.5 ms. A `render.Still` is a frame composed once in world units (a camera drawing a world
unit a pixel) and kept on the GPU run by run (`gpu.Kept`), drawn every frame scaled and moved
(`gpu.Draw.Place`) and lit (`gpu.Draw.Tint`, plain colours only) as a camera from above shows it:
the board's renderer is a `render.Direct` at `Ground` that composes a flat map under a
`look.EvenLit` dressing (the simple map's, the atmosphere's `litDressing`) into one, anew only
when the board changes (`Board.Changes`), draws it in `EvenLight` — copies across a wrapping seam
— and the grid over it on the GPU (`shaders/grid.wgsl`: a square grid's tiles darkened along
their edges, a hex grid's edges); anything else keeps composing its tiles every frame. The
clouds' shadows over a flat world (`atmosphere.Plugin.Clouds`, `atmosphere/overcast`) are a Direct too, their noise
worked out every 8 pixels of a mesh on the GPU.

How things lie on the screen is a plugin's `Look`, swappable: `world.Look` (an entity's sprite, where
it is drawn for picking, its footprint for outlines) and `look.Look` (a cell, handed as a
`look.Tile` with its box, sprite and kind), both flat from above by default. What a board is drawn
and priced by beyond its cells is its `board.Map` (`Look`, `Dressing`, `Top`, `Climb`, `Least`,
`Slope`; `board.Plugin.WithMap`): the board's own is the simple map — flat, every kind in its
`cell.Kind.Color` or drawn sprite (`cell.Kinds.Draw`; `WithRenderer(nil)` draws from the
board's own atlas of the kinds), the ways and crossings as plain bands (`internal/draw.Bands`), a step at its kind's
cost — and `plugins/topography` is the other, a map in relief: `topography.NewPlugin(world, board,
Config{Cell, TileW, TileH, HeightUnit, Headroom, Isometric, Shaping, Climbing})`, made right after
the world and the board, sets the world's camera factory (`world.SetCameras(cameras.Maker(…))`;
`camera.Config` has no projection), its Look (`billboards.Look`: billboards in relief, the world's
`FlatLook` from above, all drawn on the GPU), the board's Map (its Look `look.Nothing`: the ground is drawn on the GPU) and
the world's Ground (its `Relief`); it refuses a flat or a wrapping world. `Plugin.Renderer()` is
the ground, a `render.Direct` at `Ground` a demo must put in its composer: over a square grid
`topography/terrain` — the relief's lattice as a mesh (every corner a vertex, heights in an R32F
image), a depth prepass, the board painted flat by the painter (albedo and water sheets, `Painted`)
sampled per pixel, lit by the sun with shadows baked on the GPU as the sun moves (`shade.wgsl`),
the clouds' cover baked (`cover.wgsl`), water on wet cells only, the grid, fog, and a skirt of
level ground round the world to the horizon (`skirtRings`, `skirtReach`); over a hex grid
`topography/hexes` — every cell a prism instance to its top, a face down to each lower neighbour, coloured
from the tiles composed once from above (a `render.Still` through `look.NewRenderer` with the
flat look and the painter) and drawn every frame into a world image. The painter only paints: its
tiles are dressed in white without clouds (`tile.Light` even, `sunlit` full), into the sheets or
the hexes' still; there is no per-frame tile path in relief any more. The world's entities are
billboards drawn on the GPU (`topography/billboards`: `sprites`, instanced, tested against the ground's depth, their
shadows `sky.Sun.ShadowOf` patches draped over the terrain by `terrain.DrawShadows`, over the hex
prisms laid from the frame's depth by `shades`); from above
the flat look's sprites on the GPU and the same shadows. The cameras' `camera.Rays` (a
`RayField`: origin and direction affine in the screen point; the perspective's from its eye, the
isometric and flat views' parallel) give `camera.SceneTransform` for every GPU source. One camera,
three views (`topography/cameras`): `projection.flat` is the view from above (screen x, y the world's, no height drawn, no
sorting), the isometric and, given `Config.Perspective`, the perspective; `cameras.View{Camera}` (Tab) goes
round them keeping the ground point in the middle and a cell as wide
(zoom × Cell/TileW); the view is saved with the camera. From above and isometrically the whole
screen stays over the world at sea level (`isoCamera.place` fits the ground under the four
corners into the world along x and y, `minZoom` is where the screen's footprint — `spanX`,
`spanY` at zoom 1 — just fits the world, the top-down camera's rule), so a pan stops at the edge,
zooming out stops where the screen fits the map and the corners of a diamond map are out of reach;
`ZoomIn` keeps the ground under the cursor at its drawn height (`ground`). The perspective keeps
the ground point in the middle of the screen over the world (`perspCamera.confine` after Pan,
Translate, CenterOn, ZoomIn; not LookFrom/LookAt, not riding) and flies no higher than shows the
world's diagonal across the middle of the screen at the flattest pitch (`maxAlt`, one height
whatever the heading and pitch); zooming at the ceiling turns the head so the ground under the
cursor stays put (`aim`), the eye flying on towards it where the pitch floor holds the head
(`advance`). The isometric camera turns by any angle
(heading saved with the camera): the projection turns the ground frame from the 2:1 view, Depth is
how far down the screen the middle of the cell lies (every point of a cell ties with its tile),
Toward follows the heading. The plugin is a CommandHandler with a RunPlan (after the world, before
players; the cameras and the shaping at once, the altitudes in the simulation), gathering the
queues and keys of `cameras.Control` and `relief.Shaper`: `Turn{Camera,
Angle}` (Q/E held, `TurnStep` 2° a tick), `Tilt{Camera, Angle}` (R/F held, 1° a tick; the
projection's `Pitch` from 10° to 90°, the 2:1 view at asin(TileH/TileW), scaling the ground down
the screen by sin and heights by cos; saved with the camera; a fastened camera pans its unit
`shoulder`·cos(pitch) of the screen below the middle), `LookOut{Camera}` (V given `WithSelection`
and `Config.Perspective`: rides in the selected unit, first person — the camera a `camera.Rider`,
bindings `In(camera.FirstPerson)` fire: W/S/A/D `Drive` (W with Shift `Drive.Sprint`, read from the
`KeyHeld` context's `Mods.Shift`: `steering.Helm.RequestSprint` to `Sprint`·MaxSpeed, the
world's step cap still holding; the climate's Shift+W holds `In(camera.Free)` only), the mouse
`Look` (`control.CursorMove`, cursor captured by players; across turns the view and the unit via
`steering.Driven.Face`, up/down the head; riding writes `Driven.Flown` and `Driven.Climb` =
−sin(pitch): the drive system asks a flyer for the run (`Driven.Slope`) along the ground, and the
altitude system, the one writer of heights, holds a flown flyer's `Z.Altitude` over sea level,
adds the rise over the run it made this step, keeps it `Mover.Clearance` over the ground and under
`Mover.Ceiling`, and writes `Lift` back so that let go it keeps its height over the ground), V/Tab
leave back to the view it was in; the eye where the unit's `world.Eye` stands (`Eye.Level`; its top without one), the screen as wide across as `Eye.Angle` (`perspCamera.across`; the camera's own field without one); Q/E and the free camera's WASD hold `In(camera.Free)` only), `Follow{Camera}` (V
without the perspective, bound only `WithSelection(sel)`, which hands it the Selected tag as
navigation takes it: fastens the camera
behind the one selected unit — centred, turned with an ease of `followEase` until its `Vel.Dir`
runs up the screen — held through other selections, orders, pans and turns until V again or the
unit is gone), `Drive{Camera, Ahead, Turn, Sprint}` (arrows: the camera system attaches `steering.Driven` on
V, writes the keys every tick, writes a stop and detaches it on letting go; navigation's
`driveSystem`, after the orders, turns `driveTurn` a tick, walks on while the cell just ahead
admits the domain and the keeping lets it on — the occupancy under `CellSpacing`, nobody touched
just ahead under `BodySpacing` — stops dead otherwise, removes a MoveOrder a hand touches and
keeps Cell, occupancy and the `Entered` marker with the unit) and `relief.Raise`/`Lower`/`Level` (=, -, L-drag).
Commands carry `control.Context.Camera`, as `selection.Follow` does, so the plugin never knows
players; which camera is fastened to what is the camera system's state, cameras being no entities.
The topography's parts are packages none of which imports the plugin (it composes them, registers
their systems, gathers their queues and keys, hands them its sky through `liveSky`): `relief`
(heights, climbing, shaping, the heights' entity, altitudes), `painter` (the board's Dressing in
relief, the sheets, `Style`), `water` (the materials, `Shore`, the water layers), `terrain`,
`hexes`, `billboards` (the world's Look, the entities' shadows), `cameras` (the views and their
control; `camera.Rider`, `camera.Eyed` and `Projection().Sorts()` are what the billboards ask of
them) and `internal/vec`.
selection must not import topography, even in tests (topography imports selection). Everything
that is the view and nothing else — the projection, the camera, the billboard, the blocks and
their shading — is private to the plugin; `camera` has the contract and `TopDown`, `internal/camera`
the plain top-down camera of a world without a topography. `world.Z`, relief and `Heights` are not the view but the world's heights: sight
over walls and hills reads them in a top-down game too (navigation-vision-demo); the scan runs
observers on every CPU, a `scanner` per goroutine (`vision.Plugin.WithWorkers`), after the board's
cover has read the cells' (`ground.Readied`) and one query has settled the space's index. **Layering: the
world knows its entities and nothing else; the ground is the board's, the sky the atmosphere's,
`render` generic.** A relief is lit by the sun of its `topography.Atmosphere`
(`WithAtmosphere(atmospherePlugin)`; without one `sky.DefaultSun` in still clear air): `sky.Sun`
(`Dir`, `Strength`, `Ambient`, the colours of its light and of the sky — zero is white;
`Sun.Light`/`Shaded` give a `render.Light`: Ambient × Sky plus the direct light × Color; `Sun.Frame`
hands the frame the uniforms of `sky/shaders/sun.wgsl`, `Sun.ShadowOf` is the `sky.Patch` an
entity's shadow covers on the ground). The ground is lit on the GPU per pixel from its normal, the
shadows of the relief and of what stands on it baked as the sun moves (`topography.Plugin.WithShadows`,
on by default); pieces of a frame carry a `render.Shade` — a `render.Light` (RGB) per corner,
`render.Even(v)` grey, `render.Lit(l)` one light. The world's renderer asks its `Look` for every
entity in white light with its `Appearance.Sway`; the topography's look lights it by the sun on
level ground, leans it with the wind and lays its shadow on the relief away from the sun,
stretched by its height and pushed off by how far above the ground it stands; the world's own flat
look draws it as it is.
The time of day, the climate and the weather are `plugins/atmosphere` on the world's clock
(`plugins/world/clock`; see below). `atmosphere/calendar` is the clock at a fixed scale — a day
every `Config.Day` of game time from the moment a fresh game begins at (`Start`, the hour on the
clock as a `time.Duration`, in the middle of `Season`), a `GameYear` of 8 days and a 4-day moon or an `EarthYear` — with no state of
its own: `Calendar.Now()` is a `Moment{Date, Time, Year}` (`OfYear`, `Season`, `Moon`, `Hour`,
`Written`), `Daily`/`Yearly`/`Seasonal` give a schedule entry its period and offset.
`atmosphere/sky` sets the world's sun once a tick, in the interface part, to `Config.LightAt` the
moment — as it goes, or at every one of `Config.Steps` a day where a game asks for steps — the sun (with `NoonWay` `celestial.South`: east at 6,
south at noon, west at 18; the default `celestial.NorthWest` turns the whole path so noon is beyond the
isometric view's sea; the path worked out for the climate's zone's latitude: declination 23.44° ×
sin(2π·ofYear), the hour angle from noon — polar day and night past the circle), and below −0.1 of
height the moon (`moonStrength` 0.5 × how full, `moonColor`; `Sky.SetMoon` off: none) — the strength rising and falling,
the sky's and the sun's colours and the ambient blended from the `daylight` table by the sun's
height: blue by day, orange at sunrise and sunset, deep blue at night. The terrain bakes its shadows
anew as the sun goes on a strip a frame (`shadeStrips` 16, a round past every tenth of a degree:
0.4–0.85 ms of the GPU a frame at 2560x1440), all at once when the sun leaps a degree or the ground
changes; H (`topography.CoarseShadows`, `Plugin.WithCoarseShadows`) bakes them half as fine a side. The light can be frozen (`Freeze` P, `Later`/`Earlier` Shift+]
and Shift+[ move it half an hour): only the light, in memory, not saved; the calendar and the
weather go on. The celestial sphere is `atmosphere/celestial`, which `sky` imports, never the
other way round: `celestial.Heavens` (`Place.HeavensAt`, a `Place{Latitude, NoonWay}` that
`sky.Config` makes with its zone's latitude) is where the sky's bodies stand: the sphere turned by
the sidereal time (`SiderealTime`: the sun's right ascension along the ecliptic,
`λ = 2π·ofYear`, plus its hour angle) over the latitude (`Sphere`, `Pole`, `OnSky`), the sun on
its path (`SunPath`), the moon on its own (`MoonAt`: ecliptic longitude the sun's + 2π·moon,
5.14° off it, rising and setting with the stars, drifting 13° a day eastwards among them; `Phase`),
the stars `sky.Config.Stars` chooses (`RealStars`: `celestial.Stars()`, the Yale Bright Star
Catalogue to V 6.0, 5080 stars in `stars.bin` from `stars_gen.go`; `ScatteredStars` made up). `atmosphere/backdrop` (`backdrop.Renderer`) is the
viewport in the sky's colour on `render.Backdrop` (tier 0), a Direct: through a perspective the
sky of the day (`backdrop/shaders/backdrop.wgsl`: the gradient from the horizon up, the sun a white disc
in a halo and a wider glare, the moon's face `celestial.MoonFace()` (`moon.png`, NASA SVS CGI Moon Kit) —
lit by its phase, north to the pole; the discs `discAngle` of the camera's focal length, so they
grow as it zooms; the clouds on their layer from the clouds' tile, hazed towards the horizon,
hiding what lies behind them), the real stars drawn first as instances (`celestial.StarField`,
`celestial/shaders/stars.wgsl`: placed on the CPU through `camera.Vanisher`, ~0.2 ms for 5080) on black,
the sky laid over them with the alpha of what hides them; otherwise the viewport in the sky's
colour wherever the ground does not cover it. `atmosphere.Running` (`Config.Running`,
`Plugin.SetRunning`) switches the day, the weather's changes, the wind, the clouds, what falls,
the weathering, the stars and the moon, all on by default, not saved (the backdrop takes the
stars and the moon of it through `backdrop.Renderer.WithShown`). The atmosphere's root keeps no
shaders and draws nothing itself: every part — `calendar`, `sky`, `celestial`, `climate`, `air`,
`precipitation`, `weathering`, `backdrop`, `overcast` — is a package that never imports it.
`cell.Kind.Shine` (0–1) makes a kind glint, per pixel in the topography's materials
(`plugins/topography/water/shaders/sea.wgsl`, `stream.wgsl`; every material a plugin registers with
`render.RegisterMaterials` joins the composer's library and every mesh shader built on it): the
painter paints the wet cells' shine and flow into the water sheet, and the terrain's shader calls
`SeaGlintAt(p, shine, lit, shore, pixel, toward)` and `RunningWater` over the wet cells. The sea
is five octaves of stretched value noise (`chop`: `noised`, analytic gradient, each fading out
where a pixel spans too much of it, `seenAt`) running along x — never turned with the wind,
whose wander would swing the whole sea to and fro; the wind only roughens it
(`calmSea`..`stormSea`) — shaded to and from the light, with a narrow glint and a broad sheen,
within `shoreReach` cells of the shore (`water.Shore`, per corner of a square grid) a swell rolling in
and breaking into foam; it reflects the sky by Fresnel against the way to the eye — per pixel in
a perspective, `camera.Projection.Toward()` otherwise — so it pales towards the horizon. The
skirt round the world carries the sea on to the horizon. An effect altering `Ground` can make a
cell shiny. The islands with heights give their water 0.9.
The climate is `atmosphere/climate`: a `climate.Zone{Latitude, Factors}` (`Factor.Shape(*Profile)`;
`SeaCurrent`, `DrySummer`; `Equatorial` 3°, `Tropical` 20°, `Mediterranean` 38°, `Temperate` 55°,
`Cold` 66°, `Polar` 78°) is `Zone.Profile()` — `Mean` 27 − 20 sin²φ − 27 sin⁶φ, `Year` 1 + 16 sin²φ,
`Day` 4, `Wet` per season by latitude band. The kinds of weather are the subpackage
`climate/weather` (`weather.State`, its `Wind`, `Clouds` and `Billow` ranges thrown as it comes;
`weather.Default`: clear, fair, cloudy, rain, storm; `State.Likely`). The weather now, a
`climate.Weather` on its own entity (made at Setup or found after a load, saved with its dice)
begins in its first step in `Config.Start` or a state thrown by `Often[season]`, already at its
clouds, fall and temperature, and goes from one of `Config.Weathers` to the next (weights `Next` ×
`Likely(season)` × the zone's `Wet[season]` for one with `Falls`, `Lasts`), blending the wind
(`Blow` towards `Target`, `Heading` wandering), the clouds, what falls and the `Temperature` into
the state's (`Blend`; the temperature the zone's `Mean` ± `Year` through the year, ± `Day` through
the day, and `State.Warmth`, the day the calendar's), what falls coming down as snow below
`snowsBelow` 1°C, integrating `Drift`, and keeps the air as it stands (`Climate.Air()`, an
`air.Weather`; `atmosphere.Plugin.Air()`; `Climate.SetRunning` leaves out what is stopped) — every
step of the simulation (`Climate.System` under `clock.Simulate`), so the tempo hurries it and the
tactical pause stops it. It hosts rules of `Weathering` (`host.EachHost`, fired every step
with the weather and season; `atmosphere.Plugin.Hook`). `Change`
(Shift+W) and `Set{Name}`. `atmosphere/precipitation` is what falls (screen-space streaks and
flakes from a hash of their number and `Frame.Time`, tier `render.Air` 350, depth +∞);
`atmosphere.Plugin.Precipitation()`. `atmosphere/weathering` is what the weather does to a board:
`weathering.Config{Snowy, Ice, Water, Sway, Swaying, High, Seed}` names the game's own kinds — a
kind's snowy twin, what water freezes into, what sways — and `New` defines three effects
(`effects.Alter[cell.Ground]`: snow swaps the kind for its snowy one, ice water for ice, sway sets
`Sway`), laid on the world's schedule once a second of game time: snow settling in drifts (`High`
ground, a noise's seeds, next to snow) while it snows in the frost, melting lonely and late cells
first once warm, ice growing from the shore below −3°C, what sways swaying above a wind of 15 and
stopping below 10; a winter begun has its drifts and shores laid at once
(`atmosphere.Plugin.WithWeathering(board, cfg)`). `plugins/atmosphere/air` is the weather as
drawn (`air.Weather`): `Weather.Frame(f, sun)` hands the frame `Wind`, `Drift` (a `HeapTile` at
most), `Cover`, `Billow` and the `Fog` colour (`air.Overcast(sky, clouds)`), `Weather.Sway` leans
what sways, `Weather.Cloud`/`Shade` are the clouds' noise and shadow on the CPU, the very numbers
the shaders' `cloudField`, `cloudCover` and `cloudShade` (`air/shaders/cloud_noise.wgsl`,
`cloud_shadow.wgsl`: shreds of `cloudSize` 420, four octaves, and heaps' cores in cells of
`heapSize` 3150 — one heap a cell, flat topped, forming as the cover reaches its own, lobed —
mixed by `Billow` (`cloudMix`), spread by `cloudContrast`, the shadow straight under, dimming the
sun by `cloudDark`) work out on the GPU, `Weather.Haze` is how much the air hides a point from a
camera's eye. The noise is periodic (`CloudTile` 12600, `HeapTile` 50400) and baked once by
`air.BakeTile` into a 1536×1024 tile with six levels (red the shreds, green the heaps' cores); the
sky and the terrain (a copy right of the shade in its baked image) look it up per pixel
(`cloudTileSpot`, `cloudTileLevel`, `cloudFromTile`), the rest works out `cloudField` itself;
waves steepen with `Wind` (`calmSea`..`stormSea`), water reflects `overcastSky()`. A flat world
takes the clouds' shadows from `atmosphere.Plugin.Clouds()` (`overcast.Renderer`, tier `Objects+50`, a Direct: a mesh
over the viewport, the noise at its corners every 8 pixels where the camera's lines of sight meet
the ground, the shadow per pixel) and its light by the hour through
`atmosphere.Plugin.WithBoard(board)`, which wraps the board's Map (`litDressing`, `look.EvenLit`:
the sun on level ground, one light for every tile, so the board is composed once and tinted on the
GPU) and the world's Look (`litLook`: sprites lit, what sways leaning on the CPU, handing the flat
look's GPU sprites through). A flat board without an atmosphere is drawn as it is.
A plugin adds lines to the telemetry through a `render.Reporter` (`Report(line func(label, value))`,
reading its own components through its own query); a scene hands it over with
`render.NewTelemetryRenderer(...).With(p.Reporter())` — the sky's shows the time of day. The renderers keep
their data (queries, `View`, `Drawing` rules, the cells) and ask the Look only for geometry;
selection picks and outlines through the world's Look, navigation lays routes on the ground through
the camera. Heights (`Heights`) are the model and work in either view. `plugin`
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
(`ScanSystem`, `entitySystem`, `altitudeSystem`) and never leaks out of
its plugin: no `Plugin` method hands out a system's state, and no plugin takes a
callback from the game that reaches into another plugin's system. A system
keeps no table beside the ECS — what it knows about an entity is a component on
that entity (`vision.Transparency` on a see-through unit, not a map of id →
value), and whoever needs it reads the component through its own query.

Each `plugin.go`/`module.go` groups methods under banner comments — contract
methods first, then everything plugin/module-specific — so a file's shape
shows how much of it is boilerplate vs. real behavior.

### API in packages, machinery in internal

A plugin split into packages keeps what a game or another plugin uses — contracts, entry
points, facts — in its normal packages, and the machinery its own packages share — stores,
systems, implementations — in `plugins/<plugin>/internal/...`, which only the plugin's own
packages may import. Names exported inside `internal` are not API; a type from `internal`
never appears in a public signature, and tests outside the plugin cannot use it (a test that
must read a package's insides uses an `export_test.go`). A system in `internal` follows the
topography's pattern: a part type whose method returns a private `goke.System`
(`terrain.Cells.System()`, `rule.Rules.StandingSystem()`). `plugins/board` is split this way;
the other plugins follow as they are split.

### Behaviour goes through rule

gram is a library: whatever a game may want to change is written in the rule
formalism (`plugins/world/rule`), never as a policy inside a plugin's system. A
plugin **perceives** — moments its pass catches, for rules (`unit.Standing`,
`collision.Meeting`, `navigation.Touch`), and facts, for plans — and **carries out**
commands (`navigation.StepAside`, `world.Despawn`), keeping the engine's own rules
inside the handler (a unit is never stepped into water, off a cliff or into a wall).
**Effects** hold state and a rule's memory. What to do when is **rules** (`rule.On`)
and **plans** (`rule.Plan`). A plugin hooks its own defaults itself, unexported
(navigation's crowd); a game adds its own with `Hook` or replaces them where the
plugin offers it (`navigation.Plugin.WithCrowd`). Export only what a game needs: the
public API is small. A new behaviour adds a moment, a fact or a command where
perception or an action is missing, and writes the rule with `rule`; it never adds a
branch to a system.

A game is written the same way: its **states are effects** (burning, frozen, alarmed),
each with its own marker (`Effect.Mark()`) that rules of any plugin filter by; its
**rules connect** the plugins' moments to those effects (`Apply`, `Keep`, `Dispel`,
`Unless`, `Chance`, `Then`, `Here`, `Around`); and the plugins give **knobs** — components
an effect's `Alter` turns (a cell's `Ground`, `Steering`, `Sight`, `Physics`, `Appearance`).
**A knob is a component its plugin only reads**; what a system writes lives in another
component beside it (`steering.Course` beside `Steering`, `vision.Sighted` beside `Sight`,
`Ground` apart from the cell's heights), given where missing — an `Alter` ending puts back
the original, and would put back stale state too. A plugin's own effects stay private. A
step a rule cannot say is a missing moment, step or knob, not a reason to write Go code in
a rule (`doc/rule.md`, "A game: states as effects"). A **player acts** the same way, by
putting an effect on something with a command from a binding: `selection.Apply{Effect}` on
its own selected units (an ability), `world.Apply{Effect}` on the world itself (a state of
the whole game, a lever pulled), which rules and plans read with `During` (the trapdoor
demo: a lever an effect each, its trapdoors cells tagged with a tag of `cell.Family` each, a
rule a pair, made in a loop).

### Built-in plugins (`plugins/`)

- **`world`** — foundation a Stage installs by calling
  `ctx.UseWorld(cfg)` in `Init`, once (a second call panics); `cfg` sizes
  the space, toroidality, entity bounds and camera, and a Stage that never
  calls it gets no world. `SpaceCfg.Edges` (`aabbworld.Edges`) sets the edge rule
  per axis — `aabbworld.Torus`, `WrapX`/`WrapY` alone, `OpenX`/`OpenY`, a closed
  axis by default: a box stops whole at a closed edge, wraps at a wrapping one,
  and may leave by an open one. An entity wholly past an open edge carries
  `world.Outside`, put on by whoever moved it there (`MoveSystem`, collision's
  solver); every tick it does, rules of a `world.Leaving`
  hooked on the world hear of it, and with none it is despawned; back inside
  it loses the mark. An entity gives itself `world.Despawn{}` (`Order` in its plan or a rule) to go;
  `world.Apply{Effect}` puts an effect on the world's own entity, the clock's (drained by the
  moments' system before the clock's rules; `plugin.Tick.World` names that entity, `During`
  reads it); the world
  carries the commands entities give themselves (`control.Carrier`, `world.Plugin.Carry`/`Commands`;
  the engine carries every `plugin.CommandHandler` a stage uses, and a host's `plugin.Tick.Commands`
  hands the carrier to its rules).
  `world.Roster()` is what the plugins in the game ask of a unit's kind, gathered as the plugins
  are made: `kind.Require[T](&roster.Unit, by, why)` names what the game must supply (world:
  `Position`; board: `At` (the unit's cell), `Mover`; navigation: `steering.Steering`), `roster.Unit.Default(comp.Const(v))`
  what a plugin brings itself (world: `Velocity{}`; collision: `Collider{}`, `Physics{}`, dropped
  with `comp.Without[T]()`); a game builds a unit's Spec with `roster.Unit.Spec(own...)` and a
  missing requirement panics by plugin and reason. A plugin's requirements go in its `NewPlugin`.
  `world.Layers` are the planes an entity is on, one bit each (none, or the component absent:
  every plane); collision and vision read it, so a hawk on `Air` and a walker on `Land` neither
  push nor block each other. A world without heights is a set of planes: that is the 2D model.
  `world.Config{Heights: true}` gives the world heights (`Plugin.HasHeights()`): entities carry
  `world.Z{Altitude, Height}`, written by the board in relief from its ground, sight follows
  geometry (`world.Eye`) while collision stays on planes. The dimension is the game's choice in `world.Config`; no plugin
  guesses the mode from the data, and each refuses the other mode's facts where it first meets
  them (a `Z` in a flat world, `Blockers` in one with heights). `world.Config.Scale{Metres}` says what
  a world unit is (one unit system: heights and lengths alike; games give metres through
  `Scale.Units`); with it the ground sinks under an eye's level (`Scale.Drop`, curve and
  refraction) in the perspective and in sight, and the air hazes far off (`Weather.Visibility`).
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
  `internal/camera`) exposed via `world.Plugin.Camera()`, more via `NewCamera()`. World hosts rules of five moments, all through
  `Hook`: a `Moving` (every entity before it moves, to scale `Base.Vel.Value`;
  board's terrain speed is one), a `Leaving` (every tick an entity is `Outside`) and a
  `Drawing` (every entity about to be drawn; `world.Draw.Overlay[T]`, `Draw.As[T]`,
  `Draw.With[T]`, `Draw.Facing` are ready-made), and a `clock.Moment` (every step, its own system
  just before the effects' pass). Every `plugin.Tick` a host hands its rules comes from
  `world.Plugin.Tick(cb, d)` (a `plugin.TickSource`): the carrier, `Time` (game time at the step's
  end) and `Seed` (`world.Config.Seed`), which `Chance` draws from. The space keeps no state of its own between ticks:
  `MoveSystem` moves every box under the edge rules (`Space.Move`), then hands
  the space every `Base` as an `aabbworld.Item` (`Space.Rebuild`) — `Query`,
  `Scan` and collisions read that grid until the next tick. After movement the `view.System` refreshes every
  `view.View` (a rectangle plus the `view.EntitySet` of entities the space finds in it; `Plugin.NewView`
  over any bounds source, `Plugin.View()` is the camera's, `Plugin.ViewFor(cam)` any camera's) — the
  entity renderer draws only what the camera's View contains. `Populate` and
  `PostLoad` rebuild it too, so it is whole before the first tick; a despawned
  entity is gone from it on the next. Anything reading the space in its own pass
  sees the boxes as they were after the last rebuild.
- **`board`** — optional grid + terrain over `world`; its grids wrap per axis,
  following the world's `Edges`. The root keeps the `Plugin`, the `Board` (the terrain's façade:
  `Kind`, `Bare`, `Set`, `SetAll`, `Way`, `SetWay`, `Crossing`, `SetCrossing`, `Along`,
  `CellVersion`, `Changes`, `Version`, `Touch`, `Shape`), the `Layout`, the `Map` contract and
  `NewUnits`; the ground the others meet is its field's (`Plugin.Cover`, `WithCollision`). Public
  subpackages: **`board/cell`** — `cell.ID`, `cell.Kind` and the board's `cell.Kinds`
  (`board.Plugin.CellKinds()`), `cell.Domain` (`cell.Land`, `Water`, `Air`), `cell.Name`/`Named`,
  the cell entity's `cell.Plot`, `cell.Ground`, `cell.Way`/`Crossing`/`Links`, the game's tags of
  places (`cell.Family`, `cell.Tag`, `cell.Tags`), the moment `cell.Now`, the Layout's entries
  (`cell.Entry`, `cell.WayEntry`), `cell.Terrain`, `cell.TerrainMap` (terrain in plain maps, the
  board's seed, a `Terrain` of its own in tests), `cell.Occupancy` (`SingleOccupancy`,
  `MultipleOccupancy`); **`board/unit`** — `unit.At`, `unit.Mover`, `unit.DomainAt`, the moment
  `unit.Standing`; **`board/grid`** — `grid.Grid` (with `Toward`), `grid.DefaultGrids`,
  `grid.Link`, `grid.Shape`/`ShapeOf`; **`board/look`** — `look.Look`, `Dressing`, `EvenLit`,
  `Parallel`, `ParallelLook`, `Tile`, `FlatLook`, `Nothing`, `Map` (the drawing part of
  `board.Map`), `Renderer`/`NewRenderer` (reading the board as a `look.Board`), `RenderState`,
  `MinGridCell`; **`board/ground`** — `ground.Heights`, `Cover`, `Readied`. The machinery is in
  **`board/internal`**, importable only inside the board: `internal/terrain` (the cells' state —
  seed or entities, `Cells`, the entity system, the counts of changes, the kinds' dictionary),
  `internal/rule` (`Rules`: the hosts of `unit.Standing` and `cell.Now`, their systems, `Around`,
  `TerrainSpeed`), `internal/field` (the ground's cover and solid field), `internal/grids` (the
  square and hex grids' types), `internal/draw` (the simple map's bands), `internal/occupancy`
  (the release of the gone), `internal/boardtest` (what the board's tests share: an installer, a
  world of world, collision, board and vision, a game of one stage; tests sit in the package whose
  code they test). A `cell.Kind` says which `cell.Domain`s it admits
  (`Land`, `Water`, `Air`, a game's own bits), whether it is `Solid` (a wall), how much it
  `Veil`s sight (a forest at 0.6) and whom it `Veils` (a forest veils `Land`, not `Air`), and
  what it costs — `Costing(domain, cost)` prices it differently per domain, and
  `CostFor(domain)` is what a unit pays in the planner and in the Moving rule board
  registers on the world (only entities carrying `Mover` are slowed); a `Graded` kind (a road, a
  bridge; `Way.Over` and `Crossing.Over` carry it) is spared the slope in both; `Board.Along(from,
  to)` tells a step along a way's links from one over the ground beside it, `Board.Bare(c)` is
  that ground, the kind under the way; slopes
  cost too, through the board's Map — `relief.Climbing{Up, Down, Ease, Steep, Free}`
  (`topography.Config.Climbing`, `relief.DefaultClimbing`: 1 in 10 up takes twice as long, 1 in 10 down is the
  quickest at 0.7, steeper down slows by 5 a unit, Air free), multiplying the kind's cost (the
  island: road and bridge 1, the rest 2.5 times what it was) slows the Moving rule along the
  heading (`Map.Slope`) and prices the planner's steps (`Map.Climb`, `Map.Least`; navigation
  takes them from `board.Plugin`) — both read a cell's slope off its own corners — so steep is the
  relief, never a kind; the simple map prices nothing beyond the kinds. A shiny kind with a `Flow`
  runs down its cell's slope (`Tile.Flow` → `water.Stream`, a flow map: ripples and foam
  carried with the current, white where it is fast); `plugins/board/water` works brooks, streams, rivers and fords out of a relief
  (`water.Drain`, `Network.Carved` cutting their beds into the heights), handed over as a
  `plugins/board/network` graph (`Network.Net`: nodes of board kinds, `Link` for roads, `Flow`
  down for water, `Along`, `Crossings`, laid by `Ways()`; roads by `network.Route` + `Path`, over
  rivers as `cell.Crossing`s by `Across`), laid as a `cell.Way` — a
  second layer on every cell entity, a band through the cell's middle whose kind decides who may
  cross it (`Way.Over`, `Board.Kind`), drawn by `Tile.DrawWay`. Kinds with a `Spread` blend
  (`Tile.Blends`/`DrawBlends`, `render.Frame.SpriteBlend`): a neighbour's kind weighed at the
  tile's corners, side middles and middle by the share of the cells meeting there, shown where the
  weight is over a half — one line across the tiles, not the cells' edges; a kind whose `painter.Style` lies `Under`
  (water) is drawn as the tile's base (`Tile.Base`) under its neighbours, glint and all, the land
  laid over it the same way. Ways curve round the cell's middle; a way's `Fade` has it fade out
  (a river running out to sea: `water.Config.Plume`, `water.Mouth`, `Network.Fade`).
  `Board.CellVersion` counts each cell's changes; the painter keeps a cell's read and its tile's
  blends and way (baked relative to the tile) until stale and paints them — the whole board flat
  for the terrain over a square grid (`render.Paint`), the tiles once from above for the hex
  prisms (`render.Still`) — anew where cells changed; a unit's
  `Mover` says which domains it moves in (none: `Land`) and, in a world with heights, how high it
  flies (`Lift`), the least it keeps over the ground (`Clearance`) and how high over sea level it
  may climb (`Ceiling`, 0 none). `board.NewUnits[Row](brd, board.Shape{Size, Height}, at)` is how a game defines
  its units: `units.Define(name, unit.Mover{…}, steering, extra...)` derives `Position` and
  `At` (its cell) from the one point `at` reads off a row, `Layers` from the domain, in a world with heights a
  `world.Z{Height}` from the shape, runs the world's roster and `kind.Define`, and hands back the
  usual `kind.Of[Row]`. Every cell is an entity for good, without a `world.Base`: `Plot` (its
  cell), `Ground` (its kind, kept apart so an effect ending restores the kind alone), `Way` and
  `Crossing`, made at Setup by the entity system of `internal/terrain` out of the seed (a
  `cell.TerrainMap`, the Layout's tags included) or found after a load; `terrain.Cells` reads and
  writes them, or the seed before Setup and on a board no ECS runs. `Version` and `CellVersion`
  count every change: writes through the board, effects on cell entities (`effect.Changed` on
  the cells), and `Board.Touch(c)` by whoever changes a cell beyond the board.
  The board is flat: the ground's heights are the topography's `relief.Relief` — on a square grid a
  lattice of corners the neighbouring cells share by construction (no vertical walls, no sealing),
  on any other a level per cell; `Corners`, `SetCorners`, `Altitude`, `GroundAt`, `SetHeights`
  (`relief.MeanOfCells`), `Lift`, `Flatten` — living on the topography's own entity as
  `relief.Heights`, runs of `HeightsRun` (1024) heights of a fixed size on as many entities as
  the relief takes, so the ECS keeps them in its own memory — written when the relief's version
  changes, taken back by a loaded game when they cover it exactly — seeded by `topography.Plugin.Seed(heights)` at Populate and shaped by the commands
  (`relief.Raise`, `Lower`, `Level`, `Shaping`). The `Relief` is the world's `Ground`; the relief's
  `altitudeSystem` writes every `Z.Altitude` each step from the ground under the entity plus its
  `Lift`; the board asks its Map's `Top` for a cell's level where sight needs a veil's band. In a
  world with heights a `cell.Kind` has a `Height` (what stands on it); a flat world refuses what stands
  at a height at the first sight (`cell.Kinds.Create`, `NewUnits`, `Units.Define`,
  `Kinds.Register`) and a topography refuses a flat world.
  Every tick, after
  collision's `RunPlan`, `board.RunPlan` reports a `Standing` (cell under the centre, its kind, the
  box and the `Mover`'s domain) to the rules hooked on the board; `Standing.Fallen()` is a land
  unit in water or in a hole, and the reaction is the game's (the demos:
  `rule.On("drown", rule.All, func(m *rule.Moment[unit.Standing]) rule.Step { return m.If(unit.Standing.Fallen, m.Order(world.Despawn{})) })`,
  `bhooks.LogFalls()` beside it). Then, while a rule of it is hooked, every cell as a `cell.Now`
  (its entity, which cell, its kind now). Both moments are data alone and `rule.Placed`: the
  board's `internal/rule.Rules` put their `around` into the `plugin.Tick.Around`, and
  `m.Here(step)` runs the step on the entities of the cells under the box (a `cell.Now`'s own),
  `m.Around(rings, step)` on the rings of neighbours too, each cell once (a scratch of passes, not
  reentrant) — the effect demo's
  witch freezes `Around(1, Apply(frost))`, fire spreads cell to cell from a `cell.Now`.
  `Plugin.Hook` routes by the moment's type. Every cell carries for good the game's tags of
  places, `cell.Tags` (defined by name through the world's kinds, given in the
  `Layout` as `cell.Entry.Tags` before the cells are made — tagging panics after): a rule
  of a `cell.Now` filters cells with `rule.Self(tag)` (the trapdoor demo's strips), and
  `Standing.Places` are those of the cell under a unit (the pressure plate demo: whoever stands
  on a plate orders `world.Apply`). The unit's own cell component is `unit.At{Cell}` (was
  `board.Cell{ID}`). A system of its own (`internal/occupancy`) has the `cell.Occupancy` let go of
  whoever left the world, every step (`Occupancy.Release`): a despawned unit kept its holds before,
  blocking cells. What the board does to its terrain over time is effects on the cells' entities
  (a `Spec`'s `Alter` of `cell.Ground`, `cell.Way`). Terrain is never an entity in the space: the
  board's field (`internal/field`) is collision's `Field` (`Solid`: the cells under a box that are `Solid` and keep out one
  of the entity's layers, sides open towards open ground; a hex gives the boxes of
  `Grid.CellBoxes`; `Overhang`: the area over ground a kind does not take), handed over by
  `Plugin.WithCollision`, and sight's `ground.Cover` (`Walk`: the cells along a ray whose `Veils`
  meet the observer's `Blockers`, τ = 1 - `Veil`, band from the cell's ground up by `Height`),
  `Plugin.Cover`; both read the cell entities whenever collision or sight asks, so a change counts
  from the next tick. `Solid` and `Veil` are independent. Depends on `world` and `collision`.
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
  recording a `Contact{Terrain: true, Cell}` (no `Meeting`: pairs are of entities). A push apart
  never puts a unit further over ground that does not take it (`Field.Overhang`, the board's:
  the area over cells whose kind allows none of its layers — any, for no layers — water to a
  walker, a hole; the user's rule, 2026-09-30): `footing` in `resolve` holds the side whose half
  push would (it bounces as the ground, `holdA`/`holdB`) and doubles the pen for the other, and
  `moved` keeps any box the tick's later passes — which the engine asks no handler about — would
  leave worse, rebuilding the space. Ground turning under a unit is no push: it stays, fallen in.
  A bounce's velocity is the entity's own afterwards; a steered unit's speed is its steering's.
  Reactions are rules hosted inside the `CollisionSystem`'s own pass: of a `Meeting` per
  confirmed contact between two tags (`rule.Between(a, b)`, `rule.All` for every pair), of a
  `Struck` per entity per tick, with what it struck the tick before. Ready-made rules are in the
  flat `collision/hooks` package — `Hook(hooks.CountContacts(&stats), hooks.ShowHits(hit))`,
  `LogContacts(opts…)`, and `HitOverlay(hit, with)` for the world's Hook; a game's own is
  `Hook(rule.On(name, rule.Between(a, b), func(m *rule.Moment[collision.Meeting]) rule.Step { … }))`;
  the hit is an effect
  (`hooks.Hit(w, d)` → `effect.Effect`, cast by `ShowHits(hit)` on a `Struck` that `Hit()`s, drawn
  by `HitOverlay(hit, with)`, which reads its marker, `rule.Self(hit.Mark())`). `Collider` is the plugin's one
  aggregate: what the entity struck (`Collider.Contacts()`). Depends on `world`.
- **`navigation`** — pathfinding/movement toward a `MoveOrder` across a
  `board`. `pathFinder.price` is what a step costs: the destination's `CostFor` over the step's
  length, times `Map.Climb` unless the kind is `Graded`; a slantwise step not `Along` a way is
  priced by the destination's `Bare` ground (the corner is cut beside the road) and refused where
  that ground does not admit the unit, so roads are followed round their bends and slantwise
  roads taken along their links. A navigated unit carries a `steering.Steering` profile: navigation only asks it for a
  heading (at a lookahead point, so turns start before the bend) and for its own top speed, braking
  from the profile before the goal; a waypoint is passed by projection, the goal by radius. A
  `MoveOrder` queues up to `MaxWaypoints` further goals; one with a `Round`
  (`navigation.Patrol(pause, cells...)`) never ends: reached or given up — a pit opening ahead,
  say — it goes on to the round's next goal (`goOn`, in `arrive`), standing `Pause` on each one
  reached; a step aside on the move carries the round over (the trapdoor and pressure plate demos
  give each wanderer its round by `comp.Load`, one kind for all). Its `Face` is the point the unit turns
  towards on arrival — a right click on the unit's own cell (`MoveTo.At`) or a `LookAt` (finish
  the step, stop, turn). The right button's bindings are a `Drag` (a click is the button up
  within `clickSlop` of where it went down; further, nothing moves) and a `ButtonHeld` past the
  slop into a `LookAt` at every move: a right drag turns the selected units to look at the
  cursor. A trigger may ask for a key held besides its modifiers
  (`control.Mods{}.Holding(key)`). `Spacing` (`WithSpacing`; `AutoSpacing` by the largest box
  against a cell, a third or less keeping boxes) is how units keep apart, through the internal
  `keeping` seam (`cellKeeping`, `bodyKeeping`), one `navigationSystem.Update` loop for both.
  Both plan routes blind to the others (the `pathFinder` gets `openOccupancy`); the keeping
  holds what it holds. What a unit does about another is not the keeping's: it is rules
  (`rule.On`) of the moment `Touch` navigation hosts (`Plugin.Hook`, a `host.PairHost[Touch]` whose
  tag families read on a query of their own, `marksOf`); `crowd()` (StarCraft II's, unexported) is hooked at
  Install unless `WithCrowd(rules...)` gave others, none too. A `Touch` is two units touching —
  `BodySpacing`: the Collider's contacts (`feel`, each pair once: collision records a contact on
  the one that struck), dispatched before the chunks so their commands land the same tick;
  `CellSpacing`: a step refused (`touching.refused`), dispatched after them — handed to each of
  the two from the per-tick `bodyIndex`: moving, `GivingWay`, `LastGoal`, `Ally`, `Groupmate`
  (the order's `Group`, else `LastOrder.Group`, which every unit carries for good — roster
  default, attached where missing — written as an order of a group ends; `nextGroup` counts past
  it too), `OnMyGoal`, `HeadOn` (bodies: velocities; cells: the two refused into each other's cells
  this tick or the last, `wanted`/`wanting`), `Room` (the one standing could `stepAside`),
  `WaitedOut`/`Cornered`. The rules' commands (`StepAside`, `Detour`, `Pass`, `Hold`, `Settle`,
  `Stop`; `Aimed` at the other — `rule` aims a rule's command at its moment's `Subject`) land
  in `Plugin.Queues`, drained into `told` by unit, and are carried out for that unit alone:
  standing ones in `standing` (`StepAside` only), ones under orders in `carryOut`. The keeping
  carries them out and keeps the engine's rules there: `stepAside` goes where `open`/`aside` say
  the ground takes the unit, no steeper than `yieldClimb`, nobody standing — never into water, a
  hole, off a cliff or into a wall — and stays (`GivingWay`, no home queued); on the move a while
  (`Linger`), its goals queued after. `bodyKeeping.detour` steps round square off the box's side
  (`wayOff`, `stepRound`) and notes a standing one's cell (`Avoid`, `Met`); `cellKeeping.detour`
  learns the refused cell and plans round it (`Cornered` with no way), not round one giving way.
  `pass` steps round without touching the route (cells: nothing, the step waits). The last word
  stays navigation's whatever the rules: `CellSpacing` refused `stallAfter` marks the order
  `Bumped` and `Held` for `bump` to learn the cell and plan round it, `giveUp` after `maxStalls`;
  a held corner is learnt at once and gone round square; `BodySpacing` stalls (`watch`) re-plan
  and give up the same way, and the solid ground struck (`struckBy`, terrain only) is stepped
  round. Struck bodily under cells (`bumped()`) a unit stops, re-plans and holds that route for
  `bumpInterval`. Occupancy is seeded from `At` + `Mover` at Setup. `BodySpacing`: the
  occupancy is `openOccupancy` (legs are bookkeeping), a unit routes over the ground alone and
  learns of the others by touching them; a group gets its spots from `bodyKeeping.place` (lattice
  round `MoveTo.At`, a box's width apart, far rows first) and goes cell centre to cell centre
  (lanes were tried and dropped at the user's word). Never make a unit see the others ahead: the
  user asked for it to learn by striking. Pushes are collision's and keep units on their ground
  whatever the rules (see collision).
  A `MoveTo{Cell, At, Append}` command orders every `Selected` entity the player owns — or, given
  by an entity for itself (`Order`), that entity alone (`LookAt` too); a
  `plugin.CommandHandler`, its `DefaultBindings()` make a right click one, Shift appends.
  `WithRenderer` builds the `PathRenderer`: for every selected unit its goals (`goals`: not a
  `GivingWay` order's Target, a step aside being no goal) as the entity's outline where it will
  stand (`world.Look.Footprint` on the ground, `Marks` tier, always) and, on `Routes{}` (Shift+P,
  `ShowRoutes`), its routes as thin lines over the ground in pieces of the ground's step at the
  ground's depth (`Overlays`), straight through any camera (sprites were interpolated affinely in
  perspective and wobbled); `RouteStyle` via `WithRouteStyle`. Depends on `board`, `world` and
  `selection` (its `Selected` tag picks whom a command orders). A unit with a tree is told the
  facts `Blocked` (from its touches while on the move, held `blockedLasts` after the last) and
  `Arrived` (its order over, until the next); `MoveOrder.HitUnit` tells a unit struck from the
  ground (entity ids start at 0: never use 0 as "nobody").
- **`world/entity`** — what every entity carries: `Base`, `Position` (`StepReach`, `MaxStep`,
  `MaxSpeed`), `Velocity`, `Z`, `Layers`, and `Eye{Height, Angle}` for one that looks (where
  from and how wide; `Eye.Level(z)`), read by vision's cone and the first-person camera alike. A
  leaf: the world's sub-packages read the components
  from it, and the world re-exports them as type aliases (`world.Base = entity.Base`, …), so
  every other plugin and a game say `world.Base` as before and the component is one type for goke
  and the saves. Nothing outside `plugins/world` needs to import it. Beside it, `world/entity/kind`
  (+ `kind/comp`) is what an entity is (a kind's `Spec`, `Define`, `Of`, the `Registry`) and
  `world/entity/tag` the tag families (`tag.Tags[F]`, `tag.Tag[F]`, `tag.Any`), a leaf `plugin`,
  `plugin/host` and `comp` import.
- **`world/steering`** — `steering.Steering{TurnRate, Reflex, MaxSpeed, Sprint, Accel, Brake, V0,
  Halted}`, the knobs (`Braking`), and `steering.Course{Want, Pending, Delay, Speed, WantSpeed}`,
  the state, given to every unit by the world's roster and by the system where missing; a
  `steering.Helm{*Steering, *Course}` is what steers (`Request(heading)`, `RequestSpeed`,
  `RequestSprint`, `RequestBack`, `Steerable`). `steering.System` (`steering.NewSystem()`),
  registered by the world in every simulation step before movement: turns `Vel.Dir` towards
  `Want` by at most `TurnRate` a tick after `Reflex` ticks, writes `Vel.Value` from the profile;
  `Halted` holds speed at 0 and the heading. `steering.Driven{Ahead, Turn, Face}` is an entity steered by hand
  (the topography's camera system writes it, navigation's `driveSystem` carries it out). Imports
  `world/entity`, not `world`.
- **`world/view`** — `view.View{Bounds, Culled, In}` (`Contains(id)`, `Refresh(space, area)`),
  `view.EntitySet` (a bit set by entity index), `view.New(bounds)` and `view.System`
  (`view.NewSystem(space, &views, w, h)`), registered by the world after movement. The world's
  `Plugin.View/NewView/ViewFor/DropView` hand out `*view.View`; `players.Player.View` is one.
  Imports nothing of the world.
- **`world/clock`** — the tactical clock, made and run by the world (`world.Plugin.Clock()`):
  game time is the sum of the simulation's steps, `clock.State{Time, Tempo, Paused}` on the
  clock's own entity, saved. Space is the tactical pause, ] and [ the tempo (`Config.Tempos`, ½ 1
  2 4; sub-steps of one length, or one longer step with `Config.BiggerStep`); the players carry
  the world's commands always. A plugin's `RunPlan` is the interface part of its tick, once a
  tick; what simulates it hands to `clock.Simulate(c, ctx, d, block)` (at once with a nil clock),
  and the engine's plan wrapper (`stage_runtime.go`) replays every block after the game's
  `Update` as many times as the tempo says (`Clock.Replay`), moving game time on a step each.
  `Clock.Behind` lowers a tempo above 1 after 30 slow frames. `clock.Phase` tags on the clock's
  entity, `Clock.In(phase)`, `Clock.Entity()`; `Reporter()`, `HUD()`. `render.Frame.Time` is the
  clock's `Shown` through `render.Clocked` (the world's renderer): game time past the last tick by
  the real time the engine holds toward the next (`Clock.Pending`, every frame) at the tempo, so
  animations move every frame, not in the ticks' steps.
- **`world/rule`** — how entities behave, one vocabulary (`doc/rule.md`; the package was `act`,
  before it `conduct`): rules, plans, effects, commands, facts. Two constructors, each taking a
  function that writes the steps for a builder (Go 1.27's methods with type parameters):
  `rule.On(name, filter, func(m *rule.Moment[P]) rule.Step)` is a `plugin.Rule` for the host's
  `Hook` (built on `plugin/host`); `rule.Plan(name, func(a *rule.Actor) rule.Step)` is the
  `comp.Template[Mind]` a kind gives its entities (registered by its name hashed, one name one
  plan). Filters: `All`, `Self(tag)`, `Between(a, b)` (pairs, `tag.Any` for either side),
  `Having[T]()`. Moments are `rule.About` (`Who()`), pairs `rule.Met` (`Whom`), standing on places
  of their own `rule.Placed` (a marker, `Placed()`; the host tells the places round in
  `plugin.Tick.Around`: `unit.Standing`, `cell.Now`). A Moment's
  steps: `OneOf`, `Steps`, `If` (on the moment), `Not`, `Apply`, `Keep`, `Dispel`, `Chance`,
  `Unless`, `Under`, `During` (the world's effects), `Order`, `ForOther`, `Here`, `Around` — all
  instant; a lasting step in a rule
  is refused as it is made (`build`), and so are `Here`/`Around` on a moment that is not Placed.
  There is no step running Go code (`Call` was removed on 2026-10-01: it hid what was missing); a
  rule says what it can in steps, and a missing moment, step or knob is added to the plugin. An Actor's: the same plus `When[F](name, func…)` and
  `On[F](name, func…)` (branches on a fact; On latches for one-tick facts), `If[F]` over a fact,
  `Until[F]` (a fact coming afresh, or coming to hold a condition), `Wait`, `Timeout`, `Cooldown`,
  `Idle`, conversation `Ask`/`Agree`/`Refuse`/`Relay` with facts `Asked[W]`/`Replied[W]`,
  delivered a tick later, `Chain` ≤ `MaxChain`, dropped after `AskLife`. `Order(cmd)` gives a
  command for the entity — fire and forget, the handler told `control.Issued{Entity, ByEntity}`;
  an `Aimed` command gets the subject of the fact it stands under or of the rule's moment — and in
  a plan hands back a `Command` whose `.Until[F](…)` or `.Stay()` keeps a reactive branch from
  giving it every tick. One type, `rule.Step`, for both; a function writing part of a rule or a
  plan takes the Moment or the Actor (`func whenBlocked(a *rule.Actor) rule.Step`); ready-made
  hooks are whole, in a plugin's `hooks` package (`chooks.CountContacts(&stats)`,
  `bhooks.LogFalls()`). Plans run in every simulation step after the world's decision systems
  (`rule.Plans`, made by the world). A `clock.Moment` is the clock's own entity's
  (`Moment.Clock`): an effect a clock rule applies lands there, a phase. `Mind{Plan, Running, Slot,
  Since}` holds per-step slots in fixed arrays (`MaxSteps` 128, `Running` a `StepSet`; goke needs
  exported, fixed-size fields). goke registers 128 component types at most — every fact is one.
- **`world/rule/effect`** — states on entities for a while, cast from anywhere, made and
  installed by the world (`world.Plugin.Effects()`): `e.Define(name, Spec{Lasts, Stacking,
  Then(next), Grant(tags...), Alter(func(*T))})` hands back an `effect.Effect` carrying its owner
  (`Cast`/`CastFor`/`Dispel`/`On`/`Mark`; `e.Cast`/`CastFor`/`Dispel`/`Has` the same), `Active`
  slots saved with the entity, originals of altered components kept and saved with the game;
  effects last in game time. Every effect has its own marker of `effect.States`, "effect.<name>",
  on while it runs (`Effect.Mark()`, for `rule.Self` in any plugin; at most 63 effects). `Then`
  casts the next effect when the time is up, not after a `Dispel`; a cast after a `Dispel` in the
  same step takes the slot back. `Active` stays on an entity once an effect came (empty when none
  runs — a marker's second form); the entity's `effect.States` markers (the world gives them to
  every unit, the board to every cell, attached at the first effect where missing) have
  `effect.Changed` on for the step after an `Alter` rewrote one of its components. A cast before
  the effects' pass lands the same step. The world fires the rules of a `clock.Moment{Last, Now}`
  every step (a `host.ListHost`, its own system just before the effects' pass): what happens when
  is a rule of `clock.Moment` holding `m.If(clock.Every(period, offset), …)` or `clock.At(t)`
  (`calendar.Daily/Yearly/Seasonal` give the period and offset), fired by clock time, never again
  after a load; it casts effects or grants a `clock.Phase`.
  `board.Plugin.CellEntity(c)` is a cell's own entity, carrying its `Ground` and `Plot`, so an
  `Alter[cell.Ground]` is a temporary change of terrain.
- **`selection`** — a `Select` command (ids, or a world box, additive or not) → the `Selected`
  tag on `world` entities that carry `Selectable`, both bits of `selection.Family` from
  `Plugin.Tags()` (a kind's choice via `comp.Tagged`); a bit flip, seen
  the same tick. A `plugin.CommandHandler`: its `DefaultBindings()` make a left drag one (Shift adds),
  the left button held a `Marquee` (the box being dragged, drawn by its renderer in the dragging
  camera's view until the `Select` that ends it), an `Apply{Effect}` puts the effect on the player's
  own Selected units (no binding of its own: a game binds its abilities), and F a `Follow` — the third tag, `Followed`, on the one selected unit (none with several; F
  again stops), which the `FollowSystem` keeps in the middle of the camera every tick
  (`camera.Camera.CenterOn` at its altitude) until the player moves the camera by hand; zooming
  keeps it. A `Select` hits and unselects only what the issuing player owns
  (`players/owner.Obeys` over the optional `tag.Tags[owner.Family]`), so one `Selected` tag
  serves every player; `Follow` takes the issuer's one selected unit. Depends on `world` and the
  leaf `players/owner`.
- **`players/owner`** — whose a unit is, a leaf importing only `world/entity/tag` and `control` (as
  `world/entity` is `world`'s): `owner.Family`, `owner.Of(id)` (bit id−1, players 1–64),
  `owner.Name`, `owner.Obeys(owners, by)` — an owned unit obeys its owners alone, an ownerless one
  the virtual player `control.Nobody` alone (the game's code, a script, an AI run as nobody). Read
  by selection, navigation's `moveCommandSystem` and the topography's cameras (`theSelected`):
  a player selects, orders and rides only its own units. A game gives a kind to a player with
  `comp.Tagged(player.Owner())`; a side of its own (the wild, a rival) is `players.Add` owning
  its units; the island's blue walkers are such a rival's.
- **`players`** — whoever acts in the game, over `plugin.CommandHandler`s:
  `players.NewPlugin(world, s.selection, s.nav, ...)` has the world carry each one's `Queues()`
  (the `control.Queue[C]` it drains in its own pass; the world's `control.Carrier` is a stage's
  one carrier, for players and entities alike) and gathers its `DefaultBindings()`; nothing clears
  a queue — a command waits for its handler's pass, given after it for the next frame's (the old
  end-of-frame clear dropped entities' `Despawn`s given in later passes); `Defaults()` is all of
  them plus `CameraBindings()` for players' own `Pan`/`Zoom`. `Local(name)` is a player at the
  keyboard over the world's camera and `View`, `Add(name)` one without (an AI, a client);
  `Issue(player, cmd)` is how any command comes in (`ErrUnknownCommand` for a type no command handler
  defines). The contract — `Queue`, `Issued`, `PlayerID`/`Nobody`, `Binding` (`Trigger`s
  `KeyPress`, `ButtonPress`, `Drag`, `Wheel`, `ButtonHeld`, `CursorAtEdge` with exact `Mods`,
  `Command[C]` built from a `Context` with `World`/`WorldBox` through the camera — a
  `camera.Picker`'s own pick of the ground under the cursor, else `FromScreen`) — lives in
  `control`, and `plugin.CommandHandler` names what defines and carries out commands (one handler
  per command type; events have subscribers, commands a handler), so a plugin with commands never
  imports players. `Viewports(screen)` is what a Scene showing the world returns as its
  `game.Viewer`: a viewport per camera the local players look through, in equal columns; players
  draws nothing, the Scene lists every layer itself. `NewPlugin` registers the owners' family
  (`players/owner`) with the world's kinds, 64 names saved by name; `Player.Owner()` is the
  player's tag. Its `eventHandler` is the layer from input to
  commands: per local player it keeps what it has seen of keys and buttons (`input`), matches
  events against the player's bindings and issues what they build; `Player` holds only who the
  player is.
  `Player.OwnCamera()` (before Use; `world.Plugin.NewCamera`, saved by players) splits the screen
  (`Columns`, `WithLayout`); keys reach every local player, the mouse the one under it in its
  area's pixels; `control.KeyHeld` fires once a tick while its key is down (issued at the end of
  players' RunPlan, for the next tick, so a slow frame still drives every tick). Commands that depend
  on a camera carry the player's (`selection.Select.Camera`, `Follow{Camera}`). `Player.Bind` refuses two on one
  trigger holding in one camera mode (`control.Binding.In`, `camera.ModeOf`; only bindings holding
  in the camera's mode fire and are listed under K), Setup refuses a command nobody defines; `WithRenderer` draws the marquee of a drag.
  The Scene hands input to `players.EventHandler()`; `players.RunPlan` runs last and empties the
  queues. Depends on `world`.
- **`vision`** — narrowed perception: a `Sight` cone scanned against `world`'s
  space each tick fills the entity's `Sighted` (who it can see, nearest first; given by vision
  where missing — `Sight` is a knob vision only reads, `Sight.Ahead` looks the way it moves,
  `Sight.Looking(dir)`), and
  `SightOutline` on an entity gets its view's shape computed and drawn. An entity carrying
  `Transparency` dims sight instead of cutting it, and the world's `Cover` (the board's veiled
  cells) is walked along every ray, dimming or cutting it the same way without being seen, a
  ray spending its radius as a budget through it; whatever the ray reaches is seen, a forest
  looked into as much as a wall. `Sight.Blockers` are the `world.Layers` that cut or dim this
  sight at all (zero: every entity): a hawk with `Blockers` of `Air` looks over walls, forests
  and walkers and still sees them; a walker with `Land` looks under the hawk. In a world with heights
  sight has heights instead: the cone's eye is where the unit's `world.Eye` stands (`Eye.Level`:
  `Height` over `Z.Altitude`, its top for none) and as wide as `Eye.Angle` — the one Eye the
  first-person camera rides at, so the cone is what the rider sees; a Sight without an Eye is not
  scanned — every entity spans its `Z`, the ground is the board's heights sampled every
  `WithGroundStep` (default: a sixteenth of the radius; the renderer drapes the shadows in the
  same step), and a hawk 40 up looks over the wall, the forest and the hill a walker's cone stops
  at; `Blockers` are refused there, `Eye.Height` in a flat world. It
  hosts rules of a `Sighting` inside the scan's own pass: once
  a tick per observer carrying `a` (`rule.Between(a, b)`), with everything in view carrying `b`
  — a directed pair, grouped by observer, empty included. A ready-made hook tells its
  seen entities apart with `seen.Carries(tag)`, and steers only
  through `Sighting.Helm.Request`. Ready-made ones live in the flat `vision/hooks`
  package (`hooks.DefineTags`, `NewFlee(tags).Rule()` for the Skittish, `Chase(tags, every)`
  between Predator and Prey, `LogSightings()`); a file using both plugins' hooks imports them as `chooks`/`vhooks`. A `plugin.CommandHandler`: the views start hidden and
  `Cones{}` (Shift+C) shows every view drawn — cones and shadows — and hides them again
  (`Plugin.Hide`, `Hidden`; the renderer composes nothing while hidden, the scan goes on); a
  look, not saved. Hand the plugin to `players.NewPlugin` for the key. Depends on `world`.

Each package has a `doc.go` describing the gameplay capability it adds.

### Stage / Scene

A `game.Game` also supplies `Props()` (window/tick-rate config, read
once at startup by `gram.Run(g)`; `Resizable` makes the screen the window — the engine's
`Layout` follows it and hands the active world's camera `SetViewport`, whose zoom floor scales a
world smaller than the window up to cover it; F11 toggles fullscreen in every game,
`Runtime.ToggleFullscreen`; `TargetTPS` is the engine's own fixed
step — the window's loop (gogpu, paced by the swapchain) runs one `Update` per frame, stepping as
many times as the time gone says, and a frame that falls behind runs at most 5 steps and drops the
rest, so the game slows down instead of spiralling; what is drawn goes by `clock.Clock.Shown`, the
game time run on past the last step by the time held toward the next) alongside a named collection of
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

`internal/engine.Engine` (aliased `gram.Engine`) owns the window's
loop (gogpu; `Run` opens it, every frame `Update`, `Draw` into an image the size of the screen, then
`Present`) and drives a user's `game.Game` one active `Stage` at a time — see
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

`doc/roadmap.md` lists what is left to do, and only that: take an item out when it lands.
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
