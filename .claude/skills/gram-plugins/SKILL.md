---
name: gram-plugins
description: Reference of gram's built-in plugins, one entry each with its current API and rules — world, board (cells, units, entities on the board, bridges, maps as files, gramap), topography, atmosphere, collision, navigation, entity, steering, view, clock, selection, dialog, players and owner, cameras, driving, gravity, vision, bridges, bullet. Use when working in or with a specific plugin, to learn what it keeps, hosts, handles and depends on before reading its code.
---

# gram-plugins

Every plugin's `doc.go` ends with `# Vocabulary`: its moments (with their words and subjects),
facts for plans, knobs and commands (who may give each). That section is the authoritative list;
`go doc ./plugins/<name> | tail` shows it. The entries below are the prose round it.

- **`world`** — the foundation a Stage makes from `world.Config` (`stage.World(cfg)`;
  `ctx.UseWorld(cfg)` underneath, once): the space's size and edges (`SpaceCfg.Edges`:
  `aabbworld.Torus`, `WrapX`/`WrapY`, `OpenX`/`OpenY`, closed by default — a box stops whole at a
  closed edge, wraps at a wrapping one, may leave by an open one: wholly past it the entity
  carries `world.Outside`, rules of a `world.Leaving` hear of it every tick, with none it is
  despawned), entity bounds, `Heights`, `Scale{Metres, Figures}` (what a world unit is;
  `Scale.Units` metres; `Figures` how many times bigger than life the figures are drawn,
  `Scale.Figure(m)` their boxes, heights and eyes; speeds and sight stay real; with it the ground
  sinks under an eye's level, `Scale.Drop`, and the air hazes far off), `Seed` (which `Chance`
  draws from). `world.Roster()` is what the plugins ask of a unit's kind, gathered as they are
  made: `kind.Require[T](&roster.Unit, by, why)` names what the game must supply (world:
  `Position`; board: `At`, `Mover`; navigation: `steering.Steering`), `roster.Unit.Default(
  comp.Const(v))` what a plugin brings itself (`Velocity{}`, `Collider{}`, `Physics{}`; dropped
  with `comp.Without[T]()`); `roster.Unit.Spec(own...)` builds a unit's Spec, a missing
  requirement panics by plugin and reason; `roster.Cell` is what every cell carries besides the
  board's own. `world.Layers` are the planes an entity is on (none: every plane); collision and
  vision read it. `world.Config{Heights: true}` gives the world heights: entities carry
  `world.Z{Altitude, Height, Step}` (`Z.Reach()`: how high it steps up), written by the board in
  relief or by gravity; collision and sight follow Z; a flat world asks none of it, and each
  plugin refuses the other mode's facts where it first meets them. `Base` is the one component
  every entity carries (`Position`, `Velocity`, `TypeID`, `Caps`). Entities are spawned before
  the game (`Seed`/`Populate`) and during it (`world.Spawn{Entry}`, `Plugin.Spawn`: the world's
  own system makes the entity at the next step after the plans, refusing with a log line an
  unknown kind, a wrong row, a full world, a bad size, a box past an open edge) and go by
  `world.Despawn{}` (an entity's own, `act.Do`). Movement is capped per entity at half its
  shorter side (`world.StepReach`, `Position.MaxStep`/`MaxSpeed`); the move pass moves every box
  by its `Velocity` and `world.Drift` (gravity's) under the edge rules, then rebuilds the space
  (`Space.Rebuild`): `Query`, `Scan` and collisions read that grid until the next tick. After
  movement the `view.System` refreshes every `view.View` (`Plugin.View()` the camera's,
  `ViewFor(cam)` any's); the renderer draws what the View contains. The world makes no camera.
  It hosts rules of a `Moving` (every entity, in the velocity pass, which also scales
  `Base.Vel.Value` by the entity's `steering.Pace`, the board's share; its words `world.Still()`,
  neither walking nor strafing, and `world.Braking()`, slowing down — `Moving` carries the
  steering's `Course`), a `Leaving` and a `clock.Moment` (every step, just
  before the effects' pass). It handles `steering.Away{From}`, `Toward{To}` (`host.Aimed`),
  `Turn{Angle}` (the steering system asks the `Helm`, one a step), the commands about effects,
  `Pause` (Shift+P), `Faster` (]) and `Slower` ([). `Figures(f...)` notes kinds of entity in
  one piece (`world.Figure`: `Define(w)`, `Look(a, w)`; `Stateful`, `Ruled`), `FiguresIn(part)`
  runs their parts as the Stage reaches Effects, Rules and Kinds; `ctx.Looks` declares their
  looks on the atlas once the Stage is set up. `Picture()` is the Stage's one picture: every
  source its plugins gave `ctx.Draws`.
  Every `host.Tick` comes from
  `world.Plugin.Tick(d)`: the carrier (`world.Plugin.Carrier()`, the stage's one
  `command.Carrier` for players and entities alike; the engine carries every
  `plugin.CommandHandler` a stage uses), `Time`, `Seed`, `Roles`, `World` (the clock's entity).
- **`board`** — optional grid and terrain over `world`, standing only in a world with heights
  (`board.NewPlugin` panics in a flat one; a world without a board — collision, minimal,
  rules/vision, scenes — stays flat); its grids wrap per axis, following the world's `Edges`. The
  root keeps the `Plugin`, the `Board` (`Plugin.Board()`: `At(x, y)`, `Kind`, `KindAt`, `KindFor`,
  `Bare`, `Set`, `SetAll`, `Way`, `SetWay`, `Along`, `DecksOver`, `Reaches`, `Fenced`, `Lane`,
  `CellFor`, `LevelFor`, `CellVersion`, `Changes`, `Version`, `Touch`,
  `Shape`, `Built`, `States`), the `Layout`, the `Map` contract, `NewUnits`, `NewEntities`, `Lay`
  (`level.go`), `Spawn[R]`, `Each[R]`, `Sides`, `Shape`. Public subpackages: **`board/cell`**
  (`cell.ID`, `cell.Kind`, `cell.Kinds` through `board.Plugin.CellKinds()`, `cell.Domain` —
  `Land`, `Water`, `Air`, a game's own bits — `cell.Plot`, `cell.Ground`, `cell.Way`/`Links`,
  `cell.On`, `Floor`, `Wall`, `Rail`, `Built`, the moment `cell.Now`, `cell.Entry`, `WayEntry`,
  `cell.Terrain`, `TerrainMap`, `cell.Occupancy`), **`board/unit`** (`unit.At`, `unit.Mover`,
  `unit.DomainAt`, the moment `unit.Standing`, the markers `unit.Entered`, `unit.Airborne`),
  **`board/grid`** (`grid.Grid` — every grid knows its own directions: `Directions`, `Opposite`,
  `Offset`, `Step`, `Corner`, `Flanks`, `Toward`; nobody else writes a table, a bit or a vector
  of them; bit i of a `cell.Links` is direction i — `grid.DefaultGrids`, `Link`, `Way`,
  `Straight`, `Direction`, `Nearest`, `Exit`, `Rim`, `Shape`/`ShapeOf`), **`board/look`**
  (`look.Look`, `Dressing`, `EvenLit`, `Parallel`, `Tile`, `FlatLook`, `Nothing`, `Map`,
  `Renderer`/`NewRenderer`, `RenderState`, `MinGridCell`), **`board/ground`** (`ground.Heights`,
  `Profile`, `Cover`, `Readied`, `Surfaces`, `MeanOfCells`, `Shaper`, `RoadFits`, `RoadJoins`,
  `RoadSlope`), **`board/water`** (brooks, streams, rivers and fords out of a relief:
  `water.Drain`, `Network.Carved`) and **`board/network`** (`Network.Net`, `Route`, `Path`,
  `Ways()`). The machinery is `board/internal`: `terrain` (the cells' state, `Cells`, the entity
  system, the kinds' dictionary, `built.go`), `moments` (`Rules`: the hosts of `unit.Standing` and
  `cell.Now`, their passes, `Around`), `field` (the ground's cover and solid field), `grids`,
  `draw` (the simple map's bands and covers), `occupancy`, `boardtest`.
  A `cell.Kind` says which `cell.Domain`s it admits, whether it is `Solid`, how much it `Veil`s
  sight and whom it `Veils`, what it costs (`Costing(domain, cost)`, `CostFor(domain)`, paid in
  the planner and in the Moving rule — only entities carrying `Mover` are slowed), whether it is
  `Graded` (spared the slope), its `Height` (with heights), `Shine` (glint per pixel), `Flow`,
  `Spread` (blending between kinds). Slopes cost through the board's Map (`relief.Climbing{Up,
  Down, Ease, Steep, Free}`, `Map.Slope`/`Climb`/`Least`), so steep is the relief, never a kind.
  A `cell.Way` is a second layer on every cell entity, a band through the cell's middle whose kind
  decides who may cross it (`Way.Over`), drawn by `Tile.DrawWay`; ways curve round the cell's
  middle, a way's `Fade` has it fade out (a river into the sea). Every cell is an entity for good,
  without a `world.Base`: `Plot`, `Ground` (kept apart so an effect ending restores the kind
  alone), `Way`, `Built`, made at Setup out of the seed or found after a load; `Version` and
  `CellVersion` count every change (writes, effects on cells, `Board.Touch(c)`). A cell plays the
  roles of its kind and its entry's own and is called by its entry's `Name` and `Group` (such
  cells are made apart). What the board does to its terrain over time is effects on the cells'
  entities (`Alter` of `cell.Ground`, `cell.Way`); a state of the ground is an adjective, the
  kind the noun; what lies on it is a **cover** (`board.Plugin.Covering(e)`, `effect.Shows`);
  `Board.States(c)` a cell's effect markers, `unit.Standing.States` those under a unit,
  `unit.StandingOn(e)` the word.
  The board is flat; the ground's heights are the topography's relief (`topography.Relief`). The
  `altitudeSystem` writes every `Z.Altitude` each step from
  `Relief.Surface` (the board's `ground.Surfaces`: the highest of the ground and a `cell.Floor`'s
  deck carrying its domain within its step of its feet; the ceiling a slab's underside) plus its
  `Lift` — except one carrying a `world.Drift`, which gravity carries.
  Every tick, after collision's `RunPlan`, `board.RunPlan` reports a `Standing` (cell under the
  centre, its kind on the unit's level, the box, the domain, `Airborne`, `Feet`, `Level`,
  `Reach`) to its rules: `unit.Fallen()` a land unit fallen into water or a hole (never in the
  air nor on a deck), `unit.OnKind(k)`; the reaction is the game's (`act.Self.Do(world.Despawn{})`;
  `board.Plugin.WithLog(log.Default())` writes a line each fall). The same pass writes every
  unit's `steering.Pace{Share}`. Then, while there is a rule of it, every cell as a `cell.Now`.
  Both moments are `host.Placed`: `cond.Around(n).If(x)`/`act.Around(n).Do(cmd)` turn to the
  cells within `n` rings of the box (0: the cells under it) and the entities standing on them
  (`cell.On`), each cell once; a `Standing` is `host.Headed` too (`Heading`, its `Vel.Dir`):
  `Ahead(n)` turns to the cells one to `n` steps in front of the unit (`grid.Nearest` +
  `Toward`, stopping at the edge), one kind of places a rule.
  **What stands on the board is an entity**: `board.NewEntities[Row](brd, board.Shape{Size,
  Width, Height}, at)`, `Define(name, extra...)` — a box at the cell's middle, `world.Velocity{}`
  (one among the extra takes its place, its `Dir` the way a ford runs), `cell.On{Cell}`, with
  heights a `world.Z{Height}`, a `world.Lie` for a band; the game gives the rest (a plate's
  `Collider{}` without `Physics`, a sensor; the roles). Its knobs: `cell.Floor{Allows, Height,
  Thickness, Cost, Graded, Links, Width}` (carries the domains over whatever the ground takes: a
  trapdoor over a pit, a ford at its `Cost`, a bridge `Graded` at its deck's `Height`; with a
  `Width` a **band** from the cell's middle out to each neighbour its `Links` name; with `Height`
  the cell has two levels and where a unit is is its heights' — it steps onto a deck within its
  step from any side, passes under one it fits under, strikes the slab of one it does not, falls
  off its edge under gravity), `cell.Rail{Keeps, Height}` (a railing along both edges of the band;
  a fence is a band at the ground's level with a Rail: slanted walls in collision's field, a
  barrier for what drives, a leap over its top clears it; the planner never steps across one,
  `Board.Fenced`) and `cell.Wall{Keeps, Height}` (a gate). The `builtSystem` gathers them every
  step on the cell's entity as `cell.Built` (domains joined, the highest Height, the bands' Links
  joined; `Built.Over` adds the Floor's domains and takes the Wall's off in `Board.Kind` and
  `Bare`, so the planner, the driving, `unit.Fallen` and collision's field see them). An effect
  opens one (`Alter` a `Floor`'s `Allows` or a `Wall`'s `Keeps` to 0). **A bridge** — the cells
  whose `Graded` bands join along their links — runs straight and level from a **head** to a
  head (a head's band runs from the middle of its flat tile into the bridge alone, no railings
  on it), its deck `Clearance` over the ground under every cell between (`board.Plugin.WithBridges(
  board.Bridges{Clearance})`): one breaking the rules panics as a fresh board is first gathered
  (the file and line of its entity in a map), one broken later is not built, said in the log.
  `Board.DecksOver(p, visit)` gives every deck over a point, `Board.KindFor(c, p, feet, reach)` the
  kind there, `Board.CellFor`/`LevelFor` the cell and level whoever stands there is on.
  `board.NewUnits[Row](brd, board.Shape{Size, Width, Height}, at)` defines a game's units:
  `units.Define(name, unit.Mover{…}, steering, extra...)` derives `Position` and `At` from the one
  point `at` reads off a row, `Layers` from the domain, with heights a `Z{Height}`, runs the
  world's roster and `kind.Define`; `units.Named(name)` is the `kind.Of[Row]`; a Shape bigger
  than the grid's `CellSpan` (a square's side, a hex's edge) panics: the planner's clearance
  rests on it. A unit's `Mover`
  says which domains it moves in and, with heights, `Lift`, `Clearance`, `Ceiling`.
  `Figures(f...)` and `Tiles(t...)` note figures standing on the board (`board.Figure`:
  `Define(b)`, `Look(a, w)`) and kinds of cell (`board.Tile`: `Define(b)`, `Look(a, b)`, covers
  included), `Stateful`/`Ruled` with their own effects and rules, among the world's figures, so
  the Stage leads them through its sections; the renderers gather their looks when first asked;
  `Plugin.World()` is the world under the board. The occupancy
  (`internal/occupancy`, `KeepSystem` after the units' pass) lets go of whoever left the world
  and has every unit hold the cell it stands on, each on its level (a walker on a deck and one
  under it share a cell); one holding nothing lets go of the cell it was `At`, is `At` the one it
  stands on with `unit.Entered` on; one mid-step holds both and is left to navigation.
  `board.Holds` (Shift+O, `Plugin.ShowHolds`) outlines every cell held, lying 0.15 over it.
  **A map is a file**: `board.Plugin.Lay(fsys, path)` in Spawn seeds the board and hands back a
  `board.Level`; `board.Spawn[R](level, board.Sides)` seeds the world with the entities whose
  kind's rows are `R` (`R`'s fields tagged `yaml` are the file's keys), one no Spawn took fails
  Populate with its line. The file: `board` (`default`, a `legend` — a character a kind, or
  `{kind, plays, name, group}` — the `cells` a character a cell, a hex row indented), `entities`
  standing on it (a line each: `kind`, `at`, `name`, `group`, the row's keys), `relief` (`cells` or
  `corners`, metres through `world.Scale`; `Level.Heights()` for `topography.Seed`), `ways`
  (`roads` by kind, `width`, `paths` of points routed point to point, `network.Route`; `rivers` a
  list of paths, each flowing down in the order given to its last point, a point `{at, kind,
  width}` turning its kind and width; a road never runs over a river: `{onto: [x, y]}` leads it
  onto the bridge or ford there; with a relief every cell of a `Graded` road lies level or on an
  even ramp of `ground.RoadSlope` straight up, a diagonal, a bend, a fork, an end level,
  `ground.RoadFits`, refused with the path's line; the shaping `relief.KeepRoads` takes back
  what would break a road), `units` (flat entries: `kind`, `side`, `name`, `group`, the row's
  keys; a `cell.ID` written `[x, y]`). Every error says the file and the line. Madeira's maps
  (204 x 106 cells of 333 m, a map a demo) are made by **`cmd/gramap`** out of a recipe
  (`cmd/gramap/madeira.yaml`): heights from the Terrain Tiles on AWS (cached under the user's
  cache; the README credits the sources), the ground by slope and height, rivers by `water.Drain`
  + `Carved`, roads routed with the road's height in the search (no tunnels: a leg past `deep` m of
  cut or fill is left out and named), bridges planned by `plugins/bridges/site` as a game plans a
  player's, fords where a river is shallow, the grading (`grade`) joining corners into level
  classes, the map laid on a board and run a step (`check`) before it is written, the old file's
  `units:` kept.
  Terrain is never an entity in the space: the board's field (`internal/field`) is collision's
  `collision.Field` (`Solid`, `Overhang`), handed over by `Plugin.WithCollision`, and sight's
  `ground.Cover` (`Plugin.Cover`); both read the cell entities whenever asked. A flat board takes
  its light by the hour through `board.Plugin.WithAtmosphere(atmosphere)` (`lit.go`, reading
  `board.Atmosphere{Sun, Air}`: the board composed once and tinted on the GPU, sprites lit, what
  sways leaning, the clouds' shadows laid on its ground right after it, `draw.Clouds`), called in a
  scene before the first frame; without one it is drawn as it is. The
  board's own Map is the simple map — flat, every kind in its sprite from the game's atlas
  (declared by `ctx.Looks` from the tiles, the sprites the cell's bounds across), the ways as bands
  cut out of their kinds' sprites, a step at its kind's cost; `plugins/topography` is the other.
  Depends on `world` and `collision`.
- **`topography`** — a map in relief: `topography.NewPlugin(world, board, Config{Cell, TileW,
  TileH, HeightUnit, Headroom, Shaping, Climbing, Perspective, Solid, Fine, EdgeTurn, EdgeTilt})`,
  made right after the world and the board (so a world with heights); refuses a wrapping world. It
  hands its cameras as makers (`Plugin.Views(start)`, a `cameras.Maker`; each camera begins as
  `topography.Start` says — `FromAbove`, `Isometrically`), sets the world's Look
  (`billboards.Look`), the board's Map (`look.Nothing`: the ground is drawn on the GPU) and the
  world's Ground (its `Relief`: `At`, `Step`, `Altitude`, `SetHeights`, `Corners`, `Lift`,
  `Flatten`; on a square grid a lattice of corners the neighbouring cells share, on a hex a level
  per cell; living on its own entity as `Heights`, runs of `HeightsRun` 1024, seeded by
  `Plugin.Seed(heights)` at Populate, shaped by `Raise`/`Lower`/`Level` — Shift+=, Shift+-,
  L-drag — `Shaping`). The plugin draws the ground, a `render.Direct` in the opaque pass (`ground`
  or `hexes`, the one it has, given `ctx.Draws`): over a square grid `internal/terrain` (the lattice
  as a mesh, a depth prepass, the board painted flat by the painter into albedo and water sheets
  sampled per pixel, lit by the sun with shadows baked on the GPU as the sun moves — a strip a
  frame, `shadeStrips`; H `CoarseShadows` bakes them half as fine; `WithShadows` on by default — the
  clouds' cover baked, water on wet cells, the grid, fog, a skirt of level ground to the horizon),
  over a hex grid `internal/hexes` (every cell a prism from tiles composed once from above). The
  entities are billboards on the GPU (`internal/billboards`, standing upright in the world across
  the screen's level right, so as tall on the screen as their `Height` stands there and inside
  their solid (Shift+X), shorter the steeper the view; tested against the ground's depth, their
  shadows `sky.Sun.ShadowOf` patches draped over the terrain); a bridge's deck and piers are
  `world.Block`s whose shadows the terrain bakes with its own (`maxBlocks` 96; a deck stands on
  its beam down its `Z.Height`). One camera, three views (`internal/cameras`): from above (screen
  x, y the world's, no sorting), isometric (turning by any angle, Q/E `Turn`, R/F `Tilt`, the
  pitch 10°–90°) and, given `Config.Perspective`, the perspective; `View{Camera}` (Tab) goes
  round them keeping the ground point in the middle. From above and isometrically the screen stays
  over the world at sea level, a pan stops at the edge (in relief the part stopped turns or bows
  the view; drawn back the view lies down again as far as the edge bowed it, `viewCamera.bowed`;
  the cursor in a corner of the window turns the view a quarter as fast as Q and E, left corners
  as Q, right as E, `viewCamera.Scroll(dx, dy, corner)`, `Config.CornerTurn`; the user's word
  2026-10-14), zooming out stops where the screen fits the map, `ZoomIn` keeps the ground under the
  cursor; the perspective keeps the ground point in the middle, flies no higher than shows the
  world's diagonal (`maxAlt`). A pick (`camera.Picker.Pick`) finds a deck a line of sight passes
  under (`Config.Solid`, `Fine`). The cameras' `camera.Rays` give `camera.SceneTransform` for every
  GPU source. **The fastening is the camera's** (`camera.Fastening{Entity, How}`, `camera.How`
  `Loose`/`Centred`/`Behind`/`Inside`, `Outside` all but `Inside`, `HowOf(cam)`): the cameras
  plugin keeps `Centred`, the topography's camera system `Behind` (centred, turned until the
  unit's `Vel.Dir` runs up the screen) and `Inside` (first person: the eye where the unit's
  `world.Eye` stands, the screen as wide as `Eye.Angle`, the mouse always looks — `Driven.Look`,
  the cursor captured by players; up and down it writes `Eye.Tilt` (−pitch), the unit's own
  given back on letting go — and a flyer rides: `Driven.Flown`, `Climb`); `Ride{Camera}`
  (V) goes `Centred` → `Behind` → `Inside` → `Centred` over the entity the camera is fastened
  to (`cameras.Follow`, C through `selection.FollowKey`). Bindings `.In(camera.Inside)` fire
  riding (the driving's `Shooter` WSAD, Space `Jump`, the mouse, V, Tab), `.In(camera.Outside)`
  the free camera's (Q/E, R/F, Tab, the cameras' WASD pan, Shift+W), the arrows `.In(camera.Behind)`
  drive the followed unit. Commands carry `control.Context.Camera`, so the plugin knows neither
  players nor selection; cameras are no entities. Its parts are packages none of which imports the
  plugin: `relief`, `painter` (the board's Dressing in relief, the sheets, `Style`), `water` (the
  materials: the sea five octaves of value noise along x, roughened by the wind, a swell within
  `shoreReach` of the shore, Fresnel against the way to the eye; `stream.wgsl` a flow map),
  `terrain`, `hexes`, `billboards`, `cameras`, `internal/vec`. selection must not import
  topography, even in tests. Everything that is the view alone — the projection, the camera, the
  billboard, the blocks' shading — is private; `camera` has the contract and `TopDown`,
  `internal/camera` the plain top-down camera. `world.Z`, relief and `Heights` are the world's
  heights, not the view: sight reads them in a top-down game too. **Layering: the world knows its
  entities and nothing else; the ground is the board's, the sky the atmosphere's, `render`
  generic.** A relief is lit by the sun of its `topography.Atmosphere` (`WithAtmosphere`; without
  one `sky.DefaultSun`, which the topography hands the frame itself). Its water reflects the
  sky's own colours (`skyAt`).
- **`atmosphere`** — the time of day, the climate and the weather on the world's clock, over a
  world with heights alone (`NewPlugin` panics in a flat one); its root
  keeps no shaders and draws nothing itself: every part is a package that never imports it.
  `atmosphere/calendar`: the clock at a fixed scale — a day every `Config.DayRealTime` from
  `Start` in the middle of `Season`, a `GameYear` of 8 days and a 4-day moon or an `EarthYear`;
  `Calendar.Now()` a `Moment{Date, Time, Year}` (`OfYear`, `Season`, `Moon`, `Hour`, `Written`),
  `Daily`/`Yearly`/`Seasonal` a schedule's period and offset. `atmosphere/sky`: the world's sun
  once a tick to `Config.LightAt` the moment, or at `Config.Steps` a day — the path for the
  climate's latitude (`NoonWay` `celestial.South` or the default `NorthWest`, noon beyond the
  isometric view's sea), below the horizon the moon (`sky.Moon{Color, Strength, Face}`, a knob on
  the atmosphere's entity: board-topography's blood moon by a rule of `sky.Moonrise` or M;
  `Sky.SetMoon`; `Config.Moon` the one it begins with, `Sky.Moonlight`), the colours blended from
  `Config.Daylight` (rows of `sky.Daylight`; `EarthDaylight()` when none) by the sun's height,
  the sun's own colour at its height on `Heavens.SunColor`; `sky.Sun`
  (`Dir`, `Strength`, `Ambient`, colours; `Sun.Light`/`Shaded`, `Sun.Frame`, `Sun.ShadowOf`);
  `Freeze` (P) freezes the light alone, `Later`/`Earlier` (Shift+], Shift+[) move it half an hour,
  not saved. `atmosphere/celestial` (imported by `sky`, never the other way): `Heavens`
  (`Place{Latitude, NoonWay}.HeavensAt`), the sphere turned by the sidereal time, `SunPath`,
  `MoonAt`, `Phase`, `Stars()` (the Yale Bright Star Catalogue to V 6.0 in `stars.bin`;
  `StarField.Place` takes a catalogue), `MoonFace()` (NASA SVS CGI Moon Kit).
  `atmosphere/backdrop`: the viewport in the sky's colour behind the world (`render.Furthest`,
  `Image.FillBehind`: only where nothing was drawn), through a perspective the sky of the day in
  four parts, a `backdrop.Dome` (2026-10-16): the vault, the sun, the moon (`Size`, `Color`,
  `ByDay`, `Image`), the stars (`Catalogue` drawn as instances behind the sky, `Dots` its shader
  adds); `Earth()` part by part (`EarthVault`/`Sun`/`Moon`/`Stars`, `ScatteredStars` the dots);
  a part is WGSL to `shaders/sky.wgsl` (`SkyVault(s: SkyAt)`, `SkyStars`, `SkyMoon(s, d: SkyDisc)
  -> SkyShine`, `SkySun`) with `Reads` its uniforms and `Feed(v *Values)` setting them; without
  WGSL it draws nothing (a stub stands in). `Renderer.Dress` assembles the shader as the Stage's
  looks are declared, panicking on a uniform or a module-scope name two parts (or the library,
  `render.SharedUniforms`) declare; compiled at the first draw, cached by source. The clouds'
  layer stays the sky's own. A game's sky: `atmosphere.Sky{Look(d *backdrop.Dome, p)}` in its
  `sky.go`, noted in the Figures link (`a.atmosphere.Skies(dusk{})`), each Look in order, a later
  part replacing an earlier; none is the Earth's (examples/look/sky; board/flat and
  board/topography note `earth{}`). naga v0.19 (v0.19.1 too) traps for a part: a `var` without
  an initialiser is undefined (start from `SkyShine()`), a module-scope vector `const` reads as
  zero in a runtime expression (spell vectors inside the function). `atmosphere/climate`: a
  `climate.Zone{Latitude, Factors}` (`Equatorial`, `Tropical`, `Mediterranean`, `Temperate`,
  `Cold`, `Polar`; `SeaCurrent`, `DrySummer`) is `Zone.Profile()`
  (`Mean`, `Year`, `Day`, `Wet` per season); the kinds of weather `climate/weather`
  (`weather.State`, `weather.Default`: clear, fair, cloudy, rain, storm); the weather now a
  `climate.Weather` on its own entity (saved with its dice), beginning in `Config.Start` or a state
  thrown by `Often[season]`, going from one of `Config.Weathers` to the next, blending the wind, the
  clouds, what falls and the `Temperature` (snow below 1°C), every step of the simulation under
  `clock.Simulate`; `Climate.Air()` an `air.Weather`. It hosts rules of `Weathering`
  (`host.StepRules`, about the atmosphere's own entity). `Change` (Shift+W), `Set{Name}`. The
  weather is born at the horizon and carried by the wind (2026-10-16, stage 7): `air.Dome` holds
  the `air.Mass`es (cover, billow, falls, damp) over the world and its sky out to
  `Config.Horizon` metres past it (0: 3 × `air.ClearAir`, 120 km), level 0 over the world in
  cells of `air.BoardCell` (128 world units), each further level in cells ≥ 2× (rect ±23 cells,
  4×4 patches), at most 6, every level a ring of `air.Patch` (16×16 cells, `[256]Mass`,
  `[256]int16` states, `Version`; fixed size, no pointers: **a component never holds a pointer,
  slice or map, and nothing of the weather is saved but by goke** — the user's rule). The
  climate's system creates the patches in `Init` (`Dome.Slots()`; from a save it holds those
  that fit, removes the rest, lays anew when any are missing) and `Hold`s them every step (their
  memory may move), `Carry(drift, born, fresh)`: cells entering the coarsest level take the mass
  born there, finer levels' entering cells the coarser level's; fresh, every cell takes `born`.
  `climate.Weather.Lanes[0]` is the weather the horizon brings (`Patchy`: lanes across the wind,
  `ClearAir/2` wide, ≤ `MaxLanes`, blended between their middles, each its own chain, a
  neighbour's state twice as likely); `Over` is the state over the world's middle (the panel,
  temperature, `Rain`/`Snow` there). Shift+W/`Set`: lane 0 jumps to its targets and the dome
  over the world and upwind of it (a ramp of one cell past its far side downwind) settles with
  a time constant of 2 s of game time (`setting`) for 5 of them, a quarter of the patches a step
  (`setParts`), all of them the first step; the user asked for the change to show within
  seconds (2026-10-16). Without wind the whole dome settles to the targets over Blend, 1/15 of
  the patches a step (`Dome.Settle(to, share, part, parts)`).
  `Left = max(Lasts, crossing)` so a weather lasts till the wind carried it across.
  The front takes distance/wind to come: over Madeira about an hour of game time at a wind of
  5–8, from a dome edge ~20000 units upwind (a headless sim of the climate showed it).
  `air.Weather.At(x, y)` the mass there (nil Dome: uniform from the globals),
  `Most()` the most anywhere (gates `U.Cover`/`U.Billow`), `FallsAt` rain and snow under the
  clouds (none from their gaps), `Visibility(x, y)` from the mass (field `Sight` = clear air),
  `air.Under(cam)` the eye's point or the view's middle (an iso camera is `Eyed` but has no eye),
  `Hazer` the haze worked out once a frame. WGSL `airAt(p)` reads the frame's image `AirMasses`
  (`air.DomeImage`, held by the sky's renderer, which sets it in Compose; a block of pixels a
  level, its kept cells in order from the first, not the ring, so the GPU's filter blends them:
  a level is copied out of the world's memory and written whole as its window moves or a patch
  of it changes, drawn in place by gpu, see gram-render): `U.AirLevelNA` (the block's first
  cell in the world, cell, block across the picture) and `U.AirLevelNC` (the rect read in), the
  finest level holding p, mixed with the next over its 2-cell band, one `textureSampleLevel`
  with `wrapped` a level, as `Dome.At` does on the CPU (a test keeps them within 2.5/255, also
  after the windows moved). Four `textureLoad`s a level with the ring's arithmetic cost the
  terrain, reading it every pixel, ~15% of topography's frame rate. Every cloud function takes
  the mass. `atmosphere/aloft`: the clouds seen from above (`aloft.Renderer`, a Direct among
  what is seen through at the layer's height): one full-screen pass, the pixel's line of sight
  from `camera.Rays()` (lines all one way — iso, top-down — start anywhere along them, so `t`
  may be negative: their origin lies over the highest ground, under the clouds), five slices
  from `air.Base` up 0.4 `cloudSize`, a higher one only where `cloudThick` reaches it, composed
  from the top, lit like `skyClouds`; `airAt` once a pixel at the layer's middle; worked out at
  a quarter of the screen's resolution each way into an image of its own and laid over enlarged,
  linear (full resolution cost ~4 ms of a UHD 620's frame, a quarter ~0.4 ms, looking alike);
  none while
  hidden, under no cover, or from a perspective eye under the layer (the sky draws them).
  `atmosphere.ShowClouds(bool)` from the scene (every demo with an atmosphere shows them),
  `Clouds` (Shift+V) switches: bodies in both views — from under them the sky draws heaps
  (`backdrop.WithBodies`, `shaders/cumulus.wgsl`: 24 steps through the layer 0.5 `cloudSize`
  deep, top `sqrt(thick)`, at a quarter resolution each way into image2, which the sky shader
  lays over in place of the flat clouds; as cheap as the flat layer it replaces), off: a flat
  layer on the sky and shadows alone from above. look/sky runs no clouds (`Running` without
  `Clouds`). The clouds' tile (`air.Tile`) keeps a pixel of wrap round every level: reading it
  clamped half a pixel in left a seam where the heaps' cores rise steeply.
  `atmosphere/precipitation`: what falls
  (screen-space streaks and flakes, before the whole world, `render.Nearest`), only under the
  clouds: an 8×6 mask of `FallsAt` at the screen's points thrown to the ground (and over the eye),
  worked out every 8 frames a camera, a drop kept where its share is under the mask.
  `atmosphere/weathering`: what the weather does to a board — `weathering.Config{Snowy, Ice,
  Water, Sway, Swaying, High, Seed}` names the game's kinds, `New` defines three effects
  (`Alter[cell.Ground]`: snow, ice, sway), laid by its own system once a second of game time (snow
  in drifts where it snows in the frost, `FallsAt` on the cell, melting once warm, ice from the
  shore below −3°C, what sways swaying above a wind of 15);
  `atmosphere.Plugin.WithWeathering(board, cfg)` as the atmosphere is made.
  `atmosphere/air`: the weather as drawn (`Weather.Frame`, `Sway`,
  `Cloud`, `Shade`, `Haze`; the clouds' periodic noise baked once into `air.Tile()`, the frame's
  image `CloudTile` (2026-10-16), which the sky and the terrain look up per pixel
  (`cloudTileAt`); `Weather.Frame` hands the frame the tile, `Fog`, the sky at the horizon, and
  `SkyOverhead`). The plugin draws the sky and what falls itself (`ctx.Draws`); its
  sky hands every frame the sun and the weather; the clouds' shadows are drawn by whoever draws
  the ground (the board's flat ground, the terrain, the hexes). `atmosphere.Running`
  (`Config.Running`, `Plugin.SetRunning`) switches the day, the weather's changes, the wind, the
  clouds, what falls, the weathering, the stars and the moon, all on by default, not saved.
- **`collision`** — collision detection over `world`'s space, one `collisionSystem` a tick. An
  entity collides exactly while it carries `Collider`; the system settles every `Collider`'s
  `Base.Caps` (`CanCollide`, `Static` for an immovable `Physics`, `Sensor` for none) and rebuilds
  the space when any changed; two colliders touch only where their `world.Layers` meet and, with
  heights, where their `collision.Band`s overlap. The tick is one `collide.Engine.Tick`
  (`github.com/kjkrol/aabbworld/collide`; the system is its `Handler`): the engine pairs whoever
  may touch within a step, tests the pairs, pushes the overlapping apart and reports each pushed
  box, written back to `Base.Pos`; one pushed out through an open edge is marked `world.Outside`.
  Overlapping boxes touch: the box is the shape. **A pair parted stays met while it touches**:
  the pairs of the tick before that the engine did not push but whose boxes lie within `restGap`,
  layers and bands meeting, neither swept, get a `Contact` with `Resting` on both sides, no
  impulse, and a `Meeting`; a resting contact is no strike and no count. An entity carrying
  `Physics` (`Mass`, `Restitution`) is pushed out and bounces (an infinite `Mass` is a wall); one
  without is only detected (a trigger). With a `collision.Field` (the board's solid cells,
  `Config.Field`) the engine pushes every movable collider out of the solid ground on its `Layers`
  and in its `Band`, recording a `Contact{Terrain: true, Cell}`. A push apart never puts a unit
  further over ground that does not take it (`Field.Overhang`; `footing` in `resolve` holds that
  side, `moved` keeps any box the engine's later passes would leave worse); the arithmetic is
  `collision/internal/response` (`Footing`, `Worse`, `Exchange`, `Normal`, `Sweep`). Ground
  turning under a unit is no push: it stays, fallen in. A wall never turns a unit a hand drives
  (`contactSide.Steered`, `response.Side.Steered`, `Velocity.Slow`: its speed alone); what a wall
  leaves of the speed below a ten-thousandth is a stop, facing on (`Velocity.stopped`); one under
  orders glances along the wall. Reactions are rules hosted inside the system's pass: of a
  `Meeting{Self, Other, Impact, Normal, Heading, Toward}` per confirmed contact between two roles
  (`collision.Facing(within)` the word of Self turned to Other within the angle), of a `Struck`
  per entity that struck something the tick before (`host.Rules.RunWhere`; the contacts lie in
  `Collider` for good, `Collider.Contacts()`). A `collision.Sweep{From, Ignore, Ignoring}` on an
  entity moving further than the step cap (a shot) has the space hold the stretch of its step;
  `resolve` refines each pair to the segment, `nearest` keeps one contact a step (`Contact.Along`,
  `Sensed`); a swept entity is a sensor, two swept pass through each other; a wrapping world refuses
  it. Counting and logging the contacts is collision's own work (`WithStats(&stats)`, a
  `collision.ContactStats` with its `Reporter`; `WithLog`). `Boxes` (Shift+X) switches every
  entity's box in red, `BoxesShown`: `Bases` (the footprint at its altitude, after its figure in
  the order, under its own sprite where there is a depth), `Solids` (the twelve edges from its
  altitude to its top, `Frame.Edge`, only in a world with heights), `NoBoxes`; the game hands the
  plugin to players for the key. Depends on `world`.
- **`navigation`** — pathfinding and movement toward a `MoveOrder` across a `board`. **A route
  once planned is followed**: navigation plans again only when it breaks — the ground under its
  remaining steps turns against the unit (`admitsAll` on a terrain version change), a step is
  refused (`reserveLeg`), the unit is pushed off it (`actual != p.at()`), it strikes someone
  (`Bumped`, `stopAndPlan`) — or runs out (the next chunk). A better way opening (a road laid, a
  wall opened) changes no route; the unit takes it only on an impulse: a new order, a plan's step
  or a rule's command (rules/guard: the alarm's `MoveTo{To: GateName}`, `StepAside` when
  `Stuck()`, `Onward` on the round). An unreachable goal waits `targetWaitTimeout` (500 ms), then
  the order's Target becomes the nearest free reachable cell. The route finder is
  `internal/pathfind` (`Finder`: `Find`, `FindAround`, `NearestFree`), given the unit's
  `pathfind.Body{Feet, Step, Height, Size}` (`member.body()`, its box from `Position.Size`; the
  route preview passes it too). **Clearance**: a unit is no bigger than a cell's side
  (`board.NewUnits` panics past it), so every step takes it but one along a deck's band: `fits`
  asks the board's `Lane` (the band less its railings keeping the domain, on the step's level; a
  band on the ground counts on the ground's) and refuses a body whose box across the step
  (`Size.X·|s.Y| + Size.Y·|s.X|` over the step's length) is wider — in both searches and in
  `Steps`; a zero Size fits anything. The plugin always plans by levels (a board's world has
  heights; the planner's plain mode is its own tests' alone): its nodes are a cell and a level (a
  deck's or what lies under it; a step onto a deck where the ground at the shared edge lies within
  the step, off one only onto a surface within the step, under one only for a unit that fits,
  `underway`; the goal reached on either level, the goal's cell met mid-route a waypoint; a step
  across a railing refused, a slantwise step on or off a deck only along it, one on the ground
  cutting a corner of water only along a deck lying on the ground, `along`: the flat map's
  slantwise bridges); `price` is what a step costs (the destination's `CostFor` over the length,
  times `Map.Climb` unless `Graded`; a slantwise step not `Along` a way priced by the destination's
  `Bare` ground, so roads are followed round their bends). A navigated unit carries a
  `steering.Steering`: navigation asks it for a heading at a lookahead point and its top speed,
  braking before the goal; a waypoint is passed by projection, the goal by radius; the lookahead is
  the turning radius (`|Speed|·dt/TurnRate`, ~5–8 px in the demos), so a turn's corner is cut by
  about that — at a bridge's head, onto the band past its railings' ends, which collision then
  pushes back (a unit wider than the lane slid along a railing's end over the water and stood there,
  frozen in); a `MoveOrder` queues up to `MaxWaypoints` goals. **A patrol is a plan**: the unit
  carries its `navigation.Round` of cells (`NewRound`), `navigation.Onward{}` sends it to the next,
  and the kind's plan is `plan.Steps` of `plan.Self.Do(Onward{})` with `Until:
  cond.Self.If(Arrived{})` and `plan.Self.Wait(pause)`. `MoveTo{Cell, To, At, Append, Level, Face}`
  orders every `Selected` entity the player owns, or the entity giving it itself (`act.Do`); `To`
  names the goal (`ceremony.Named`: a cell's `Plot` or a unit's `At`); `Level` (`EitherLevel`,
  `OnGround`, `OnDeck`) is what the camera picked; `Face` the point it turns to on arrival (a right
  click on its own cell, or `LookAt`); the right button's `Drag` is a click, a `ButtonHeld` past the
  slop a `LookAt` every move; Shift appends. `Spacing` (`WithSpacing`; `AutoSpacing`) is how units
  keep apart through the `keeping` seam (`cellKeeping`, `bodyKeeping`), one
  `navigationSystem.Update` loop; both plan routes blind to the others. What a unit does about
  another is rules of the moment `Touch` navigation hosts (a `host.PairRules[Touch]`; words
  `PushedByAlly()`, `Blocks()`, `WaitedLong()`…; `crowd()`, StarCraft II's, added at Install unless
  `WithCrowd(rules...)`): `BodySpacing` the Collider's contacts (a `Resting` or `Sensed` contact
  bumps nobody), `CellSpacing` a step refused; handed `GivingWay`, `LastGoal`, `Ally`, `Groupmate`
  (`LastOrder.Group`), `OnMyGoal`, `HeadOn`, `Room`, `WaitedOut`/`Cornered`. The rules' commands
  (`StepAside`, `Detour`, `Pass`, `Hold`, `Settle`, `Stop`; `Aimed` at the other) are carried out
  for that unit alone, the engine's rules kept there: `stepAside` goes where the ground takes the
  unit, no steeper than `yieldClimb`, nobody standing; the last word stays navigation's
  (`stallAfter`, `maxStalls`, `bumpInterval`). A group gets its spots from `bodyKeeping.place` and
  goes cell centre to cell centre. The board has every unit hold the cell it stands on, navigation
  the cells of the steps in progress on their level (`Path.Decks`, a `Leg`'s `FromDeck`/`ToDeck`); a
  unit standing off its route plans again. The driving plugin gets its `driving.Keeping` from
  navigation (`handKeeping`); a unit whose `steering.Driven.Steers()` gives its order up in
  navigation's pass; navigation's RunPlan comes before the driving's. The renderer of routes
  (`internal/routes`, made in `NewPlugin`): every selected unit's goals as its outline lying 0.8
  over the goal's ground and, on `Routes{}` (Shift+R), its routes as thin lines at the ground's
  depth; `WithRouteStyle`.
  Facts told a unit with a tree: `Blocked`, `Arrived`; `MoveOrder.HitUnit` (entity ids start at 0:
  never use 0 as "nobody"). Depends on `board`, `world`, `selection`, `driving`.
- **`entity`** (core) — what every entity carries: `Base`, `Position` (`StepReach`, `MaxStep`,
  `MaxSpeed`), `Velocity` (`Dir`, `Value`, `Side`), `Z`, `Layers`, `Drift{X, Y, Z}` (the velocity
  it is carried at beyond its walk, written by gravity alone), `Eye{Height, Angle, Vertical,
  Tilt}` (where one that looks looks from and how wide; `Vertical` the field up and down, 0
  every height; `Tilt` how far the eye is turned up, radians, written by the riding camera;
  `Eye.Level(z)`, `Eye.Within(lo, hi)`; read by vision's cone and the first-person camera
  alike), `Lie`, `Block`. A leaf; the world re-exports them as type aliases.
- **`world/steering`** — `steering.Steering{TurnRate, Reflex, MaxSpeed, Sprint, Accel, Brake,
  V0, Halted}` the knobs, `steering.Course{Want, Pending, Delay, Speed, WantSpeed}` the state
  (given to every unit by the roster and by the system where missing), `steering.Helm{*Steering,
  *Course}` what steers (`Request(heading)`, `RequestSpeed`, `RequestSprint`, `RequestBack`,
  `RequestSide`). `steering.System` runs in every simulation step before movement: turns `Vel.Dir`
  towards `Want` by at most `TurnRate` a tick after `Reflex` ticks (`TurnRate` 0 turns at once),
  writes `Vel.Value` from the profile; `Halted` holds speed at 0. `steering.Driven{Ahead, Turn,
  Sprint, Face, Side, Look, Flown, Climb}` is an entity steered by hand, two writers: the driving
  the hand's (`Ahead`, `Turn`, `Sprint`, `Face`, `Side`), the topography's camera system the eye's
  (`Look`, `Flown`, `Climb`); `Steers()`. `steering.Pace{Share}` the ground's share of the speed.
- **`world/view`** — `view.View{Bounds, Culled, In}`, `view.EntitySet`, `view.System`.
- **`clock`** (core) — the tactical clock, made and run by the world (`world.Plugin.Clock()`):
  game time is the sum of the simulation's steps, `clock.State{Time, Tempo, Paused}` on the
  clock's own entity, saved. Shift+P the tactical pause, ] and [ the tempo (`Config.Tempos`;
  sub-steps of one length, or one longer step with `Config.BiggerStep`). A plugin's `RunPlan` is
  the interface part of its tick; what simulates it hands to `clock.Simulate(c, ctx, d, block)`,
  and the engine's plan wrapper replays every block after the game's `Update` as many times as the
  tempo says (`Clock.Replay`); `Clock.Behind` lowers a tempo above 1 after 30 slow frames.
  `clock.Phase` tags on the clock's entity, `Clock.In(phase)`, `Clock.Entity()`, `Reporter()`,
  `HUD()`, `Shown` (game time run on past the last tick by the real time held toward the next;
  `Clock.Pending`). Rules of a `clock.Moment{Last, Now}` fire every step (`clock.Every(period,
  offset)`, `clock.At(t)`; by clock time, never again after a load), about the clock's entity.
- **`selection`** — a `Select` command (ids, or a world box, additive or not) sets the
  `Selected` tag on entities carrying `Selectable`, both bits of `selection.Family`
  (`Plugin.Tags()`; every unit carries the family, off, and is told `Allow{}` as it is made;
  `Plugin.IsSelected` a drawing rule's condition). `DefaultBindings()`: a left drag selects
  (Shift adds), the left button held a `Marquee` (drawn by its renderer), C `FollowKey` (a
  `cameras.Follow` of the issuer's one selected unit, `Plugin.Chosen(by)`, for the camera the
  player looks through), `Hover` every tick (`control.CursorOver`: the entity under the cursor
  carries `Hovered` until the next tick; `ui.Tagged(Hovered)`). A `Select` hits and unselects
  only what the issuing player owns (`owner.Obeys`). Its routers: `Plugin.Selected(roles...)`
  (the giving player's selected units playing a role), `Plugin.Pointed()` (the entity drawn under
  the cursor a key was pressed with). Depends on `world`, `cameras`, `players/owner`.
- **`dialog`** — conversations as data: `dialog.NewPlugin(world, Config{Rest})`, handed to
  `players.NewPlugin`. A `Node` (`Speaker`, `Say`, `Choices`) of `Choice`s (`Text`, `If`, `Then`,
  `Next` or `End`, `Do` — command names of `world.Commands()`, a `ui.It` in them the speaker), by
  name: `Plugin.Load(fsys, path)` reads a YAML map of them (known fields only; errors with the
  line and the node), `Plugin.Define(name, node)` in code, both in the plugin's own section
  `dialog.Conversations(f)` (`stage.With`, after the commands, before the kinds); an answer to an
  undefined node panics as the Stage is set up. **What an answer changes is the game's**:
  `Plugin.Changes().Define(name, dialog.Alter(func(v *T, listener)))` (a function of the speaker's
  component `T`; `AlterListener` the listener's), `Plugin.Conditions().Define(name,
  dialog.When(func(v *T, listener) bool))` (an effect's name on the speaker works too, `!` denying);
  `T` is a component — fields of a fixed length, no strings. `Plugin.Script(node)` is the `Script`
  component where a unit's conversations begin. `DefineEffects()` defines `dialog.TalkingEf` (on
  the speaker while it talks; taken off by anyone, the conversation is over) and `TalkedEf`
  (`Config.Rest`). Commands: `Begin{At}` (an entity's own, `host.Aimed`; refused while either
  talks), `Choose{Index, Speaker}` (a `ui.About`, from the window's button, by a player the
  listener obeys), `Feel{Change}` (an entity's own, aimed). State: the speaker's `Talk{Node, With}`,
  saved; the `talkSystem` under `clock.Simulate`. For a scene: `Window()` (one for every speaker,
  `Under(TalkingEf)`: `SpeakerText`, `LineText`, a button per `ChoiceText(i)`, `MaxChoices` 6).
  Depends on `world`, `ui`, `players/owner`.
- **`players/owner`** — whose a unit is, a leaf: `owner.Family`, `owner.Of(id)` (players 1–64),
  `owner.Name`, `owner.Obeys(owners, by)` — an owned unit obeys its owners alone, an ownerless one
  the virtual player `command.Nobody` alone. A unit is a player's by the command `players.Give{To}`
  (which it gives itself as it is made, `kind.Entry.Told`, or later); `selection.Allow{Selected}`/
  `Forbid{}` the same for selectability. Both families every unit carries, all off; told at spawn,
  a unit is nobody's and unselectable through its first step. A side of its own (the wild, a
  rival) is `players.Add` owning its units.
- **`players`** — whoever acts in the game, over `plugin.CommandHandler`s: `players.NewPlugin(
  world, s.cameras, s.selection, …)` has the world carry each one's `Queues()` and gathers its
  `DefaultBindings()` (`Defaults()` all of them); nothing clears a queue — a command waits for its
  handler's pass. `Local(name)` a player at the keyboard, `Add(name)` one without; **a player owns
  no camera**: it acts through the picture a scene wires it to (`ui.Image(feed).Input(
  players.Through(pl))`: before each pass of input the active scene tells the player where the
  picture lies and what it shows, so the mouse over it is the player's and every command it gives
  carries that picture's camera, `control.Context.Camera`; two of one player's pictures in one
  pass panic). `Issue(player, cmd)` is how any command comes in (`ErrUnknownCommand`);
  `IssueAs(pl)` for a ui scene's buttons and keys; players implements `ui.Looker` and `ui.Owner`
  and draws nothing but the marquee. The input's contract — `Binding`, the `Trigger`s `KeyPress`,
  `KeyDown`, `KeyHeld` (once a tick while the key is down), `KeyReleased` (as the key comes up,
  `Context.Held` how long it was down — real time, `players.ClockWith` in tests; not after a
  chord with modifiers fired while it was down: Shift+X is no throw); these three fire with
  Shift, Ctrl or Alt held (the key's own modifier aside) only when the binding is `InChords()`
  (`Binding.Takes(mods)`): the cameras' scroll leaves Shift+W to the weather and the
  topography's head Shift+R to the routes, driving's keys and `JumpKey` are in chords (sprint,
  a jump while sprinting, walking while a grenade charges) — `ButtonPress`, `Drag`,
  `Wheel`,
  `ButtonHeld`, `CursorAtEdge`, `CursorOver`, `CursorMove`, with exact `Mods`
  (`Mods{}.Holding(key)`), `Build[C]` from a `Context` (`World`/`WorldBox` through the camera's
  `Picker`), `Give(trigger, label, cmd)`, `Binding.In(hows...)`, `With(tool)` — lives in
  `control`, so a plugin with commands never imports players. A player holds a tool in hand
  (`control.Tool`: `control.Take`/`Drop`; while it does, a binding `With` that tool takes its
  trigger from those of no tool). `Player.Bind` refuses two bindings on one trigger holding in one
  fastening; Setup refuses a command nobody defines. Local players through pictures of their own
  split the screen (`ui.Columns`); keys reach every local player, the mouse the one under it. The
  Scene hands input to `players.Plugin.Handle(events, runtime, composition)`: the bindings turn it
  into commands, `ShowShortcuts` (K) is carried out there, the game's own `SceneKeys`
  (`Plugin.OwnKeys`: a debug toggle, no command) run; `GameBindings()` (K, F5 `game.Save`, F9
  `game.Load`, Shift+Esc `game.Quit`) give the engine's commands. The list of shortcuts
  (`players.Shortcuts`, a `game.Scenic` scene made in `NewPlugin`, so K works in every game) is a
  `ui.Scene` whose tree `Open` builds afresh (`ui.Scene.Replace`): `Props.Description`, the title,
  the groups — "Game" first (the Controls section and `SceneKeys`), then the plugins' keys by
  plugin, then "General" — each a `ui.Heading` over two aligned columns of keys and labels, in
  columns as wide as their lines, as many as the height takes. `players.RunPlan` runs last and
  empties the queues. Depends on `world`.
- **`cameras`** — the cameras a game looks through: `cameras.NewPlugin(world)` makes no camera;
  **each is made where a scene shows it**, beside the picture in the Scenes section,
  `render.NewFeed(s.cameras.New(maker, camera.Config{...}), picture)`, with `cameras.TopDown()` (a
  flat world, wrapping on a wrapping axis) or a view plugin's maker. `camera.Config` has no pixels
  (`render.Feed.Resize` every frame): `Zoom`, `Whole` (the whole world in view at every size, a
  minimap; refused in relief), `Follow` (the name of the entity it starts fastened `Centred` over),
  `MinZoom`, `MaxZoom`. Every camera is saved (window, zoom, fastening), in the order made. Its
  commands name their camera, so it knows no players: `Pan` (lets go of a fastening), `Zoom`,
  `Follow{Camera, Entity, On}` (fastens `Centred`; its system keeps every `Centred` camera over its
  entity), `LookAt{Camera, Entity}` (once, fastened to nothing). `cameras.Keys{Up, Down, Left,
  Right, ZoomIn, ZoomOut, ScrollSpeed, Wheel, Drag, Edge}` are a player's keys, `DefaultKeys()`
  W/S/A/D, = and - (`ZoomStep` about the cursor), the wheel, the middle drag, the edge (a
  `Pan{Edge: true}`, in a corner of the picture — `control.Edges.Corner`, a square of
  `control.CornerReach` 120 px a side, a quarter of the shorter side at most
  (`control.CornerSide`: a 200-px test viewport would be all corner), `Edges.Any` true there —
  slantwise with `Corner: true`;
  a `camera.Scroller`
  takes it as `Scroll(dx, dy, corner)`, the relief's camera turning in a corner at
  `topography.CornerStep`, a quarter of Q, and any other camera pans). `Zones` (Shift+H,
  `ShowZones`, `ZonesShown`, marks over every other, hidden by default, shown at
  the start of board/topography) lays the strips and the corner squares over every picture in
  translucent yellow, rounded (`render.Frame.Fan`), the strips a gap short of the corners and
  fainter (the user's sketch, 2026-10-14). `CursorAtEdge` fires once an `Engine.Update`, before
  the ticks, not once a tick; a demo
  whose WSAD drive binds `cameras.Keys{Wheel, ZoomIn, ZoomOut}` alone. The engine sizes no camera.
  Depends on `world`.
- **`driving`** — units driven by hand, with or without navigation: `driving.NewPlugin(world,
  selection).WithGround(board)`. Its commands `Ahead{Camera, Sprint}`, `Back`, `Turn{Way}`,
  `Toward{Way}`, `Strafe{Camera, Way}` add up a tick into a hand — a player's through the camera the
  command names: on the unit it is fastened to, else on the player's selected units; an entity's own
  (`act.Do`) — written into `steering.Driven` with the marker `driving.Driving` (`handSystem`); the
  `driveSystem` (in the simulation) turns the unit (by `Turn`, or to `Look` before `Face`), walks,
  brakes, backs, strafes, flies it, never onto ground its domain may not take on the level of its
  feet nor where the `Keeping` says no — one under gravity refused only what is higher than its
  step; asked to go more than 45° off its way while walking (`turnBrake`) it brakes first, turns,
  then walks; with no hand it brakes and is let go once it stands or `Keeping.Ordered` — held still
  by a wall it stands at once; `WithTap(d)`: a `Toward` key held no longer than d on a standing unit
  only turns it (`Driven.Held`). `Jump{Camera}` has every unit the hand is on give itself a
  `gravity.Leap` (`driving.JumpKey(key)`; Space `In(camera.Inside)` among the defaults). Keys are
  sets: `driving.Tank{Ahead, Back, Left, Right, In}` (Left and Right turn), `driving.Shooter{…}`
  (Left and Right strafe, the mouse turns) and `driving.Compass{Up, Down, Left, Right, In}`
  (`Bindings()`); `DefaultKeys()` a `Shooter` of WSAD `In(camera.Inside)` and a `Tank` of the arrows
  `In(camera.Behind)`. Depends on `world`, `selection`, `board`, `players/owner`, `gravity`.
- **`gravity`** — bodies falling, standing, leaping (a character controller, not rigid bodies;
  one plugin, two modes, chosen by the pull's axis, not by the world): `gravity.NewPlugin(world,
  Config{Pull})`, its own entity carrying the knob `gravity.Pull{X, Y, Z}` (an effect's `Alter`
  turns it over within its axis; turned from Z to X/Y or back it panics; a pull both ways, or
  along Z in a flat world, panics; zero, the Earth's as the figures feel it). A kind falls
  carrying `gravity.Body{Leap}`; `giveSystem` puts a `world.Drift` on it where missing. Along Z
  the `fallSystem` (`internal/vertical`) stands a body every step on the surface under its centre
  (`WithSurfaces(board.Surfaces)`, refused for a pull across) within its reach down, flies it
  under the pull, stops its head at a slab's underside and lands it; across X/Y (a side view, the
  platformer: a world with heights, `Pull{Y}`, Z its depth) the `planeSystem`
  (`internal/planar`) has collision's contacts (not `Sensed`) take the drift
  running into them, a body borne by one facing against the pull, braked across the pull while
  borne. `unit.Airborne` is on while in the air. Commands, an entity's own: `Leap{}` (none in the
  air), `Away{Speed, Rise}` (`host.Aimed`: thrown off from the moment's subject). A `Landing{ID,
  Speed}` is told the rules hosted once as a body lands, its word `gravity.HarderThan(v)`. Run it
  right after collision's RunPlan. board-topography's walkers leap 1.5 m over 1 m railings (a leap
  no higher than a railing never clears it). Depends on `world`, `board/ground`, `board/unit`,
  `collision`.
- **`vision`** — narrowed perception: a `Sight` cone scanned against the space each step fills
  the entity's `Sighted` (who it can see, nearest first; `Sight` is a knob vision only reads,
  `Sight.Ahead`, `Sight.Looking(dir)`), `SightOutline` gets its view's shape drawn. An entity
  carrying `Transparency` dims sight; the world's `Cover` (the board's veiled cells) is walked along
  every ray, dimming or cutting it, a ray spending its radius as a budget; `Sight.Blockers` are
  the `world.Layers` that cut or dim this sight (a hawk with `Air` looks over walls). With heights
  sight has heights instead: the cone's eye is the unit's `world.Eye` (a Sight without an Eye is
  not scanned), every entity spans its `Z`, the ground is read along every ray — exactly where it is
  a `ground.Profile` (the relief's `Along`), else sampled every `WithGroundStep`, `Blockers` cutting
  as in a flat world; `Eye.Height` and `Eye.Vertical` refused in a flat world; an `Eye.Vertical`
  keeps what the scan found to the field `Tilt ± Vertical/2` by elevation (`scanner.vertical`, after
  aabbworld's cone, which knows no vertical angle), the views drawn the ring of ground the field
  meets (near and far on the CPU from the eye's height, per pixel on the GPU by `atan2`, the
  instance's fifth vec4). Every observer is scanned every step, or
  `WithFrequency(perSecond)` with the observers taking turns, shared among goroutines
  (`WithWorkers`). It hosts rules of a `Sighting` inside the scan's pass, once a step per observer
  on what it saw at its last scan (`cond.Other.If(b)` everything in view; its `Subject` the nearest
  seen, so an aimed `steering.Away{}`/`Toward{}` is about that one); words `vision.SeesNobody()`,
  `vision.Closing()`. The views drawn are every observer's unless `WithViews(render.Show(...))`
  picks some (given in the scene); `Cones{}` (Shift+C) shows and hides them (`ShowCones`,
  `HideCones`, `ConesShown`; not saved). `WithLog`. Depends on `world`.
- **`bridges`** — bridges built by one set of rules whoever builds them: `bridges.NewPlugin( world,
  board, Config{Rules site.Rules, Builds})`, `WithShaper(ground.Shaper)` (the topography's
  `Flatten`). `bridges/site.Plan(Ground, Rules, from, to, deck)` is where one may stand, pure:
  straight along one of the grid's directions, `Longest` cells, its heads Takes and on no bridge,
  every span's ground `Clearance` under the deck, no higher than `High`, never over or beside
  another bridge's span, something crossed (water, or ground `Under` the deck; a road under it only
  square across), the heads levelled within `Reshape` breaking no road, a pier between every two
  spans; a `Refusal` says why. Kinds: `Plugin.Types().Define(name, Spec{Allows, Width, Thickness,
  Cost, Rail, Pier})` — the name a level's lines call a bridge by (`bridges.Row{From, To}`),
  `<name>.deck` (`board.NewEntities`: a Graded band) and `<name>.pier` (a still collider, drawn
  standing); `Type.Deck()`/`Pier()` the atlas's slots; a kind of bridge in one piece is a
  `bridges.Figure` (`Define(p)`, `Look(a, p)`; `Stateful`, `Ruled`), noted by `Plugin.Figures`
  among the world's, `Plugin.World()` the world. The flow of a bridge (`types.go`,
  `plugin.go`): `Types.Define` registers three kinds — the name itself over `Row{From, To}`,
  never spawned, only the type the level's line `{kind: bridge, from, to}` is read as; the
  deck through `board.NewEntities` over the private `deckRow{At, Links, Height, Dir}` with a
  `cell.Floor` built from the Spec, the `Rail`, the heading, a `Block`; the pier over `pierRow{At,
  Bottom, Top}`, spawned only over a map in relief (`Rules.Heights` follows the relief: a level's
  `Heights()`, the board's `Heights()`; without it a bridge is a deck at the ground over water or a
  cell its road may not take, no piers) — each kind getting its `SpriteID` from the world's registry
  as it is registered (`kinds.Register`), which `Type.Deck()`/`Pier()` hand out empty for the
  figure's `Look` to fill; `Plugin.Spawn(level)` in Spawn keeps the rows,
  `Populate` plans each with `site.Plan` and `parts` builds the entries (a deck a cell, a
  head's `Links` one way, a span's both, a pier between spans), every entity drawn by its
  kind's slot. The plugin never calls the atlas: the look is the figure's alone.
  `Plugin.Spawn(level)` in Spawn lays a level's bridges, held to the rules at Populate (an
  error with the line). A player's: `Tool`, Shift+B,
  the cursor `Aim`s, a left click `Mark`s each head, right click and Esc drop; `Window()` asks
  "Build the bridge?", `Reason()` says why not; `Build` levels the heads and spawns the parts at the
  next step.
- **`telemetry`** — the panel in the top-left corner: `telemetry.NewPlugin(w)` with the
  standard lines (FPS, TPS, `Frame` from `render.Spent`, `Entities`, the clock's `Game time`),
  `With(reporters...)` (`render.Reporter`), `Line(label, value)`, `Clear()` — all before the
  scenes are made (later panics); a `game.Scenic` scene `telemetry.SceneName` that is never
  focusable; `Show`/`Hide`/`Toggle`/`Shown` on a `game.Composition`. The Stage's
  `World(cfg).Plugins(f)` link makes one where `telemetry.Of(w)` finds none, so every Stage
  with a world has it, hidden; `.Shows(MainScene, telemetry.SceneName)` shows it from the start.
  Its command `Panel` is carried by players (`GameBindings`, ~, "General"), which
  toggles it through the Composition in `Handle`. Depends on `world`, `ui`.
- **`bullet`** — shots as entities: `Plugin.Shots()` — `Shots.Define(name, Body{Size, Speed,
  Range, Gravity, Lands}, extra...)`, `Named` the `Ammo`, a kind whose entities carry a sensor
  `Collider`, a `collision.Sweep` ignoring the shooter, the `Body` (a knob), the `Flight` (`At`,
  `Dir`, `Range`, `Flown`, `Climb`, `Shooter`, `Ending`, `Other`, `Cell`, `Landed`), the shooter's
  owners, with heights a `Z`. `Shoot{Ammo, At, Targeted, Camera, Power}` (`host.Aimed`; no
  default bindings) fires from the unit `Camera` is fastened to (the player's own; as driving's
  hand), else every `Selected` unit the player owns, or from the entity giving it itself, at the
  muzzle just outside its box, towards `At`, the subject, else the way it faces, from its
  `Eye.Level`; a thrown shot untargeted goes `Power` of its `Range` (0 the whole; bullet's demo
  throws by Shift held, `control.KeyReleased`); a thrown shot gets the `Climb` that brings it
  down where it goes on the ground
  `WithGround(board.Heights)` gives. The `flightSystem` runs **before the world** (`Stage.Update`
  calls `bullet.RunPlan` first): it flies every shot past the step cap, writing the `Sweep.From`
  and the box, noting the `Ending`; the next step the nearest contact lands the shot just short of
  it, else the `Ending` lands it: a `Landing` for the rules hosted (`Subject` the entity struck);
  a shot that does not `Lands` lies one more step and is despawned. A landed shot that `Lands` is
  a `Resting` every step; `Burst{Radius}` (an entity's own) has the `burstSystem` dispatch a
  `Blast{Self, Other, Distance}` (`bullet.Within(d)`) per entity within the radius, then despawn
  it. A wrapping world is refused. A kind of ammo in one piece is a `bullet.Figure`
  (`Define(p)`, `Look(a, p)`; `Stateful`, `Ruled`), noted by `Plugin.Figures` among the world's,
  `Plugin.World()` the world. A shot is dressed as any entity (`.Turning()` on the ammo's
  Slot, the heading kept on `Base.Vel.Dir`). A weapon — ammo, reloading, who carries it — is the
  game's rules and effects. Depends on `world`, `collision`, `selection`, `players/owner`,
  `board/ground`.

Each package has a `doc.go` describing the gameplay capability it adds.
