# Changelog

## Unreleased

Saves written by v0.2.0 do not load: `Base` and the marker components changed shape, the sky's
and the climate's entities are gone, the clock's is new.

**Shift sprints in first person; the hawk flies where the rider looks**
- Riding in a unit, W with Shift held drives it at its profile's `Sprint` times its top speed
  (`steering.Steering.Sprint`, `RequestSprint`; `steering.Driven.Sprint`, `topography.Drive.Sprint`),
  speeding up past the top `Sprint` times as quickly; following without the perspective, the up
  arrow with Shift does the same. The world still moves a body at most half its size a tick, so
  the island's walkers go about 3.75 times, the hawk about 2.5 times as fast. The weather's
  Shift+W holds with the camera free only.
- An entity that flies, ridden, holds its height over sea level whatever the ground under it
  does, and climbs and dives only along the way the eye looks: the camera writes
  `steering.Driven.Flown` and the look's rise (`Driven.Climb`, the sine of the pitch), the drive
  system goes as much less along the ground (`Driven.Slope`, 80° at the steepest), and the
  topography's altitude system climbs it by the rise over the run it made in the step. It keeps
  `board.Mover.Clearance` over the ground, pushed up where the ground rises to it, and stays under
  `board.Mover.Ceiling` over sea level; its `Lift` follows, so let go it keeps its height over the
  ground again. The island's hawk keeps its own height, 20 m, over the ground and stops 100 m
  under the clouds.
- The island demo's `Sprint` is 4.

**A day four times as long**
- A day is 16 minutes of game time unless the calendar says otherwise (`calendar.DefaultDay`;
  4 before): the sun, the light and the weather go by four times as slowly in real time.

**The ground traced on the GPU with its water, waves, clouds' shadows and grid**
- The traced ground (G) draws what the tiles lay over the ground: the sea's glint, its waves
  turning to the shore and breaking into foam on it, the water running down the rivers and over
  flowing cells, the glint where a river turns into the sea, the clouds' shadows, and the grid
  (B) where a cell spans six pixels or more — with the topography's own materials
  (`SeaGlint`, `RunningWater`), per pixel. It is traced at half the window's size, so its grid is
  about two pixels wide and soft.
- The dresser paints the board's water beside its albedo, in the same passes and repainted for
  the cells that change: running water's flow, the running and the still water's shine, the
  glint at a mouth, each with its coverage in a colour of its own, so what the tiles lay over the
  water — the coast's grounds, a road, a bridge — covers it here too (`heightfield.WaterLayers`,
  `FlowSpan`). The way to the shore from every corner is worked out anew only round the cells
  that begin or cease to shine (`dresser.Coast`), not as the ground is shaped.
- `heightfield.Surface` (`Painted`: albedo, water, shores, the grid) takes the place of
  `heightfield.Albedo`; the lattice is one image of four quadrants: heights, normals, the cells'
  tops, the shores.
- `render.ShaderSourceWith` builds a shader on the composer's library — its uniforms, helpers and
  every material registered (render/library.kage, split from compose.kage) — for a source drawing
  itself; no material may be registered after. `render.Direct.Draw` is handed the frame's
  uniforms (`render.Uniforms`, `UniformsOf`), so such a shader reads the sun, the air and the
  clock the frame set. `board.MinGridCell` is exported; `weather.kage` has `cloudShade`.

**S brakes, then backs away; the island's units four times slower**
- A driven entity asked back (`steering.Driven.Ahead` -1: S riding, Down following) brakes to a
  stop at its Brake, no longer at once, and then backs away at its V0 — a quarter of its top
  speed without one — facing as it does, as long as it is held, stopping at an edge behind it.
  `steering.Steering.RequestBack` asks for it: a `Speed` under zero backs away along the facing,
  and from one way to the other the steering brakes to a stop first. A backing entity's velocity
  (`world.Velocity.Value` under zero) keeps its facing through a bounce (`SetDelta`), and the
  board's slope slows it the way it goes, not the way it faces. A camera letting go of its unit
  leaves it braking, no hand on it, where it stopped it at once.
- The island in relief walks its units at three quarters of a cell a second, a quarter of what
  it did.

**The heightfield coloured from the board painted flat, lit smoothly, whole from a low eye**
- The ground traced on the GPU is coloured from the board's albedo: the whole board painted
  flat, 16 pixels a cell — every cell's base (the sea under a coast), the grounds running in,
  the ways and the crossings, as the tiles' ground sheet paints them (`dresser.Albedo`,
  `heightfield.Albedo`; `board.Plugin.Atlas` is the atlas it paints from) — blended between its
  pixels, so rivers, roads, bridges and coasts show on it; the kinds' flat colours are gone. It
  is painted anew for the cells that change, as the ground sheet is.
- The way the ground faces is worked out per lattice corner on the CPU as the tiles' light is,
  from the rise between the corners either side, and blended across a cell in the shader, so
  the light runs on smoothly from cell to cell; the terrain's shadows fade in over a penumbra
  instead of stepping.
- A line of sight going up from under the top of the relief — the eye riding in a unit looking
  at a peak — was clipped away before it marched; the range shows from a low eye now.
- The shader walks a line of sight cell by cell over the lattice and meets the two triangles of
  a cell exactly, passing over a cell whose highest corner it stays above with one read (a
  fourth image, the cells' tops); it stepped half a cell or more before and jumped over the
  steep peaks' tips, cutting them flat and the silhouettes into steps.
- The island demo's `TestShots` adds first person in one selected walker, the head raised,
  traced and as tiles.

**The clouds drawn pixel by pixel, higher; the heightfield traced at half size**
- The clouds on the sky are worked out per pixel: each looks along its own line of sight (the
  frame's `EyeAt`, `LookDir`, `LookDX` and `LookDY` uniforms, the camera's `camera.RayField`) up
  to the cloud layer and takes the clouds' noise there, so they stand still as the head turns;
  before, the noise was read at the corners of 64-pixel pieces and blended between them, and
  swam about. The ground's cloud shadows are worked out per pixel the same way. One noise on the
  CPU and the GPU (`air.Weather.Cloud`, weather.kage's `cloudField`): value noise on a
  permutation-polynomial hash mod 289, exact in floats on both. The layer hangs 6 km up
  (`air.CloudBase`; 3 before), so the clouds look far. `Weather.CloudQuad` takes the piece alone.
- The heightfield is traced at half the viewport's size and scaled up (`heightfield.Config`
  `Downscale`, 2 unless set; 1 every pixel), its normal from the cell's four corners alone, its
  shadows in 16 steps of three quarters of a cell, its march no finer than half a cell: a quarter
  of the pixels and a third of the texture reads a pixel. On the island's Intel UHD 620 the
  tiles' perspective ran at 30 frames a second, the heightfield traced whole at 30 to 40.

**The ground traced on the GPU from its heightmap**
- `topography/heightfield` is the topography's other way of drawing the ground: one shader
  traces every pixel's line of sight over the relief (heightfield.kage) — the heights a lattice
  image of 16 bits a corner, a colour a cell from the kinds' Colors, the ground split into the
  two triangles a tile is, lit by the sun with its shadows cast, hazed towards the Fog as far off
  as it lies — in place of the tiles, which lay nothing then (`board.Nothing`); what stands on
  the ground is drawn as before, a billboard the ground hides from the eye left out
  (`Renderer.Hides`). `Config.Heightfield` reaches it and G switches ([`Heightfield`],
  `Plugin.ShowHeightfield`); `Plugin.Renderer` is its renderer, for the scene's composer beside
  the board's. Drawn through every view, from above, isometrically and in perspective, from the
  camera's lines of sight (`camera.Rays`, a `camera.RayField`: the ray of a screen point from six
  vectors). Not yet measured on a GPU, nor drawn with the ground sheet's blends and water: a
  first cut, the kinds' colours only.
- `render.Direct` is a Source that draws a part of the picture itself, with a shader of its own,
  where its tier comes among the frame's pieces; the composer leaves a nil layer out.
- The island demo's `TestShots`, run with `GRAM_SHOTS` naming a directory on a display, draws
  the demo's views into PNGs — the isometric start, zoomed out, the perspective, the view from
  above, and the three with the ground traced on the GPU — for a look at what the GPU makes of
  a frame. It found the heightfield reading its images outside their texture regions (a flat
  black plane); fixed.

**The tiles dressed and the cones scanned on every CPU**
- The board's renderer dresses the tiles on several goroutines at once when the Map's Dressing
  is a `board.Parallel` and its Look a `board.ParallelLook` (`board.Plugin.WithWorkers`: 0 every
  CPU, as it starts; 1 none): every visible tile is Warmed on the frame's goroutine, the dressing
  made Ready, then the tiles are shared out in runs, each drawn by a Worker of the dressing and
  of the look into a frame of its own (`render.Frame.Branch`, `Append`) and appended in order —
  piece for piece the picture one goroutine draws. The topography's dresser is one: a worker
  reads the tops as Ready read them (frozen), keeps its own scratch and its own clouds, and
  writes only its own cells' light, shadows and bakes; the shores, met by the tiles round a
  corner, are warmed first. The island composes in 2.6 ms from above, 1.8 isometric and 3.9 in
  perspective through 1080p — 4.3, 2.7 and 6.8 on one goroutine (8 threads).
- Vision scans observers enough on several goroutines at once (`vision.Plugin.WithWorkers`), a
  scanner of its own each — its view, its lookup, the cover walked for it — in runs cut across
  the ECS's chunks (a chunk of outlines holds a few); the behaviors then run one observer at a
  time, in order, as before. The board reads every cell's cover at once beforehand
  (`Board.Ready`, the `board.Readied` contract) rather than cell by cell through the ECS as a ray
  meets it, and one query settles the space's index first. 500 observers scan in 0.26 ms, 0.49
  with outlines — 0.54 and 1.31 on one goroutine. An outline's shadows are cleared only as far as
  its samples reach.
- `internal/parallel` shares a run of items among goroutines. The benchmarks' harness replays the
  world's clock after the plan, as the engine does: `Benchmark_Vision_*` had measured an empty
  tick since the clock came; `Benchmark_Board_Island` and `Benchmark_Vision_*` measure on every
  CPU and (`serial`) on one goroutine.
- The composer hands Ebitengine a run of quads in one append rather than quad by quad
  (`Benchmark_Composer_Render`: 0.39 → 0.35 ms for 18 thousand pieces, 1.9 → 1.7 for 72
  thousand). Handing the frame over without copying its vertices was measured and left: the
  composer's own share is a tenth of composing the island — 0.3 ms of the whole island from
  above, 0.2 in perspective through 1080p, 0.03 isometric — and Ebitengine copies and converts
  every vertex it is handed per call whatever is passed, so a frame's vertices handed whole to
  every call would cost more, not less.

**The sky of the day, with the clouds on it**
- Through a perspective the backdrop is a mesh of the sky from the horizon up: paler at the
  horizon, deeper overhead (`air.Overhead`), greyed by the cover, the sun in it, and on a layer
  `air.Base` high (`air.CloudBase`, 3 km) the clouds themselves — the same clouds, by the same
  noise at the mesh's corners, that lay their shadows on the ground (`air.Weather.CloudQuad`,
  the `Clouds` material of weather.kage), hazed away towards the horizon; nothing from above the
  layer. `camera.Rayer` is a camera that says which way a screen point looks; the topography's
  perspective is one. `render.Frame.Quad` fills a quad in a colour per corner.

**One Eye for the cone of sight and the camera riding in the unit**
- `world.Eye{Height, Angle}` is where an entity looks from and how wide: `Height` over its
  bottom (zero: its top), `Angle` the whole field across. Vision's cone takes its width and its
  eye from it (`Sight.HalfAngle` and `Sight.Eye` are gone; a Sight without an Eye is not
  scanned; `Eye.Height` is refused in a flat world), and the first-person camera rides where the
  Eye stands and shows the Eye's angle across the screen, the height following the screen's
  shape, so the cone drawn is what the rider sees. `MaxSightRadius` is 600; the vision renderer
  drapes the shadows in the scan's ground step (`Renderer.WithGroundStep`, set by the plugin's).
- The island's units see 3 km, 72° across from their top, the ground sampled every 50 m.

**Routes drawn as lines, goals as outlines**
- The navigation renderer draws every selected unit's goals as the entity's outline where it
  will stand — on the ground at the spot, orange-yellow, the Marks tier, always — and its routes,
  on Shift+P (`navigation.Routes`, `Plugin.ShowRoutes`), as a thin orange line over the ground in
  pieces of the ground's step, straight through any camera; the arrow sprites, which wobbled in
  perspective, are gone with `PathSprites`, `RegisterDefaultPathSprites`, `SetPathSprites`,
  `Direction` and `WithTops`. `RouteStyle` (`Plugin.WithRouteStyle`) is the colours and width;
  `WithRenderer` takes no atlas to use.
- The vision cones start hidden; Shift+C shows them.
- The island's water runs half as fast: `Flow` 30 on the brooks and streams, 22 on the rivers.

**Cells kept reactively**
- Under `CellSpacing` a unit routes over the ground alone, blind to where the others stand, as
  under `BodySpacing`: a step into a held cell is the collision — it waits, asks the one standing
  there off the cell, and after a second of no headway notes the cell for its routes to go round;
  one standing gives way to a free cell square off the way, else beside, and comes back; two
  meeting head on, the greater id goes round; a goal someone stands on is settled beside after
  half a second, a passer-by waited for; a corner of a slantwise step held is gone round square.
  `board.SingleOccupancy.Holder` and `MultipleOccupancy.Holder` tell who holds a cell.
  `MoveOrder.Held` is new (saves change shape). A unit lingering on a goal lets its step's cells
  go, so a yielder no longer holds the cell it left.

**Turning units with the mouse**
- A right drag turns the selected units to look at the cursor as it moves (a `LookAt` at every
  move past `clickSlop`, 4 px) and moves nothing when the button comes up. A right click — the
  button up where it went down — moves as before, but on the release, not the press; Shift alike.
  The Shift+S right click is gone: the drag looks there. The `LookAt` command stays.

**Roads followed round their bends**
- A slantwise step between two cells not linked by a way cuts the corner beside the way and is
  priced by the ground under the way at the destination (`board.Board.Bare`), not by the way: a
  road is followed round its bend instead of cut across the grass (a step between two road cells
  across the bend cost the road's √2 before, less than the two steps round), and no corner is cut
  onto a bridge over water a walker may not enter. A road laid slantwise is taken along its
  links (`board.Board.Along`) at its own price.
- `board.CellKind.Graded`: ground built up and cut into the slope — the islands' roads and
  bridges — is spared the slope, in the planner and on the move; `Way.Over` and `Crossing.Over`
  carry it. On the island no route between two road cells 4 to 20 apart leaves the road now;
  with the corner rule alone 1108 of 2841 still did, over steep road.

**Cameras in relief kept over the map**
- From above and isometrically the topography's camera keeps the whole screen over the world at
  sea level, as the top-down camera does: a pan (WASD, the cursor at an edge, a middle drag)
  stops where a corner of the screen reaches the world's edge, and zooming out stops where the
  screen just fits over the world — the widest view is nearer than before (on the 96x64 island
  zoom 0.66 for 0.25), the void beyond a diamond map is never shown, and neither are its corners.
  A save with a lower zoom comes back at the floor.
- Zoom keeps the ground under the cursor where it is drawn, at its own height: a hill under the
  cursor no longer slides away as the wheel turns.
- In perspective the ground point in the middle of the screen stays over the world, and the eye
  flies no higher than shows the world's diagonal across the middle of the screen at the flattest
  pitch; zooming at the ceiling turns the head so the ground under the cursor stays put, the eye
  flying on towards it where the pitch floor holds the head. `LookFrom` and `LookAt` still place
  the eye anywhere.

**The cones of sight on and off**
- `vision.Plugin` is a `plugin.CommandHandler`: `vision.Cones{}` hides every view drawn — the
  cones and the ground out of sight — or shows them again, Shift+C by default; `Plugin.Hide(bool)`
  and `Plugin.Hidden()` from code, `Renderer.Hide`/`Hidden` on the renderer. A look, not saved;
  the scan goes on. Hand the plugin to `players.NewPlugin` for the key; the demos with sight do.

**The world in sub-packages, `Heights` for `Quasi3D`**
- `plugins/world/steering`: `steering.Steering`, `steering.Driven` and `steering.System`
  (`steering.NewSystem()`) were `world.Steering`, `world.Driven`, `world.SteeringSystem`
  (`NewSteeringSystem`). Every profile in a kind, `board.Units.Define`, `vision.Sighting.Steering`
  and the navigation say `steering.Steering` now.
- `plugins/world/view`: `view.View`, `view.EntitySet` and `view.System` (`view.NewSystem`) were
  `world.View`, `world.EntitySet`, `world.ViewSystem`; `view.New(bounds)` is what `Plugin.NewView`
  makes, `View.Refresh(space, area)` what the system does to each. `world.Plugin.View/NewView/
  ViewFor/DropView` and `players.Player.View` hand out `*view.View`.
- `plugins/world/entity` holds `Base`, `Position`, `Velocity`, `Z` and `Layers`; the world
  re-exports them as type aliases, so `world.Base` and the rest stay, the same types, and saves
  are unchanged by it.
- `world.Config.Heights` was `Quasi3D`; `world.Plugin.HasHeights()` was `Quasi3D()`. The refusals
  say "set world.Config.Heights".

**Layering: the world knows its entities, the ground is the board's, the sky the atmosphere's**
- `render` is generic drawing again. `Frame.Uniform(name, values...)` hands the shader any
  uniform a material declares in its own Kage; the composer's own are `Toward`, `Clock`, `Pixel`
  and `Fog`, the colour a plain sprite far off turns to (`Frame.Fog`, the former `Hazed`). Gone
  from render: `Daylight`, `Weather`, `Frame.Daylight/Weather/Wind/Drift/Clouds/Haze/Hazed/
  Overcast/OvercastOn/OvercastQuad`, `Sway`, `CloudAt`, `CloudCover`, `Shadowed`, `Overcast`,
  `CloudShadow`, `overcast.kage`, and the sun's and the weather's uniforms and functions in
  `compose.kage`. `NewTelemetryRenderer(tps, entityCount)` prints no collisions: the collision
  plugin's `behavior.ContactStats.Reporter(tps)` does, through `With`.
- `plugins/world` holds entities only. Gone: `Sun`, `Lamp`, `DefaultSun`, `Weather`,
  `SetSun/Sun/Sunlit/SetWeather/Weather`, `Scale.Visibility`, `ClearAir`, the contracts `Ground`,
  `Cover`, `Field`, `FieldBox` and `SetGround/Ground/SetCover/Cover/SetField/Field`. The world's
  renderer hands its `Look` every entity in white light with its `Appearance.Sway`; the Look —
  a view plugin's, or the atmosphere's — lights it, leans it and lays its shadow. The flat look
  draws as it is.
- `plugins/board` is the ground: `board.Heights` (a point's height and the sampling step; the
  Map's, `Map.Heights()`, a topography's relief, nil on a flat map; `Plugin.Heights`),
  `board.Cover` (the Board; `Plugin.Cover`), and `Plugin.WithCollision(c)` hands collision the
  Solid cells as its `collision.Field`. `vision.Plugin.WithBoard(brd)` (or `WithHeights`,
  `WithCover`) has sight follow them; navigation reads the board's heights. A flat board's tiles
  are drawn as they are; `board.NewRenderer(brd, atlas, m)` takes no sun.
- `plugins/atmosphere` is the sky. `sky.Sun`, `sky.Lamp`, `sky.DefaultSun` (from world), with
  `Sun.Frame` (the uniforms of `sky/sun.kage`: `Sun`, `SunStrength`, `SunColor`, `SkyColor`,
  `Ambience`, `sunWay()`) and `Sun.Shadow` (an entity's shadow, from the world's renderer);
  `sky.New(cal, cfg, latitude)` holds its light, `Sky.Sun()`. New leaf `plugins/atmosphere/air`:
  `air.Weather` (from world), `Visibility(scale, w)`, `ClearAir`, `Weather.Sway`, `Cloud`, `Shade`,
  `Shadowed`, `Overcast/OvercastOn/OvercastQuad(f, ...)`, `Haze(cam, x, y, z)`, `air.Overcast(sky,
  clouds)` and `Weather.Frame(f, sun)` (`air/weather.kage`: `Wind`, `Drift`, `Cover`, the
  `CloudShadow` material, `overcastSky()`). The climate keeps the air (`Climate.Air()`);
  `atmosphere.Plugin.Sun()` and `Air()` give both; `Backdrop` moved to the root
  (`NewBackdrop(space, sun, air)`); `precipitation.New(sun, air)`, `weathering.New(brd, air, ...)`;
  `climate.Weathering.Weather` is an `air.Weather`. `Plugin.WithBoard(brd)` lights a flat board
  and the world's sprites by the hour and leans what sways, wrapping the board's Map and the
  world's Look.
- `plugins/topography` stands under an `Atmosphere` (`Sun() sky.Sun; Air() air.Weather`):
  `Plugin.WithAtmosphere(a)`, `sky.DefaultSun` in still clear air without one, so a relief
  without an atmosphere is shaded as before. The dresser lights, shades, lays cloud shadows and
  hazes by it; the topography's look lights the entities on level ground, leans them with the wind
  and lays their shadows on the relief. `Plugin.Heights()` is the relief; `water.kage` reads
  `Clock` and `SunStrength`.
- `control.Context.Ground` is gone: `World` is a `camera.Picker`'s pick, else `FromScreen`.
- Demos: `board.NewPlugin(...).WithCollision(c)`, `vision.NewPlugin(w).WithBoard(brd)`,
  `topography.WithAtmosphere(atmo)` in `board-topography`, `atmosphere.WithBoard(brd)` in `board`.

**Time**
- `plugins/world/clock`: the tactical clock. Game time is the sum of the simulation's steps, kept
  as `clock.State{Time, Tempo, Paused}` on the clock's own entity and saved with the game. Space
  is the tactical pause — the simulation stands while the player selects, orders, plans routes and
  shapes terrain — ] and [ set the tempo (½, 1, 2, 4 by default, `Config.Tempos`), as sub-steps of
  one length so every tempo runs the same simulation (`Config.BiggerStep` runs one longer step
  instead). The engine pause (`Runtime.Pause`) stays the menus'. When a frame hits its cap of
  ticks for a while the tempo comes down a notch and the report says "held back".
- `RunPlan` is the interface part of a plugin's tick, run once a tick at every tempo and in the
  pause; what simulates is handed to `clock.Simulate` inside it and replayed by the engine after
  the game's `Update` as many times as the tempo says. Every built-in plugin is split so:
  movement, collision, navigation's driving, vision, the board's cells and standing, the
  atmosphere's weather and the effects simulate; views, commands, selection, shaping and the
  cameras run at once. `clock.Simulate(c, ctx, step, block)` runs the block at once with a nil
  clock, for a module run without a world. Behaviours run where their host does: every host runs
  in the simulation.
- `world.Plugin.Clock()`, `Queues`/`DefaultBindings` (the players carry the world's commands
  always), `Clock.Reporter` ("00:00 (paused)", "(x2)", "(x2, held back)") and `Clock.HUD`, a
  screen layer with the game time and the tempo. `render.Frame.Time` is the world's clock's
  (`render.Clocked`), so water, sway and rain stand in the pause and hurry with the tempo.
- `plugins/effects` moved to `plugins/world/effects`, made and installed by the world
  (`world.Plugin.Effects()`); effects last in game time. `effects.Schedule` (`Effects.Schedule()`):
  entries in code — `At(moment)`, `Every(period, offset)` — fired by clock time in the step their
  moment falls in, so a loaded game does not refire what came before the save. `clock.Phase` is
  the family of the clock's tags: an effect the schedule casts grants one to the clock's entity
  (`Clock.Entity()`) and a behaviour asks `Clock.In(phase)` to run only while it holds. `Idling`
  lost its `Base`.

**Boards and topography**
- `board.Map` is what a board is drawn and priced by beyond its cells: `Look`, `Dressing`, `Top`
  (a cell's corners and level as drawn), `Climb`, `Least` and `Slope` (what a step and the speed
  cost beyond the kind's). `board.Plugin.WithMap` sets one, `Map()`, `Top`, `Climb`, `Least` and
  `Slope` delegate; `SetLook` and `SetDressing` are gone. The board's own map is the simple map: a
  flat world from above, the ways and crossings as plain bands in their kinds' colours, a step at
  its kind's cost times the distance. `CellKind.Color` is how a kind looks without an atlas of the
  game's, `CellKindDict.Draw(name, drawer)` a drawn look; `WithRenderer(nil)` draws from the
  board's own atlas of them (`DefaultAtlas`). `board.FlatLook()` is the flat look for another
  map to fall back on.
- The board is flat: its heights, shaping, climbing and the units' altitudes are `plugins/topography`'s.
  `Plot` is the cell alone; `Relief`, `SetRelief`, `SetHeights`, `GroundAt`, `Altitude`,
  `Corners`, `Climb`, `Climbing`, `Lift`, `Flatten`, `MeanOfCells`, `Layout.Heights`, `Shaping`,
  `Raise`/`Lower`/`Level`, `WithShaping` and `WithClimbing` left it; `Board.Touch(c)` counts a
  change made beyond the board. The board is no longer a `plugin.CommandHandler`.
- `plugins/topography` is `plugins/landscape`, `plugins/isometry` and the heights in one: a map
  in relief. `topography.NewPlugin(world, board, Config{Cell, TileW, TileH, HeightUnit, Headroom,
  Isometric, Shaping, Climbing})` is the board's Map, the world's Ground and the maker of its
  cameras; `Style`, `StyleOf`, `WithShadows`, `WithSelection`, `Seed(heights)`, `Relief()`. The
  ground's heights are a `topography.Relief`: on a square grid a lattice of corners the cells
  share — no vertical walls by construction — on any other a level per cell; `Corners`,
  `SetCorners`, `Altitude`, `GroundAt`, `SetHeights`, `Lift`, `Flatten`, `Climb`; they live on the
  topography's own entity (`topography.Heights`), saved with the game. `topography.Climbing`,
  `DefaultClimbing`, `Shaping`, `Raise`, `Lower`, `Level`, `MeanOfCells` as they were in board.
- Two views, switched at play: `topography.View` (Tab) has a camera look isometrically or from
  above, keeping the ground point in the middle of the screen and a cell as wide as it was; the
  view is saved with the camera and a game begins from above unless `Config.Isometric`. From
  above the tiles lie flat and the entities as the world draws them (`world.Plugin.FlatLook`);
  isometrically blocks and billboards. `Turn` (Q, E) and `Tilt` (R and F now, PageUp/PageDown
  were) work isometrically; `Follow` (V) and `Drive` (the arrows) in both views.
- Navigation prices slopes through the board's map (`board.Plugin.Climb`, `Least`) and lays its
  route sprites on the tiles as the map draws them (`PathRenderer.WithTops`).
- The isometric view tilts no flatter than `topography.Config.MinPitch` (30° by default, the 2:1
  view's; 10° was): flatter, the near relief hides what lies behind it. In a view with depth the
  board's renderer draws only the cells on the screen, not every cell under the rectangle round
  it.
- Bindings may hold in some camera modes only: `control.Binding.In(camera.Free)` or
  `.In(camera.FirstPerson)`; `camera.ModeOf` is FirstPerson for a `camera.Rider` riding in an
  entity. The players plugin fires only the bindings holding in the camera's mode, accepts two
  bindings on one trigger in modes apart (`Binding.Overlaps`), and the shortcuts (K) list only
  what holds now, titled "first person" while riding. The camera's WASD, middle drag and edge
  scroll hold while it is free. `control.CursorMove` fires on a mouse move; while a local
  player's camera rides, the players plugin captures the window's cursor and the move reaches
  that player wherever the cursor is.
- `world.Config.Scale` (`world.Scale{Metres}`): how many metres a world unit spans. With one the
  world is a stretch of the Earth — `Scale.Drop`, the ground sinking (1 − 0.13)·d²/(2·6371 km)
  under an eye's level, the air's refraction taken in; `Scale.Horizon`; `Scale.Visibility`, how
  far one sees through the weather's air (40 km clear, far less in rain and snow), set into
  `Weather.Visibility` by `SetWeather` — and a game gives heights and sizes in metres through
  `Scale.Units`. The perspective sinks the ground far off (past the horizon out of sight), sight
  sinks the ground, the cover and the entities under the observer's level (nothing past its
  horizon is seen), and the air hazes the tiles and units far off to the sky's colour
  (`render.Frame.Haze`/`Hazed`, `camera.Eyed`). Riding in a unit the eye is on its top
  (`world.Z.Top`), never under the top of its cell as drawn, the near plane a thousandth of a
  cell. The island demo is 100 m a cell: peaks to 2 km (`island.Metres`), units 2 m tall with
  eyes at 1.7 m, the hawk 300 m up, snow from 480 m; `island.Kinds` takes the forest's height.
- Riding low in perspective, the tile the eye stands in — and those round it, and those behind it
  — had corners behind the eye, which a projection throws millions of pixels off: the tile covered
  the sky, as if the head were under the ground, and one whose middle lay behind the eye took the
  far detail, blurred. Such a tile is drawn in pieces now, those in front alone (16 a side near the
  eye), a tile wholly behind not at all, the blends and ways laid over tiles left out where not
  wholly in front, and a tile's detail taken at its nearest corner in front.
- `world.Look.Sprite` and `Drawn` take the entity's `world.Z` rather than its altitude: billboards
  stand as tall as the entity's Height. The island demo's units are giants, 3 world units (9.4 m)
  across and 20 m tall, eyes at 18 m, their sprites painted at 22 px.
- `topography.Relief.GroundAt` reads a square cell's ground on the two flat triangles its top is
  drawn as, split as `render.Frame.Fold` splits it, not between its corners: what stands on the
  ground stands on the ground drawn (a unit's eye 2 m up fell under the drawn slope on 759 of the
  island's 6144 cells, by up to 267 m at a kilometre a cell).
- Through a perspective each tile gets the detail of where it is drawn — water, smooth grounds and
  ways near the eye, the ground sheet on the horizon — rather than the whole frame the detail the
  middle of the screen has: looking far off, the river and the roads near the eye were drawn as
  bare cells. `camera.Scaler` / `camera.ScaleAt` (a world unit's size at a point, 0 behind the
  eye) size it, and the grid per cell, the units' soft shadows, the sight's shadows and the
  billboards too.
- `render.Frame.Soft`: a piece taller than 4.5 of its fades read as a blended sprite and was
  drawn white (the sight's shadows through a perspective); the blended sprites' mark is now far
  over what any fade can read (`softCap`). The sight's shadow pieces not in front of the eye are
  left out, not thrown across the screen.
- Rain slants with the wind where the middle of the screen looks, and never flatter than it falls:
  through a perspective the world's origin could lie behind the eye and the streaks ran across the
  screen as bands.
- In relief the cameras' `Bounds` and `Visible` hold the ground the screen may show at any height
  from the relief's lowest ground to `Headroom` over its highest (`topography.Relief.Extent`), not
  only at sea level: the isometric view no longer loses a strip of high ground at the bottom of
  the screen, and the perspective and first person no longer lose the ground about the eye (the
  sky showed under the hills) or everything when looking up. The sun is drawn where its direction
  vanishes (`camera.Vanisher`), not from two far points: it no longer vanishes as the eye turns a
  little or shows where it is not.
- A third view, in perspective, where the game reaches it (`topography.Config.Perspective`; Tab
  goes round from above, isometric, in perspective): an eye flying over the world, never lower
  than two cells over its highest ground, seeing `Config.FieldOfView` degrees top to bottom (45).
  WASD move it along the ground, Q and E go round the ground point in the middle of the screen,
  R raises the head and F bows it (the eye where it is; in the isometric view too, the other way
  round from before), the wheel comes in down to the ceiling and narrows the field of view from
  there, and widens it back and lifts the eye the other way. `LookFrom` and `LookAt` put the eye
  and its look where the game wants them. Follow (V) in perspective looks down more steeply where
  the ground would hide the unit and eases back as the way clears. `LookOut` (V, given the
  perspective) rides in the selected unit, first person: the eye a cell over the unit's top,
  going with it and pinned to the way it faces; W walks it on, S stops it, A and D turn it, the
  mouse looks round (`topography.Look`, the cursor captured): across turns the view at once and
  the unit to face it (`world.Driven.Face`), up and down raise and lower the head; the wheel
  narrows the view, Q, E, R and F do nothing, V or Tab leave it, back to the view the camera was
  in. Without the perspective V is Follow as before, the arrows driving. Through a
  perspective the sky's backdrop (`sky.Backdrop`) draws the sun's disc and glow where the way
  towards it vanishes, over the horizon, dimmed by the clouds; the hills draw over it. The view
  from above and the isometric view are as they were; the three share the ground point in the
  middle, the heading, the pitch and the scale there as Tab goes round, and the view is saved
  (the eye inside a unit is not: a load comes out). First-version limits: the composer sorts by
  cell as before, textures interpolate affinely across a tile, the sun glints towards one
  direction for the whole screen.
- The flat `island-demo` is gone; `island-25-demo` and `island-isometric-demo` are the island in
  relief through the topography, from above and isometric (Tab switches either).
- A click lands where it is drawn on any relief. `camera.Picker` (`Pick(sx, sy)`) is a camera
  that finds the ground under a screen point itself; the topography's walks the line of sight —
  from the eye in perspective, the Earth's curve and all, from over the highest top isometrically
  — half a cell at a time over the top as drawn (a kind's `Height` standing on its cell), then
  halves down to a thousandth of a unit. `control.Context.World` asks a Picker first; the rounds
  of unprojecting at the height of the last answer are left for other cameras, since they settle
  only on gentle slopes: on the island's 100 m cells and 2 km peaks a click fell on the wrong
  cell, off the board or on the unit's own cell. The perspective's middle ground point, which
  Turn goes round, is picked the same way, and the cameras read the drawn top, not the bare
  ground.

**Demos**
- `examples/island` is the island the board demos share, a public package: `Layout(grid)` — the
  board's layout, the heights for a topography and the stops — `Kinds(relief)` in their `Colors`,
  `Style(topography)`; its tests moved with it. `island-demo`, `island-25-demo` and
  `island-isometric-demo` are gone; in their place `board` (the island on the simple map, from
  the board's own atlas, a flat day over it), `board-topography` (the island in relief, isometric
  or from above on Tab, the weather on the ground) and `board-atlas` (a small flat board drawn
  from the game's own atlas of drawn sprites). The navigation-vision demos take the topography
  for their hills. Every demo with players has the shortcuts scene on K. Old demo binaries at the
  repo root were removed.

**Keys and the shortcuts scene**
- One key table for every game with players: W, A, S and D scroll the camera (on the screen, so
  along a turned isometric view too); Q and E turn it and R and F tilt it (PageUp and PageDown
  were); Tab switches the view from above and isometric; C has the camera follow the selected
  unit (F was); V fastens it behind the unit and the arrows drive it; Space is the tactical pause,
  ] and [ the tempo, P freezes the light, Shift+] and Shift+[ move the frozen light; Shift+W
  changes the weather (W was); a right click with Shift and S held looks there (S alone was);
  = and - raise and lower the ground, an L-drag levels it.
- `players.SceneKeys`: a scene's own keys — quit, save, the grid, full screen — with labels and
  what they do, run from the scene's HandleEvents (`Handle`). `players.Plugin.Shortcuts(keys)` is
  a ready scene listing every key of the game by plugin and the scene's under "Game"
  (`players.Written` writes a trigger); a game adds it to its stack, opens it on K
  (`Shortcuts.Open`, the engine paused meanwhile) and Esc or K closes it. The demos with players
  use it: K the list, Shift+Esc quits (Esc did), F5 saves, B the grid. The engine's full-screen
  key is F11 (Shift+F was: it shared F with the tilt held).

**Atmosphere**
- `plugins/sky` and `plugins/climate` are `plugins/atmosphere`: one plugin over the world on the
  world's clock, `atmosphere.NewPlugin(world, Config{Calendar, Sky, Climate})`, with
  `Calendar()`, `Sky()`, `Climate()`, `Renderer()` (the backdrop), `Precipitation()`, `Clouds()`,
  `Reporter()`, `HUD()`; the players carry its keys. Its RunPlan runs the light at once and the
  weather in the simulation.
- `atmosphere/calendar`: the clock at a fixed scale — a day every `Config.Day` of game time from
  the moment a fresh game begins at — so the date is saved with the clock and never jumps.
  `calendar.Year` (`GameYear`, `EarthYear`), `Season`, `Moment{Date, Time, Year}` with `OfYear`,
  `Season`, `Moon`, `Hour`, `Written`, `MoonName`; `Calendar.Now`/`At`; `Daily(hour)`,
  `Yearly(ofYear)` and `Seasonal(season)` give a schedule entry its period and offset;
  `Reporter`, `HUD`.
- `atmosphere/sky`: the sun and the moon of the calendar's hour (`Config.LightAt`), set into the
  world in `Config.Steps` a day, at the climate's zone's latitude. The light can be frozen: P
  freezes it at the hour it stands or lets it go, Shift+] and Shift+[ move a frozen light half an
  hour on or back; only the light freezes — the calendar, the weather and the schedule go on — it
  changes at once, in the tactical pause too, and is not saved. `Config.Frozen`/`Hour` begin a
  game with a frozen light. `sky.Backdrop` is the sky behind the world. The day's pace and its
  own pause are gone: the clock's tempo and pause are the day's.
- `atmosphere/climate`: the zones and the weather as they were, on the calendar's day and the
  simulation's steps (`climate.New(world, calendar, cfg)`, `Climate.System`); `Weather` lost
  `SeenDate`/`SeenTime`; `climate.Every` hears every step; Shift+W changes the weather (W was).
- `atmosphere/precipitation`: the rain and the snow, `precipitation.New(world)`.
- `atmosphere/weathering`: what the weather does to the board, once the game's own — snow lying,
  ice on the water, what sways swaying — as effects on the cells from the world's schedule, once a
  second of game time (`atmosphere.Plugin.WithWeathering(board, weathering.Config{Snowy, Ice,
  Water, Sway, Swaying, High, Seed})`); the game makes the snowy kinds and the ice, the weathering
  looks them up by name.
- The clouds' shadows are `render`'s: the `CloudShadow` material of `render/overcast.kage`,
  `Frame.Overcast`, `Frame.OvercastOn` (the landscape's were) and `Frame.OvercastQuad`, laid on
  its own over a screen quad (`Frame.Material` for any material on its own; `render.Screen` gives
  a viewport's corners and the world under them). The clouds' noise is worked out on the CPU at a
  piece's corners (`render.CloudAt`, `Frame.Drift`) and shaded between them by the shader
  (`render.CloudCover`), where every pixel worked out four octaves of noise before; a piece the
  clouds miss gets no shadow pass at all (`render.Shadowed`). The water takes the shadow off its
  light the same way, once. A flat map's `atmosphere.Plugin.Clouds()` lays a mesh of pieces over
  the screen. Full-screen on an integrated GPU this is what the frame was paying for.
- A flat world is lit too: `world.Plugin.Sunlit()` is true in a world with heights and, in a flat
  one, once something set the sun (`SetSun`) — the sky of a day going by tints the tiles and the
  sprites by the hour, night dark, dawn warm — and `atmosphere.Plugin.Clouds()` lays the clouds'
  shadows once over the screen. `board.Tile.Light` without a dressing is the sun's light on level
  ground where the world is sunlit.

**Movement costs**
- A kind's Cost 1 is full speed and the cheapest step: on the islands a road and a bridge; the
  ground off them costs 2.5 times what it did (earth 2.5, sand 4, rock 3.25, forest 7.5), flyers
  and boats still 1. `board.Climbing` gains `Ease` and `Steep`: a descent is quickest at a fall of
  `Ease` (1 in 10 by default, 0.7 as long) and slows past it by `Steep` a unit, so the steeper
  either way the dearer; the slope multiplies the kind's cost, in the planner and in the Moving
  behavior alike.

**Networks**
- Roads and bridges. `board.Crossing` is what crosses a cell over its way — a bridge over a river:
  a way of its own on the cell entity, admitting whoever it admits over too at its cost, the water
  running on under it (`Crossing.Over`, `Board.Crossing`/`SetCrossing`, `Layout.Crossings`);
  `Board.Kind` lays ground, way and crossing in turn. `network.Route` finds the cheapest way over
  a grid at a game's cost, `Network.Path` lays it, and `Network.Across(river, kind)` lays a road
  over a river as ways and crossings. The landscape draws a crossing over its way. The islands'
  roads run from stop to stop round the lowland, round the rock where they can, a bridge over
  every course they cross. Saves written before the crossings do not load.
- `plugins/board/network`: what runs from cell to cell across a board as a graph over its grid —
  nodes of a board kind, a width and a fade (`Network.Set`); `Link` for a road both ways, `Flow`
  down for water, its last cell on to where it leaves; `Links`, `Down`, `Along`, `Crossings` (where
  a road meets a river), laid on a board by `Ways()`. `water.Network.Net(kinds)` hands the courses
  over as one; `water.Network.Links` and `Along` are gone. The islands lay their water from it.

**Movement**
- `world.Steering` holds a motion profile — `MaxSpeed`, `Accel`, `Brake`, `V0`, `TurnRate` — and
  `SteeringSystem` writes the base speed every tick; navigation steers through it: a lookahead
  point, waypoints passed by projection, braking to rest on the goal, a queue of goals
  (Shift + right click), routes previewed to every queued goal.
- A unit under orders that struck someone stops, plans again and holds that route for a while
  (`MoveOrder.Bumped`, `Cooldown`), so units head-on on a road step aside instead of pushing each
  other for ever. `navigation.Plugin.WithCollision(c)`; the ground struck counts too.
- `Occupancy` is kept per domain (`CanEnter`/`Enter` take a `Domain`): `SingleOccupancy` lets one
  entity per domain into a cell, so a flyer and a walker share one; `MultipleOccupancy` stays a
  stack of tokens. Navigation seeds it from `Cell` + `Mover` at Setup, fresh or loaded — the
  demos' spawn effects are gone. island-demo uses `SingleOccupancy`.
- `world.Layers`, the planes an entity is on (one bit each; none, or no component: every plane),
  read by collision and by sight. Two colliders touch only where their layers meet; a solid cell
  stops only the layers its kind keeps out, so a wall admitting Air lets a flyer over, and the
  demos' hawk carries `Physics` on the Air layer. `Collider.Layers` is gone.
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
- How units keep out of each other's way is `navigation.Spacing`, set by
  `Plugin.WithSpacing` and read by `Plugin.Spacing()`. `CellSpacing` is the model so far: a cell
  each as the board's Occupancy says, legs, a free cell each for a group, the cell's centre.
  `BodySpacing` keeps boxes apart, several to a cell: a unit routes over the ground alone, not
  knowing where the others stand, and learns of them by striking them — it steps round whoever it
  struck towards its goal and never onto ground its domain may not take, notes the cell of one
  standing for its routes to go round, stands elsewhere round its point when that one stands on
  its spot, and after a few stalls stands where it is. A group sent to a point gets its spots
  round it, a box and a box's gap apart, the point's cell first and then the cheapest round it,
  on no ground the unit may not take and no step (the top under a corner more than half the
  unit's height off the top under its middle), the far rows to the units furthest on. One
  standing, struck by one on the move, gives way: just off the line between them, a second aside,
  then back where it stood, facing as it did. `AutoSpacing`, the
  default, keeps boxes apart when the world's largest box is at most a third of a cell. The board
  demos keep cells; `board-topography`, its units a tenth of a cell, keeps boxes.
- `MoveOrder` has `Spot` and `At`, where in the Target the unit stops and the point it was sent
  to; `Linger`, how long it stands on its Target before a queued goal, and `GivingWay`; and the
  strike and stall fields `BodySpacing` keeps. `Waypoints` are `Goal`s (`Cell`, `Spot`, `At`) and
  `Enqueue` takes one.

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
  ground under the entity plus its `Lift`.
- Vision: `Sight.Eye`; in a Quasi3D world the cone has heights (aabbworld v1.7.0: eye, entity
  bands, ground sampled every `Plugin.WithGroundStep`), so a hawk 40 up looks over a wall 10 tall,
  a forest and a hill a walker's cone stops at. `Blockers` are refused in a Quasi3D world, `Eye`
  in a flat one. Collision stays on `Layers` in both.
- In a Quasi3D world a `SightOutline` reaches the full `Sight.Radius` and keeps the ground out of
  sight as `Shadows` (up to `MaxShadowsPerSample` `Band`s per angle, from aabbworld v1.8.0's
  `View.Shadows`); the renderer fills them over the ground as holes in the view (`ConeShader`,
  `DefaultShadow`). A flat world keeps its reach cut at walls.
- The vision demos run in a Quasi3D world with a hill; aabbworld is taken from v1.8.0.
- Slopes slow a climb and speed a descent (`board.Climbing{Up, Down, Free}`, `DefaultClimbing`: a
  climb of 1 in 10 takes twice as long, Air flies over; `board.Plugin.WithClimbing`). A cell's
  slope is read off its own corners whichever way one crosses it: the board's Moving behavior
  along the entity's heading, the planner through `Board.Climb`, the slope of the cell a step
  enters, times the kind's cost (its heuristic counts every step at the steepest descent,
  `Climbing.Least`). A hex cell is level: there the step's rise between the two cells counts.
- The ground has no vertical walls: on a square grid neighbours share the corners where they meet.
  `Board.SetRelief` moves the neighbours' corners with a cell's, and a relief an effect writes into
  one cell's `Plot` is sealed to its neighbours' the same tick, and again when the effect ends.
- A flat world's ground may have heights (`Layout.Heights`): no entity stands at one, but slopes
  cost as in a Quasi3D world and the board shades them, level ground keeping its sprites' colours.
- The islands are land and water: the fields, hills, mountains and the road are gone, and a range
  of peaks up to 250, a plateau and the lowland are the heights alone — island-demo's flat island
  too. No forest grows on them until plants get a plugin; the forest kind, its snow and its
  swaying stay for it.
- Running water: `CellKind.Flow` makes a shiny kind run down the slope of its cell, as fast as the
  Flow by the square root of the slope; `board.Tile.Flow` hands the current at each corner to
  `render.Frame.Stream`, whose shader carries ripples and flecks of foam down with it and turns
  it white where it runs fast: rapids and waterfalls.
- Ways: `board.Way{Kind, Width, Links}`, what runs across a cell over its ground — a stream, a
  river, a road — on every cell entity beside `Plot` and `Ground` (saved, and an effect may alter
  it). Its kind decides who may cross the cell and what it costs (`Way.Over`; `Board.Kind` lays it
  over the ground), the ground keeps the rest. `Board.Way`, `SetWay`, `Layout.Ways`
  (`WayEntry`), `TerrainMap.Ways`; `board.Link` and `board.Toward` name a neighbour by the grid's
  direction, the bits of `Links`. `Tile.Way` cuts it into bands from the cell's middle out to
  halfway to each neighbour it runs on to, the width eased between cells, and a square where it
  turns; `Tile.DrawWay` draws them over the tile, as water running down the band where the kind
  shines.
- Kinds blend: `CellKind.Spread` has two kinds that both spread meet along a line their cells
  draw, not along the cells' edges. The landscape weighs each neighbouring kind at a tile's
  corners, the middles of its sides and its middle by the share of the cells meeting there, and
  `render.Frame.SpriteBlend` shows it where the weight is over a half, fading in over the mean of
  the Spreads: a staircase of cells becomes a slant, a cell alone a rounded diamond. A cloud's
  shadow (`Frame.OvercastAt`) is as faint as the sprite under it. `CellKind.Under` has a kind —
  water — lie under the others: a tile next to it is drawn as it, glint and all (`Tile.Base`), its
  own kind laid over by the share of cells not under, and the water's tile has the land round it
  laid over it, so a coast runs round. The islands' earth, sand and rock blend, over the sea and
  ice under them.
- A way curves: the two ways out of a cell to its widest neighbours are one band round the cell's
  middle, any other joins it curving in, so a winding stream bends smoothly from cell to cell; a
  way out to one neighbour alone ends square across itself.
- Rivers run out into the sea. `Way.Fade` (and `WayEntry.Fade`) has a way show the less the
  further it has faded, down to nothing where it ends, its water running on level ground the way
  it fades; `Frame.Stream` over a blended sprite shows only where it does. `water.Config.Plume`
  runs a course reaching the sea on out into it, a `water.Mouth` on each cell, wider and more
  faded (`Network.Fade`). `Way.Mix` (and `WayEntry.Mix`) is how far a way's look has turned into
  the kind its `landscape.Style.MixWith` names, glazed over it and blended along the band
  (`Frame.Glaze`); `network.Network.Along` is how far down its course a cell lies. `water.Drain` stops
  meandering near the sea, so a course runs straight for it instead of along the shore into
  another. The islands' running water is one fresh colour, brook to river, turning into the sea's
  down its course, all of it at the coast, and lays no plume.
- A top whose corners stand at heights of their own folds along the diagonal whose corners stand
  nearer in height (`Frame.Fold`), so a steep cell with one corner apart — a cliff along a
  stepped coast — bends towards it instead of standing up as a dark fin; what is laid over it folds
  with it.
- The board's grid is laid over all that lies on a tile (`Frame.OutlineOn`, the render's own
  `Outline` material), so coasts and grounds running in no longer hide it — only where a dressing
  covers the top (`Dressing.Covers`, `Tile.Covered`); elsewhere the tile outlines itself at no
  piece of its own, and the dressing lays it before the ways, so the frame stays in order.
- The water lies over a way: a way running out into water runs on to its middle under it, shown
  only where the land is, as the grounds round a coast are laid over the water; its look turns
  there too (`Frame.GlazeBlend`: a blended sprite glazed as far as an opacity).
- `plugins/landscape`: everything a board draws beyond its sprites leaves the board for a landscape
  (`landscape.NewPlugin(board, world)`), set as the board's `board.Dressing`
  (`board.Plugin.SetDressing`; `Tile.Base`, `Tile.Light`, `Tile.FaceLight` ask it, `Tile.Dress`
  lays what lies on a tile): the sun's light on the relief and the terrain's shadows
  (`landscape.Plugin.WithShadows`), grounds blending, coasts, water glinting and running
  (`landscape.Glint`, `Stream`, `Shore`, `Flow`), ways drawn across the cells, the clouds' shadows
  (`landscape.Overcast`) and less far off, with its materials (`water.kage`, `overcast.kage`). How a
  kind looks is its `landscape.Style{Shine, Flow, Spread, Under}`, set by name
  (`landscape.Plugin.Style`); `CellKind.Shine`, `Flow`, `Spread`, `Under` and
  `board.Plugin.WithShadows` are gone. A board alone draws its sprites in even light, and a game
  that never imports the landscape never compiles its shader. `board.NewRenderer`,
  `Board.Changes`, `Board.Square`.
- Far off, the landscape dresses the tiles from a ground sheet: under 16 pixels a cell on a square
  grid a tile's blends and ways are painted once (`render.Paint`), 16 pixels a cell below a copy of
  the board's atlas, and drawn as one piece of it (`Frame.SpritePart`); a cell is painted anew when
  it or a cell round it changes. `board.Dressing.Sheet` hands the renderer the tiles' sheet. A
  way's water eases out between 24 and 16 pixels a cell, and from far its curves are cut in fewer
  pieces where no sheet is painted. The clouds' shadow is laid once over a tile's top, after all
  on it (`Frame.Last`, `Frame.OverlayOn`, `landscape.OvercastOn`), not once per piece. The islands
  zoomed out draw within a few percent of what they did before blends and ways.
- The composer's shader is put together: `render/compose.kage` (sprites, fades, outlines, blends)
  and every material a plugin registers with `render.RegisterMaterials` from its own Kage, compiled
  once (`render.Compile`, `render.ShaderSource`). `Frame.Overlay` lays an overlay for a material.
  The landscape brings the clouds' shadows and the water (`landscape.Overcast`, `Glint`, `Stream`,
  `Shore`, `Flow`); `Frame.Glint`, `Stream`,
  `Overcast` and `OvercastAt` are gone, and so are the alpha marks 2, 4 and 5: an overlay carries
  its material's number.
- Far off, less is drawn: a tile's detail (the landscape's) eases a tile's water glint and running out between 12
  and 6 pixels a cell and its shore below 16, and the shader leaves out waves finer than a pixel
  or two (`Glint.z`, world units a pixel spans) — the whole island far off composes in about 2 ms.
- Running water is a flow map: noise in the world carried down the current in two crossfaded
  phases, seamless from piece to piece; the faster, the rougher, more flecked and at last white.
- `Board.CellVersion` counts the changes to each cell alone; the board's renderer keeps what it
  read of a cell, a tile's blends and way and its light until they go stale, so composing the whole
  island takes half what it did (`Benchmark_Board_Island`: 3.0 ms from above, 4.5 ms isometric).
- `render.World`, where each corner of a piece lies in the world: `Frame.Stream` and the new
  `Frame.OvercastAt` lay over a quad of any shape.
- `plugins/board/water` drains a relief to the sea: `water.Drain(grid, heights, sea, Config)`
  floods it from the sea up over every neighbour of a square grid's cells, across the corners
  too, each level nudged a little (`Meander`) so courses wander; gathers the rain downstream and
  lays brooks, streams, rivers and fords across them. `Network.Links` and `Network.Width` (by the
  square root of the water, a cell at most) are a course's way; `Network.Carved` cuts their beds,
  falling all the way to the sea, into the heights.
- The islands' running water is ways over their ground, worked out by plugins/board/water: a
  brook is stepped over (1.3), a stream waded (2), a river crossed only at a ford (2.5), each the
  wider the more water it carries and running slantwise where the land does; water leaving over
  the northern cliffs falls into the sea. The lowland rises gently from the sea and swells, and
  the plateau is flat at 118.
- The islands' ground is earth, sand and rock, laid by the heights and the coast: rock where the
  ground is steep or high, sand on beaches and dunes behind them, earth elsewhere; sand (1.6) and
  rock (1.3) cost on top of the climb, and each has its snowy look. Stretches of sea cliff, most
  along the north coast, rise up to 78 straight from the sea, the high ground behind them sinking
  inland.

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
- The box of a selection being dragged is selection's: a `selection.Marquee` command, issued by a
  `ButtonHeld` of the left button (whose `Context.Start` is now where the button went down), shown
  per camera until the `Select` that ends it and drawn by the selection renderer. players draws
  nothing (`Player.DragBox` and `players.Renderer` are gone); its input layer is the
  `eventHandler`, which keeps each local player's keys and buttons apart from `Player`.
- The cameras live in `internal/camera` (`basic_camera.go`, `iso_camera.go`); the root package
  `camera` is the contract and the projections (`Isometric.WithDefaults`), and a game gets its
  cameras from the world (`world.Plugin.Camera`, `NewCamera`). `camera.NewFromSpace` is gone.
- Screen rectangles are `geom.AABB`: `render.Viewport.Area`, `render.Whole`,
  `game.Viewer.Viewports`, `players.Layout`, `players.Columns`, `Player.Area`.

**Composer**
- The world of a scene is one `render.Composer` over the plugins' renderers, which are
  `render.Source`s (`Compose(frame, cam)`): each hands its pieces to a `render.Frame` — `Sprite`,
  `SpriteRect`, `Line`, `Fan`, `Soft` — with a `render.Tier` (`Ground` 100, `Objects` 200,
  `Overlays` 300, `Marks` 400, a game's own between) and a depth. Tiers are drawn in order; through
  an isometric camera everything below `Marks` back to front by depth, so a hill hides the routes
  and the cones behind it and the selection stays on top. Gone: `render.Sorted`, `Submitter`,
  `Sink`, `Overlayer`, `QuadBatch`, `LineBatch`; a `Source` listed straight in a scene is refused,
  naming the composer.
- One Kage shader draws every piece; a plain colour samples its sheet's white texel
  (`AtlasSource.White`, baked by `Atlas.Close`), so a view is one call per sheet. Lines fade their
  sides over a pixel; the shadows of sight fade in at their ends and at an exposed side
  (`vision.Shadow`, `Plugin.WithShadow`, `DefaultShadow`).
- A square board's grid is the tiles outlined by the shader along their own edges
  (`Frame.Tile`, `Frame.TileRect`), not lines: no pieces of its own, a pixel wide at any zoom, on
  the raised tops of walls and hidden with them. CPU drawing with the grid on: island-isometric-demo
  2.5 ms → 1.9 ms, navigation-demo 0.67 ms → 0.13 ms. A hex grid keeps its lines.
- `vision.ConeStyle.Compose(frame, ring)` takes the view's ring of `ConePoint`s, draped over the
  ground with the cone's edges in steps of it, each point with its depth; `ConeShader` is gone.
  `selection.HighlightStyle.Compose(frame, cam, box, alt)` composes on `Marks`.
- The island demos' frames are 25–60% shorter; gathering 5,000 sprites
  costs about 18% more (BENCHMARKS.md). An isometric camera no longer allocates for `Projection`.

**The isometric view is a plugin**
- `plugins/isometry`: `NewPlugin(world, Config{Cell, TileW, TileH, HeightUnit, Headroom})` puts the
  world in the isometric view — its cameras (`world.Plugin.SetCameras`, `world.Cameras`) and how its
  entities lie on the screen (billboards) — and `WithBoard(board)` lays the cells as blocks with
  faces. Without it everything is drawn from above; heights stay the world's and work in both.
  `camera.Config.Projection` is gone and `camera.Isometric` and `render.Billboard` with it: the
  isometric projection, camera and billboard are private to the plugin. `camera.Projection.Sorts`
  tells the composer to sort by depth.
- How things lie on the screen is a swappable `Look`: `world.Look` (`Sprite`, `Drawn` for picking,
  `Footprint` for outlines; `Plugin.SetLook`, `Look`) and `board.Look` (`Cell`, handed a
  `board.Tile` with its box, sprite, `Top` and `Beside`; `Plugin.SetLook`, `Look`). The renderers
  keep their data and ask the Look for geometry only. `selection.HighlightStyle.Compose(frame,
  footprint)`; `NewSelectionSystem` and `NewRenderer` take the world's Look. Selection, navigation,
  the board and the world renderers no longer test for an isometric camera.

**Light**
- `world.Sun` (`Dir`, `Strength`, `Ambient`; `Sun.Light` for a surface's normal), `DefaultSun`,
  `world.Plugin.SetSun`/`Sun`. In a world with heights the board lights every tile per corner from
  the slope of the ground there and at its neighbours (`board.Tile.Light`, `Tile.FaceLight`), so
  slopes run on smoothly and a map drawn from above shows its relief; the isometric blocks take the
  same light, and `DefaultSun` keeps their old look. A flat world is drawn as its sprites are.
- `island-25-demo`: the island of island-isometric-demo, the same Quasi3D world, drawn from above —
  a map in relief under the sun, sight with heights; island-demo stays the flat 2D island.
- The terrain casts shadows: a tile corner the ground or what stands on it hides from the sun gets
  the ambient light alone (`Sun.Shaded`), a face as much sun as its top's edge. Worked out as cells
  come into sight — a walk towards the sun over the frame's tops, stopped above the highest top
  within 16 cells — and kept until the terrain or the sun changes: a 96x64 board anew in 2.2 ms,
  nothing on a frame after. `board.Plugin.WithShadows(false)` turns them off.
- Units cast shadows: in a world with heights the world's renderer lays a soft patch under each
  entity with a `Z`, away from the sun, stretched by its Height and pushed off by how far above the
  ground it stands, over the ground and under what stands. The renderer's Drawing accessor is bound
  once, so a frame allocates no method value per chunk.
- `plugins/sky`: a day going by. The time of day is a `sky.Day` on the sky's own entity, saved
  with the game; every tick it moves on at its pace, and at every step (`Config.Steps`, 96 a day by
  default) the world's sun becomes `sky.SunAt` the hour — rising in the east, over the south at
  noon, setting in the west, below the horizon at night, strength and ambient light rising and
  falling with it. `Pause` (P) stops the day where it is and lets it go on; `Forward` and `Back`
  (] and [) double and halve its pace while it goes by and move it half an hour on or back while
  it stands. island-25-demo and
  island-isometric-demo have days; a unit's shadow is capped at 6 units a unit of height, the sun
  on the horizon would cast it for ever.
- `CellKind.Shine`: a shiny kind glints where its surface faces halfway between the sun and the
  eye (`camera.Projection.Toward`). The glint is worked out per pixel in the composer's shader
  over small waves running across the surface as time goes by, so a sea twinkles under a high sun
  and hardly at all under a low one; within 3 cells of a shore — the nearest cell that does not
  shine — the waves turn to face it and roll in, arriving at different times along the coast, and
  break into foam the last cell before it: the surf shows on every shore whatever the sun, dimmed
  with the light at night. A look hands `Tile.Shine` and `Tile.Shore` to
  `render.Frame.Glint` after the tile, which lays a quad of light over it; the board hands the
  frame its sun (`Frame.Sun`). The islands with heights give their water 0.9. A save of an older
  Ground does not load.
- The isometric camera turns by any angle, the heading saved with it: `isometry.Turn` (Q and E
  held, 2° a tick). `isometry.Follow` (V, with `Plugin.WithSelection`) fastens the camera behind
  the selected unit: centred on it and turning, eased, until the way it walks runs up the screen,
  whatever else is selected, ordered, panned or turned, until V again — a game walked behind a
  character's back. `isometry.Tilt` (PageUp/PageDown) has it look down from 10° to 90°, the 2:1
  view at 30°; a fastened camera holds its unit lower on the screen the lower it looks.
  `isometry.Drive` (the arrows) steers the fastened unit through a new `world.Driven`, which
  navigation carries out: turning, walking on the way it faces, stopping dead before a cell its
  domain may not stand on or the occupancy keeps it from — no walking into the sea — ending any
  order it had, its Cell and occupancy kept with it. The commands carry the camera of whoever gave
  them, so the plugin — now a
  CommandHandler with a RunPlan — turns a player's camera without knowing players. selection's
  test of picking through a Look no longer imports isometry. The blocks show
  whichever faces look towards the eye; the depth is how far down the screen the middle of a cell
  lies, the same order as before unturned.
- `plugins/climate`: the climate of a world. Its `Zone` — `Latitude` and other `Factor`s
  (`SeaCurrent`, `DrySummer`; a game adds its own) — is its climate in numbers (`Zone.Profile`: the
  mean temperature, the year's and the day's swing, how wet each season is); `Equatorial`,
  `Tropical`, `Mediterranean`, `Temperate`, `Cold`, `Polar` are the world's zones.
  `climate.NewPlugin(world, sky, Config{Zone, …})` sets the sky's latitude (`sky.Plugin.SetLatitude`:
  the sun's path worked out for it, the days long in summer, polar day and night past the
  circle). Its weather (`climate.Weather`, saved, dice and all) goes from one kind of weather
  (`climate/weather`: `weather.State`, `weather.Default`: clear, cloudy, rain, storm) to the next
  by weights, how likely each comes in the season (`State.Often`, storms in summer) and, for a wet
  one, how wet the zone has the season; a fresh Stage begins in `Config.Start` or in a
  state thrown as the season has them, already at its clouds and fall; the wind, the clouds, what
  falls and the temperature (`world.Weather.Temperature`: the year, the hour, `State.Warmth`)
  blend into each state's, the wind's way wanders, the clouds drift on it, and what falls comes
  down as snow below 1°C. It goes by in the sky's time (the day's `Pace`, held while the day
  stands), and so do its behaviours (`Tick.Dt`). `Change` (W) and `Set{Name}` script it; `climate.Every` hosts a game's
  behaviours, told the weather and the season every tick; `Plugin.Renderer` draws the rain and
  snow falling (`render.Air`, tier 350); `Plugin.Reporter` adds a telemetry line. It sets
  `world.Weather` (`SetWeather`), which the renderers draw: the clouds' shadows drifting over the
  ground (`Frame.Overcast`), the sun's glint put out under a cloud, the waves turning with the
  wind and as steep as it blows, the sky and what water reflects of it greying under clouds
  (`render.Overcast`), and whatever sways leaning with the wind (`CellKind.Sway`,
  `world.Appearance.Sway`, `render.Sway`; `world.Look.Sprite` takes the sway).
- Snow, ice and trees swaying are effects (`plugins/effects`) a game casts as the weather says,
  as effect-demo's frost: both islands with heights define snowy kinds and ice and cast snow,
  ice and sway on the board's cells from a `climate.Every` behaviour (`climate.go`) — snow
  settling in drifts while it snows in the frost and melting away patch by patch once warm, ice
  growing from the shores in a hard frost, the forest swaying in a strong wind.
- `plugins/sky` keeps a year: `Day.Date`, `Day.Calendar` — `GameYear` (8 days, the moon round
  in 4, the default) or `EarthYear` (365 days in twelve months, the moon round in 29.5,
  `Day.Written` "20 March") — `Day.Season`, `Day.Moon`, and `Day.Length`; a fresh Stage begins
  in the middle of `Config.Season` (spring by default); the sun goes the way it goes at the
  latitude (30° without a climate): 23.44° higher at noon in summer and lower in winter, the days
  longer and shorter. `Config.Noon` and `Tilt` are gone. `Config.SunAt(ofYear, t)`.
  At night the moon lights the world (`Config.LightAt`): going the sun's way as far behind it as
  it is round, as bright as it is full and high, in a paler light — the terrain shaded and shadowed
  by it, the sea glinting. Units' shadows are as dark as the light is strong. The clock reports
  the season, the date and the moon.
- The weather goes by in the sky's time, worked out from how far the day has moved since it last
  looked (`Weather.SeenDate`, `SeenTime`, `Day.Length`): a day hurried on hurries it, a day moved
  on while it stands moves it with it.
- `world.Lamp` (`Sun.Lamp`): the sun worked out once for many surfaces; the board lights with it,
  which, with a corner's light taken as it is when a piece is not split, brings back most of what
  coloured light cost.
- Light has colour. `world.Sun` has `Color` (the sun's light) and `Sky` (the sky's colour and the
  light every surface gets from it, `Ambient` of it); zero is white, so `DefaultSun` looks as
  before. `render.Shade` is a `render.Light` (RGB) per corner (`render.Even`, `render.Lit`),
  `Sun.Light`/`Shaded` return one, and entities are drawn in the sun's light
  (`world.Look.Sprite` takes it). `plugins/sky` colours the day from a table by the sun's height:
  a blue sky by day, orange sunrises and sunsets, deep blue nights. `Frame.Sun` is
  `Frame.Daylight` (`Sun.Daylight`).
- Water reflects the sky, the more the flatter the eye looks at it; its glint is the sun's colour,
  its foam lit by the sky and the sun. `Frame.Glint` takes the kind's shine and the sun reaching
  each corner (`Tile.Shine`).
- `sky.Plugin.Renderer` is the backdrop: the screen in the sky's colour behind the world
  (`render.Backdrop`, tier 0), drawn only when the ground does not cover all of it. Both islands
  with heights use it.
- `sky.Config.NoonWay`: the way the sun stands at noon (`sky.Way`: `NorthWest`, the default, and the
  seven other ways), its whole path turned with it; north-west is beyond the sea as the isometric
  view looks, so the sea glints. `sky.SunAt(t, noon)` is `Config.SunAt(ofYear, t)`.
- A plugin can add lines to the telemetry: `render.Reporter`, handed to
  `TelemetryRenderer.With`; `sky.Plugin.Reporter` shows the time of day (and the pace when it is
  hurried or held) in both islands with heights.
- `render.Shade`, a brightness per corner, blended across a piece: `Frame.Sprite`, `Tile`,
  `SpriteRect`, `SpriteRectUV` and `TileRect` take one (`render.Even(1)` draws a sprite as it is).

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
- Terrain is no longer an entity in the world's space: the board is the world's solid ground and
  cover (`world.Field`, `world.Cover`, `world.Plugin.SetField`/`SetCover`, on aabbworld v1.9.0's
  `collide.Config.Field` and `Cone.Cover`), read from the cells whenever collision or sight asks.
  A cell changed counts from the next tick and costs nothing to change; a tick of sight and
  collision over a quarter of a board rough costs about the same laid out in one block or
  scattered (3.5 ms and 4.3 ms, where the merged bodies cost 4.5 ms and 11.1 ms, and a whole
  rebuild with 330 KB of garbage on every change). `Solid` and `Veil` are independent: a solid kind
  cuts sight only with a `Veil` of 1 (the demos' walls have one), a thicket dims and is walked
  through, a forest may be both. A unit slides along a wall of many cells without catching on the
  seams. A contact with the ground is a `collision.Contact` with `Terrain` and `Cell` set; a
  `Meeting` is still between entities. Gone: `board.Plugin.WithCollision`, `Body`, `Collision`,
  `board.Family`, `MaxBodyCells`; the demos' `MaxEntCount` counts their units alone.

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
- Solid terrain pushes units out and veiled terrain dims sight, read from the cells (see Terrain
  in the ECS); a hex is covered by `Grid.CellBoxes` — a middle band and capped strips.
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
  them as values; `Selectable` and `Selected` and the vision behaviors' tags are bits. `navigation.NewPlugin` takes the selection plugin.
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
- A tick running a `world.Moving` behavior allocates nothing any more (the per-chunk accessor was a
  fresh method value).
- The board's grid is drawn in one batch (now `render.Frame.Line`) instead of one
  `vector.StrokeLine` a line, and is left out where a cell spans fewer than 6 pixels. island-isometric-demo draws a frame in 4.2 ms of CPU instead of
  6.9 with the grid on, and 5.9 instead of 17.5 zoomed out to the whole island.
- A square grid's `CellsUnder` walks the rows and columns a box touches instead of sampling a
  lattice into a map, and counts the cells across a wrap seam, which the lattice missed; the board
  renderer finds the cells in view the same way (on a hex grid, by sampling marked in a slice).
  A frame's drawing costs 10–25% less CPU in the island demos.
- The board renderer reads each cell's heights, kind and sprite once a frame, not again for every
  neighbour's face: island-isometric-demo draws the whole island in 5.5 ms of CPU instead of 6.9.

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
