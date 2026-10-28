# Changelog

## v0.7.0 — 2026-10-28

Needs astar v1.2.0 (a search's states in arrays by an integer id, a hand-written heap, one
dictionary for the open and the closed, the same paths; the A* contract stated).

**A step's weight is a share of the top speed** (2026-10-28, branch `feat/pace`,
`doc/plans-perf.md` „Udziały tempa")
- Breaking: a kind of cell says the share of its top speed a unit keeps on it, `cell.Kind.Pace`
  in [0, 1] (`Paces` by domain, `Paced`, `PaceFor`), in place of `Cost` (1 and above, times as
  long) and `Costs`, `Costing`, `CostFor`; `cell.Floor.Pace` and `bridges.Spec.Pace` likewise. A
  kind admitting someone at no pace, or at a share over 1, is refused as it is defined.
- Breaking: how fast a unit goes is the game's `cell.Mobility`, given beside motion
  (`board.Plugin.WithMotion(m, mobility)`; `NewUnits` panics without one): `Pace(cell.Step)` is
  the share on a step onto a kind, on its level, for a domain, over the slope between the two
  cells' middles, 0 refusing the step; the factors multiply. `cell.DefaultHiking` is Tobler's
  hiking function cut at the flat: half the pace at a rise of 1 in 5, a descent never quicker
  than the flat, Air flying over. A game's own domain bits tell its classes of unit apart. The
  planner (a step's cost its length over the share, the heuristic the grid's distance), the
  board's pace (`steering.Pace.Share`, 0 where the step ahead is refused) and the driving (a
  refused step not driven, under gravity too) read one strategy, so what the planner refuses a
  unit never walks; a descent is never quicker than the flat, so `MaxSpeed` is a maximum.
- Breaking: `board.Map` keeps the geometry alone, `Rise(from, to)` in place of `Climb`, `Least`
  and `Slope`; `topography.Config.Climbing` and the package `plugins/topography/relief` are
  gone; `pathfind.Slopes` is `Pacing`; `spacing.YieldClimb` is `YieldShare`: a step aside is
  given way where its slope leaves the unit a third of its pace on the level, so a flat forest
  is stepped into and a cliff is not.
- Breaking: a save from before holds costs where shares are read: it loads, and the first step
  panics naming the kind. Start the game afresh.
- The demos: navigation/hex and square give `cell.DefaultHiking`; board/topography walks by its
  own `madeira` (the plateau's units highlanders climbing to 2 in 1 where the walkers stop at 1
  in 1, up or down, the hawk over all); board/custom-map keeps its hills' heights in its map and
  their climb in a mobility of its own; rules/effect's snow slows a walker to a third and carries
  the witch at full speed, ice to a half.
- The planner keeps a cell's share on the level across searches by the board's version, as it
  keeps its answers (`Finder.Searches` counts the searches); a step aside's slope gate costs
  nothing on a flat board.
- `plugins/navigation/testdata/routes.golden` pinned anew: a descent is no longer cheaper than
  the flat.
- Measured (`main` v0.6.0 with astar v1.1.3 against the tree with astar v1.2.0, alternately on
  an idle machine, the lower of three rounds; `BENCHMARKS.md`): `Navigation_Tick` crowded 1,000
  walkers 8.83 → 7.93 ms (−10%, at `BENCHTIME=200x`), crowded 100 −11%, open 100 −8%, open 1,000
  within noise; `Navigation_Path` −2.5%; `Routing_Path` 15.8 → 15.2 ms, `Routing_Routes` 255 →
  211 ms on one goroutine (−17%) and 87.8 → 75.8 ms on the pool (−14%); `PathFinder_Terrain`
  −20…−22% (cells 6.78 → 5.27 ms), its new `relief` rung, the hiking function asked for every
  step, about 9 ms. The standing pass pays the strategy a walker a step, the hiking function
  over the rise to the cell ahead: `Board_Altitude` and `Gravity_Standing` +9…+10% at 1,000
  walkers on a relief (367 → 404 µs), within noise at 100.

## v0.6.0 — 2026-10-27

Needs aabbworld v1.19.0 (kjkrol/aabbworld#24: `Cone.Heights` and `Cone.Bend`, the scan's cone
query, its slabs and the relief walked with SIMD) and ceremony v0.3.2 (kjkrol/ceremony#4:
`host.PairRules.For`).

**Sight in SIMD and only where it is read** (2026-10-26, branch `perf/sight-simd`,
`doc/plans-perf.md` „SIMD w widzeniu")
- Breaking: vision scans only the observers something reads — those a hosted rule of a
  `Sighting` is for (they play the rule's role), those carrying `SightOutline`, and those carrying
  the new marker `vision.Watched{}`, which a game puts on a kind whose `Sighted` it reads itself;
  `Plugin.WithScanAll()` scans every observer; a log (`WithLog`) reads nobody and shows what is
  scanned, so navigation/square and hex log nothing (no one reads their scouts). An observer no
  longer read has its `Sighted` cleared once and no `Sighting` dispatched. Whether a rule is for
  an observer is ceremony's `PairRules.For` of its tags. The views drawn on the GPU read no scan.
  board/topography's 37 relief scans a tick are gone: nothing there reads them. Breaking too:
  `vision.NewScanSystem`, exported and unused, is gone.
- Sight over the topography reads the relief's corners inside aabbworld's scan (`Cone.Heights`
  over the relief's own heights, re-read every scan, wrapping as the world does, the world's
  curvature as `Cone.Bend`) in place of calling the relief's `Along` ray by ray: the same
  sightings to the last bit (`Vision_Relief`: `along` keeps the old path to compare). Grounds
  without a lattice (hex boards, a game's own) go through `ground.Profile` or a sample every
  `WithGroundStep` as before.
- With aabbworld's cone query, its slabs and the relief walked four rays at a time (the start-up
  log, `gram: SIMD on`, names the sight beside the collision sweep):
  `Vision_Scan` 100 observers 79 → 73 µs (serial 129 → 106), 500 observers 295 → 249 µs (serial
  596 → 484); `Vision_Relief/profile` 1.76 → 1.21 ms, at 15 Hz 774 → 495 µs; with outlines 2–11%
  less. The ground sampled through callbacks (`step=cell`, `step=50m`) is 2–6% slower in the SIMD
  build with the scan in parallel alone — on one CPU 2–4% faster, in the plain build within 2% —
  the AVX2 kernels on every hyperthread lowering the clock for the scalar callbacks; grounds with
  a lattice no longer take that path.
- Benchmarks scan all (`WithScanAll()`): `Vision_*`, `Board_Terrain`.

- Fixed: a picture in layers drew vision's views in its outer layer alone (the whole frame at
  60%), the layers drawn after covering them: a cone's edge was seen only round the frame's
  edges, thick and blurred, and gone where the middle layer begins. The views are drawn in
  every layer, the sight baked once a frame; a Direct's Draw runs once a layer, so what its
  Compose gathered serves every one (`render.Direct`).

**The controls screen** (branch `fix/hot-corners`)
- Breaking: the shortcuts scene is the controls screen, `players.ControlsScene`, `ControlsName`
  (`gram.controls`) and `ShowControls` (K, labelled "Controls"), laid as the settings screen is:
  the title in a framed panel at the top, a framed section a group ("Game", a plugin each,
  "General") with a heading and a line a binding, the trigger in a column and what it does
  beside, wrapped past 300 px, in as many columns as the width takes, scrolled where they do not
  fit. One layout serves both screens (`menu.go`). The game's `Props.Description` is shown
  nowhere any more.
- `render.DefaultLargeFont`, a title's, is Go Bold at 28 (was 20): the settings screen's own
  theme is the default theme now, and both screens are drawn in it.

**Hot corners, the minimap's click, macOS** (branch `fix/hot-corners`)
- `ui.Element.Passing`: the pointer passes through the element and all in it to whatever lies
  under, a button or a control inside aside. A HUD laid over the picture covered it: a label, a
  picture and a Blank are hit over their whole box, so the player's picture under was covered —
  no edge scroll, no wheel, no click there — though the cameras' zones are drawn over the HUD.
- The atmosphere's `TimeOfDay`, `Date`, `Weather` and `Bar` pass the pointer: the top scroll strip
  works under the bar. The bridges' `Reason` passes too; the demos' PAUSED word likewise.
- The minimap's compass passes the pointer and hides with the minimap (`Write.Show`): laid over
  the minimap in its box reaching the screen's corner, it took every press (a click on the
  minimap never reached `cameras.CenterAt`, promised since 2026-10-19), the wheel, and the corner
  zone under it even where the circle left it uncovered. The scene test lays the compass as the
  demos do.
- The engine switches full screen at the window's next pass, on the thread that owns the window
  (AppKit takes it on no other); `GRAM_FULLSCREEN` works on a Mac, where the window's config
  carries no full screen. The controls screen names the platform's key: `control.FullscreenKey`,
  the engine setting Ctrl+Cmd+F (the window's own menu) on a Mac, where the system keeps F11.
- The mouse look while riding works where the window gives no deltas for a caught cursor (macOS:
  gogpu's pointer lock is a stub there): the look follows the cursor's place, which stays visible
  and stops at the screen's edge; a window that does give deltas is noted at the first and never
  falls back. Where the window fills the screen, a cursor reported on the far edge (Cocoa's flipped
  origin puts the bottom row at the height) counts as the last row.
- `GRAM_INPUT_LOG=1` logs the cursor's place, the window's sizes and flags once a second and every
  click, to read what the engine sees on a platform at hand.
- The cursor out of the window scrolls nothing: the engine reads the window's leave and enter
  (gogpu's `PointerLeave` and `PointerEnter`, on every platform) into
  `control.InputEvents.CursorInWindow`, and the players' handler takes nothing of the cursor's
  place while the window has it not. The scroll ran on when a flick of the mouse left the window
  through a strip, and in full screen on one of two monitors.
- The scroll zones reach the window's edge in every mode: the dead strip at the edge
  (`control.EdgeDeadZone`, 10 px, the hack for the above) is gone, so a cursor pinned by the
  monitor's edge on a window touching it scrolls at full speed. Breaking: `EdgeDeadZone`,
  `Context.FillsScreen` and `InputEvents.WindowFillsScreen` are gone; a handler fed by hand (a
  test, a probe) says `CursorInWindow: true` for the cursor's place to count — a zero
  `InputEvents` has no cursor, so a probe never turns the world from (0, 0).
- A game may let the right button grab the world and drag it (`cameras.Config.Grab` for the
  defaults, `Keys.Grab` for keys of its own; off by default, on in board/topography and
  board/flat): past a click's slop, yielding to whatever else takes the right drag. For that a
  binding may yield (`control.Binding.Yielding`: it fires only when no other on its trigger
  built a command; `Player.Bind` takes one beside a plain one), the click's slop is
  `control.DragSlop` and `Context.Dragged`, and navigation's right drag turns the selected units
  only while the player has any (`selection.Read.Any`): with nothing selected the drag grabs the
  world, with a selection it turns the units, a right click sending them as before.
- The corners turn the world in pairs across the diagonals (`cameras.Keys.CornersTurn`): the
  bottom-left and the top-right anticlockwise as Q, the bottom-right and the top-left clockwise as
  E, so the world under the cursor slides along the top or the bottom edge towards the middle; the
  bottom corners turn as before, the top ones the other way round (they turned as the corner under
  them).

**The camera behind a unit on an arm, V without C, the zones' opacity** (branch `feat/camera-arm`)
- The camera behind a unit is held on an arm the game sets (`cameras.Config.Arm`, saved as
  `Settings.Arm`, the command `Hold{Arm}` from the settings screen): `Pitch` degrees over the
  ground in perspective (5 to 89, `DefaultArm` 30), `Distance` cells (0.5 to 8, 2), `Aim` of
  the unit's height over its feet the camera looks at (0 the feet, 1 its top, up to 3: the
  higher, the lower the unit on the screen), and whether the player may tilt it with the mouse
  (`Tilts`) and draw it in and out with the zoom keys and the wheel (`Zooms`): a Zelda, a racing
  game or a Warcraft camera by three numbers, the player's hands on or off. The settings screen
  lists them under Cameras: "Tilt behind the unit" with the pitch beside, "Zoom behind the unit"
  with the distance beside (each slider live while its switch is on), "Aim behind the unit". V
  goes round Centred → Behind → Inside → Centred (`Ride`). The camera behind a unit is the
  perspective's alone: V takes a camera in the orthographic view into the perspective, and
  letting go (C) gives the view it came from back with its scale and its stick
  (`viewCamera.tactical`, saved with the camera, which is saved as "stick 4" now: a camera saved
  before begins afresh); a camera whose Config reaches no perspective is fastened behind nothing
  and binds no V (navigation/hex and navigation/square lose it); Tab, R and F hold where the
  hand is free alone (`In(camera.Free)`), Q and E turn the camera round the unit.
- Third person as two sticks, the keyboard the unit's and the mouse the camera's: behind a unit
  W, S, A and D walk it the way the screen lies (`driving.Compass` `In: camera.Behind`, its
  `Toward` turned by the camera's heading, `camera.Headed`, in the hand; `Toward.Sprint` with
  Shift), the unit turning to face its way, and the mouse turns the camera round it
  (`cameras.Look` `In(camera.Behind)`, the keeper's `orbit`): the view holds while the unit
  stands and a second after the mouse moved, and comes back behind the unit as it walks on. The
  arrows a tank's behind it. The mouse up and down tilts the arm where `Arm.Tilts` lets it, under
  the tactical pitch floor too (the floor taken back on letting go). Space jumps riding and
  behind. The cameras' scroll keys hold where the hand is free (`camera.Free`); the cursor is
  caught behind a unit as it is inside (a `CursorMove` reaches the player wherever the cursor
  is); the controls screen is titled "third person" there. look/material's own tank on W, S, A
  and D holds where the hand is free (`camera.Free`), the defaults driving behind.
- V works with the camera loose: `selection.Plugin.RideKey` (V among its DefaultBindings, held
  `In(camera.Loose)`) builds `cameras.Ride{Camera, Entity}` of the one unit chosen, and the keeper
  fastens the camera Behind it at once; a fastened camera goes round its own. C from behind or
  inside a unit goes Centred over the chosen unit; from Centred it lets go as before.
- The demos whose units start under orders (a target from the map, a patrol) start them
  selected no longer (`selection.Allow{}` on the player's side: board/atlas, flat, custom-map
  and topography, look/light, navigation/square and hex, rules/guard); a unit is chosen with a
  click before C or V. navigation/spawn, rules/dialog and start/split-screen keep theirs.
- The zones and the pointer's pans hold only where the hand scrolls: `camera.Free`
  (`Loose | Centred`) gates the edge, the corners, the middle drag and the grab, and the zones
  are drawn there alone; behind or inside a unit an edge neither scrolls nor lets go of the
  fastening. `Outside` stays every How but Inside (Q/E); R/F and Tab hold where the hand is
  free.
- The zones' opacity: "Zones opacity" on the settings screen beside "Show the zones", live while
  it is on (`cameras.ZonesOpacity{Opacity}`, `Settings.ZonesOpacity`, `DefaultZonesOpacity` 40 of
  255, `LeastZonesOpacity` 5%). For that a Range may lie beside a Switch (`control.Range.Beside`,
  the settings screen laying the slider in the switch's row) and an element may be live only
  while a function says so (`ui.Element.Live`: else drawn dimmed, the theme's text, borders and
  marks at half their alpha, and passed by the pointer).
- board/topography starts its telemetry panel with the basics as every demo does (it forced
  everything).

**Shaders on Metal, naga v0.20.3** (branch `fix/hot-corners`)
- Fixed: on a Mac every demo panicked at its first pipeline ("metal: shader module has no WGSL
  source"): wgpu's Metal backend translates a shader module's WGSL to MSL and takes no SPIR-V,
  and gram handed every backend the SPIR-V it keeps on disk since v0.5.0. The SPIR-V goes to
  Vulkan alone, which takes it as it is; Metal, DX12 and GL get the WGSL (`render.UseDevice` is
  told the backend gogpu names, a headless device asks its adapter). The shaders a stage made are
  compiled at once, as many as there are CPUs (`render.Prepare`, `gpu.CompileAll`), which a
  first start and every start on Metal feel: board/topography's shaders and uploads 810 ms one
  after another, 255 ms at once on 8 CPUs, 17 ms from the cache (45 ms read one after another).
- naga v0.20.3 (was v0.19.0): a module-scope vector `const` in a runtime expression, `+=` on an
  element of a local array and `all()`/`any()` work, `render/internal/gpu/compile_test.go` guards
  them; a `var` without an initialiser is undefined still (its OpVariable has no initialiser).
  wgpu's write at an origin still spoils the texture left of it, so a big part is still drawn
  into place from a texture of its own; a whole texture's write no longer submits a command
  buffer of nothing after it.

**The benchmarks measured anew** (2026-10-27, `main`)
- `BENCHMARKS.md` holds the numbers at v0.6.0 alone: every benchmark measured again (the median
  of 5 runs, `make bench-save`, the later benchmarks of a long run measured again apart, the
  machine warm by then), CPU profiles of the main ones, the demos' frames and collector at
  1920×1080. What each stage measured, with its method and date, moved whole to
  `doc/benchmarks-history.md`, which `doc/plans-perf.md` names where it cited those numbers.
- Removed: `Benchmark_Board_Island`, `Benchmark_Board_Shadows` and `Benchmark_Board_Shores`,
  planning only since the light plugin (~0.1 µs a frame); the island's frame is
  `Benchmark_Frame_Topography`.

## v0.5.0 — 2026-10-25

Needs aabbworld v1.18.0: its geometry in float32, a box 32 bytes; two boxes overlapping within
float32's rounding touching, not pushed; `collide.Handler.Touch` handing the share of a push;
`SIMD()`. Built with `GOEXPERIMENT=simd` and run on a CPU with AVX2, the
collision's pair sweep tests a crowded cell eight boxes at a time, finding the same pairs; a game
logs when it starts whether it does (`gram: SIMD on` or `off`).

**The world's numbers in float32** (2026-10-25, branch `feat/aabbworld-float32`)
- Breaking: every number of the world's geometry and of the screen is a float32, as aabbworld's
  and the GPU's: `world.Position`, `Velocity`'s `Value` and `Side`, `Z`, `Drift`, `Eye`, `Lie`;
  `camera.Camera`'s `MoveTo`, `CenterOn` and `Translate`, `camera.Detail`; `control.Context`'s
  `Wheel` and `Resolution`, `Edges.Depth`, `CornerSide`; ui's sizes (`Size`, `Fraction`,
  `Margin`, `Padding`, `Border`, `Share`, `Fixed`); `world.Scale`, `sector.Band`; the steering's
  `Steering`, `Course`, `Driven` and `Pace`; `collision.Field.Overhang`, `Band`, `Contact`,
  `Physics`; the board's `ground.Heights`, `Cover`, `Profile` and `Surfaces`,
  `grid.Grid.CellBounds`, `grid.Shape`, `board.Shape`, `board.Map.Top`, `cell.Floor`, `Wall`,
  `Rail`, `cell.Kind`'s `Height`, `Veil` and `Sway`, `unit.Mover`; `vision.Sight`,
  `Transparency`; the bullets' `Body`, `Flight`, `Shoot.Power`, `Burst`; gravity's `Pull`,
  `Body` and `Landing`; `minimap.Config.Reach`; the bridges' specs and rules;
  `air.Weather.Sight` and `Visibility`, `climate.Config.Horizon`, `weathering.Config.Swaying`;
  the heights a map is built from (`board.Level.Heights`, `ground.MeanOfCells`, the
  topography's `Seed` and `SetHeights`, `water.Drain` and `Carved`, the depths, widths and
  meander of `water.Config`, `network.Node`'s `Width` and `Fade`). Costs and time ratios stay
  float64 (`cell.Kind.Cost`, `grid.Grid.NeighborCost` and `Distance`, `board.Map.Climb`, `Least`
  and `Slope`, `relief.Climbing`), as do the water gathered (`water.Network.Gathered`), a
  setting's value (`control.Range`, `ui.Slider`) and how often sight scans
  (`vision.WithFrequency`).
- Breaking: a save made before does not load, goke refusing components of another size
  (`world.Base` 104 → 52 bytes); the plugins' resources would load as they were.
- What sums works in float64 and answers in float32: the board's cover and the relief's profile
  along a ray, the relief's ground at a point, the draining of a relief, the mean of the cells
  round a corner; the ortho camera keeps its stick in float64.
- What compared coordinates to 1e-9 goes by float32's rounding (`internal/f32.Eps`): a shot
  stopped at a closed edge, a heading's guards in navigation, a unit snapped onto its spot, the
  cells under a box on a hex board, a bridge's ends, the cells a relief's corner is the mean of.
- A map's relief is read as float32, as the topography keeps it; the hex routes among equally
  short ways may go by another cell, the cells' middles rounded.
- `Benchmark_World_Tick` 5,000 entities 43.6 → 37.3 µs (−14.5%), 1,000 10.2 → 9.4 µs;
  `Collision_Tick` 4–14% less (5×5 boxes over 20%: 2.95 → 2.55 ms); `Sectors_Expanse` 13% less,
  a third fewer allocations; `World_Publish` 7–12% less; `Navigation_Tick` 3–6% less; sight
  0–5% more, aabbworld's rays reading float32 heights.

**A unit struck, not pushed along** (2026-10-25, branch `feat/aabbworld-float32`)
- Breaking: a `collision.Physics` without a `Mass` weighs its box's area, not 1, and
  `collision.DefaultMass` is gone. A small unit striking a big one loses nearly all its speed
  along the normal, a big one striking a small one hardly any.
- A push apart is shared by weight: collision hands aabbworld each pair's share, the lighter
  side moved the further (it was half each). A side held over ground that does not take it gets
  none, in place of a push doubled and the held box put back.
- What an impulse takes from a steered entity's speed stays taken: collision lowers its
  `steering.Course` `Speed` in the same share, and its steering sets off again from what is
  left, by its `Accel`; it gave the speed back the next tick. The ground's walls too: a unit or a
  hero pressing into a wall holds no speed, one sliding along it goes on at what it kept.
- A contact at rest leaves by the face the two touch at (`Contact.Normal`, `Meeting.Normal`), as
  a struck one does, not along the line between their middles.
- Navigation: a unit never moves into one it touches, struck or at rest. Facing into them it
  turns on the spot, a way straight through them leaves it standing till navigation's last word,
  and a turn that would swing it through them goes round their far side.
- In a lane with no way round, a unit moves the one standing in it 0.09 instead of 6.52; with no
  rules of `Touch` it stops at the one in the way (1.07, the two of a weight) instead of pushing
  it to its goal. The Madeira demo's 36 units, all of a weight, sent to one cell push each other
  790 times instead of 1,800 in the first 100 s, the standing ones moved 9.0 instead of 14.2.
- `Collision_Tick` 2–5% more, each impulse reading the steering's course and each pair weighing
  its share; `Navigation_Tick` as it was.

**The renderer's boundary** (2026-10-23, branch `feat/render-boundary`)
- Breaking: the device is render's own, `render/internal/gpu`; the engine reaches it through
  `render.UseDevice`, `render.Windowed`, `render.Present` and `render.Submit`, a test that draws
  readies it with `gputest.Need(t)` from `render/gputest`. `Image.Texture` is gone.
- A stage is readied before its first frame (`render.Prepare`): every shader made is compiled,
  every deferred fill done, every texture written before the device put on it, and a frame drawn
  and thrown away, for what is baked, laid out or built as it is first drawn; the world's time
  starts after it, so the first frame no longer catches up five ticks. `GRAM_FPS_LOG=1` logs the
  readying and the first three frames. The window's device is promised before the first stage's
  Init, so a draw from an Init is an error rather than a device of its own.
- A shader's SPIR-V is kept on disk and read back the next time (`os.UserCacheDir()`'s
  `gram/shaders`, named by the WGSL and naga's version; `GRAM_SHADER_CACHE=off` keeps none, a
  folder keeps it there): naga no longer translates every shader at every start, nor wgpu
  parses its WGSL again. Readying board/topography 899 → 574 ms the first time, 323 ms after
  (the shaders 601 → 34 ms), look/sky 744 → 426 / 200 ms, on an Intel UHD 620.
- What is drawn can be published once a frame, after its ticks, and swapped before it is drawn
  (`plugin.Publisher`, `render.Buffered`; the engine finds publishers among what it tracks, what
  is drawn and the scenes' drawers, and its `Frame` updates and publishes, for tests).
- Breaking: the looks' rules are the world's: `world.Rule`, `world.Rules`, `world.Own`,
  `world.Over`, `world.As`, `world.Swap`, `world.With`, `world.Show` (were `render`'s).
- Breaking: render no longer depends on goke (a test keeps it so). `render.Drawer` is gone and
  no drawing type has `Init`: a `Renderer` draws, a `Reporter` reports, a Source composes, a
  Picture draws its world, and the Composer readies nothing. What reads the world is a
  `plugin.Bound`, readied once by the engine; `game.Scene.Drawers` returns `[]render.Renderer`,
  so a picture or a source listed in a scene no longer compiles (the engine's check is gone).
- The world's entities are published once a frame and drawn from what was published: the looks'
  rules run once for every camera's sight together, not once a camera (none where a scene has
  no rule), and each entity some camera sees is kept in 80 bytes (its box, heading, speed and Z
  in float32, its layers and how far their materials have run, a Lie apart, the light's mark);
  each camera draws through a copy of its view (`view.View.CopyInto`). `GRAM_FPS_LOG` shows the
  publishing apart. 5,000 boxes (`Benchmark_World_Draw`, new `Benchmark_World_Publish` and
  `Benchmark_World_Frame`), before → drawing + publishing: in full view 445 → 467 + 70 µs, a
  quarter 125 → 120 + 27, a twentieth 30 → 24 + 12, two cameras a quarter each 242 → 239 + 45;
  motion/collision 1.45 ms → 1.24 + 0.42. Done one after the other the copy is a price, meant to
  be paid on the ticks' side once the frame is drawn beside them.
- Breaking: `world.Lighting` marks an entity as it is published (`Mark(cur, i) uint16`) and lights
  it by the mark (`Of(mark, box, z, a)`); `light.Write.Of` takes the mark.
- What is drawn over the entities is published once a frame too: the selection's outlines and
  the box being dragged, the collision boxes, the cells held (their outlines with their heights),
  the bridges in hand, the views (their Show rules run once a frame, their outlines copied), the
  routes and goals (the route to a queued goal found as it is published, no longer as it is
  drawn) and the minimap's marks (at most 30 times a second, a deadline and a version; the
  minimap is drawn anew as they are). `world.Read.ViewFor` is a camera's view as the frame drawn
  was published, one copy a frame for everything that draws; `Read.Views` and `Read.InView` are
  the views as the last tick left them, for what publishes. New: `vision.Renderer.WithInView`.
- render lights the frame; the light plugin declares and publishes. The composer, given a scene's
  lights (`render.Composer.Light`, `render.Lights`), hands each part of a frame the sun and the
  lamps nearest its camera — the ground of a sharp-figures frame lit by the lamps too, which it
  was not —, lights what a source draws (`render.Frame.Lit`), lays the live shadows of what
  a source declares (`render.Frame.Cast`) once every source of the part has composed — the
  camera's own, of the frame drawn, where a sharp-figures frame drew those of the composition
  before or of another camera — and bakes the ground's shade first of the opaque pass. The
  light's math, its WGSL (`lightWay`, `lampLightAt` and their uniforms, render's library now),
  its baker and its caster are render's: `render.Distant`, `Lamp`, `Umbra`, `Beam`, `Casting`
  (`Patch`, `Silhouette`, `Baked`, `NoShadow`), `Shadowing`, `ShadowPolicy`, `Block`,
  `Lattice`, `MaxLamps`. The light plugin publishes `render.Lights` once a frame (the Baked
  blocks scanned once a frame, not at every composition of every camera; blocks beyond every
  band left out) through `world.Write.SetLights`; `world.Read.Lights` is what draws reads.
- Breaking: `world.Lighting` is `Join` and `Mark(cur, i) render.Shadowing`; gone:
  `light.Write.Begin`, `Of`, `light.Read.At`, `Casting`, the light plugin as a render source,
  `world.Read.Lighting`, `look.Renderer.WithLighting` (now `WithLights`). `light.Distant`,
  `Lamp`, `Umbra`, `Beam`, `Casting` and `ShadowPolicy` are render's; a shadow is of what stands
  from an altitude up a height.
- The ground is drawn as published too. The board publishes its cells once a frame, each of two
  buffers brought up to the board where a cell's version differs from the one it holds (a cell
  32 bytes, its kinds and way interned), and its renderer its grid's toggles; the topography its
  relief, a copy of the heights as their version moves, and its settings. What draws reads them:
  the board's tiles and covers, the painter, the kinds' shade, the terrain, the care's lines,
  the hexes, the billboards, the light's live shadows, the routes and the views
  (`board.Read.Drawn`, a `board.Drawn`; `board.Read.DrawnHeights`;
  `routes.Renderer.WithDrawnHeights`), and the sectors' view its bands, the cameras' zones and
  layers their settings. The terrain hands the frame the span of the heights it draws, a frame
  late after the relief changed before; the board's miniature is drawn anew as the sun's light on
  it changes visibly, never before; a sector's tints go to the GPU only as one changes.
- The sky is drawn as published too. The atmosphere copies once a frame the weather, its air
  masses where a patch changed since that buffer was drawn (`air.Dome.CopyInto`; the copies of a
  dome share its picture, `air.DomeImage`), the sun, the heavens, which workings go on and its
  Settings; the sky, the low clouds and what falls read only that, and so do the board's and the
  topography's ground, haze and swaying through `atmosphere.Read.DrawnAir`. The sky hands the
  frame the sun only where the composer has not lit it; the clouds' setters and the frame
  budget's lever write the Settings alone. New: `backdrop.Values.Heavens`, for a sky part's Feed.
- Breaking: `board.Atmosphere` and `topography.Atmosphere` ask for `DrawnAir()` (were `Air()`);
  `atmosphere.Read` gives both, `Air` the live weather.
- The screen is drawn as published too. A ui scene's drawer is a `plugin.Publisher`: once a frame,
  after the ticks, it reads where its pinned elements' entities stand and whether each picture
  shows them, every `ui.Text`'s words (a pinned label's for each of its entities), every
  checkbox's and slider's value and which elements are hidden, and the frame is laid, drawn and
  clicked as that says; an element put in since reads them as it is laid. A Text is read once
  a frame where it was read four to seven times; `GoToIt` moves the camera at the publish after
  the frame that laid it. A `ui.Drawer` that is a Publisher is published with its scene. A
  scene's drawers publish only while the scene is shown: a hidden one reads nothing.
- The telemetry's lines and reporters are read after the ticks while the panel is shown, four
  times a second and as the mode changes; the panel only wraps the last reading and draws it.
  The topography's cares are handed to its reporter as it swaps. The clock's HUD reads its line
  after the ticks; the moon icon shows the phase the atmosphere published; a minimap is aimed,
  and its compass turned, as published with its marks.
- Fixed: the HUD's weather said the weather of before one set while the clock stood.
- A frame is the engine's steps: the input, the simulation (the ticks and the publishing), the
  barrier, where nothing ticks or draws, and the drawing. On the barrier what was published is
  swapped in, the scenes shown are noted for the frame drawn, their drawers laid out
  (`game.Laying`: a ui scene's layout, its text measured and its pictures sized, there and no
  longer as it is drawn) and the last frame's times handed to what is paced (`plugin.Paced`,
  `plugin.FrameTimes`: the wait for the GPU, the simulation, the barrier, the drawing, the
  presenting). The cameras follow the frame budget there (`cameras.Plugin.Pace`), coarser only
  while the drawing's side holds the frames back, not the simulation. `GRAM_FPS_LOG` names the
  barrier and the waiting.
- A camera is drawn from a copy of itself (`camera.Snapped`: `Snap`, `Drawn`, `Origin`;
  `camera.DrawnOf`, `OriginOf`), taken as its feed is laid out on the barrier: the layers and the
  sharp figures crop the copy, never the camera the ticks move, and a relief camera's copy has
  its ground's extent frozen as it stood, the cage no longer read as it is drawn. What is kept
  for a camera (the world's views, the marquees) is looked up by the camera the copy was taken
  of. A minimap's outline of its camera's screen and its reach are published with its marks;
  what the marks are gathered round is a lens of the ticks' own.
- Nothing drawn reads the game as it ticks:
  - Vision's views are held back by the cover as it was published: the board's cover is walked
    as the observers are published, where it changed, and the drawing only sends it to the GPU.
  - A stage is readied before the barrier lays its scenes out, so its first frame is laid out
    there too; a ui picture drawn straight onto a screen it overhangs is laid at the screen's
    size, so drawing it resizes no camera.
  - What is shown after the frame's publish is published on the barrier: a ui scene's tree put
    in by `Replace` as the scene is laid out, the drawers of a scene shown since.
  - `world.Read.ViewFor` reads only the frame's Views once one was published: a camera the
    frame holds none of has no View, no longer the one the ticks keep.
- The next frame is ticked while a frame is drawn: its ticks and publishing run on a goroutine of
  their own, started as the barrier ends; the next barrier waits for them, swaps what they
  published in and takes the frame's input there, so what the input does is shown a frame later.
  A stage's first frame, the frame a stage switch shows its overlay in and every frame with
  `Props.Serial` (or `GRAM_SERIAL`, whatever the game says) set run as before. The game's pool is one short of the CPUs then, the drawing's
  thread left a core (`Props.Workers` says otherwise). `GRAM_FPS_LOG` notes how long a frame
  awaited its ticks. A minimap hidden still publishes where it looks, for its compass; a ui
  picture is drawn straight onto a target only at the size it was laid out at.
- A game's own board map shown: `examples/board/custom-map` (hills of the game's own, shaded by
  height, walked round and climbed slower) and `ExamplePlugin_WithMap`. The tests that draw skip
  where there is no GPU, so CI passes again; Codecov counts no coverage of `examples/`, `bench/`
  or `cmd/`.
- `Benchmark_World_Publish`, `_Draw` and `_Frame` name their cases by the boxes in view, a world
  of 50000 besides; publishing in parts on the pool, from the cameras' views or by the entities'
  indexes was measured and not kept (BENCHMARKS.md).

**The flat board drawn by the GPU** (2026-10-23, branch `perf/flat-gpu`)
- No tile is composed on the CPU any more. A flat board is laid on the GPU every frame out of an
  image of its cells (`look/tiles.go`): the sprites, the ways, the covers, what sways, the cells
  of a material, the light per pixel as each cell takes it, the sectors seen at their levels; a
  changed cell writes its block and its neighbours' alone. The minimap's board and a hex
  topography's tops are drawn by it whole from above (`look.Renderer.Whole`), the painter's ways
  over the hexes' tops as GPU instances (`Painter.DrawWays`, the curves, the glaze into the mix,
  the water running and glinting as before). `Benchmark_Frame_Flat` (new, 1440p, read back),
  before → after, measured apart: from above 11.5 → 13.4 ms, a cell changed every frame
  17.1 → 13.7 ms (4.8 MB → 55 KB a frame), isometric 17.6 → 13.8 ms. The cells of a material
  are no longer the plain pass's: it lays the others alone, the full pass the overlaid ones, which
  took back the ~0.7 ms the later stages had cost (measured back to back: from above
  15.3 → 14.0 ms, a cell changed 15.5 → 14.2, isometric 15.2 → 14.1).
- Where tiles overlap, on a hex board and round what sways, each tile and its covers lie over
  the cells before it, row by row, in every view; the ways over them all, the cells of a
  material after the clouds' shadows. The cells of a material, composed on the CPU every frame
  besides the GPU's pass, are drawn once.
- New: `look.Laid` (a Dressing's ways and covers), `look.Overlaid` (a material laid over some
  tiles), `look.Renderer.Whole`, `look.Renderer.Animated`, `Painter.Above`, `Painter.Overlay`,
  `Painter.DrawWays`.
- Breaking: `look.Look` is how the cells lie (`look.Flat`, `look.Swaying`, `look.Nothing`), no
  interface; a `look.Dressing` is `Begin` and `Base(c, own)`. Gone: `look.Tile`, `look.FlatLook`,
  `look.EvenLit`, the Dressing's `Light`, `FaceLight`, `Dress`, `Across` and `Sheet` (every
  Dressing handed back the atlas it was given), `render.Still`, `gpu.Kept`, `draw.Weigh`,
  `draw.Quarters`; a board of no world, or seen through a camera without lines of sight, draws
  nothing. The board's parallel compose is gone, no game used it: `look.Parallel`,
  `look.ParallelLook`, `Renderer.Workers`, `Renderer.UsePool`, `Renderer.Reads`,
  `board.Plugin.WithWorkers`.
- What looks different: what sways moves every frame from above (the still held it till a cell
  changed), the clouds' shadows lie on a flat board in isometry too, the light is worked out per
  cell on a hex board and across a wrapped world's seams, and nothing past the board's edge is
  darkened.

**The topography's ground painted on the GPU** (2026-10-23, branch `perf/flat-gpu`)
- A square topography's board is painted flat on the GPU: the painter lays what its shaders read
  of each cell into an image (its kind's sprite and Style, its way, the ground at its corners),
  anew as the cell changes, and the shaders work out the grounds running into each other, the
  ways' pieces (in the vertex stage: the curves, the stem and the joins, the fading, the mix, the
  water running down them), the water's layers and each pixel's kind; the cells within two of a
  changed one are painted anew, all of them past an eighth. The shores (the way to the nearest
  cell that does not shine from every corner of the lattice) are worked out there too and copied
  into the terrain's lattice on the GPU. The painter's bake on the CPU is gone: 740 lines of Go
  fewer, 950 of WGSL more.
- What looks the same: a board with seas, rivers, roads and details bakes to the pixel as the CPU
  painted it, whole and after a change; a hex board's ways within 11 pixels of 208,000. Where a
  cell may hold more than its kind's detail and where water may lie is worked out by rule, not out
  of the pieces: a few more cells read the kinds' sheet.
- What it costs on an Intel UHD 620 (a weak GPU; most are far faster): a frame the same; a cell
  changed 8.5 → 6 ms a frame; the board painted whole (the first frame, a Style changed, many
  cells changed at once) ~25 ms a frame on the CPU, ~110 ms on the GPU at first, ~23 ms
  (`Benchmark_Painter_Bake`, new) once the ground's cells are worked out once a cell, not a pixel,
  the mouths' layer and the kinds' (no kind with a Detail) are left to what they hold and the
  ways are drawn in as many pieces as the most of them lays. `Benchmark_Frame_Topography` waits
  for its first frame, so the board painted at it no longer lands in the frames measured.

**The render package taken apart** (2026-10-23, branch `perf/flat-gpu`)
- `render` is a facade: the machinery is `render/internal`'s, `wgsl` (the shaders' library, the
  materials and the frame's images), `gfx` (images, depths, meshes, shaders), `atlas`, `text`,
  `feed`, `compose` (the composer and its frame) and `instanced` (sprites as instances), each
  type named in `render` by alias; the composer's shader compiles from the same source, byte for
  byte, and the demos draw as before.
- Breaking, what nothing used: `render.CachedRenderer`, `render.Paint`, `render.DebugPrint`
  (`DebugPrintAt` stays), `render.TextWidth`, `render.ActualTPS`, `render.Looped`, `render.Slot`
  (`Atlas.Add` hands nothing back), `Frame.Fold`, `Frame.Fan`, `Frame.Last`, `Frame.OverlayOn`,
  `Frame.Mark`, `Frame.Camera`, `Frame.Branch`, `Frame.Append`, `Frame.GlazeBlend`,
  `Frame.SpriteBlendPart`, `Overlay.Under`, `GroundHeights.At`, `DrawMeshOptions.Multiply` (and
  `gpu.Multiply`), `Composer.Composed`, `GeoM.Reset`, `Sprites.Len`, `gpu.Ready`, `gpu.Device`,
  `Layout.Has`; `render.SetRates` takes the frames alone; `Clocked`, `FromImage`, `NewSheet`,
  `GlyphWidth` and `LineHeight` are render's own.
- Breaking: the 2D fills are the image's: `dst.FillRect(…)`, `StrokeRect`, `StrokeLine`,
  `FillCircle`, `FillQuad`, `FillPolygon`, `StrokePolygon` in place of `render.FillRect(dst, …)`;
  `Image.PlaceDepth` is render's own. Text is the font's to draw: `f.Draw(dst, s, x, y, c)` in
  place of `render.DrawText(dst, f, s, x, y, c)`.
- Breaking: `render/gpu` is `internal/gpu`, the engine's and render's alone; a test that draws
  readies the GPU with `gputest.Need(t)` (`internal/gpu/gputest`), skipped without one.

## v0.4.0 — 2026-10-23

Needs aabbworld v1.15.0, astar v1.1.3, ceremony v0.3.1 and goke v3.6.0.

**Wave 5** (2026-10-22/23, `doc/plans-perf.md` "Fala 5")
- One pool of goroutines for the whole game (`internal/parallel`: `New`, `Begin(task, idle)`,
  `Batch.Add`, `Batch.Wait`; tasks taken by an atomic counter, none allocating), sized by
  `game.Props.Workers` (0 every CPU, 1 none) and handed to the plugins by
  `plugin.Installer.Pool`. Vision scans and the board's renderer lays its tiles on it;
  `parallel.Workers` and `parallel.Run`, which made goroutines on every call, are gone.
  `Vision_Scan` 9 → 0 allocations a tick, with outlines 100 observers −7.7%.
- Breaking: a plugin hands its state out through `Read(w)`, safe on goroutine `w` of the pool (0
  the game's), and `Write()`, on the game's goroutine; configuration, registers and lifecycle
  stay on the plugin. The board: `Read(w).Board()` a `BoardRead` (the grid, the terrain),
  `Write().Board()` a `BoardWrite` (`Set`, `SetAll`, `SetWay`, `Touch`), `board.NewBoard` a
  board with no plugin; a `Map`'s `Top(c, height)`; the cover kept cell by cell as cells change
  (`ground.Readied` gone). The world: `Read(w)` with `Space` (aabbworld `SpaceRead`), `Clock`
  (`clock.Read`), `Effects` (ceremony `EffectsRead`), `Write()` with `Space`, `Clock`, `Effects`,
  `Carrier`, `Spawn`, `Despawn`, `NewView` and the setters. Light, topography, selection,
  navigation, atmosphere, cameras, telemetry, players, minimap, vision, collision, motion and
  bridges likewise, their switches, levers and setters on `Write`; `clock.Clock.Replayed`.
- Navigation in internal packages (`order`, `bodies`, `spacing`, `routing`, `traffic`,
  `pathfind`, `routes`), the public names by alias.
- A unit without a route asks for one in its order (`MoveOrder.Ask`) and the routing answers it
  the same step, the traffic moving the units that asked after the answers. The routing's
  searches are shared on the pool among `WithRouters(k)` routers (`DefaultRouters`, 4), each a
  finder of its own, sharing one memory of answers: a search one goroutine works out another
  waits for instead of running it again. An idle goroutine wipes the stamps of the routers idle
  that tick near their wrap (astar `Solver.Wipe`). A `MoveTo` no longer searches: a group's
  targets are the cells round its goal, nearest first, one to a unit under cell spacing however
  many they are. The crowd moves differently from the first group order on.
- A step onto a cell allocates nothing: the occupancies hand the slice of a cell `Release`
  emptied to the next cell entered (open 1,000 20 → 14 allocations a tick).

**Wave 3** (2026-10-21, `doc/plans-perf.md` "Fala 3")
- A route search asked again over the same board is answered as before, unrun: a search blind to
  the others reads only the board and its question (start, target and level, domain, body, the
  cells gone round), so its answer is kept till the board's `Version` changes (at most 4,096).
  The crowd's Detour comes back to sets of learned cells it asked a few ticks before:
  `Navigation_Tick` crowded 1,000 94.9 → 40.3 ms, crowded 100 −24%, open 1,000 −24%, open 100
  −7%, every box the same tick by tick.
- Navigation's A* on astar's slice dictionaries by the cell's ordinal, reset by a stamp
  (`astar.WithIndexedSliceDict`): crowded 1,000 40.5 → 37.6 ms, a route across 200×200 −5%.
- Motion moves a chunk's boxes in one call (aabbworld `Space.MoveRun`), exactly as box by box:
  `World_Tick` 1,000 / 5,000 15.5 / 67.1 → 11.9 / 49.6 µs.
- `clock.Clock.Settling(ctx)`: the engine hands the Stage's `Update` the clock's settling
  context, so a Sync in a plugin's `RunPlan` outside the clock (navigation's `MoveTo`, driving's
  hand, bullet's shoot) settles the space at once and a reader later in the tick (selection by
  box) no longer finds a phantom id 0, misses a migrated entity or reads another's box.
- Metal (Apple GPUs): every demo panicked at its first frame with the lamps' casts, the routes or
  the cones ("declaration with attribute 'position' already specified"): their fragment shaders
  took `@builtin(position)` both as a parameter and as a field of their input struct, which WGSL
  forbids and only Vulkan let by. They read the field now; `TestWGSL_EntryPointsTakeEachBuiltinOnce`
  walks every `.wgsl` file and every Go literal holding an entry point and fails on a builtin or
  a location taken or given twice.
- Fewer allocations, time not worse: ceremony's steps give their commands without allocating
  (`Navigation_Tick` open 1,000 2,599 → 90 allocations a tick, crowded 1,000 3,946 → 1,406), and a
  warm aabbworld `Scan` in a wrapping world allocates nothing (3 → 0 a scan across a torus's seam).
- Wave 4, fewer allocations at a time not worse: the route finder writes its routes into buffers
  it keeps (astar `SolveInto`; `Navigation_Tick` crowded 1,000 1,406 → 13 allocations a tick, a
  route across 200×200 16 → 4); ceremony's steps on the other of a pair, on places and a clause
  of the other run without allocating (2 → 0 a firing); every Direct boxes its own uniforms once
  (the sky 28 fewer a frame, aloft 22 a layer, the drops 15, a lit and gridded flat board 23, the
  clouds' shadows 8, the billboards 6, the casts 3) and the blits (`gpu.DrawImage`, `Sprites`,
  `DrawImageIn`) share their uniform blocks, `DrawImageIn` gathering its triangles in the
  device's buffers. The pictures unchanged.
- world: the `unindexed` flag is gone; the indexer's chunk check (lengths, addresses, `TableVer`)
  alone decides when the space gathers its runs.

Saves made before the back-off of a failed search do not load: `navigation.MoveOrder` has `Retry`
and `Fails`.

Saves made before wave 5 do not load: the navigation's components are saved as `order.MoveOrder`,
`order.LastOrder`, `order.Blocked`, `order.Arrived` and `order.Round` (were `navigation.…`), and
`MoveOrder` has `Ask`.

**The space reads every box where goke keeps it** (2026-10-21, etap 3 of `doc/plans-perf.md`)
- Motion, the world and collision hand the space runs over the `Base` columns (aabbworld v1.14.1
  `Space.RebuildRuns`, `world.RunOf`, one run a chunk) instead of copying every `Base` into an
  `Item` each step; the collision engine pushes the boxes in `Base` itself, so `moved` needs no
  Seek and a leaver's box is already where `Left` says. The world gathers the runs anew after
  every Sync the clock replays (`Clock.OnSync`, new: a settle after every Sync of the replay and
  as each Replay ends) once a chunk came, went, grew or shrank or a table's `TableVer` changed.
  `Benchmark_World_Tick` 1,000 / 5,000 boxes 23.3 / 110.9 → 13.9 / 58.0 µs (the copy was 52% of
  the tick), the collision ticks 1–10% shorter, the same contacts in every scene.

**A failed search waits before it runs again** (2026-10-21)
- A unit's search for a route that finds nothing (round the cells it learned, or any route when
  it learned none) is not run again from the same cell round the same cells for 0.5 s, then 1 s,
  then 2 s; meanwhile the plain route is taken, or with none the unit waits, its target still
  given up at the same moment as before. A way found, the next goal, a new order or another
  target start the waits over; another cell or another cell learnt lets it run at once. The
  searches round the cells learned that find nothing fall from 26 to 9 a tick in the open with
  1,000 walkers and from 128 to 95 in a crowd; the demos walk exactly as before.

**An instanced draw's instances copied at once** (2026-10-21)
- `gpu.Mesh` copies the instances of a draw with one copy, not a float at a time: a tick with a
  frame of 5,000 sprites over the whole view 9–11% shorter (`Benchmark_World_TickDraw`).

**Navigation's A* without maps and allocations** (2026-10-20, etap 2 of `doc/plans-perf.md`,
part 2a)
- The route planner reads what it asks of a cell, its kind on the ground priced for the
  search's domain, its raised deck, its bare ground, once a search, into a table by the cell's
  ordinal; walks the grid's directions with `Toward` instead of a `Neighbors` list a node; asks
  `DiagonalNeighbors` once a step and `Along` only where it can change the price; and makes its
  solvers with room for every step of a node. Every route, step and price is as it was
  (`TestRoutes_AsPinned`, `plugins/navigation/testdata/routes.golden`). `aside`, the spots round
  a goal and `NearestFree`'s breadth-first walk walk the directions too.
- A square grid's `Toward`, `DiagonalNeighbors` and coordinates divide in 32 bits and fold only
  a coordinate off the grid.
- `Benchmark_Navigation_Path`: a route across a 200×200 board of rocks and walls, 94 ms and
  14,740 allocations → 18 ms and 19. `Benchmark_Navigation_Tick` at fixed counts: open
  100/1,000 0.80/9.6 → 0.42/6.3 ms, crowded 100/1,000 2.9/306 → 1.2/104 ms.
- A tick allocates nothing per unit in the bodies' index, a step's ground check, a route round
  the cells a unit learned or a leg's cells, and a step aside asks the ground only of the cells
  it tries (etap 2b; `Navigation_Tick` open 1,000 5.67 → 4.55 ms, 5,004 → 2,880 allocations;
  the others as before, a crowd's cost its Detour's searches). No unit moves otherwise.

**The layers leave out what covers them** (2026-10-20, the layer levers after etap U, part R2)
- In layers each layer but the innermost leaves out the ground under the next one's window,
  inside its feather (`render.Laid.DrawLayer` takes the window a later layer covers,
  `gpu.FillDepth` fills it with the nearest depth before the ground): the preset's layers draw
  about 0.52 of the frame's pixels, not 0.71; the island's layered frame is 1.3–1.8 ms faster
  (iso in layers 15.6 → 13.9 ms, footed 17.1 → 15.4, sharp 18.2 → 16.6 and 19.3 → 17.9), the
  pictures the same.

**The clouds over a coarse ground** (2026-10-20, the layer levers after etap U, part R1)
- With the figures at every pixel over a ground drawn coarser (a resolution under 1, or in
  layers), aloft works its clouds out at a quarter of the ground's pixels each way, not the
  screen's, and lays them over the screen against its depth: their march costs as many pixels
  as the ground (1440p, the ground at 0.6: 6.1 → 3.7 ms in the worst case, full cover from
  above) and the pictures show no seam. `render.Target.Coarse` tells a Direct in
  `Laid.DrawOver` how many of the screen's pixels a pixel of the coarsest laid ground spans.

## v0.3.0 — 2026-10-20

Saves written by v0.2.0 do not load: `Base` and the marker components changed shape, the sky's
and the climate's entities are gone, the clock's is new. Nor do saves made on this branch before
the topography was split into packages: its heights are `relief.Heights` now. Nor before the core
left the world: goke names `tag.Tags[clock.Phase]` and `tag.Tags[effect.States]` by their
argument's full path, which moved. Nor those made before decks ran as bands: `cell.Floor`,
`cell.Rail` and `cell.Built` changed shape. Nor those made before a route knew its levels and
where it begins: `navigation.Path` changed shape. Nor those made before the behaviours went to
`ceremony`: the component types' paths changed (`ceremony/tag.Tags[…]`, `effect.Active`,
`plan.Mind`, `ceremony.Label`). Nor those made before ceremony's store went behind its contract:
`effect.Active` and `Wide` are the binding's `goke.Active` and `Wide`, `plan.Mind` is `store.Mind`.
Nor those made before a unit could strafe: `entity.Velocity` has a `Side`. Nor those of a board
in a flat world: a board stands only in a world with heights now. Nor those made before the
weather came from the horizon: `climate.Weather` changed shape, the air masses are `air.Patch`
entities. Nor those made before motion left the world: `world.Outside` is `motion.Outside`. Nor
those made before the light became a plugin: `entity.Shadow` is `light.Shadow`, and the
atmosphere's entity carries a `light.Source` and a `light.Shining`.

**The picture's resolution and the ground's care** (2026-10-19, etap R of `doc/plans-perf.md`)
- A camera draws its picture at a resolution of the pixels it is shown at (`camera.Resolved`,
  `cameras.Config.Resolution`, 0.4 to all): the world is drawn smaller and shown at the
  element's size, the UI stays as it is; Alt+= and Alt+- step it, Alt+A has it follow the frame
  budget (`cameras.Config.FrameBudget`, a sixtieth of a second), the telemetry's "Picture"
  line shows it. Positions reach the bindings in the camera's pixels (`control.Context.Resolution`).
- The ground is shaded with less care ring by ring from the sector under the screen's middle
  (`topography.Config.Care`, Alt+Shift+= and Alt+Shift+-): far water keeps its colour and a level
  mirror of the sky but no waves, glint or shore, farther still no ground shade or lamps.
- `Benchmark_Frame_Topography`: the island's frame on a headless GPU at 1440p.

**The picture in layers** (2026-10-19, etap R2 of `doc/plans-perf.md`)
- A camera's picture drawn in layers (`camera.Layered`, `Cropping`): the whole frame at 0.6 of its
  pixels, the central 60% at 0.8, the central 35% at every one, each a centred crop of the same
  camera (`render.Feed`) laid over the last with a feathered edge (`DrawImageOptions.Fade`).
  `cameras.Config.Layers` and the setting "Picture drawn in layers" (`Settings.Layers`, saved with
  the game); `Adapt{On, Layers}` carries both, Alt+A in turn: following the frame budget,
  following in layers, still and whole. The adapter and Alt+= / Alt+- step the outer layer first,
  then the middle, the centre last, and back in the reverse order; the telemetry's "Picture"
  line shows the three. `Benchmark_Frame_Topography` has `iso,layered` and `persp,layered`.
- The cameras' settings (the zones, the following) are saved with the game: they never were.
- The composer keeps depth buffers for eight target sizes, the clouds' images one a size, so a
  view in layers recreates no texture a frame.

**The settings screen in sections** (2026-10-19, etap S)
- The settings screen (F10) lists the sections the plugins share first, centred: `control.Picture`
  with its resolution, the strategies following the frame budget, the ground's care, the areas
  shown and the graphics (a setting's `Section`), then a section a plugin as before;
  `ui.Subheading`.
- The picture's resolution follows the frame budget by a strategy (`cameras.Settings.Strategy`:
  still, the whole picture, in layers; `Adapt{Strategy}`, Alt+A in turn), every knob of both set
  by hand while still: the resolution, the frame budget, the layers' shares and resolutions
  (`Picture`, `Layers`, `LayerShares`, `LayerResolutions`, `Budget`); `cameras.Config.Adaptive` is
  `Strategy`. The telemetry's "Picture" line says the strategy and its settings.
- The ground's full care reaches as far as a slider says, up to every sector; "Show the layers"
  and "Show the care" outline the layers and the care's bands over the picture.
- The cameras' settings changed shape: a save from this branch before it loses them.

**The ground at the camera's resolution, the figures at every pixel; five cares** (2026-10-20,
etap T of `doc/plans-perf.md`)
- A picture drawn below every pixel draws its ground alone at that resolution and the figures,
  what falls or floats over them and the marks at every shown pixel over it, tested against the
  ground's depth spread to the screen's size (`render.Grounded`, `Overlying`, `gpu.ScaleDepth`;
  the sources over the figures say so). The setting "Figures at every pixel" (`cameras.Settings
  .SoftFigures`, zero sharp; `SharpFigures{On}`, `Config.SoftFigures`) switches it; in layers
  the picture is drawn as before. The telemetry's "Picture" line says "the figures sharp".
- The ground is shaded with five cares, one less every R sectors from the sector under the
  screen's middle or the eye (`Full care out to` is R): the shores, the surf, the running water
  and the mouths' glint go first, then the waves and their glint, then the baked shade and the
  lamps, then the grid, the clouds' shadow and the water's mirror. The "Ground" line counts the
  sectors from full to least; "Show the care" colours the bands.

**The HUD: the atmosphere's widgets, the telemetry in two modes, north on the minimap**
(2026-10-20, etap V of the plan)
- The atmosphere gives a scene its HUD as elements to place (`TimeOfDay`, `Date` with the
  moon's phase beside it as an icon while the moon runs, `Weather`, absent without a running
  climate, and `Bar`, the three on one panel for the top of the screen); the old `HUD()`
  renderers are gone. `Reporter()` lists the daylight (the sky's line, "Light" before), the
  clouds and their shadows alone: the time, the season, the moon and the weather are the
  widgets' (`Calendar().Reporter()`, `Climate().Reporter()` for a game without them).
- The telemetry panel has two modes, the basics (FPS, TPS, Frame, Entities) and everything
  (`telemetry.Mode`, `Settings{Extended}` saved with the game, `Panel{Shown, Extended}`); ~
  cycles hidden, the basics, everything; the settings screen has a switch and a choice for it.
  Every many-valued line is a value of lines under its label, as Frame's: the light's, the
  cameras' Picture, the topography's Ground (a care a line), the world's sectors, the
  calendar's season.
- The minimap marks north: `Compass(cam)`, a red triangle hung on the circle's edge three
  quarters out and pointing north, turned round the circle and with it as the minimap turns,
  laid over the minimap by the scene in a box `minimap.Overhang` bigger on every side.
- The telemetry panel draws itself: the labels bold, a line too wide wrapped with its rest
  indented, the plate shrinking back as the mode goes to the basics. The settings screen's
  atmosphere section chooses the weather by name (`Climate.Now()`), under the pause too; its
  slider "Hour" shows the day's time as it goes and, moved, freezes the light at that hour
  (`Hour{At}` freezes; "Frozen light" beside it lets the day go on).
- The clouds follow the frame budget by their pixels first: "Clouds drawn at" a quarter, a
  sixth or an eighth of them (`CloudsDrawnAt{Coarse}`, `Settings.CloudCoarse`,
  `aloft.Renderer.Coarse`), the lever stepping down (4, 1), (6, 1), (8, 1), (8, 0.5) of
  (pixels, reach) and back, the bodies' reach cut last and never under half the sight, the look
  never; the demo starts the clouds still. "Show the care" sits under Picture/Care beside the
  care's following.
- P pauses too (Shift+P stays; the start demo's panel moved to I); the demos say PAUSED beside
  the atmosphere's bar or the date, on the right, while the clock stands (`ui.HeadingOf`,
  `ui.While`). The settings screen's title is larger, on a panel, and every top section is
  framed; a border is stroked inside its box, so a clip at the edge loses none of it. The
  scroll zones are twice as wide (`control.EdgeMargin` 60) and drawn as a gradient, clear
  inside and yellow at the edge, on the glass over the picture (`render.Glass`,
  `Feed.DrawGlass`, drawn by `ui` after everything laid over the picture, so the HUD never
  covers them; `render.FillQuad`); the edge scroll starts at a quarter of its speed at the inner
  bound and reaches it at the edge (`control.Edges.Depth`). The cameras' section has three
  settings for the zones: "Scroll zones" (scrolling at all, `EdgeScroll{On}`), "Show the
  zones" (`Zones{Shown}`) and "Edge scroll speed" (`EdgeSpeed{Speed}`, 2 to 24 px a tick).
- An option on the settings screen has no key any more: Shift+X the boxes, Shift+H the scroll
  zones, B the grid, Shift+O the cells held, Shift+R the routes, Shift+C the cones, Shift+W the
  weather and Shift+U the rain, Shift+V the clouds' look, Shift+S the high clouds' shadows, P the
  frozen light and Shift+] and Shift+[ its hour, Alt+= and Alt+- the picture's resolution, Alt+A
  its following the frame budget, Alt+Shift+= and Alt+Shift+- the ground's care, Ctrl+F the
  haze, H the coarse shadows, Shift+L the lamps' shadows, Shift+G the sectors, Ctrl+G their
  bands, Shift+M the minimap shown and Ctrl+M its turning are gone from the plugins' default
  bindings (`cameras.Keys` loses `Finer` and `Coarser`). The commands stay, for a game to bind a
  key of its own; the pause, the tempo, the telemetry panel, the menus, the tools, the views and
  the motion keep theirs.

**The picture by the camera's state: profiles, footed layers, one composition** (2026-10-20,
etap U of `doc/plans-perf.md`)
- The picture's rings are layers (`camera.Layer`, `Layered`, `LayersOf`; the cameras' `InLayers`,
  `Layers{On}`, `LayerShares`, `LayerResolutions`, `ShowLayers`, "Drawn in layers"; the feed's
  `LayerFeather`): a layer's `Share` of the frame lies as the picture's `camera.Layout` says,
  `Inset` in the middle or `Footed` standing on the frame's bottom edge, centred across, the
  coarser layer round it like a table, a top and two legs; `camera.Arrange` gives each its
  `Window`. A camera is cropped to any window of its frame (`Cropping.Crop(win, s)`, `Window()`;
  the perspective keeps its principal point); a footed layer fades on the sides inside the
  frame alone.
- The cameras' picture settings are a profile each, tactical and riding (`cameras.Settings
  {Zones, FrameBudget, ShowLayers, Tactical, Riding}`, `Knobs`, `Config{Tactical, Riding}`;
  `ProfileOf`): a camera inside an entity wears the riding profile, any other the tactical one,
  dressed anew as its state changes; in a tactical perspective its layers stand footed. The
  commands carry the profile (`Adapt{Profile, Strategy}` and kin; Alt+A the key's camera's);
  the settings screen has a section a profile under Picture, each with its layers and its
  "Show the layers" (`ShowLayers{Profile, Shown}`, `Knobs.ShowLayers`), the two laid side by
  side; the telemetry's Picture line names the lead's profile. A save from before loads the defaults.
  board/topography starts both profiles following the budget in layers, the figures sharp.
- The settings screen lays twin sections (the same settings under each, as the profiles) side
  by side, a line a setting with a control a twin, in as many columns as the screen's width
  takes, fewer and taller where it takes none, scrolled then by a bar or the wheel
  (`ui.Scroll`: an element taller than its box, its content clipped to it): every setting is
  reached whatever the screen's size, and the listing is built anew for the screen the scene
  is laid over (`ui.Scene.Laid`) as the window is resized or goes full screen, the shortcuts'
  (K) too. A menu up (F10, K) lets the cursor go while riding.
- The frame budget turns levers besides the pictures' resolution (`cameras.Config.Levers`,
  `cameras.Lever{Following, Coarser, Finer}`): coarser the levers first, finer the pictures
  first; the levers return first too, so the care takes the rate's wavering and the pictures
  move under a lasting change alone. The thresholds the following steps at are settings,
  "Coarser under" and "Finer over" (`Thresholds{Under, Met}`, 92% and 98% of the budget by
  default). The light and the atmosphere are levers too: "Shadows follow the frame budget"
  (`AdaptShadows{On}`: the lamps' shadows dropped first, the shadows baked coarse next) and
  "Clouds follow the frame budget" (`AdaptClouds{On}`: the clouds are bodies out to a share of
  the camera's sight from where it looks and a sheet beyond, "Clouds as bodies out to",
  `CloudReach{Share}`, the budget halving the reach and doubling it back, the look as set);
  the clouds' shadows on the ground and the figures are switches too (`GroundShade{On}`,
  `FigureShade{On}`); the shadows' settings sit under `Picture/Shadows`
  (`control.PictureShadows`), the clouds' beside the weather in the atmosphere's section.
  board/topography starts with all three following, in the order care, shadows, clouds. The
  topography is one: "Full care follows the frame budget" (`AdaptCare{On}`,
  `Settings.AdaptiveCare`) steps the full care's reach down its ladder as the rate falls under
  and back up as it stays over, the slider beside it showing it and setting it by hand. The
  rate the adapter follows is averaged over a window set on the settings screen, "Samples the
  frame rate over" (`Sampling{Over}`, `Settings.Sampling`, 1 to 20 s, 2 by default): coarser
  once a window falls under the budget, finer once two meet it. board/topography starts both
  profiles in layers.
- A picture in layers is composed once, in the frame's pixels, and drawn a layer at a time
  with the pieces placed into each (`render.Laid{Lay, DrawLayer, DrawOver}` in place of
  `Grounded`; `gpu.Draw.Place`), the Directs drawing through the cropped camera (the sprites
  and the routes gather once a composition), so the three composes of a frame in layers are
  one; the figures at every pixel are drawn over layers too, tested against the layers' depths
  placed into the screen's (`Image.PlaceDepth`, `gpu.PlaceDepth` in place of `ScaleDepth`);
  "Show the layers" outlines every inner layer whatever the pass. `Benchmark_Frame_Topography`
  has `persp,footed`, `iso,layered,sharp` and `persp,footed,sharp`.

**Faster ticks and frames** (2026-10-19, etap 1 of `doc/plans-perf.md`)
- The world tick at 5,000 boxes from 215 to 169 µs and no allocations: `clampStep` compares
  squares instead of calling `Hypot`, every plugin's simulation block and the world tick's roles
  reader are bound once. With aabbworld v1.13.0 (`Translate` keeps a box inside the world where
  it is) 128 µs.
- The collision tick at 8,388 boxes from 4.06 to 3.35 ms and no allocations: `rest` finds the
  pairs met this tick in a set, not a scan of the contacts.
- The views' ground and cover are copied once, not every frame: the board's cover keeps the
  cells' `Version` and `Changes`; `Benchmark_Views_Copy`.
- `render/gpu` keeps its passes, submits, bind offsets (an arena reset after `Submit`, as GLES
  keeps the slice till then) and the present quad; text is drawn without splitting its lines,
  the glyph pieces kept; the telemetry panel's text refreshed four times a second; the lamps'
  uniform names and the bake's strips kept; the input buffer swapped, spawn groups reused.
- `gram-probe`: `run_demo.sh` takes `PROBE_SEED`.

**The light** (2026-10-19)
- `plugins/light`: the light over a world as one plugin (`light.NewPlugin(world,
  Config{LampShadows})`, made before the board and the topography). What shines is an entity
  with a `light.Source` knob: a lamp reaching from where it stands, or the sun, a distant source
  whose way and strength of the hour the atmosphere writes beside it each tick (`light.Shining`;
  a scene without a sky sets one with `WithDistant`). `light.Distant` (from `atmosphere/sky`'s
  `Sun`) is the working light, `Shining × Source`, so an effect on the atmosphere's entity dims it (an eclipse).
  The eight lamps nearest a camera light its picture: the entities, and the ground through the
  shaders' `lampLightAt`.
- `light.Shadow{Cast, Take}` (from `entity.Shadow`): how an entity casts its shadow and which
  shade it takes, `render.Taking` bits (the relief's, the blocks', the clouds', the lamps' light;
  `render.Shines` none), honoured by every shader; a cell takes what its kind's `Takes` says.
  The world's `Lighting` hands every drawn entity its `render.Illumination` (its light, the
  lamps', the direct light's share, what it takes); `world.ShadowLook` and `board/lit.go`'s light are
  gone, the board keeping the weather alone (`WithAtmosphere`).
- The shadows in one place: the ground's shade baked by the light (`internal/bake`, moved from
  the terrain: the relief's shadows from the frame's `GroundHeights`, over the topography's
  relief given it with `OverRelief`, and the `Baked` blocks'; over level ground the blocks'
  alone, so a flat board gets its towers' and bridges' shadows), the live shadows laid by it over
  whatever ground the frame drew (`internal/cast`, from the terrain's and the hexes' shaders):
  the sun's as the `ShadowPolicy` turns them (`WithShadowPolicy`, from the topography), and on
  level ground the lamps' too, from the `Config.LampShadows` lamps nearest the camera, Shift+L
  off and on for a weak GPU. H `CoarseShadows` is the light's. The flat sprites from above take
  the shade in relief too. A flat board's still is shaded and lit by the lamps per pixel
  (`look/shade.go`, `render.DrawMeshOptions.Multiply`).
- The clouds' shadows declared in a scene: `atmosphere.CastClouds(CloudShadows{Ground, Figures,
  High})`, the atmosphere taking the light plugin; `topography.NewPlugin` and
  `atmosphere.NewPlugin` take it. Madeira: a lantern at a bridge's head, shining warm, casting no
  shadow and taking none. A world without the light plugin is lit white and casts no shadow:
  navigation/square and navigation/hex make one and light it with `sky.DefaultSun` in the scene.
- `examples/look/light`: a flat board under a sun of the game's own (`sun.go`, `WithDistant`
  each tick from the clock): a tower Baked, walkers' patches, the hero's Silhouette, a lantern
  lighting the night and casting their shadows by it, lava taking no shade (`cell.Kind.Takes`).

**The minimap** (2026-10-18)
- `plugins/minimap`: a small picture round a player's camera (`minimap.Plugin.For(cam, player)`,
  placed and dressed by the scene): the ground in miniature (`minimap.Ground`:
  `board.Plugin.Miniature`, `topography.Plugin.Miniature`), a dot for each kind whose figure
  marks it (`Styled(minimap.Mark{Color, Size})`), the camera's view outlined. It reaches as far
  as `Config.Reach`, never past the world's last band, turns with the camera
  (`Config.Turning`, Ctrl+M) or keeps north up, shows and hides with Shift+M, and is drawn anew
  at most 30 times a second, a picture of its own neither anchoring the bands nor saved.
- A click on it takes the camera there: `ui.Clicked`, an Input taking its picture's press
  itself; `cameras.CenterAt{Camera, X, Y}`. The cursor over an element covering a player's
  picture scrolls and turns nothing there (`ui.Hovered`). `ui.Element.Show`, `Hide`.
- `camera.Headed`: the way along the ground that runs up a camera's screen. `render.GeoM.Rotate`.
  `render.StrokePolygon` is one draw, not one a side.
- Madeira (turning), Madeira on the simple map and motion/expanse have a minimap.
- Fixed: on a torus a camera's View in a world cut into sectors lost the entities across the
  seam from its window.

**The sectors shown, the bands switched off, a world of a hundred thousand** (2026-10-18)
- Shift+G shows a world's sectors over the ground, by their bands, by how the camera sees them,
  or not at all (`world.SwitchSectors`, `world.Plugin.ShowSectors`): in relief over the ground's
  depth on the GPU, on a flat world as quads under the figures.
- Ctrl+G switches the bands off, every sector in the first and the whole world alive, and on
  again (`world.SwitchBands`, `world.Plugin.SetBanded`); the panel says "bands off". Both keys
  are the world's only where it is cut into sectors.
- `sector.Grid` gives its `Side`, `Cols` and `Rows`. Madeira's sectors are two buckets a side.
- motion/expanse: a hundred thousand boxes on a torus of about 98 km, alive in bands round the
  camera; switched off, the frame rate falls from about 700 to under 10.

**The ground by the sectors seen** (2026-10-18)
- In a world cut into sectors the ground drawn is that of the sectors a camera sees. The relief's
  mesh is laid sector by sector, each whole and coarse (every second corner within, the edge
  whole and stitched, no crack against a whole neighbour), drawn by ranges of its indices at one
  go (`render.DrawMeshOptions.Ranges`); a hex board's prisms by sector, their tops alone
  coarsely; the flat board's cells of the sectors seen, plainly undressed with no grid coarsely
  (`look.Tile.Coarse`).
- The ground turns into the fog before the camera's sight ends, by the pixel
  (`view.View.Fogging`, the shaders' `Sight`); a flat tile through `Frame.Fog`, which now takes a
  rectangle (`look.Tile.Fog`).

**What a camera sees of the sectors** (2026-10-18)
- In a world cut into sectors a camera's View is of the sectors it sees (`view.NewSeen`), each at
  a `view.Level`: Exact; Coarse where a world unit spans fewer screen units than the camera's
  `Detail.Coarse`; Fogged in the fog before the end of its sight; Hidden past it, off the picture
  or beyond every band. It holds their entities alone; `View.Level`, `View.Fog`, `View.Sectors`.
  `world.Plugin.Seen()` lays the first camera's on the telemetry panel.
- An entity seen coarsely is drawn plainly: no shadow, no edges, flat where it would stand. One in
  the fog turns into the fog's colour (`world.FoggyLook`; `render.Sprites.Fog`, the billboards'
  haze), the atmosphere's or a scene's (`world.Plugin.WithFog`).
- The look's rules run on the chunks with an entity in view alone; vision draws the views of the
  observers in view alone.
- Fixed: a camera seeing the whole world drew the entities beyond every band.

**How far a camera sees** (2026-10-18)
- `camera.Detail{Sight, Fog, Coarse}` (`camera.Detailed`, `DetailOf`): how far a camera sees from
  its `camera.Viewpoint` (the eye, else the point it observes), the fog before that, the scale
  below which it draws coarsely. `cameras.Config.Detail` sets it for every camera; `cameras.Sight`
  on Ctrl+= and Ctrl+- (`Keys.Farther`, `Nearer`) sees a `SightStep` farther or nearer.
- A camera saves its sight: a camera from above is saved as `"top 1"`, one in relief as
  `"stick 3"`; saves of either from before begin afresh, logged.

**The board's places** (2026-10-18)
- `board.Around(n)` and `board.Ahead(n)` are the board's places, for ceremony's `cond.At` and
  `act.At`: `cond.At(board.Around(1)).If(lake)`, `act.At(board.Ahead(2)).Do(cmd)` where
  `cond.Around(1)` and `act.Ahead(2)` were. Ceremony names no places of its own (v0.3.0: the
  kinds are a host library's, `host.NewPlacing`); `unit.Standing` and `cell.Now` are no longer
  `host.Placed` nor `host.Headed`, the board's kinds taking them.

**Systems in the bands** (2026-10-18)
- Every system goes in the world's bands by a policy (`sector.Policy`): `Everywhere` every step
  in every band (navigation), `Paced` the farther bands a step every `Pace` over as many steps at
  once (motion's steering, velocity, movement and leaving; gravity along Z; the world's plans
  and effects), `Near` the first band alone (collision, bullet's flight, vision, gravity
  across). Nothing goes beyond the last band. A world without bands goes as before.
- `world.Plugin.Pace(policy)` is a goke query option (`goke.PacedBy` by the clock's steps, a rule
  a band) a system adds to its query and walks with `Query.Paced`; `steering.System.Paced` and
  `vision.ScanSystem.Paced` take one, the world hands it to ceremony's plans and effects.
  `clock.Clock.Steps()` numbers the steps.
- What lies beyond the last band is out of the world's space: no view, sight, query or
  collision meets it. The world keeps it asleep by sector and wakes a sector's sleepers as the
  sector comes back into a band.
- Far off a box collides with nothing and keeps no contacts, an observer sees nothing, a shot
  hangs where it is.
- Needs goke 3.5 (paced queries) and ceremony whose `NewPlans` and `NewEffects` take query
  options.

**Sectors of the world in bands round the players** (2026-10-18)
- `world.Config.Sectors` cuts the world into square sectors, `Buckets` of the space's buckets a
  side (4 by default), in up to five `sector.Band{Reach, Pace}` round the anchors: the first at
  the world's pace, each farther a step every `Pace`, what lies past the last off. Without bands
  the world is as before.
- Once a tick the world lays each sector in the band its nearest anchor reaches (farther only
  half a sector past the reach) and marks the entities by their sector's band: `sector.Band2` to
  `Band5`, `sector.Off`, the first band unmarked; the marks are saved. No system reads them yet.
- The cameras plugin anchors the world at the point each camera observes (`camera.Focused`: the
  window's middle, the stick's foot, the eye inside a unit), a minimap's none.
- `world.Plugin.Sectors()` reports the sectors and the entities by band on the telemetry panel;
  board/topography shows them.

**The stick in perspective** (2026-10-18)
- The free perspective camera is the stick too: its foot in the middle of the screen, the eye at
  the stick's end, Q and E turning and R and F leaning it about the foot; it no longer flies two
  cells over the highest ground nor narrows its view (but inside a unit). The wheel draws the eye
  and the foot towards the cursor, the stick between a cell and the world's diagonal, coming in
  no further than the eye stays free; a pan slides the foot at its height.
- Where the cage puts something solid at the eye, the spring lifts it straight up at once,
  still looking at the foot, and lets it down as the way clears.
- Tab keeps the foot, the turn, the pitch and the scale both ways: the stick as long as the focal
  length over the scale. LookFrom puts the eye, LookAt the foot.
- A relief camera's save is `"stick 2"`: one saved with the orthographic stick alone begins
  afresh, logged.

**The tactical camera on a stick, orthographically** (2026-10-18)
- A camera in relief hangs on a stick whose foot is the point in the middle of the screen: a pan
  slides the foot at its height, Q and E turn the stick about it (2.5° a tick, `TurnStep` π/72,
  eighteen to an eighth), R leans it steeper up to straight down and F flatter down to
  `MinPitch` (the other way round before; `Tilt` grows towards straight down). From above and
  isometrically are one orthographic view at another turn and pitch: the view from above turns
  too. A turn or a tilt stands the stick first on what is seen in the middle of the screen, so
  that stays there.
- Tab only switches between the orthographic view and the perspective, keeping the foot, the
  turn, the pitch and the scale, and only where `cameras.Config.Perspective` reaches it.
- The cursor in a corner of the window turns the view the way Q does in the left corners and E
  in the right ones, a third as fast (`cameras.CornerStep`), where the Config turns
  (`cameras.Keys.CornersTurn`); elsewhere it scrolls slantwise.
- One measure: `Zoom` is screen pixels a world unit in every view; isometrically a camera begins
  at `TileW/(√2·Cell)` (√2 for 64 x 32 tiles of 32 cells; was 1), `camera.Config.Zoom` and the
  zoom limits read so. The marquee meets a figure as wide as it is drawn.
- Behind a unit the camera keeps it in the middle of the screen (it lay a quarter of the screen
  lower, and in perspective below the screen's edge). In perspective the eye is held on an arm
  two cells behind it, not at the ceiling over the highest ground: R and F lean the arm, and it
  shortens at once where the cage puts something solid between, easing back out as the way
  clears (it used to look down more steeply instead). Isometrically it stands as near, the scale
  that arm's. Behind a unit nothing zooms the camera, and letting go gives the tactical zoom and
  eye height back.
- The ground and what stands on it are clear through a camera with an eye too, in perspective
  and riding, until the haze is asked for: one switch for every camera, off as a game starts
  (Ctrl+F, `topography.Plugin.WithHaze`, `Hazes`, `topography.Haze`, were `WithHazeFromAbove`,
  `HazesFromAbove` and `HazeFromAbove`; `air.Weather.Seen` takes whether to haze at all).
- A relief camera saved before the stick begins afresh, the log naming it; one from above loads.
- Seen straight down with the view turned, a flat sprite turns with the world: its corners lie
  where the camera lays them, where it lay in the screen's box round them; so do the outlines a
  footprint draws (the selection's, a route's, the boxes). A sprite the screen turns other than
  by quarters is read smooth, its border fading over a pixel, so it no longer crawls as it moves.
- navigation/square crosses a river by a bridge, and its wagon, too wide for it, by the ford, as
  navigation/hex does.

**The cameras, all in their plugin** (2026-10-17)
- Every camera is the cameras plugin's: the plain one from above (was `internal/camera`) and the
  views of a world in relief, from above, isometrically and in perspective, riding with a unit
  (were the topography's). `cameras.NewPlugin(w, cameras.Config{…})` takes what the game's cameras
  may do — `Turn` (Q, E), `Tilt` (R, F), `Perspective` (Tab) — how the relief is drawn (`Cell`,
  `TileW`, `TileH`, `HeightUnit`, `Headroom`, `MinPitch`, `FieldOfView`, out of
  `topography.Config`, which keeps `Shaping` and `Climbing`) and where the cameras may be: a
  `cameras.Cage` (`Free(x, y, z)`, `Span()`, optionally `Grained`), the topography being one.
- A scene makes a camera as it begins: `s.cameras.New(cameras.FromAbove, …)` or
  `cameras.Isometrically` (was `cameras.TopDown()` and `topography.Plugin.Views`); `cameras.Maker`
  is gone. The views' commands `View`, `Turn`, `Tilt`, `Ride`, `Look`, `LookFrom`, `LookAt` are the
  cameras'; the old `cameras.LookAt{Camera, Entity}` is `CenterOn`.
- The cursor at the window's edge only pans: the view no longer turns and bows when a pan meets
  the world's edge (`camera.Scroller`, `Pan.Edge`, `Pan.Corner`, `topography.CornerStep` gone).
- A camera behind a unit keeps it in sight against whatever the cage says is solid, forests and
  decks too, not the bare ground alone. Tests take a plain camera from `cameras/cameratest`.

**Motion, a plugin of its own** (2026-10-17)
- `plugins/motion` moves the entities: the steering (`plugins/motion/steering`, was
  `plugins/world/steering`), the velocity with the rules of a `motion.Moving` (`Still`,
  `Braking`), the move and the leavers past an open edge (`motion.Outside`, `motion.Leaving`);
  `motion.Drift` and `motion.UnitTemplate` were the world's. A game makes it first
  (`motion.NewPlugin(w)`) and runs its `RunPlan` right after the world's; a world without it
  keeps its entities where they stand (look/sky).
- What steers or pushes takes it as it is made: `collision.NewPlugin(w, m)`,
  `bullet.NewPlugin(w, m, sel)`, `driving.NewPlugin(w, m, sel)`,
  `navigation.NewPlugin(brd, w, m, sel, drv)`, `gravity.NewPlugin(w, m, cfg)`; a board's units
  need `board.Plugin.WithMotion`. Collision and bullet hand a leaver to `motion.Plugin.Leave`.
- The world keeps the spawns and the despawns (an entity's own `Despawn` is carried out in the
  world's step) and indexes the space after either; motion indexes it after moving.
- Motion runs after the world's step, its effects among them: an effect on the steering tells the
  step it begins, and what a rule of `Moving` or `Leaving` puts on or takes off lands with the
  next step's effects.
- The world's entity template carries the effects' markers and a `Patch` shadow by default
  (things standing on the board theirs from the start); motion's unit template a course.

**Kinds' templates by name** (2026-10-17)
- `kind.Template` is a handle a plugin names (`kind.NewTemplate(name, parent)`): the world's
  `EntityTemplate`, the board's `StandingTemplate` and `CellTemplate`. A plugin asks of them in
  its `KindTemplates(r *kind.Roster)` (`plugin.KindTemplater`): `kind.Require`, `r.Default`;
  `r.Spec(t, own...)` builds a kind's spec, a template's defaults over its parent's. The roster
  is written as it is first read, by the world and every plugin made over it in the order made;
  one made after that panics.

**Shadows as strategies** (2026-10-17)
- How an entity casts its shadow is its `world.Shadow{Cast}` (`entity.Shadow`), a knob: a soft
  `Patch` every frame, the `Silhouette` of its sprite, `Baked` with the ground, or `NoShadow`.
  Units cast patches and entities standing on the board are baked unless their kind declares
  otherwise; an effect turns it for one entity. `entity.Block` is gone: saves made before do not
  load.
- Baked shadows have no limit: every `Baked` entity is drawn into the shade as an instance, the
  darkest kept (`render.DrawMeshOptions.Darkest`), after the relief's. The shade fades band by
  band from its last bake into the new instead of jumping as the sun turns.
- The sun on the ground is a frame image every shader reads (`render.GroundShade`; WGSL
  `groundShadeAt`, `standingShadeAt`): what stands in relief darkens in a mountain's shadow at
  its height, in a block's near the ground and under the clouds.
- Each entity casts its live shadow once, through the world's `ShadowLook`, as the topography's
  `ShadowPolicy` turns it by the distance from the eye (`Plugin.WithShadowPolicy`).
- `weathering.Wring` is saved: a knob under an effect at a save goes back to its value as the
  effect ends after the load (ceremony keeps the originals).
- `render.Image.CopyFrom`. The topography demo: G has the selected cast their silhouettes; its
  fords cast no shadow.

**The board's surface as a look** (2026-10-17)
- What the board draws beyond its tiles is a look a game declares as it does a tile's or a sky's:
  a `board.Surface` noted in the Stage's Figures link (`board.Plugin.Surfaces`), its `Look`
  setting on the board's atlas the line the grid is drawn in (`Atlas.Grid(board.Line{Color,
  Width})`) and how wide a cell held is outlined (`Atlas.Holds`). Without one the board draws
  `board.DefaultGridLine` and `board.DefaultHoldWidth`. Every demo with a board has its
  `surface.go`; board/atlas draws its grid of its own.
- The grid is drawn in that line wherever it is: from above over a flat board, tile by tile, on
  the topography's ground and on its hex prisms (`look.RenderState.Grid`). Tile by tile every
  side of a cell is drawn once, in lines over the tiles and the materials alike.
- `board.Plugin.Surfaces()`, where whoever moves stands, is `board.Plugin.Footing()`.
- Gone: `render.Frame.Tile`, `TileRect` and `OutlineOn`, the composer's outline of a sprite,
  `look.Tile.Outlined` and `Covered`, `look.Dressing.Covers`,
  `look.RenderState.ToggleShowGridLines`.
- Looks a little different: the hex grid's border on a flat board is as dark as its inner sides,
  the hex prisms' lines are a dark grey instead of black, the square grid on the relief fades over
  a pixel instead of three quarters of one.

**The weather in places** (2026-10-17)
- Rain, storms and clouds never cover the whole map: a storm in the middle of rain, rain amid
  clouds, the clouds with fair breaks, each on at most the share of the world the knob
  `climate.Spread` on the atmosphere's entity gives — rain a quarter, a storm an eighth, fair
  breaks up to a quarter (`climate.DefaultSpread`), an effect turning it. The telemetry panel
  names the weather of the time and the one over the map's middle where they differ.
- The high clouds are no longer drawn over the map from above: from the tactical view they
  veiled it. They show on the sky and as their shadows on the ground.
- The topography's ground and hexes haze only through a camera with an eye: from above, all the
  lines of sight one way, the start of the camera's field was taken for an eye, and the whole map
  but its corner turned the fog's grey, the greyer the cloudier the sky. Seen from above the
  ground is clear unless asked (`topography.Plugin.WithHaze`, Ctrl+F, `topography.Haze`): then
  hazed by the air over it (`air.Weather.Seen`).

**Clouds at the peaks** (2026-10-17)
- The air masses carry their weather's damp (`weather.State.Damp`). Ground standing up into the
  low clouds holds them back: they pile up before a ridge, spill over it and clear past it
  (`air.Dome.Lift` over an `air.Land` the weathering walks, `climate.Climate.OnLand`;
  `air.Mass.WithLow`).
- Every cell of a weathered board hears the air over it once a second
  (`weathering.Overhead`, its words `Clouded`, `Damp`, `Reaches`, `Hours`), and a rule of a role
  it plays turns its knob `weathering.Wring`: the rain the low clouds over it let fall, draining
  the air's damp, and the damp gathering into clouds over it. Madeira rains on its windward
  slopes and gathers clouds over its hills of an afternoon.
- `weather.Default` is written a field a line.
- An effect on a board's cell changes the board only if the terrain changes under it: shown as a
  cover, granting tags or turning a knob of the board's own; one turning another plugin's knob
  alone leaves it, and the topography's surface, as they are.

**Low clouds over the relief** (2026-10-17)
- The low heaps hang in three bands fitted to the ground's relief, whatever its height: under the
  peaks, covering them and over them (`air.Weather.LowBand`, WGSL `cloudLowBand`), each in
  places of its own as the weather shares them (`weather.State.Ceilings`, `Weather.Ceilings`),
  drifting slower than the clouds, so a cloud flying on rises or sinks slowly. The relief runs
  from the board's low ground to its peaks (`atmosphere.Plugin.Over(board)`, which
  `WithWeathering` calls; `climate.Climate.Over`, `air.ReliefOf`, a relief under 1 km taken as
  1 km), the high shreds always over the low heaps (`air.BasesOver`). A heap's deck lies in its
  band, the heaped ones rising past it.
- Seen from under their top the low heaps stand before the slopes that reach into them, and the
  clouds hide and are hidden by whatever stands nearer: they lie as deep as where they begin
  (`aloft`; `backdrop.Clouds.Before` leaves those before the ground to it).
- `render.DrawMeshOptions.ReadDepth` reads a depth buffer at the size it was drawn at, whatever
  the size of the target reading it.

**Low heaps and high shreds** (2026-10-16)
- The clouds hang in two layers (`air.Layer`): the heaps low, 1.5 km up (`air.CloudLow`), their
  bodies with lumpy tops reaching `air.Weather.Top`, and the torn shreds, thin sheets each at its
  own height from into the heaps up to 6 km (`air.CloudBase`, `Weather.HighHeight`), as the
  weather's `Bases` say (`air.Bases(scale)`). The heaps come the more the sky is covered and
  heaped, the shreds with a few clouds already and in patches (`air.Mass.Layer`,
  `Weather.HighAt`): the two together or apart, in rain and in sun. The heaps are four times
  smaller than before, an ellipse with puffs of many sizes on its rim.
- Each layer's shadow falls along the way to the sun from its own height, the low ones nearer,
  the high ones further off; ground standing over the low clouds is out of their shadow
  (`air.Weather.Shade(x, y, z, sun)`, WGSL `cloudsShadeAt`). The terrain, the hexes, the flat
  board's ground and `air.CloudShadow` read the clouds' tile for it; `air.Weather.Cloud` takes
  the layer.
- On the sky the high layer lies behind the low one, flat or heaped; seen from above the line of
  sight passes through the heaps' bodies and meets the shreds' sheets at their heights
  (`aloft.New(weather, layer)`), the clouds thinning away into the ground; an eye under a layer's
  top sees it on the sky, behind the hills. The sun's and the moon's discs hide behind either.
- `climate.Config.OneLayer` keeps the clouds in one layer as before, for a flat map nothing stands
  up into: heaps and shreds mixed, their shadows straight under them.
- `Clouds` (Shift+V) goes through four looks (`atmosphere.CloudLook`, `Plugin.ShowClouds(look)`):
  flat sheets on the sky and from above, the low clouds as bodies, both layers as bodies (the
  high ones heaped thin on the sky too, the game's start), their shadows alone. Seen from above
  the low clouds stand deep and let the map show through.
- The telemetry panel shows the moon on a line of its own, the weather a line each for its name,
  its temperature and its wind, and how the clouds look.
- The high clouds shade the ground only with `air.Weather.HighShade`, switched by `HighShade`
  (Shift+S, with the camera free): on as the game starts, off the low ones' shadows alone.
- Rain and snow fall from the low clouds alone (`air.Weather.FallsAt`), drop by drop in the world
  between them and the ground (`precipitation.Drops`): a dot from above, a short line from the
  side, leaning with the wind, too small to make out from far off, where the rain's fog alone
  shows it. The rain drawn on the screen is gone.
- The clouds' shadows on the relief are worked out at its lattice's corners, cheaper than before.
- `render.GroundHeights`: whoever draws the ground hands the frame its heights
  (`Frame.GroundHeights`, the terrain does), which a shader reads with WGSL `groundHeightAt`; the
  clouds from above hide inside the ground by them.

**Clouds as bodies** (2026-10-16)
- Seen from above the clouds are drawn half see-through over the world
  (`plugins/atmosphere/aloft`): thin slices of the clouds' noise from the layer's height up, the
  higher ones only where a cloud is thick, lit from above.
- Seen from under them they are heaps: the sky looks through the layer, a cloud's base flat, its
  top domed as high as it is thick, darker underneath, its thin edges shining towards the sun
  (`backdrop.Renderer.WithBodies`), worked out coarser than the screen.
- A scene has them so (`atmosphere.Plugin.ShowClouds`), `Clouds` (Shift+V) switches to their
  shadows alone from above and a flat layer on the sky.
- The clouds' tile holds a pixel round every level of what lies past its far edge, so it reads
  across its wrap: a seam across the heaps' cores is gone.

**Chords left to the shortcuts** (2026-10-16)
- A key held, down or let go no longer fires with Shift, Ctrl or Alt held unless its binding is
  in chords (`control.Binding.InChords`, `Binding.Takes`): Shift+W changes the weather without
  scrolling the board, Shift+R shows the routes without raising the head. Driving's keys and its
  jump are in chords: a sprint with Shift, a jump while sprinting, a walk while a grenade charges.

**Pixels written among the draws** (2026-10-16)
- `gpu.Texture.WritePixels` of up to 65536 pixels, a part of a texture or the whole of it, draws
  them in place among the frame's draws, from the draws' uniforms: no texture of its own, no
  submission, in the order of the draws. The air masses' picture, vision's views and the
  terrain's lattice, up to that size, are written so as they change; a bigger write goes as
  before.
- The buffers a frame's draws are gathered in grow to what a frame gathered, so a frame of many
  draws is submitted once.

**The weather born at the horizon and carried by the wind; rain only under its clouds**
(2026-10-16)
- The weather over a point is its air mass (`air.Mass`: cover, billow, falls, damp). The masses
  over the world and its sky out to the horizon are an `air.Dome`: level 0 over the world in
  cells of 128 world units (`air.BoardCell`), each level further out in cells at least twice as
  big, out to `climate.Config.Horizon` (three times the sight through clear air, 120 km, when
  zero), every level a ring of `air.Patch`es, entities of a fixed size saved with the ECS. The
  shaders read them from the frame's image `AirMasses` (`air.DomeImage`, a level written as its
  window moves or a patch of it changes) through WGSL `airAt(p)`.
- A new weather is born at the horizon the wind blows from and carried in over the world as the
  old one sails off, the clouds never growing out of nothing over the land; the weather now (the
  panel, `climate.Weathering`, the globals of `air.Weather`) is the one over the world's middle
  (`climate.Weather.Over`). A weather lasts at least as long as the wind takes to carry it across
  the world; without wind it settles where it is. `ChangeWeather` (Shift+W) and `SetWeather`
  change it over the world and upwind of it within two seconds of game time. `climate.Config.Patchy` has the horizon
  bring weathers lane by lane (`climate.Lane`), each its own, likelier its neighbours'.
- Rain and snow fall only under their clouds, as thick as the clouds let fall
  (`air.Weather.FallsAt`): the drops on the screen, the snow laid on the board; the rain's haze
  and the sky's greyness are the mass's over the eye (`air.Under`), the sun's and the moon's discs
  hidden by the clouds where their light crosses the layer.
- `air.Weather` gains `At`, `Most`, `Falls`, `FallsAt`, `Visibility(x, y)` (the field is
  `Sight`, clear air's), `Dome`, `Dry`; `air.Falling`, `air.Sight`, `air.Under`, `air.Hazer`;
  `Weather.Frame` takes the point under the eye, `Weather.Shade` a point. The WGSL cloud
  functions take the mass (`cloudField(p, billow)`, `cloudCover(field, m)`, `skyClouds(…, m)`).

**Frame images** (2026-10-16)
- What every shader of a frame reads that is too big for uniforms is a frame's image: a name a
  package registers as it starts (`render.RegisterImage`, up to `gpu.FrameImages`, 8), read in WGSL
  as a texture of that name, handed by a source's Compose (`render.Frame.Image`) to the composer
  and every Direct through the `Uniforms` they already copy; a draw binds them in group 3, with a
  sampler `wrapped` repeating past their edges (`gpu.Draw.Frame`, `MeshDraw.Frame`).
- `render.Image.Defer` has an image filled the next time a draw reads it, when there surely is a
  GPU.
- The clouds' tile is the frame's image `CloudTile` (`air.Tile()`, baked once, read by
  `cloudTileAt`): the sky and the terrain no longer bake a copy each; `air.BakeTile`,
  `TileWidth`, `TileHeight` and the terrain's `CloudFrom` are gone, the sky's image0 is free.

**A sky of the game's own, of four parts** (2026-10-16)
- The sky is drawn in four parts, a `backdrop.Dome`: the vault, the sun, the moon and the stars,
  each WGSL to the contract of `backdrop/shaders/sky.wgsl` (`SkyVault`, `SkySun`, `SkyMoon`,
  `SkyStars`, handed a `SkyAt` and a `SkyDisc`) with uniforms of its own a `Feed` sets each frame.
  `backdrop.Earth()` is the Earth's sky, part by part `EarthVault`, `EarthSun`, `EarthMoon`,
  `EarthStars`; a part without WGSL draws nothing; `Sun.Size` and `Moon.Size` scale the discs,
  `Moon.Color`, `Moon.ByDay` and `Moon.Image` are the moon's. The shader is assembled as the sky
  is dressed (`Renderer.Dress`), panicking on a uniform or a name two parts declare or one of the
  library's (`render.SharedUniforms`), compiled the first time it is drawn, once for a source.
- A game's sky is an `atmosphere.Sky` in its `sky.go`, noted in the Figures link
  (`atmosphere.Plugin.Skies`), each Look setting the parts it draws its own way; none is the
  Earth's. board/flat and board/topography note the Earth's; the new demo look/sky has its own:
  a turquoise vault turning violet at night, a big orange moon showing by day, made-up stars.
- Which stars are drawn is the sky's look: `sky.Config.Stars`, `celestial.StarSky`,
  `RealStars`, `ScatteredStars` and `Heavens.Stars` are gone; `backdrop.Stars` holds a
  `Catalogue` (`celestial.Stars()`, drawn by `StarField.Place`, which takes it) and `Dots`
  (`backdrop.ScatteredStars()`). `Place.HeavensAt` takes no stars.

**The sky's colours from one place; the clouds' shadows in the ground** (2026-10-15)
- The atmosphere's sky hands every frame the sun and the weather itself: its clouds no longer
  wait for a lit board or a relief. `air.Weather.Frame` hands the sky's colours, `Fog` at the
  horizon and the new `SkyOverhead`, the WGSL `skyAt(up)` between them; the vault and the relief's
  water both take it (`overcastSky()` and the sea's own 1.15/0.7 blend are gone). Without an
  atmosphere the topography hands the frame its still sky, giving way to one already there
  (`render.Frame.Uniformed`); the board's lit dressing and the painter hand none.
- The light by the sun's height is a game's: `sky.Config.Daylight` (`sky.Daylight` rows,
  `sky.EarthDaylight()` when none), `Config.Moon` the moon the night begins under
  (`Sky.Moonlight`); `Heavens.SunColor` is the sun's own colour where it stands, `SunColorAt` is
  gone. `celestial.Up`, `Risen`, `Gone`, `Night` hold the thresholds the light and the sky share.
- The clouds' shadows on a flat board are part of its ground, laid right after it in the opaque
  pass (`look.GroundDressing`, the board's own `draw.Clouds`), as the relief's terrain shades its
  own: the materials, the grid, the cells held and the sight drawn over them are no longer
  darkened. `atmosphere.Plugin.Clouds()`, `board.Atmosphere.Clouds()` and the package
  `atmosphere/overcast` are gone.

**The Stage's picture: a plugin says what it draws** (2026-10-15)
- `plugin.Plugin.Renderer()` is gone. A plugin gives what it draws to `plugin.Installer.Draws`
  in `Install` and has its looks declared through `Installer.Looks`, run once the Stage's Init
  returns, before Spawn or Load; `world.Plugin.Picture()` composes everything the Stage's plugins
  drew, one picture for every scene, readied once (`Composer.Init` runs once).
- A scene shows `render.NewFeed(cam, a.world.Picture())` and names nothing that draws: the
  demos' `picture()` is gone, its look options open `screen()`. Every plugin made draws: the
  selection and the routes in motion/bullet, motion/platformer and start/split-screen, the cells
  held (Shift+O) in every demo with a board. `board.HoldsRenderer`, `atmosphere.Precipitation`
  and the atmosphere's `Renderer` are gone; the board draws the atmosphere's cloud shadows once
  `WithAtmosphere` puts it under the sky (`board.Atmosphere.Clouds`).
- `render.Layer` is `render.Drawer`, `game.Scene.Layers` is `Drawers`, `ui.Layer` is
  `ui.Drawer`; `render.NewComposer` takes `render.Source`s.
- Nothing hangs on the order plugins register in: the marks lie at levels, `render.OnTop(level)`
  (the selection's outline, its box being dragged over it, the cameras' scroll zones over all);
  a route's goals, the cells held and a bridge in hand lie over their ground in the world.
- Shift+X switches the entities' boxes between their bases, lying at their altitude under their
  figures, their solids, the twelve edges from altitude to top in a world with heights, and none
  (`collision.BoxesShown`); the solids lie over the figures, hidden by the ground alone
  (`render.At.OverFigures`); `render.Frame.Edge` is a line whose ends lie at heights of their own.
- A figure in relief stands upright in the world, as tall on the screen as its Height stands
  there: inside its solid, shorter the more steeply the view looks down (it was a card turned
  to the screen, taller than its body seen from above).

**What is drawn over what is Z's alone** (2026-10-15)
- `render.Tier` and its tiers are gone. A piece of a `render.Frame` lies at a `render.At`
  (`Frame.At(x, y, z)`, `Lifted`, `OnGround`; `OnTop` for a mark, `Furthest` the sky, `Nearest`
  the rain); every drawing method takes it in place of `(tier, depth)`, `Frame.Each` hands it.
- The composer draws the opaque ground first (depth written), the world back to front by where it
  lies, tested against the depth, its depth worked out on the GPU from its height
  (`gpu.Vertex.DstZ`, `gpu.Sight`), then the marks on top. A frame without a ground has no depth.
- `render.Direct` has no `Tier()`: it asks in Compose where it draws (`Frame.DrawOpaque`,
  `Frame.DrawAt`, `Frame.ReadGround`), `Target.Pass` saying which pass, `Target.Ground` the
  ground's depth alone for one that reads it (copied by a draw, `Image.CopyDepth`; a buffer read
  must be `Depth.Readable`); `Frame.Places` lists the asks.
- What lay on a tier above another lies at a lift: board materials, grid lines, sight, cloud
  shadows, routes; the figures at `world.FigureLift`. So the routes and cones drawn in the frame on
  a flat board, and the cloud shadows, lie under the figures; a material look under the figures
  standing higher.
- `render.Sprites` takes world rectangles and lays them on the GPU through the camera's matrix,
  a wrapping world's copies in one call (`Rect`/`Turned` without a camera, `Layer(at, under)`,
  16 floats an instance); the world's renderer no longer culls every figure on the CPU:
  `Benchmark_World_Draw` −35…−50%, a frame of motion/collision drawn in about 1.5 ms in place of
  2.15.
- The sky is drawn behind the world (`Image.FillBehind`), `celestial.StarField.Draw` takes a
  `render.Target`; `world.DirectLook.Begin` takes the frame; `board/look.Dressing` lays what runs
  across the board apart (`Across`), `Dress` takes an `At`; `vision.ConePoint` carries an `At`;
  `render.Still.Draw` takes the target and the camera, and draws its pieces in the order they came.
- A topography camera's pixel step is taken by the origin: far from it the relief and what is drawn
  on the CPU met a pixel or more apart. `GRAM_SHOT` saves a frame for comparing pictures.

**A board stands only in a world with heights** (2026-10-15)
- `board.NewPlugin` and `atmosphere.NewPlugin` panic in a flat world, as the topography did; the
  board's flat branches are gone (`terrain.NewKinds`, `BuiltSystem`, `Field.SetHeights`; units
  and entities on the board always carry a `Z`; a relief laid in a flat world is no longer an
  error of its own). A veiled or Solid kind of no `Height` stands at every height, so a flat
  map's forests dim and its walls cut as before. Every demo with a board sets `Heights: true`.
- Gravity chooses its mode by the pull's axis, not by the world: along Z a body falls onto the
  surfaces, across X and Y (`Pull{Y}`) it falls in the plane, now in a world with heights too —
  motion/platformer is one, Z its depth. A pull both ways, along Z in a flat world, or turned
  from one axis to the other in play panics; `WithSurfaces` is refused for a pull across.
- `vision.Sight.Blockers` work in a world with heights: an entity or a cover on none of the
  layers is looked over whatever its height.
- Bridges choose their kind by the map's relief, not by the world: over a map without relief a
  bridge is a deck at the ground's level, one cell of water or of what its road may not take, on
  no piers; over a relief a deck at a height on piers. `Type.Pier` no longer panics.
- Navigation always plans by levels; a slantwise step on the ground passes a corner of water
  along a deck lying on the ground (a flat map's slantwise bridge), as the plain planner did.

**One hero on WSAD, a lever and a plate that always answer, the figure in the look demos** (2026-10-14)
- `cond.Ahead(n)`/`act.Ahead(n)` (ceremony v0.2.1): the places on the cells one to n steps in
  front of a unit, the way it faces, beside `Around(n)`; `unit.Standing` carries its `Heading`
  and is `host.Headed`, the board filling `Tick.Ahead` (the grid's direction nearest the
  heading, stopping at the edge; a cell faces nowhere, so a `cell.Now` refuses it).
- Figures and tiles: a kind of entity in one piece — `world.Figure{Define(w); Look(a, w)}`,
  `board.Figure{Define(b); Look(a, w)}` for one standing on the board (defined over it with
  `NewUnits`/`NewEntities`, looked on the world's atlas as any), `board.Tile{Define(b); Look(a,
  b)}` for a kind of cell with its covers — and, `Stateful` (`Effects`) or `Ruled` (`Rules`),
  with the effects and the roles that are its own. The Stage's new `Figures` link, right after
  `Plugins`, notes them (`world.Plugin.Figures`, `board.Plugin.Figures`, `board.Plugin.Tiles`)
  and the Stage runs their parts in the Effects, Rules and Kinds sections after the game's own
  functions, in the order noted (`world.Plugin.FiguresIn`), so a game's `Effects` and `Rules`
  keep only what several kinds share; `world.Atlas.Looks` and `board.Atlas.Looks` declare every
  look in a scene; `Kinds` is for what is no figure. `board.Plugin.World` is the world the
  board lies over. The wire demo is written so (`hero.go` with its boredom, `lever.go`,
  `plate.go`, `trapdoor.go`, `gate.go`, `tiles.go`; the arena keeps the mortal role, the
  commands and the map), as is every demo (collision's balls, minimal's box and scenes' mover
  are figures in their `demo.go`): kinds alike are one table of values (the
  island's `ground` tiles, its `walker{kind, color, ordered}`); a plugin with a register of
  kinds gives its own figure — `bridges.Figure` and `bullet.Figure` (`Define(p)`, `Look(a *world.
  Atlas, p)`, `Stateful`, `Ruled`), `bridges.Plugin.Figures`/`bullet.Plugin.Figures` noting
  them among the world's, `World()` on both — so the bridge and the ammo are figures too
  (topography's, flat's and hex's `bridge.go`, bullet's `round.go` and `grenade.go`) and
  no demo has a `Kinds` link left; a figure's numbers —
  its size, speed, timings — stand beside it, lowercase; the demos' arenas are received as
  `a`, not `s`.
- `WithRenderer` is gone and a scene names no atlas: `world.Plugin.Renderer` and
  `board.Plugin.Renderer` dress themselves from the figures' and tiles' looks the first time
  they are asked (the board's sprites the cell's bounds across); `world.Atlas.Close` and
  `board.Atlas.Close` bake the sheet and hand it to their plugin (`SetAtlas`, for a foreign
  `render.AtlasSource` by hand), `Looks()` returns the atlas; every other plugin makes its
  renderer as it is made (`Renderer()` never nil; the demos' 41 `WithRenderer(nil)` lines are
  gone). What a plugin draws about a kind is
  styled by the figure, as the bridge is: `render.Style` (a marker, asserted beside the type),
  `render.Styles` and `render.StyleOf`, `world.Slot.Styled(s)` on the kind's slot,
  `world.Plugin.Styles()` after `WithRenderer` — `selection.Highlight{Line, Width}`
  (`DefaultHighlight`; `HighlightStyle`, `HighlightStyleFn` and `DefaultHighlightStyle` went),
  `vision.Cone{Color}` (`DefaultCone`; `DefaultConeStyle` went; the cone's colour is an instance
  attribute of the views' shader, `WithStyle` still a composer for every kind), `navigation.
  RouteStyle` (`DefaultRouteStyle` or `WithRouteStyle`'s the kinds' default; `routes.Lines` carry
  colour and width a stretch). `board.Plugin.ShowGrid`/`GridShown` are the grid's option, which
  `WithRenderer` keeps (it set the grid on itself before). Square's walkers are outlined and
  routed in their colours, the vision demo's hawk edges its view in its own.
- An eye sees up and down within its `Vertical` field, turned by its `Tilt` (`entity.Eye`,
  `Eye.Within`): the scan keeps what lies in the field by elevation, the view drawn is the ring
  of ground the field meets (the CPU's from the eye's height over level ground, the GPU's per
  pixel), and riding a unit `Inside` the camera's pitch tilts the unit's eye, given back on
  letting go — a sentinel on a wall looking down sees the foot of the wall, looking up what is
  over it. `Eye.Vertical` is refused in a flat world. Madeira's walkers look 50° up and down.
- In relief a pan drawn back lays the view down again as far as the far edge bowed it when
  pushed on, down to the pitch it started from, and no further.
- The cursor in a corner of the picture turns the relief's view a quarter as fast as Q and E
  (`topography.CornerStep`), the left corners anticlockwise, the right clockwise; at a side it
  scrolls as before; a corner is a square of `control.CornerReach` 120 px a side
  (`control.Edges.Corner`, `Corner.Signs`; `Edges.Any` there), where any other camera scrolls
  slantwise
  (`cameras.Pan.Edge`, `Corner`; `camera.Scroller.Scroll(dx, dy, corner)`). `cameras.Zones`
  (Shift+H, `ShowZones`, `ZonesShown`) lays the scroll strips and corner squares over every
  picture in translucent yellow, rounded, the strips a gap short of the corners, drawn by
  `cameras.Plugin.Renderer()`, which every demo's composer takes;
  hidden but in board/topography, which shows them from the start.
- A `color.RGBA` given to gram is straight alpha, as every colour in it is written, and is
  premultiplied on its way to the GPU (`render.Frame`'s pieces, `Image.Fill`, the shapes and
  text, the sprite `Canvas`, the board's grid lines, the routes' and the views' shaders). Go
  defines `color.RGBA` as premultiplied, which gram followed: a translucent yellow at alpha 10
  then added nearly full yellow light and came out as bright as an opaque one. Translucent
  colours — the cones, the bridge in hand, the panels, the ghost, the zones — are as faint now
  as their alpha says.
- `control.KeyReleased{Key}` fires as a key comes up, `Context.Held` how long it was down, and
  not after a chord with modifiers fired meanwhile. `bullet.Shoot` fires from the unit its
  `Camera` is fastened to, as driving's hand drives it, and throws `Power` of its range. The
  bullet demo is driven as wire is — WSAD, the camera following the soldier, a tap turning him
  — Space fires and Shift held and let go throws a grenade, the further the longer held.
- Demos: `rules/wire` is `rules/mechanisms` (`make demo-mechanisms`), drawn plain — the hero a
  rectangle, the lever a square of its position's colour — its rules as they were;
  `look/animation` is a gallery of the figure's and the lever's animations, each kind labelled:
  a walker on a patrol stepping and braking, an idler bored by stages, a puller, a lever
  swinging by itself on a plan; `rules/effect` gains haste (J, the selected twice as fast for two
  seconds); `look/appearance`, `rules/trapdoor` and `rules/pressure-plate` are gone, what they
  showed being in the others.
- The planner knows a unit's width: `pathfind.Body` carries its box (`Size`), and a step along a
  deck's band takes only a body no wider across the step than `board.Board.Lane`, the band less
  its railings on the step's level; a wagon too wide for a bridge goes round by a ford. Every
  other step takes any unit: a unit is no bigger than a cell's side, `board.NewUnits` panics past
  it.
- The navigation-vision demos are navigation's square and hex now, the old two gone (`make
  demo-navigation-square`, `demo-navigation-hex`): navigation alone, with no collision, units
  sent by a right click, each side outlined and routed in its own colour, sight cut by walls and
  dimmed by forests; the hex one has a river with a bridge its 16 px scouts cross and a wagon of
  22 px goes round by the ford.
- The world seen from above draws by height: a standing sprite at the top of its box
  (`Z.Top()`), a lying one at its Altitude and before those standing as high
  (`render.Sprites.Layer`, sorted on buffers kept from frame to frame); a flat world, with no Z,
  draws as before. An entity standing on the board takes a Z given besides in place of the
  board's. rules/mechanisms has heights: the hero walks over the trapdoors and the plate and
  under the gate's top.
- A unit driven by hand looks ahead from its side the way it goes, not from half its longer side:
  the mechanisms' hero, 16 across and 32 down, walks up to the fence inside the open gate instead
  of stopping 8 px short of it.
- `look/material` and `look/ember` are one demo, `look/material`: four embers drawn out of their
  own state circle the ward's pulsing material; G calms the ward (C follows the selected), F
  douses the embers.
- The make targets are named after the demos' directories, `demo-<group>-<name>`:
  `make demo-rules-effect`, `make demo-start-minimal`, `make demo-board-flat` (was
  `demo-board`), `make demo-navigation-square` (was `demo-navigation`); each runs
  `examples/<group>/<name>` after `go mod tidy`, and `split-screen` has one now.
- The entities' boxes (Shift+X) are collision's: `collision.Boxes`, `Plugin.ShowBoxes`,
  `BoxesShown`, drawn by `collision.Plugin.Renderer()`, which every demo with collision has in
  its composer and whose plugin it hands to players, so the key works everywhere (`world.Boxes`
  and `BoxesRenderer` are gone). The list of shortcuts puts a
  key under the plugin whose defaults gave it (the bridge tool's `control.Take` is carried by
  players and was listed under "General").
- The wire demo's hero is bored in a round: still 5 s he is bored (`BoredEf`), and bored he
  goes round the stages his plan (`plan.New`) puts on and takes off — 5 s blinking
  (`BlinkingEf`), 5 s stamping a foot now and then (`StampingEf`; a series of three quick
  stamps is the effect `StampEf`, drawn `Phased` along its course), two turns to look
  about (`LookingEf`, a quarter turn either way, each 4 s or, on a coin, 10: the long look an
  effect that lasts, `LookingLongEf`, the plan idling until it is gone) — until he moves.
  Blinking and stamping are looks:
  `render.Timed`/`TimedFacing`, a look going round a `Beat` of named frames each for its span
  (a blink is two beats of two frames), so periodic
  cosmetics draw few sprites and need no effects. `render.Loops.Frame(now)` picks the frame.
  The hero's old idle frames are gone; blink and stamp frames are derived from the standing one.
- Fixed: under two effects at once an entity was drawn by the look declared *earlier*, not
  later as said — a `render.Swap` finds the kind's own sprite, so the first swap to match held
  and the rest missed the twin it put in. The world's atlas now adds the effects' swaps latest
  declared first (`Atlas.swaps`), so the later look wins as documented; the wire demo's stamp
  showed nothing under the blinking stage before.
- `render.SpriteID` is two bytes: an atlas holds `render.MaxSprites` (65 536) sprites, every
  frame of every way under every effect among them, where a byte held 256 — the wire demo's
  figure alone took nearly all. `world.Kinds().NewSprite()` and the board's panic past the last
  in place of wrapping round to sprite 0 and failing later as "added twice". Saves holding an
  `Appearance` do not load.
- A tap on a standing unit only turns it: `driving.Plugin.WithTap(d)` has a `Toward` key held
  no longer than d turn the unit and not walk it (`steering.Driven.Held`, how long the hand has
  lain on it). `world.Braking()` is the word of an entity slowing down (`Moving.Course`, the
  steering's, nil without one): the wire demo's hero holds `BrakingEf` while braking and is
  drawn bracing (two frames more in `hero.png`); his profile sets off from nothing
  (`V0` 0, `Accel` 4x, `Brake` 6x his speed), so a tap turns him and a hold walks him.
- `plugins/telemetry`: the panel in the top-left corner saying how the game runs — FPS, TPS,
  where a frame's time goes (`render.Spent`, set by the engine), the entities, the game time,
  the plugins' `render.Reporter`s (`With`) and the game's own lines (`Line`; `Clear` drops
  them all) — a scene of its own (`telemetry.SceneName`) in every Stage with a world, made by
  the Stage where the game made none (`telemetry.Of`), hidden until ~ (among the
  players' `GameBindings`, listed under "General"; `telemetry.Panel`, carried by the players)
  or `.Shows(MainScene, telemetry.SceneName)`; the panel never takes the input. Drawn with
  `ui` in the theme's font. `render.TelemetryRenderer` and `NewTelemetryRenderer` are gone;
  the six demos that had them show the panel from the start; the panel has no border, is
  see-through, never shrinks (`ui.Element.Steady`: an element keeping the widest and tallest it
  has needed, for words that change) and lays a value of several lines under its label.
  `players.SceneKey` has `Ctrl`.
- Every plugin's `doc.go` ends with a `# Vocabulary` section: its moments (with their words
  and subjects), the facts for plans, the knobs and the commands, saying who may give each;
  `doc/behaviour.md`'s tables say the same of all of them. `cell.Rail` and `bullet.Body` are
  marked knobs (they were called so); `navigation.Blocked` asserts its subject; navigation's
  `Touch` words are named as every other word is ("pushed by ally").
- gram needs goke v3.4.1: `Query.Seek` after an `All` or `Pick` iteration on the same query read
  another archetype's column, so a `Trigger`'s entity was read with a wrong `Label` and the
  plate's command fired only when the stale offsets happened to fit — the trapdoors it opened
  shut two seconds later with the hero still standing on it.
- `cameras.Keys` has `ZoomIn` and `ZoomOut` keys, = and - among `DefaultKeys`; the topography's
  `Raise` and `Lower` moved to Shift+= and Shift+-.
- `world.Boxes` (Shift+X) outlines every entity's box in red through `world.Plugin.
  BoxesRenderer()`, a layer for a scene's composer: a look for finding out why things touch or
  miss. The wire demo shows it; its plugins' keys are bound in the Players section, so K lists
  them by plugin, the game's own (Space, G) under Game.
- The list of shortcuts (K) is a `ui` screen: the game's description first, a gap, the title,
  then the groups of keys in columns as wide as their lines, filled down the screen's height —
  the columns no longer overlap. `ui.Scene.Replace(screen)` puts a tree built afresh in a
  scene's place, for such listings. The groups' names are `ui.Heading`s, in the theme's new
  `Bold` font (`render.DefaultBoldFont`, Go Bold), the title a `ui.Title` in its `Large`
  (`render.DefaultLargeFont`, Go Bold at 20), and the keys and their labels are two
  columns laid by `ui`, aligned, not padded with spaces. A `ui.Fixed` part counts its pixels in
  what its split needs: a `Fit` column of a table with fixed gaps measured too narrow and ran
  into the next.
- A hand-driven unit asked to go more than 45° off its way brakes first, turns, then walks
  (`driving`): a hero reversing no longer walks a circle.
- `render.Sheet`, `NewSheet`, `DecodeSheet`, `Sheet.At`: frames cut out of an image in cells.
- The wire demo's action key is Space; a press is `ReachingEf` (a second), answered when the
  lever stands still, `PullingEf` the hands on it for the swing: pulls and pushes alternated
  quickly are all answered. `character.png` went.
- The animation and appearance demos draw the figure of `hero.png` (a `hero.go` beside each):
  stepping in eight directions while moving, standing at rest, stamping while angry.
- Under two effects at once an entity is drawn by the look of the effect looked under later
  (the world's atlas swaps in declaration order; it was a map's order). The wire hero's pull
  takes his boredom off, and he turns at once (`TurnRate` 0): a key pressed standing moves him
  the new way, not a step the old way first.
- Boxes need not be square: `board.Shape{Size, Width, Height}` — `Width` across the screen
  where the box is no square — builds a unit's or an entity's box (`Shape.Box`), and a
  `render.Sheet` cuts frames `w` by `h` (`NewSheet(img, w, h)`, `DecodeSheet(name, png, w, h)`;
  drawn into a square sprite a frame is stretched, and a box of its shape draws it true). The
  hero's frames are 16 by 32, his box the same, pixel for pixel, so he stops at the lever's post
  itself; the lever's frames are cut to the 12 pixels drawn, its box what is drawn.

**A lever used by touch, an effect as the axis of time of an animation** (2026-10-13)
- `collision.Meeting` carries `Heading` (the way Self faces) and `Toward` (from its middle to
  Other's); `collision.Facing(within)` is the word of Self turned to Other within the angle, so
  a thing used by touch — a lever — needs no board: touching it and facing it.
- `render.Phased(n, draw)` (`render.Phases`) is a look of n frames spread along an effect's
  course, once; the world's atlas takes it `Under` an effect that `Lasts` (reading what remains
  of it on the entity through ceremony's `Active.Left`), and a material under an effect reads how
  far it has run as its fraction. `render.FromImage(img)` draws an image into a sprite pixel for
  pixel: frames cut out of a sheet.
- A pair of colliders parted stays met while it touches: a `Contact` at rest (`Resting`) on
  both sides, no impulse, no `Struck`, a `Meeting` every tick, until the two come apart — a hero
  resting against a post uses it by touch.
- The world's atlas draws a figure in eight directions: `Facing` first, then `Animated` with a
  drawer of the angle and the frame (frames per way), `WhileMoving` (the frames go only while the
  entity moves), and looks under effects drawn every way (`render.PhasedFacing`,
  `render.LoopedFacing`; `render.Looped` is a look of frames round and round by the clock while
  the effect is on). `world.Still()` is the word of a `Moving` entity neither walking nor
  strafing.
- The wire demo is one hero walked on WSAD (`driving.Compass`, the camera `Follow`ing him), a
  figure drawn from a sheet (`hero.png`, after `character.png`, without the sword): standing,
  stepping, bored after three seconds (`StillEf` `Then` `BoredEf`, rules of `world.Moving`),
  pulling. The lever is a post of infinite mass he walks up to, pulled and pushed back at hand
  (U), drawn from a sheet of four frames (`lever.png`, after OpenGameArt's "Lever and Button",
  CC0) along its transitions; pulled it opens its trapdoors, pushed back it shuts them
  (`Toggle`); a prompt over the hero while a lever is at hand. The plate's trapdoors open
  `.For(2 * time.Second)`. The scouts, porters, wanderers, haste and the keys 1 and J went.

**First person as a shooter plays** (2026-10-13)
- Riding inside a unit the mouse looks round at once: the opt-in `camera.Config.MouseLook`,
  `cameras.MouseLook`, `cameras.MouseLookKey` and `camera.MouseLooker` are gone, and the cursor is
  captured while a camera rides `Inside`.
- A and D strafe (`driving.Shooter`, the default keys inside; `driving.Strafe{Way}`,
  `steering.Driven.Side`, `Helm.RequestSide`): `entity.Velocity.Side` is the speed across the
  heading, to the right over zero, which `Delta` adds, so the unit moves sideways facing on;
  `Tank` still turns on A and D for the arrows behind a unit and the demos that drive from outside.
- Space jumps inside a unit (`driving.JumpKey(Space).In(camera.Inside)` among the driving's
  defaults), sprinting with Shift too: `control.KeyDown` is a key going down whatever else is
  held, which `JumpKey` binds; the world's pause is Shift+P (P alone freezes the sun), navigation's `Routes`
  Shift+R.
- board-topography runs the gravity plugin: its walkers stand on the board's surfaces, leap on
  Space and fall off a deck's edge.
- Fixed: a jump at a bridge's railing in first person ended with the unit turned along the
  bridge and sliding. A wall holding a unit left a residual of its speed (`Velocity.Slow`,
  `SetDelta`), which `SetDelta` took for a heading once the hand was off and the course's
  speed ran along it: what a wall leaves of the speed below a ten-thousandth is a stop, facing
  on; the driving lets go of a unit held still at once (`Course.Speed` is no momentum then) and
  asks for no speed, side speed either, whenever the hand is off. A wall — the ground's or an
  entity's, a pier — turns no unit carrying a `steering.Driven`, the keys held or not: a failed
  leap at it pushes the unit back facing as it stood (it was turned 90° along the wall with the
  keys let go in the air). board-topography's walkers
  leap 1.5 m: a leap as high as the railing (1 m) never cleared it. `unit.Standing` carries
  `Feet`, `Level` and `Reach`, and `unit.Fallen` holds only with the feet down at the cell's
  ground: a body over water in the air or on a deck has not fallen in, one landing in it has.

**Ceremony's store behind a contract: the binding `ceremony/goke`** (ceremony v0.2.0, goke v3.4.0)
- The world runs ceremony through its binding over goke, imported as `cgoke`:
  `world.Plugin.Effects()` is a `*cgoke.Effects` (the register embedded, the runtime beside it:
  `Cast`, `CastFor`, `Dispel`, `Has` without a command buffer; `Effect.On` is gone), the plans
  run by `cgoke.NewPlans`, a kind's components spawned by `cgoke.Spawner` and named to a save
  by `cgoke.LoadComp` (`comp.Spawner`, `LoadToken`, `TypeOf` and `RowOf` are gone: a `comp.Comp`
  is `Type`, `Row` and `Fill`).
- A plugin's pass binds its hosts to the columns of its query — `cols := cgoke.NewColumns(qb);
  host.Bind(cols)`, `cols.Own(&s.col)` for a column it reads itself — and runs them
  over each block with `cols.Row(cursor)` (`Run(t, row, n, about)`, `RunWhere`,
  `PairRules.Read(q, row, i)` in place of `InChunk` and `At`); `host.Tick` carries no command
  buffer and `world.Plugin.Tick` takes the step alone.
- `kind.Spec` takes a `plan.Planned` (`plan.New`); `navigation` reads `store.Mind`.
- `bench/behaviour_test.go`: `Benchmark_Plans_Tick`, `Benchmark_Rules_Moving` and
  `Benchmark_Effects_Tick` measure the behaviour a tick, so a change in ceremony or its binding
  is held to the numbers.

**Behaviours moved out to [`ceremony`](https://github.com/kjkrol/ceremony) v0.1.0**
- How entities behave — rules, plans, effects, roles, commands, facts — is the library
  `github.com/kjkrol/ceremony`, which gram builds on with goke as the store; its language is
  documented there (`doc/ceremony.md`), and nothing of the game is in it. What moved:
  `entity/tag` → `ceremony/tag`; `entity/kind/comp` → `ceremony/comp`; `rule/effect` →
  `ceremony/effect` (without `Look`, `Sprites`, `Looks`); `rule/cond` → `ceremony/cond`;
  `rule/act` → `ceremony/act`; `rule/plan` → `ceremony/plan` (with the store's part for now:
  `plan.Mind`, `plan.Plans`, `plan.NewPlans`, `plan.PlanID`, `plan.MaxSteps`); `rule` (`Part`,
  `NewPart`, `Plays`, `Played`, the commands `Put`/`Take`/`Toggle`, `Trigger`) and
  `entity/label.go` (`Label`, `LabelOf`, `Whom`, `Named`, `Group`, `World`, `ID`) → the root
  `ceremony`; `rule/message.go` → `ceremony/fact` (`fact.Fact`, `fact.From`, `fact.Subject`,
  `fact.Message`, `MaxChain`, `AskLife`); `internal/steps` → `ceremony/internal/steps`;
  `plugin`'s `Rules`, `PairRules`, `StepRules`, `Columns`, `Own`, `Side`, `SideOf`, `Marks`,
  `Tick`, `TickSource`, `About`, `Met`, `Placed`, `Aimed`, `Hosted`, `ErrUnhosted`,
  `ErrHostBuilt`, `Host` → `ceremony/host`; `internal/hosts.Deliver` → `ceremony.Deliver`;
  `control/command.go` (`Command`, `Queue`, `CommandQueue`, `Carrier`, `Issued`, `Routed`,
  `Unwrap`) → `ceremony/command`. `plugin.Installer.Hosts` takes `host.Host`s,
  `plugin.CommandHandler.Queues` hands back `command.CommandQueue`s.
- What stays in gram: `entity` (`Base`, `Position`…), `entity/kind` (`Spec`, `Define`, the
  roster), `control` (input alone: `Binding`, the triggers, `Context`, `Keys`, `Tool`), `plugin`
  (the extension contract), `clock`, the world's registers of effects, roles, plans and
  commands with `effect_commands.go`, and every plugin's moments, facts, words and knobs.
- A sprite's look under an effect is the world atlas's: `world.Atlas.Add(kind, size,
  look).Under(effect, look)` issues the twin slot and `WithRenderer` swaps it in; `effect.Look`,
  `Effects.Sprites`, `Effects.Looks`, `render.Dresser` and `render.Slot.Under` are gone, and
  every demo declares its looks on `s.world.NewAtlas()`.
- The player's id is `command.Issuer` (`control.PlayerID` is gone, `command.Nobody` in place of
  `control.Nobody`), and `Issued.Player` is `Issued.Issuer`: who gives a command from outside
  the entities, nothing of a player in ceremony.
- Facts are marked `fact.Fact` by a method `Fact()` and confirmed beside the type
  (`navigation.Blocked`, `Arrived`, `bullet.Flight`), knobs `effect.Knob` by `Knob()`
  (`steering.Steering`, `cell.Ground`, `Floor`, `Wall`, `collision.Physics`, `gravity.Pull`,
  `sky.Moon`, `vision.Sight`, `render.Appearance`; `effect.Alter[T Knob]`), as commands are
  `command.Command` by `Command()` and moments `host.About` by `Who()`.
- Saves written before do not load: the component types' paths changed.

**Conditions in `rule/cond`, rules in `rule/act`, plans in `rule/plan`: every clause says whom**
- `rule` is five packages: `rule/cond` the conditions, `rule/act` a rule's steps and branch,
  `rule/plan` a plan's loops, `rule/effect` the states, and `rule` what ties them to a game —
  the roles (`Part`, `Obeys`, `Plays`), the commands about effects (`Put`/`Take`/`Toggle` with
  `On`/`By`/`For`) and the messages (`From`, `MaxChain`, `AskLife`). The roles' tag family is
  `cond.Roles`, `cond.Role` what a `*rule.Part` is; `act.Rule` is the interface a host takes,
  `act.Build` a branch built by hand. A rule's steps and groups stand in a plan; a plan's
  cannot stand in a rule, and the compiler says so (`act.Step` is a `plan.Part`).
- A subject is a value with methods (Go 1.27's generic methods): the clauses are
  `cond.Self.If(x)`, `cond.Other.If(x)` (the other of the pair or the moment's subject; nothing
  holds and nothing is done while there is none), `cond.Around(n).If(x)` (each place within `n`
  rings) and `cond.On(plugin).If(x)` (a plugin's own entity); `x` an effect, a role, a word of
  the moment, `cond.Chance(p)` (drawn for whom it is asked), `cond.Always` or `cond.Not` of one
  of them. Clauses join with `.And` and `.Or`; `cond.Not` of a whole condition turns it round.
  A role of Self or Other at the top of a rule narrows whom the host walks; anywhere else it is
  a guard.
- A moment's words are the plugin's, with a side: `cond.Word[P]` made by `cond.NewWord(name,
  func(P) bool)`, `.OfOther()` of the other, `.Holds(v)` for engine code — `unit.Fallen()`,
  `unit.StandingOn(e)`, `unit.OnKind(k)`, `vision.SeesNobody()`, `vision.Closing()` (of the
  other), `bullet.Within(d)` (of the other), `bullet.Lying()`, `sky.FullMoon()`, `clock.At(t)`,
  `clock.Every(p, o)`, `gravity.HarderThan(v)`, navigation's Touch words as functions
  (`PushedByAlly()`, `Blocks()`, `WaitedLong()`…) and Blocked's `Stuck()`, `Cornered()`,
  `StrangerOnGoal()`, `IdleAlly()`, `IdleAllyOnMyGoal()`; a word of the wrong side is refused as
  the branch is built.
- A rule is `act.Branch[P]{Name, When, Then}`, every field required (`Name` at the top), a nil
  one panicking by the branch's name. The steps are the subjects' methods: `act.Self.Do(cmd)`
  (was `Apply`), `act.Self.Put(e)`, `Take(e)`, `Hold(e)`, `act.Other.Put(e)`,
  `act.Around(n).Do(cmd)`; `act.Steps` and `act.OneOf` order them with branches inside. A rule
  turning to places runs once a place: the smallest part of it holding every place clause, one
  ring a rule.
- A plan is `plan.New(name, root)` of `plan.Loop[P]{Name, When, Then, Until, Timeout,
  Cooldown}` on the fact `P` or `any`, in `plan.Steps` and `plan.OneOf`; `plan.Self` has a
  rule's steps and `Wait(d)` and `Idle()`, `plan.Other` and `plan.Around` are a rule's. A loop
  ends as its head says: `Until` is success once it holds, checked every tick from the loop's
  start before the steps — a fact's value in it, `cond.Self.If(navigation.Arrived{})`, is the
  fact come afresh since the loop began (at most 8 in one `Until`), a word of a fact holds
  while carried and holding, and the `Then` runs once from the start, the loop waiting after;
  `Timeout` fails it after `d`; `Cooldown` keeps it from beginning again within `d` of its
  last run (was `.Every`). A fact in a `When` or in `Not`, an `Until` of `Other`, `Around` in a
  plan are refused as the plan is built. A patrol is `plan.Steps` of two loops:
  `plan.Self.Do(navigation.Onward{})` with `Until: cond.Self.If(navigation.Arrived{})`, then
  `plan.Self.Wait(pause)`.
- `rule.Trigger{}` (was `rule.Triggered{}`), given with `act.Self.Do`, is how an entity sets
  off every command whose `By` names it.
- Gone: `rule.Branch`, `Rule`, `Step`, `Cond`, `All`, `Under`, `UnderCond`, `Playing`,
  `Paired`, `Chance`, `Apply`, `Hold`, `OneOf`, `Steps`, `Until`, `Wait`, `Idle`, `Self`,
  `Other`, `Here`, `Around`, `On`, `Plan`, `Build`, `Roles`, the frames `.Timeout(d)` and
  `.Every(d)`; `vision.Seeing`, `collision.Met`, `bullet.Reaching`, `navigation.Touching`;
  `unit.Over`, `unit.On`, `Standing.Fallen`, `Sighting.Nobody`/`Closing`, the Touch's and
  Blocked's exported methods, `Moonrise.Full`, `Flight.Lying`. `Here` is gone altogether: the
  cell under a unit is a word of `Standing`, `Around(0)` the cells under the box where a step
  needs them. A `rule.Command` without an `On` is no step.
- Saves holding a `Mind` written before do not load: the plans' shapes changed.

**The grammar of rules and plans; patrols as plans**
- A `rule.Branch` says when, on whom and what: `Branch[P]{Name, When, On, Do}`. `On` is required
  at the top of a rule and of a plan — `rule.Self`, `rule.Other` (the other of the pair, the
  subject of the fact), `rule.Here`, `rule.Around(n)` — and a branch inside a `Do` acts on what
  the outer one does unless it says otherwise. `ForOther`, `Here` and `Around` as steps went.
- Conditions: a predicate, `All`, `Not` (of a predicate or a condition), `Under(e)` with
  `.On(entity.World)` or `.On(plugin)`, `Playing(role)`, `Chance(p)`, a moment's own word
  (`rule.Paired(role)` underneath). `Free`, `During`, `While`, `Self(tag)`, `Other(tag)`,
  `Carrying[T]()` went; `Chance` is a condition, not a step; `Once` and `Either` are written out
  of the bricks.
- Steps: `Put(e)`, `Take(e)` (the commands `Put`/`Take`/`Toggle` with an `On`; `Cast`, `Lift`,
  `Apply` of an effect and `Dispel` went), `Hold(e)` (was `Keep`), `Apply(cmd)` (was `Order`;
  `Trigger()` is `Apply(rule.Triggered{})`). In a plan `Until`, `Wait`, `Idle`; a branch of a plan
  takes `.Timeout(d)` and `.Every(d)` (`Timeout`, `Cooldown`, `Invert`, `Ordered.Until`/`Stay`
  went).
- A fact is a thing that happened: a plan's branch of a fact begins once as the fact comes to
  hold its When, runs its Do to the end though the fact goes, and not again until the fact comes
  afresh. `Through` went. The guard demo's rest, which began again every two seconds on the
  `Arrived` it kept, is once a coming.
- Messages: a command embedding `rule.From`, given with `Apply` on a branch whose `On` is
  `Other`, lands on that entity as a fact; a branch on the command's type answers it. `Ask`,
  `Agree`, `Refuse`, `Relay`, `Asked`, `Replied` and `Answer` went.
- A patrol is a plan: the unit carries its own `navigation.Round` (`NewRound(cells...)`), gives
  itself `navigation.Onward{}` for the next cell and waits for `Arrived`; `navigation.Patrol`,
  `MoveOrder.Round`, `Pause` and `Stood` went. `navigation.MoveTo.To` names the goal
  (`entity.Named`): a cell or a unit by name.
- Saves with a `Mind` written before do not load: the plans' shapes changed.

**Every unit holds the cell it stands on**
- The board has every unit hold the cell it stands on, every step: one spawned in the game
  (`world.Spawn`) holds its cell from its first step, and one pushed, thrown or moved by a game's
  own system while nobody leads it lets go of the cell it was `unit.At`, is `At` the one it stands
  on and has `unit.Entered` on. A unit in the middle of a step holds both its cells and is left to
  navigation, which plans a route again once its unit stands off it between steps
  (`navigation.Path.From`).
- `cell.Occupancy.Holders(c, visit)` hands over every hold of a cell: a game's own Occupancy
  implements it.
- `board.Holds` (Shift+O; `Plugin.ShowHolds`, `Plugin.HoldsRenderer` for a scene's composer)
  outlines every cell held in its holder's colour, a deck's further in.
- `examples/navigation/spawn` (`make demo-navigation-spawn`): N spawns a walker under the
  cursor, and the cells held are shown.

**One brick: `rule.Branch`, and plans a kind carries**
- The notation of rules and plans is one struct: `rule.Branch[P]{Name, When, Do}` — when, what.
  Given to a role it is a rule of the moment `P`; in a plan it waits for the fact `P` (`Through`,
  it goes on once begun though the fact goes); of `any`, it asks only of state. `When` holds the
  conditions as values — a predicate of `P`, `rule.All(…)`, `rule.Not`, `Under`, `Free`,
  `During`, `While`, `Playing`, `Self`, `Other`, `Carrying` — in the moment's own words:
  `vision.Seeing(role)`, `collision.Met(role)`, `bullet.Reaching(role)`, `bullet.Within(d)`,
  `gravity.HarderThan(v)`, `navigation.Touching(role)`. New steps `rule.Once(e, step)` (a rule's
  memory, named) and `rule.Either(a, b)`. `rule.Then`, `rule.If`, the step wrappers `Unless`,
  `Under`, `During`, `While`, `Playing` and the filters `All`, `Self`, `Between`, `Other`, `Having`
  as filters are gone; a host builds the Branch it is handed (`plugin.Hosted`).
- A plan is a value a kind carries: `rule.Plan(name, root)` in its Spec, as `rule.Plays`; the
  world defines it as the kind is registered. `world.Plans()`, `plan.Actor` and the package
  `rule/plan` are gone; the steps that last (`Until`, `Wait`, `Timeout`, `Cooldown`, `Idle`,
  `Invert`, `Ask`, `Agree`, `Refuse`, `Relay`, the facts `Asked` and `Replied`) are `rule`'s. A
  plan's shape changed, so saves holding a `Mind` do not load.
- The grenade's fuse is a plan the grenade carries — `Until` its `Flight` is `Lying`, `Wait`,
  `Order` a `Burst` — in place of an effect chained by `Then` and a rule of `Resting`.
- `examples/rules/guard` (`make demo-guard`): guards on a plan — patrol, step aside when blocked
  long, rest where they arrive, run to the gate while the alarm (A) is on.
- Fixed: a route of one cell drawn between two goals panicked the routes' renderer.

**The examples grouped**
- `examples/` is a directory a group of what the demos show — `start`, `look`, `rules`, `board`,
  `navigation`, `motion` — and a short name in it (`examples/navigation/spawn`); README lists
  them a table a group, simplest first, the animation, material and ember demos among them now.
  The make targets keep their names.

**The game's own keys listed apart**
- The list of keys (K) begins with the game's own under "Game": what it binds in its Stage's
  Controls section and its `players.SceneKeys`; the plugins' keys follow by plugin, the players'
  and the engine's last, under "General" (they were under "Game").
- A plugin's keys a game sets up (`driving.Tank`, `Compass`, `JumpKey`, `cameras.MouseLookKey`,
  `selection.FollowKey`) are bound in the Players section with the defaults: the demos moved them
  there.

**Bridges built by one set of rules, by a player too**
- `plugins/bridges`: a bridge is planned by `bridges/site.Plan` over a `site.Ground` — straight
  along one of the grid's directions, three cells at most (`site.Rules.Longest`), over water or a
  valley a figure deep (`Under`: the game's figures, not metres), its deck `Clearance` over the
  ground, never beside another bridge's span nor over a road but one crossing it square, its heads
  levelled at the deck (`Reshape`) without breaking a road beside them; a pier between every two
  spans. A game defines its kinds of bridges (`Plugin.Types().Define(name, Spec)`); a bridge is its
  parts, entities on the board: a band a cell and its piers. A level's file writes a bridge as one
  line, `{kind: bridge, from: [x, y], to: [x, y]}` (`Plugin.Spawn`, held to the rules at Populate,
  an error with the line), and `cmd/gramap` plans its bridges with `site` too: leaps from a road's
  cell to a head, the absurd spans and the bridges over slight dips gone.
- A player builds one with a tool in hand: `control.Tool`, `Take`, `Drop` and `Binding.With` —
  while a player holds a tool, its bindings take their triggers from those of no tool (the
  players carry it out, `Player.Tool`). Shift+B takes the bridge; a left click marks its first
  head, the cursor lays it out yellow where it may not stand (`Plugin.Reason` says why), green
  where it may; a click on green asks "Build the bridge?" (`Plugin.Window`), and Build levels its
  heads on a `ground.Shaper` (the topography's relief: `topography.Plugin.Flatten`) and spawns it at
  the next step.
- A diagonal road lies on a level cell alone (`ground.RoadFits`): never lower on one side than on
  the other.
- `camera.Picker.Pick` gives the height of what it picked, a deck a thin slab a line of sight
  under it passes (`topography`'s `solidAt`); `control.Context.Point`; `Board.LevelFor`,
  `ThinnestDeck`; `navigation.MoveTo.Level` (`EitherLevel`, `OnGround`, `OnDeck`), kept by the
  order and its goals: a right click on a bridge sends units onto it, one on the ground seen under
  it under it, and the goal's outline stands at its height. A route under a deck reads the ground
  at every eighth of a step under every deck over it (`pathfind.Levels.Over`), so a unit sent from
  a bridge down under it goes off the bridge and comes in from beside, never back from the head
  along the deck; one standing on its goal's cell on the other level plans on, and the goal's cell
  crossed on the deck on the way is passed, not stopped on.
- A bridge's deck and its piers are blocks (`world.Block`): the terrain bakes their shadows with
  its own as the sun goes — the sun's line from the ground passing through a block, a penumbra as
  the relief's — soft, and costing nothing between the sun's moves; a deck's pieces lying end to
  end on one line are one block. A deck stands on its beam in relief, its Height
  (`cell.Floor.Thickness`) down its long sides; the island's are 10 m thick (gramap's recipe
  `roads.thickness`, the clearance and the underpass worked out from it). `world.Slot.Upright`
  stands a sprite true in relief — on the four sides of its box from its Altitude up its Height
  and over its top (`world.UprightLook`), never a billboard taller than it stands: the demos'
  piers.
- A uniform may be an array of vec4s (`gpu.Uniform.Size` over 16, a multiple of 4).
- A hold of the board's occupancy is of a cell on one of its levels (`cell.Occupancy.CanEnter(c,
  deck, …)`, `Enter`, `Holder`; `driving.Keeping.MayStep` takes the level too): kept a cell each,
  a walker on a bridge's deck and one on the ground under it share the cell. A route says which of
  its steps lie on a deck (`navigation.Path.Decks`), a step holds each of its ends on its level,
  and the board holds the cell a unit stands on under a raised deck on the level its feet are on.
- Over ground that does not take a walker only a deck's band does (`collision.Field.Overhang`,
  the board's): a walker pushed off a bridge's side is over the water, so a push never puts it
  there.
- A wall, a railing, never turns a unit a hand steers (`steering.Driven`): riding in it, walking
  into a bridge's railing no longer swings the view along the bridge.
- `board.Each[R](level, f)`: a plugin builds its own out of a level's lines.
- `ui`: a modal element left out — a window whose title says nothing — holds no input, nor does
  an anchor round an element left out or hidden: the bridge's window, there always, ate the wheel.

**Bridges begin and end in the middle of a tile**
- A bridge runs from a head to a head: a band from the middle of a flat tile at its deck into the
  bridge alone (`cell.Floor.Links` one way), no railings on it, so the road comes onto it from
  any side; no bridge ends in a point four cells share. Only Graded bands are bridges.
  `board.Bridges` lost `Reach`: a head flat at the deck takes the place of the ends on the ground.
  `entity.Lie.Ahead` lays a head half a link long from its middle.
- `Board.CellFor(p, feet, reach)`: the cell whoever stands at a point is on, a deck's over the
  ground's — navigation's and the driving's cell of a unit, the target of a right click — and a
  step along a deck through a corner passes over the cells beside it (`Leg.Decked`): a unit by a
  slantwise bridge's corner went on and off it for ever. Where a unit may stand is read point by
  point (`KindFor`, the decks' surfaces); a camera picks the deck over the ground, finer near a
  narrow one (`Board.NearDeck`, `NarrowestDeck`).
- A pan the world's edge stops in relief turns the view (left: the eye clockwise; right: the
  other way) or bows it towards looking down (forward); back, nothing.
- `cmd/gramap`: a bridge's heads graded flat at its deck, no road's corner planned over a deck;
  `area.sea` grows the area from the Earth until that many cells of sea lie round the land, the
  kept `units:` shifted with it; a `flat:` block (cell, road, deck) for `-flat`. Both Madeira
  maps are 204 x 106 cells; the flat demo's cells are 50, its figures 20, roads 44, decks 48.

**Bridges as bands: slantwise, on a hex, railed and drawn along themselves**
- Every grid gives its directions through `grid.Grid` — `Directions`, `Opposite`, `Offset`,
  `Step`, `Corner`, `Flanks` — with `grid.Way`, `Straight`, `Direction`, `Nearest`, `Exit`, `Rim`
  and `cell.Links.Has` on them; the copies of the square's table in the painter, the simple map's
  bands and covers, the planner, the field and `cmd/gramap` are gone. `ground.RoadFits(g, c,
  corners, links, run)` takes the corners in `CellOutline`'s order.
- `cell.Floor` gains `Links` and `Width`: a band that wide from the cell's middle out to the
  neighbours it names, the ground beside it the cell's own, through a corner reaching over the two
  cells flanking it. `cell.Rail{Keeps, Height}` lost `Sides`: a railing stands along both edges of
  a band carrying anyone. `Board.DecksOver`, `KindFor(c, p, feet, reach)` (a point now),
  `Reaches`, `Fenced`; `Along` counts a band's links; `Board.Railed` is gone.
- Collision's field holds slanted walls besides boxes (`collision.Field.Walls`,
  `collide.FieldWall`, aabbworld v1.12.0): railings and a band's raised slab, their ends free where
  the bridge ends; a shot strikes one where its step first touches it (`response.SweepWall`).
- The planner steps across no railing (the step's segment crossing it, a raised one on the deck's
  level alone) and slantwise along a deck over the corner it cuts; a leg along one holds no corner.
- A bridge is straight, level, its ends within `Reach` of the ground and its deck `Clearance` over
  it (`board.Plugin.WithBridges(board.Bridges{Reach, Clearance})`): broken as a map lays it, it
  panics with the file and the line of its entity; broken in the game, it is not built, said in the
  log.
- `world.Lie{Length, Width, Edged, Edge}` (`entity.Lie`): the board heads a band's entity along it
  and has it lie its length, Width wide, edged by its railings; the world lays a Lying sprite level
  over that rectangle (`LyingLook.Lay`) and `Slot.Edges(draw)` stands a sprite along its long
  sides — strips from above, quads lit by the sun in relief. The demos draw one deck sprite.
- `cmd/gramap` bridges slantwise in relief, cuts the ground under a bridge down to its deck, writes
  each bridge's `width` (the recipe's `roads.deck`, a cell's with `-flat`) and lays the map with its
  bridges' rules before writing it; both Madeira maps made anew. navigation-hex-demo crosses a
  river on a bridge at 60°.

**By name**
- Defining registers and hands nothing back; the handle is `Named(name)`, wherever a thing is
  built on, and a game keeps its names as constants — every demo has a `names.go`, each name with a
  suffix saying what it is (`Ef`, `Cmd`, `Role`, `Plan`, `Kind`, `Cell`), and its
  Stage's struct holds plugins alone. The default weathers have names: `weather.Clear`, `Fair`,
  `Cloudy`, `Rain`, `Storm`.
- Effects: `Effects.Define(name, spec)` returns nothing, `Effects.Named(name)` is the effect.
- Roles: `world.Plugin.Roles()` — `Define(name, rules...)`, `Named(name)`. `rule.Role` is gone
  (`rule.NewPart` is the world's).
- Plans: `world.Plugin.Plans()` — `Define(name, body)`, `Named(name)` the component of a kind.
  `plan.New` is gone.
- Commands: `world.Plugin.Commands()` — `Define(name, cmd)`, `Named(name)`.
  `game.Initializer.Commands` and `world.Plugin.Triggers` are gone.
- Kinds: `kind.Define`, `board.Units.Define` and `bullet.Shots.Define` return nothing;
  `kind.Named[P](reg, name)`, `Units.Named(name)` and `Shots.Named(name)` are the kinds, the row
  type checked against the definition.
- Nothing a game defines is the program's: the roles' tags and the plans' register were global
  and are each Stage's now (its world's). Two Stages may use one name for different things, and a
  Stage binds only the plans it defines.

**A level in a file, what stands on the board an entity**
- A game's map in a YAML file: `board.Plugin.Lay(fsys, path)` seeds the board in the Spawn
  section — drawn a character a cell under a legend of kinds (roles, names, groups), the relief in
  metres (`cells` or `corners`, `Level.Heights` for `topography.Seed`), roads along paths of
  points, every river a path down to where its water goes whose points turn
  its kind and width — and hands back a `board.Level`; `board.Spawn[Row](level,
  board.Sides)` seeds the world with its entities whose kinds have rows of `Row` — what stands on
  the board and the units, each a kind, a side (the commands the game gives it), a name, a group
  and the row's fields tagged `yaml`, a `cell.ID` written `[x, y]`. An error says the file and the
  line; an entity no Spawn took fails Populate. Every demo spawns its `map.yaml` so; their
  spawning code, layout helpers and board constants are gone.
- The demos' maps are their own files, their kinds, colours and styles each demo's own:
  `examples/island`, the island worked out in code, is gone. `plugins/board/water` stays, for a game working its rivers out of its heights.
- `board.NewEntities[Row](brd, shape, at)`: kinds of entities standing still on a cell —
  `cell.On`, a box at the cell's middle. Their knobs `cell.Floor{Allows, Height}` and
  `cell.Wall{Keeps, Height}` count in the cell's kind (`cell.Built` on the cell's entity,
  `Board.Built`, `Built.Over` in `Board.Kind` and `Board.Bare`): a trapdoor carries walkers over a
  pit until an effect opens it, a gate is planned round and pushed out of; a Floor's Height is a
  deck the topography stands whoever it carries on.
- `rule.Here` and `rule.Around` act on the entities standing on the cells too, never on the units.
- `cell.Now.Trodden` and `Stood` are gone: a plate is a sensor standing on the board, pressed in a
  `collision.Meeting` (`rule.Then[collision.Meeting]("press", rule.All, rule.Trigger())`).
- `relief.MeanOfCells` is the board's: `ground.MeanOfCells`. `kind.EntryOf(name, row)` and
  `world.Roles.Lookup` are new.

**Sight over a relief, exactly and as often as asked**
- board-topography fell to 8–29 FPS: 37 cones of 10 km sampled the relief every 50 m every step,
  a tick of 14–95 ms the engine caught up on. Now `ground.Profile` (`Along(origin, dir, length,
  visit)`) is Heights that knows its ground along a ray: the topography's relief hands a ray the
  points where it crosses a cell's edge or the diagonal its top is folded along, read off the
  corners, and vision reads the ground there (aabbworld v1.11.0, `Cone.Profile`) — exact, a few
  points a cell; a ground without one is sampled every `WithGroundStep` as before, the board's cell
  by default.
- `vision.Plugin.WithFrequency(perSecond)`: every observer scanned so many times a second of game
  time, the observers taking turns; the rules of a `Sighting` go on every step on what each saw
  last. board-topography scans 15 times a second: a tick of 2–3 ms, the frame bound by the GPU
  at 2560x1440. With heights a scan is shared among goroutines from two observers a worker.

**Gravity, steps, decks, railings, leaps; the planner on two levels**
- `plugins/gravity`: bodies fall, stand on what bears them and leap — a character's controller,
  one plugin in two modes, the world's dimension. `gravity.NewPlugin(world, Config{Pull})`: the
  `gravity.Pull` a knob on its entity (an effect turns gravity over); a kind carrying
  `gravity.Body{Leap}` falls, carried at a `world.Drift` the world's move pass adds to its walk.
  With heights it stands on the ground or a deck within its step (`WithSurfaces(board.Surfaces)`),
  follows a slope down, falls off an edge, strikes a slab with its head; a flat world bears it on
  what collision found it touching against the pull (a platformer; the pull turned up, it lands on
  the ceiling). `Leap{}` and `Away{Speed, Rise}` (aimed, a blast's knock-back) are an entity's own;
  `Landing{Speed}` is a moment for rules. `unit.Airborne` is on while a body is in the air, and
  `unit.Standing.Fallen` never is then (`Standing.Airborne`).
- `driving.Jump{Camera}` and `driving.JumpKey(key)`: a unit the hand is on leaps. The driving
  walks a unit only onto ground its domain may stand on on the level of its feet; one under
  gravity only where it may step up to, the rest physics.
- `world.Z.Step` (`Z.Reach()`: the step, a quarter of the height without one) and
  `ground.Surfaces` (`board.Plugin.Surfaces`, the topography's relief): the highest of the ground
  and a deck carrying the domain within the step of the feet, and the underside of a slab over
  them. `relief.DeckAt` is gone: a unit steps onto a deck from any side, passes under one it fits
  under. `cell.Floor.Along` is gone; `cell.Floor.Thickness` is a deck's slab, which collision
  strikes a unit too tall for (`collision.Band.Step`, `Band.Above`). `Board.KindAt(c, deck)` and
  `Board.KindFor(c, feet, reach)` are a cell's kind on one of its levels.
- `cell.Rail{Keeps, Sides, Height}`: railings along a cell's sides on a square grid — thin boxes
  in collision's field, a leap over their top clears them, the planner never crosses one. The
  demos' bridges have them on their long sides and a slab.
- The planner goes by the unit's Z in a world with heights: a cell under a deck has two levels,
  a route goes onto a deck within a step of it and off it only onto a surface within the step,
  under it only fitting under its slab — never a fall.
- `cmd/gramap`: a recipe's `roads.figure` (metres, the tallest figure as the game draws it) sets
  how high a bridge stands over a road under it.
- `bullet.Blast` has a `Subject`, the shot bursting: an aimed command under `ForOther` is about it.
- `examples/motion/platformer` (`make demo-platformer`): a hero in a side view under gravity, A and
  D walk, W jumps, G turns gravity over, a hard landing dazes.

**Madeira, roads as Transport Tycoon lays them, bridges and fords on the board**
- `cmd/gramap` makes a map of any stretch of the Earth at any scale out of a recipe
  (`cmd/gramap/madeira.yaml`): heights from the Terrain Tiles on AWS, the ground, the rivers, the
  roads routed and the ground graded under them, bridges and fords, laid on a board before it is
  written; `-flat` without heights. The board and board-topography demos are Madeira, 192 x 96
  cells of 333 m, in its true proportions; the old island is gone.
- Roads lie as Transport Tycoon lays them: `ground.RoadFits` (level, an even ramp of
  `ground.RoadSlope`, 10°, run straight up, or a level diagonal; a bend, a fork, an end level),
  `ground.RoadJoins` on a hex grid. `board.Plugin.Lay` refuses a road breaking it with the line,
  and the topography's Raise, Lower and Level take back whole what would break one.
- A bridge and a ford are entities standing on the board: `cell.Floor` has `Cost`, `Graded` and
  `Along`, the way a bridge is come onto — a unit stays on the deck it stands on, goes up onto one
  only coming along it from its end, and whoever comes from the side, a road under the bridge,
  stays under it. A road leads onto a bridge or a ford at its path's ends, `{onto: [x, y]}`.
  `cell.Crossing`, `Board.Crossing`/`SetCrossing`, `Layout.Crossings`, `network.Across` and a
  level's `crossings` are gone. A `world.Velocity` given to a kind of `board.NewEntities` takes the
  still one's place.
- `world.Slot.Lying()` lays a sprite flat — its looks under effects, frames and twins too: a
  `world.LyingLook` (the world's flat look, the board's lit look, the topography's billboards)
  lays it under what stands, in relief at the entity's altitude, on the ground where it stands
  higher.
- `world.Scale.Figures` and `Scale.Figure(m)`: how many times bigger than life a game draws its
  figures; speeds and sight stay real.

**The calendar's day**
- `calendar.Config.Day` is `DayRealTime`, `DefaultDay` `DefaultDayRealTime`: how much real time a
  day takes at tempo 1, counted in game time.

**Looks are the scene's**
- A flat board is lit by the atmosphere through the board: `board.Plugin.WithAtmosphere(a)`, a
  `board.Atmosphere` (`Sun`, `Air`, the shape of `topography.Atmosphere`), in place of
  `atmosphere.Plugin.WithBoard`; the demos call it, `topography.WithAtmosphere`, the painter's
  styles and `vision.WithViews` in their scenes, as the renderers are made.
- `vision.Plugin.ShowCones`, `HideCones` and `ConesShown` in place of `Hide(bool)` and `Hidden`.
- `atmosphere.Plugin.WithWeathering` is called as the atmosphere is made, its effects defined in
  the Plugins section, not in Effects.

**Saving and loading, the engine's commands**
- Saving and loading are the engine's: `game.Save{Slot}` saves the active Stage,
  `game.Load{Stage, Slot}` enters a Stage from its save — from any Stage, a menu's Continue —
  `game.Enter{Stage}` enters one afresh and `game.Quit{}` ends the game. Commands like any other,
  bound to keys or ui buttons; every Stage carries them, one without a world too, and the engine
  carries them out after the input. `Runtime.Save`, `Load` and `Issue` (a command given as nobody,
  how a ui scene without `Issue` gives its buttons') are the same at hand. A Stage's saves lie in
  `Props.Saves`, a file a slot.
- `game.Stage` has `Loaded()` in place of `Restore(Persistence)`: called instead of Spawn when the
  engine entered the Stage from a save. The `Restore` link of `game/stage` is gone.
- The players' default keys are K, F5 (`game.Save`), F9 (`game.Load`) and Shift+Esc
  (`game.Quit`) in every game; `players.Plugin.WithSaves`, `players.Save` and `players.Quit` are
  gone. The demos no longer load at start: F9, or scenes-demo's menu.

**Commands**
- A command is a `control.Command`: a type with the method `Command()`, confirmed where it is
  defined (`var _ control.Command = MoveTo{}`). Every command of the library is one, and whatever
  takes commands takes a `control.Command`, not `any`: `control.Queue`, `Issued`, `Carrier.Put`,
  `Give`, `players.Plugin.Issue`, `kind.Entry.Told`, `rule.Order`, `ui.Button`, `ui.Scene.Issue`.
  The binding built from a `Context` is `control.Build[C]` (it was `control.Command[C]`), its
  command's type `Binding.Type()` (it was `Binding.Command()`); `control.Routed`,
  `Contextual`, `rule.Router.Route` and `ui.About` hand back a `control.Command`.
- The register of a Stage's commands about effects is `world.Plugin.Commands()` (it was
  `Castings`), the type `rule.Command` (it was `rule.Casting`); the world's carrier is
  `world.Plugin.Carrier()` (it was `Commands()`).
- A command kept in a register is given to a key only as the register hands it back:
  `control.Give` panics for a `rule.Command` made on the spot.
- What a plugin gives is in its root package's `commands.go`: `board.Grid` (B shows and hides the
  grid), `players.Quit` (Shift+Esc), `players.ShowShortcuts` (K), `players.Save` (F5, given
  `WithSaves`), beside `Pan`, `Zoom` and `Give`; `atmosphere.Freeze`, `Later`, `Earlier`,
  `ChangeWeather`, `SetWeather` in place of `sky.Freeze`/`Later`/`Earlier` and
  `climate.Change`/`Set`; `world.Pause`, `Faster`, `Slower` in place of the clock's. The parts keep
  methods (`clock.TogglePause`, `sky.Shift`, `climate.Change`), no commands.
- A scene hands its input to `players.Plugin.Handle(events, runtime, composition)`, which also
  carries out `Quit`, `ShowShortcuts` and `Save` and runs the game's own keys
  (`Plugin.OwnKeys`). The list of shortcuts is the players' own scene, in every Stage that uses
  them (`game.Scenic`): `Plugin.Shortcuts(keys)` is gone.
- The default keys are the same in every demo and none repeats them: Space pauses (the world's
  tactical pause), Shift+Esc quits, K lists the keys, B the grid, F5 saves. `bullet-demo` fires
  on F.

**No hooking**
- A game hands its rules to nobody. `game.Initializer.Hook` and every plugin's `Hook` are gone: a
  rule is a role's, a role is played, and once `Init` returns the engine gives the rules of every
  role played to the plugin in use that catches their moment (`plugin.ErrUnhosted` for one none
  catches, naming the rule and its role).
- A plugin tells the engine the hosts of its moments' rules in `Install`:
  `plugin.Installer.Hosts(hosts...)`, a `plugin.Host` being a `Rules`, a `PairRules` or a
  `StepRules`.
- A kind of cell plays roles: `board.Plugin.Plays(kind, roles...)`; `cell.Entry.Roles` is gone.
- The world and the atmosphere play roles (`world.Plugin.Plays`, `atmosphere.Plugin.Plays`): a
  rule of a `clock.Moment` or a `climate.Weathering` fires while the plugin plays its role;
  `plugin.StepRules` takes a role's rule.
- `plugins/collision/hooks` and `plugins/vision/hooks` are gone: a plugin ships no ready-made
  reactions. The collision and vision demos write theirs in rules; `vision.Sighting.Nobody` and
  `Sighting.Closing` are the conditions they need.

**A plugin is an entity**
- Every plugin has one entity of its own in the world, called by the plugin's name
  (`world.Self`, made with `world.NewSelf(w, name, knobs...)` and embedded in the plugin): it
  carries the plugin's knobs, the roles the plugin plays and the effects it is under. A plugin is
  whom a command may be for — `rule.Cast(e).On(s.atmosphere)` — plays roles (`Plays`), and learns
  its knobs were turned from `Changed`. Made as the Stage is set up, found again by its name in a
  loaded game.
- The world is no exception: its own entity is the clock's, named by `entity.World` and by the
  plugin alike. `rule.While(plugin, e, step)` is `During` for any plugin's entity.
- A `climate.Weathering` is about the atmosphere's entity, no longer the world's: an effect its
  rule applies is a state of the atmosphere.
- A plugin's entity holds every effect at once (`effect.Wide`); units and cells still hold eight.
- The moon is the atmosphere's knob, `sky.Moon{Color, Strength, Face}`: an effect on the
  atmosphere turns the night's light and the moon's face (`Config.LightAt` takes the Moon).
  `sky.Moonrise` is the moment the moon comes up, with `Moonrise.Full`. The topography demo has a
  blood moon: at a full moon's rise, or on M.
- Saves made before do not load: the clock's entity bears a name now.

**A Stage defined in sections**
- `game/stage`: `stage.New(name).Plugins(f).Players(f).Cells(f).Effects(f).Rules(f).Commands(f).
  Kinds(f).Controls(f).Looks(f).Scenes(f).Shows(names...).Restore(f).Layout(f).Units(f).Update(f)`
  is a `game.Stage` defined a part at a time, always in that order: each link hands back a type
  with the later links alone, so a chain out of order does not compile; any link may be left out
  but `Update`. The Stage keeps its name, makes its stack, shows the first scene and tracks the
  Composition itself.
- What is defined out of its section is refused (`plugin/section`): a plugin used outside
  Plugins, a kind of cell outside Cells, an effect outside Effects, roles given to a plugin or a kind of cell outside Rules,
  commands outside Commands, a kind of unit outside Kinds, keys bound outside Players and
  Controls, drawing rules outside Looks, the board seeded outside Layout, units outside Units.
  Plugins takes anything; a Stage with an `Init` of its own is checked for nothing.
- Every demo and `examples/start/minimal` is defined so. `effect-demo` lost its roads.
  `atmosphere.Plugin.WithWeathering` may come after the plugin is used.
- One way to write a rule: `rule.On` and `rule.Moment` with its methods are gone, replaced by
  `rule.Then[P](name, filter, step)` and the package's steps.

**Commands, names and groups**
- What somebody asks for about an effect is one command, written as a sentence: `rule.Cast(e)`,
  `rule.Lift(e)` or `rule.Toggle(e)`, `.On(whom)`, `.For(d)`, `.By(source)`. Whom is
  `entity.Named(names...)`, `entity.Group(names...)`, `entity.World`, or the selection's
  `Selected(roles...)` and `Pointed()` (the entity under the cursor as the key is pressed). A key
  gives it with `control.Give(trigger, label, cmd)`, a script with `players.Issue`, a rule with
  `Order`; an entity sets off the commands whose `By` names it with the step `rule.Trigger()`.
  The game hands its commands to `ctx.Commands(cmds...)`, and a name one of them says that nobody
  bears, or one two bear, stops the game as it starts.
- An entity is called by what makes it: `cell.Entry{Name, Group}`, `kind.Entry.Named(name)` and
  `InGroup(group)`; it carries an `entity.Label`, saved with it.
- Gone, each replaced by the above: wires (`world.Plugin.Wire`, `rule.Wire`, `Wired`, `Signal`,
  the steps `OnWire` and `WhileWire`, `cell.Entry.Wired`), `world.Apply` and `world.Dispel`,
  `selection.Apply`, `rule.Part.Can` and `selection.Plugin.Abilities`. Saves holding wires do not
  load. `examples/rules/wire`, `trapdoor-demo` and `pressure-plate-demo` are written with commands
  for named cells and groups; `effect-demo` freezes whoever the cursor points at with F.

**A game without tags**
- What a plugin keeps of a unit is changed by that plugin's command: `players.Give{To}` hands it
  to a player, `selection.Allow{Selected}` and `Forbid{}` say whether it may be selected, and
  `kind.Entry.Told(cmds...)` has the entity give them itself as it is made — carried out in the
  first step. A kind names no owner and no Selectable tag, so one kind serves several players.
  `Player.Owner()` is gone; `selection.Plugin.IsSelected` is a drawing rule's condition.
- `rule.Other(role)` lets through the pairs whose other plays a role; `vision/hooks` are rules
  for a game's roles to obey — `Flee(threat, fleeing)`, `Chase(prey)`, `Search(prey, looked)` —
  and `hooks.Tags`, `DefineTags` are gone.
- The cells' tags of places are gone (`cell.Tags`, `cell.Family`, `cell.Tag`, `cell.Entry.Tags`,
  `unit.Standing.Places`): a place is a role its cell plays, or what it is called. Saves holding
  them do not load.

**Rules**
- `rule.Then[P](name, filter, step)` is a rule without a body: its steps are the package's own
  functions, the twins of a Moment's methods (`rule.If`, `OneOf`, `Steps`, `Apply`, `Keep`,
  `Order`, `Around`…), its conditions predicates of the moment — `unit.Standing.Fallen`,
  `unit.On(kind)`, `rule.Not(pred)`. A step its moment cannot run panics by the rule's name.
- A role a kind plays (`rule.Plays`) is hooked by the engine once `Init` returns; `ctx.Hook`
  remains for rules of no role and for roles cells alone play. One the Stage hooks itself is
  hooked once.

**Board**
- A state of the ground is a cover, the cell staying the kind it is: `board.Plugin.Covering(e)`
  is a slot of the board's atlas laid over the cells under the effect `e` along a line of its
  own — cutting the corners of a block of cells, a cell alone a diamond — not along the cells'
  edges; on the simple map, a square grid (whole cells on any other). `Board.States(c)` and
  `unit.Standing.States` are the effects on a cell, `unit.Over(e)` the condition of standing on
  one under `e`; `effect.Effect.Shows` has an effect that alters nothing change its entity as it
  begins and ends. `examples/rules/effect` keeps grass, road and water what they are: snow and ice
  are effects turning their knobs, drawn as covers.
- `render.Frame.SpriteBlendPart` blends a part of a sprite over a part of a tile.
- A kind of cell is what it is, never how it looks: `cell.Kind.Color`, `cell.Kinds.Draw` and the
  board's own atlas are gone. `board.Plugin.WithRenderer` takes the game's atlas
  (`Plugin.NewAtlas`) and panics on nil; the simple map lays the ways and crossings as bands cut
  out of their kinds' sprites (`render.Frame.SpriteFan`: a polygon filled with the part of a
  sprite under it), so a way shows its kind's look as a cell of the kind does.
- `Board.At(x, y)` is the cell at grid coordinates (it was `CellIndex`); `board.Plugin.Board()` is
  the board (it was `Res.Logic.Board`).

**Drawing**
- `effect.Effect.Look(sprite)` is the atlas slot drawn under the effect in place of a sprite,
  issued as it is first asked for — in the scene's `Layers`, where the atlas is drawn — and
  swapped in by `world.Plugin.WithRenderer` itself, after the rules of `Draw`.
  `effect.Effects.Named(name)` finds a defined effect again at building. `examples/rules/effect`
  is laid out a section a thing: plugins, player, cells, effects, roles, kinds, scenes.
- `render.Swap(twins, when...)` draws an entity as the twin of the sprite it would be drawn with,
  from a table a sprite a kind: a kind's own look under a state, swapped in under the effect's
  marker (`e.Mark().In`), a twin a way faced after `Facing`, a kind with no twin left as it is;
  two states compose in the order the rules are given. `render.With` takes conditions too.
  `examples/rules/effect` freezes each kind in its own look this way, the `frozen` effect left to
  the knobs it turns; `Alter(Appearance)` remains one look for every kind.

**Demos**
- `examples/motion/bullet`: a soldier on WSAD (`navigation.DriveBindings`) shoots rounds with Space
  the way it faces and throws grenades with G at the cursor, over a low wall and into a high one;
  a wound, a fuse and its bang are effects, the shots' doings rules of `collision.Meeting`,
  `bullet.Landing`, `Resting` and `Blast`.

**Bullets**
- `plugins/bullet`: shots as entities of the world a `Shoot` spawns at a unit's muzzle — by a
  player from its selected units, by an entity for itself, aimed at a moment's subject — defined
  by `bullet.NewShots(w).Define(name, Body{Size, Speed, Range, Gravity, Lands})` as an `Ammo`,
  flown by the plugin every step past the world's step cap and swept by collision, in an arc when
  thrown, landing where collision found a contact, at their range, on the ground or at an edge:
  a `Landing` for the rules (Struck with Other, Wall with Cell, Grounded, Left), the shot gone
  unless it `Lands`; a landed shot a `Resting` every step until a `Burst`, a `Blast` for every
  entity within its radius. A weapon is the game's: rules and effects over them.

**Effects**
- Several effects cast in one pass on an entity under none yet all land: the `Active` on its way
  to the entity carries every one of them, where only the last cast did, and a tag family they
  attach to it carries every one's tags, where only the last attached did.
- A rule's `ForOther` on a moment of one entity naming a Subject — a bullet `Landing`'s entity
  struck — acts on that one, as on the others a pair's moment met.

**Following and driving are the players'**
- A camera knows what it is fastened to: `camera.Fastening{Entity, How}` on every
  `camera.Fastenable` camera, `How` one of `Loose`, `Centred`, `Behind`, `Inside` (`Outside` all
  but the last), `camera.HowOf`. `camera.Mode`, `ModeOf` and `Rider` are gone;
  `control.Binding.In` takes hows (`In(camera.Inside)` for `In(camera.FirstPerson)`,
  `In(camera.Outside)` for `In(camera.Free)`).
- `players.Follow{}` (C, in a game with a selection) fastens the player's camera `Centred` over
  the one unit it has selected, or lets go; told by an entity as it is made
  (`Told(players.Give{To}, players.Follow{})`) its owner's camera follows it from the start. The
  players keep a `Centred` camera on its entity; `Pan` lets go, `Zoom` keeps. `selection.Follow`,
  `FollowSystem` and the tag `Followed` are gone; `selection.Plugin.Chosen` is the one selected
  unit of a player, a `players.Chooser`.
- `topography.Ride{Camera}` (V) takes a fastened camera closer round its entity: behind it,
  inside it (first person) where the game reaches the perspective, over it again; the camera
  system keeps `Behind` and `Inside` by the camera's fastening. `topography.Follow`, `Drive`,
  `LookOut` and `Plugin.WithSelection` are gone; the topography never knows the selection.
- `players.Drive{Ahead, Turn, Way, Sprint}` is the player's hand, summed a tick into
  `players.Hand` (`Plugin.Hand`, `OwnHand`); navigation, given the players
  (`navigation.Plugin.WithPlayers`), steers each unit by its owner's hand — the unit the owner's
  camera is fastened to, else the units it has selected — writing `steering.Driven`; the
  topography's camera system writes only the eye's part of it (`Face` while aiming, `Flown`,
  `Climb`). `navigation.Drive` and `DriveBindings` are gone: `players.DriveBindings()` (W, S, A
  and D outside any entity, for a game to bind in place of the camera's keys),
  `players.DriveKeys(up, down, left, right)` (four keys into a `Way`), and the players' defaults
  drive the unit the camera is fastened to (W, S, A and D riding inside it, the arrows behind it).
  A unit driven carries the marker `navigation.Driving`; a tick without a hand brakes it and, once
  it stands or has an order, its `Driven` is taken off, so it steps aside again. A contact only
  sensed — a shot — bumps no unit under orders.
- `split-screen-demo` has no systems of its own: each block is told `Give`, `Allow{Selected}` and
  `Follow` as it is made, driven by its player's `DriveKeys`.

**Spawn in the running game**
- The command `world.Spawn{Entry}` adds an entity of a kind to the running world, as `Seed` does
  before the game; `world.Plugin.Spawn` gives it as the game's own. The world's spawn system
  carries it out at the next step of the simulation, after the plans, refusing with a log line an
  unknown kind, a wrong row, a full world, a size out of bounds or a box past an open edge. After
  a load the room left under `Config.Entities.MaxCount` is counted from what was loaded.

**Swept entities**
- `collision.Sweep` marks an entity that moves itself further in a step than the world's cap: the
  space holds the stretch of its step for the tick, every pair and every solid box on the path is
  refined to the segment, and the nearest contact alone stays. `Contact.Along` says where along the
  step it lies, `Contact.Sensed` that the contact was only detected. A swept entity is a sensor
  whatever its `Physics`; two swept pass through each other; `Sweep.Ignore` (with `Ignoring`) is
  the one entity it passes through, its shooter; a wrapping world refuses it. Saves carrying a
  `Collider` from before do not load: `Contact` grew.

**Collision with heights**
- In a world with heights a pair of colliders meets only where the heights the two span overlap
  (`collision.Band`, `BandOf`, `Everywhere`), and the ground stops an entity only in a solid cell
  whose own band — from below up to its kind's `Height` over its level — meets the entity's:
  `collision.Field.Solid` takes the entity's band beside its layers. Whatever says no height (no
  `Z`, a `Height` of 0, a solid kind without `Height`) spans every height and meets all; a flat
  world asks none of it. Two short units on stepped or sloped ground, their bands apart, now pass
  each other.

**Rules hooked by the Stage; roles**
- `game.Initializer.Hook(rules...)` hooks each rule, and every rule of a role, on the plugin in use
  that hosts its moment, after the plugins are used and before `Init` returns; a rule none hosts is
  an error wrapping `plugin.ErrUnhosted`, named by its `String`. A plugin's own `Hook` still works
  until the Stage's `ecs.Setup`.
- `rule.Role(name)` is a behaviour entities play: `Obeys(rules...)` narrows each rule to its
  players on top of its own filter. A kind
  plays roles through `rule.Plays(roles...)`, a cell through `cell.Entry.Roles`; `m.Playing(role,
  step)` asks it of an entity or of a place `Around` turned to. 64 roles a program, saved by name.
- `cell.Now.Stood` (`Trodden`) says a unit stands on the cell.
- `climate.Weathering` rules are hosted by a `plugin.StepRules`, like the clock's: run once a
  step, a filtered or narrowed one is refused with `plugin.ErrUnhosted`.
- The roster's `kind.Role` is `kind.Template`, and `Roster.Cell` is what every cell a board makes
  carries.

**A box pushed out through an open edge leaves**
- Collision writes the box of whoever it pushed out through an open edge to its `Pos`, as
  aabbworld v1.10.0's `collide.Engine.Left` now tells it (a `collide.Leaver`, id and box). Before,
  the push stayed in the index: the box kept its place over what had pushed it, was marked
  `Outside` and unmarked every step, and was never despawned. Needs aabbworld v1.10.0.

**No Go code in a rule; drawing in `render`**
- `rule.Each`, `rule.Every` and `rule.Pair` are gone: a rule is `rule.On` alone, its Go no more
  than the conditions of `If`. What a plugin did through them is its own work in its own pass.
- The ground's pace: the board's pass over its units writes each one's `steering.Pace{Share}`
  (1/cost of the cell under it, the slope the way it goes) and the world's velocity pass
  multiplies the speed by it — from the step after the unit stood there, a step late.
  `TerrainSpeed` is gone.
- Navigation answers bumps by reading the `collision.Collider` contacts of its units under orders
  in its own pass; it hooks nothing on collision.
- `chooks.CountContacts` and `LogContacts`, `vhooks.LogSightings` and `bhooks.LogFalls` are the
  plugins' options: `collision.Plugin.WithStats(&stats)` (`collision.ContactStats`, was
  `hooks.ContactStats`) and `WithLog`, `vision.Plugin.WithLog`, `board.Plugin.WithLog`.
  `plugins/board/hooks` is gone.
- The weather on the board is the atmosphere's own system (`weathering.Weathering.System`), not
  a rule of the clock; `Weathering.Rule` is gone.
- Steering by command: `steering.Away{From}`, `Toward{To}` (aimed at the moment's subject) and
  `Turn{Angle}`, which an entity gives itself and the world carries out through its `Helm`, one a
  step. `vision.Sighting.Subject` is the nearest one seen; `Sighting.Helm` is gone. An aimed
  command now fails while the rule's moment names nobody.
- `vhooks.Flee(tags, fleeing)` is two rules — away from the nearest Threat, else from the nearest
  one closing — run `During` an effect the game puts on the world; `NewFlee`, `Flee.Rule` and
  `SetEnabled` are gone. It flees the nearest, not a weighted sum of everyone in view.
  `vhooks.Chase(tags)` heads at the nearest Prey; looking round when none is in view is
  `vhooks.Search(tags, vhooks.Looked(w, d))`, once every `d` of game time (it was wall time).
- `world.Dispel{Effect}` takes an effect off the world, beside `world.Apply`.
- `climate.Weathering` is about the world's own entity (`World`, `Who`), so its rules may cast a
  state of the game.
- Drawing rules are `render`'s: `render.Appearance` (`world.Appearance` is an alias),
  `render.Over`, `As`, `With`, `Show`, run every frame by `render.Rules`, given with
  `world.Plugin.Draw` and `vision.Plugin.Draw`. `world.Facing` picks the sprite from the way an
  entity goes. `world.Drawing`, `vision.Viewing`, `vision.ShowViewOf` and `plugins/world/hooks`
  are gone; `vision.Plugin.Draw(render.Show(selected.In))` draws the selected views alone
  (`tag.Tag.In` is the condition of carrying a tag). `chooks.HitOverlay` is a `render.Rule` for
  `world.Plugin.Draw`.

**Rules in one place: package rule**
- `plugin.Rule`, `Tick`, `TickSource`, `Marks`, `MaxFamilies`, `ErrUnhosted` and `ErrHostBuilt`
  are `rule`'s; `rule.Rule` is a sealed interface, made by `rule.On` (a value of any other kind no
  longer compiles as a rule). The hosts (`EachHost`, `PairHost`, `ListHost`, `Own`) are `rule`'s
  too and `plugin/host` is gone; `plugin` keeps the plugin contract alone. In `rule`, `On` and
  its filters are in `rule.go`.
- Plans are package `rule/plan`: `plan.New(name, body)` (was `rule.Plan`), `plan.Actor`,
  `plan.Command`, `plan.Mind`, the asks (`Asked`, `Replied`, `Chain`, `Answer`), `plan.NewPlans`
  (was `rule.New`). The engine running the steps of both is `rule/internal/engine`; `rule.Step`
  and the plan's types are its aliases. Saves name the mind and the asks after the engine now
  (`engine.Mind`), so a save from before does not load.
- `plugins/board/internal/rule` is `plugins/board/internal/moments`, out of the way of `rule`.

**gram's core out of the world; the world tidied**
- `entity` (with `entity/kind`, `kind/comp`, `entity/tag`), `clock` and `rule` (with
  `rule/effect`) are at the module's top beside `plugin`, `control`, `render` and `camera`:
  `github.com/kjkrol/gram/rule`, not `…/plugins/world/rule`. Every plugin and game used them and
  none imports a plugin; `plugin` itself imported `plugins/world/entity/tag`. The world still
  makes and runs their systems and registers their components; `steering` and `view` stay its own.
- Removed, as no game used them: `world.Plugin.Attach`, `Detach` and `Declare` (components come
  and go through effects and the plugins' facts), `Bodies`, `NewBodies` and `Kinds.Reserve`,
  `comp.Template.WithEffect`, `Clock.Now`.
- The world's drawing rules (`Overlay`, `As`, `With`, `Facing`) are `render`'s now (`render.Over`,
  `As`, `With`, `Show`; `world.Facing`) — see above; `examples/look/appearance` shows them
  changing what is drawn while the Appearance stays as it is.
- No longer public: `VelocitySystem` and `MoveSystem`, the `Renderer` type (`Plugin.Renderer()` stays),
  `Plugin.NewView` and `DropView` (`ViewFor` stays), `view.View.Refresh`, `clock.DefaultTempos`,
  `Clock.Paused`, `Tempo`, `SetPaused`, `SetTempo` and `Written` (the clock's commands set it),
  `steering.Steepest`, `effect.MarkerPrefix`.
- Navigation's and collision's tests that composed a step of their own out of the world's move
  system run on the world plugin, as a game does.
- The world's register of kinds and tags is `plugins/world/internal/kinds`, its flat look
  `plugins/world/internal/look`; `world.Kinds` hands a game its part. `plugin/behavior.go` is
  `plugin/rule.go`.

**The last of the behaviours gone: rules alone**
- `world.Behavior` is gone: the world's `Hook` took a bare goke system and ran it before
  movement, beside the rules and plans; no game used it. `Hook` takes rules alone now, and the
  plans run first in each step.
- `collision.Struck` comes only to an entity that struck something; `Struck.Hit` is gone — a rule
  of it is `m.Apply(hit)`, with no `If`. `rule.EachHost.RunWhere` runs a host's rules over the
  entities of a chunk a moment is about.
- Files and tests named after behaviours carry the rules' names: `collision/meeting.go` and
  `struck.go`, `vision/sighting.go`, `plugin/host/rules.go`.

**The collision tidied: the answer's arithmetic in `internal`, the engine's insides hidden**
- The arithmetic of the answer to a contact — the impulse two sides trade, the footing that keeps
  a side off ground that does not take it — is `collision/internal/response`; the system hands it
  its sides. `Field` and `FieldBox` stay in `collision`.
- Removed, as no game used them: `ShapeTest`, `Contactee`, `BoxesTouch` and
  `Plugin.WithShapeTest` — overlapping boxes touch, the box is the shape; `hooks.ContactStats.Reset`.
  `Physics.Weight`, `Bounce` and `Immovable` are the plugin's own now.
- No longer public, used by tests alone: `collision.New` (the engine without a world plugin),
  `CollisionSystem`, `NewCollisionSystem`. The collision demo's save-and-load test, which tested
  the engine's index after a load, is collision's own now; the hooks' tests run the plugin in a
  world, as a game does, on helpers the collision's tests share (`collision/internal/collisiontest`).

**The navigation tidied: what stands alone in `internal`**
- navigation keeps the game's types (`MoveOrder`, `Path`, `Leg`, `Goal`, `Round`, `Patrol`, the
  facts, `Touch`, `Entered`, the commands, `Spacing`, `RouteStyle`) and the systems working on
  them; the route finder is `internal/pathfind` and the routes' lines on the GPU, with their
  shader, `internal/routes`. Their tests went with them.
- No longer public, used by no game: `PathRenderer`, `NewPathRenderer` (WithRenderer builds it),
  `RouteTier`, `EnteredName`.

**The topography in parts: the game's entries at the root, the vocabulary in two packages, the rest in `internal`**
- A plugin's code lies in three layers (CLAUDE.md, "A plugin's packages"): the plugin's package
  holds what a game constructs and drives it with — the constructor, `Config`, options, the
  commands it handles; public subpackages hold the vocabulary a game and other plugins share and
  the contracts plugins read; `internal` holds the machinery.
- `topography` holds the cameras' commands — `View`, `LookFrom`, `LookAt`, `Look`, `LookOut`,
  `Turn`, `Tilt`, `Follow`, `Drive`, `TurnStep`, `TiltStep` (were `cameras.…`) — and the shaping —
  `Raise`, `Lower`, `Level`, `Shaping` (were `relief.…`), carried out by a system of its own.
  `Plugin.Relief()` is a `topography.Relief`: `At`, `Step`, `Altitude`, `SetHeights` (was
  `*relief.Relief`; `GroundAt` is `At`).
- `topography/relief` keeps `Climbing`, `DefaultClimbing` and `MeanOfCells`; `topography/painter`
  keeps `Style`. `cameras`, `billboards`, `hexes`, `terrain`, `water` and the rest of `relief` and
  `painter` are in `plugins/topography/internal`; the cameras read their commands through
  `icameras.Orders`, which the plugin implements over its queues.
- Tests sit in the package whose code they test; what they share is `internal/topotest`.
- Saves made before this do not load: the heights' component (`relief.Heights`) moved.

**The board in parts: an API in packages, its machinery in `internal`**
- `plugins/board` keeps the `Plugin`, the `Board` (the terrain read and written: `Kind`, `Bare`,
  `Set`, `SetAll`, `Way`, `SetWay`, `Crossing`, `SetCrossing`, `Along`, the versions, `Touch`),
  the `Layout`, the `Map` contract and `NewUnits`; the rest is in packages of their own:
  - `plugins/board/cell`, what is said of one cell: `cell.ID` (was `board.CellID`), `cell.Kind`
    (`CellKind`), `cell.Kinds` (`CellKindDict`; `board.Plugin.CellKinds()` in place of
    `CellKindDict()`), `cell.Name`/`Named`, `cell.Domain` with `cell.Land`, `Water`, `Air`, the
    cell entity's `cell.Plot` and `cell.Ground`, `cell.Way`, `Crossing`, `Links`, the game's tags
    of places (`cell.Family`, `cell.Tag`, `cell.Tags`, was `board.Places`), the moment of a cell,
    `cell.Now` (was `board.Cell`), `cell.Terrain` and `cell.TerrainMap` (now with the Layout's
    tags, `TerrainMap.Tags`), the Layout's `cell.Entry` (was `board.CellEntry`) and
    `cell.WayEntry`, `cell.Occupancy` with `SingleOccupancy` and `MultipleOccupancy`;
  - `plugins/board/unit`, an entity on the board: `unit.At`, `unit.Mover`, `unit.DomainAt` and the
    moment `unit.Standing` (were `board.At`, `Mover`, `DomainAt`, `Standing`);
  - `plugins/board/grid`, the topology: `grid.Grid`, `grid.DefaultGrids`, `grid.Link` (were
    `board.Grid`, `DefaultGrids`, `Link`); `board.Toward` is the method `Grid.Toward`;
    `grid.Shape`/`ShapeOf`, square or hex, in place of `board.SquareShape`, and
    `Board.Shape()` of `Board.Square()`;
  - `plugins/board/look`, how a board is drawn: `look.Look`, `Dressing`, `EvenLit`, `Parallel`,
    `ParallelLook`, `Tile`, `FlatLook`, `Nothing`, `RenderState`, `MinGridCell` (were `board.…`),
    `look.Map` (the drawing part of `board.Map`) and `look.Renderer`, `look.NewRenderer(b, atlas,
    m, space)` in place of `board.NewRenderer`; a tile's `Sway` is nothing where nothing stands
    at a height;
  - `plugins/board/ground`: `ground.Heights`, `Cover`, `Readied` (were `board.…`).
- The board's machinery is in `plugins/board/internal`, which nothing outside the board imports:
  the cells' state and entities (`internal/terrain`), the rules of a `unit.Standing` and of a
  `cell.Now` with their systems and the board's own rule slowing units by the ground
  (`internal/rule`), the ground's cover and solid field (`internal/field`), the grids' types
  (`internal/grids`), the simple map's bands (`internal/draw`), and the release of the holds of
  whoever left the world, a system of its own now (`internal/occupancy`).
- The board's ground is a field of its own, inside it: `Board.Walk`, `Ready`, `Solid` and
  `Overhang` are gone; `Plugin.Cover()` is the cover (a `ground.Readied` too), and
  `Plugin.WithCollision` hands collision the solid ground.
- Gone, used by nothing: `Board.SetMany`, `TerrainMap.SetMany`, `Plugin.Top`, `Plugin.Slope`,
  `Grid.Contains`, `board.CellAABB` (the tests have their own), `board.HexCapStrips`,
  `Plugin.DefaultAtlas` (`WithRenderer(nil)` still draws from it). `board.Center(pos)` is
  `pos.Center()`, a method of `world.Position`. `NewBoard(grid)` takes the grid alone.
- Saves made before this do not load: the components' type names changed.

**Entities behave in one vocabulary: rules, plans, effects, commands, facts**
- `plugins/world/rule` is how entities behave (`doc/rule.md`). Two constructors, each taking a
  function that writes the steps for a builder (Go 1.27's methods with type parameters). A
  **rule**, `rule.On(name, filter, func(m *rule.Moment[P]) rule.Step)` — a `plugin.Rule` — is what
  is done at a moment a plugin catches in its own pass — a `unit.Standing` or `Cell`, a
  `vision.Sighting`, a `collision.Meeting` or `Struck`, a `world.Moving`, `Leaving` or `Drawing`, a
  `climate.Weathering`, a `clock.Moment`, a `navigation.Touch` — hooked on that plugin; the filter
  says whom it fires for: `rule.All`, `rule.Self(tag)`, `rule.Between(a, b)` (pairs, for a moment
  that is `rule.Met`), `rule.Having[T]()` (a component). A Moment's steps are instant (`OneOf`,
  `Steps`, `If` on the moment, `Not`, `Apply`, `Keep`, `Dispel`, `Chance`, `Unless`, `Under`,
  `Order`, `ForOther`, `Here`, `Around`); a lasting one is refused as the rule is made. There is
  no step running Go code: `Call` and `CallOn` are gone. A **plan**, `rule.Plan(name, func(a *rule.Actor) rule.Step)`, is the component a kind
  gives its entities: what they do over time, run by the world in every step of its simulation and
  saved with the game; an Actor's steps are the same and `When[F]`/`On[F]` (branches on a fact),
  `If`, `Until`, `Wait`, `Timeout`, `Cooldown`, `Idle`, and conversation between entities — `Ask`,
  `Agree`, `Refuse`, `Relay`, the facts `Asked` and `Replied`, a `Chain` of at most `MaxChain`,
  asks dropped after `AskLife`. One type, `rule.Step`, for both; a function writing part of either
  takes the Moment or the Actor. **Effects** are cast by steps: `Apply`, `Keep` (held while its
  branch runs, or while a rule keeps firing it), `Dispel`, `Unless`, `Under` — an effect's
  presence is a rule's memory. A rule keeping an effect someone dispelled has it back the step
  after; a plan's `Keep` gives way. `Chance(p, step)` draws from the world's seed
  (`world.Config.Seed`), the game time and the entity, keeping nothing; every `plugin.Tick` comes
  from the world (`world.Plugin.Tick`, a `plugin.TickSource`) with `Time` and `Seed`. A **command** is what an entity gives itself, `Order(cmd)`, the same command a
  player gives, fire and forget — in a plan a `Command` whose `.Until[F](…)` or `.Stay()` gives it
  once in a reactive branch: the world carries it (`control.Carrier`,
  `world.Plugin.Carry`/`Carrier`; the engine carries every `plugin.CommandHandler` a stage uses,
  `plugin.Tick.Commands` hands it to rules), the handler told who gave it
  (`control.Issued.Entity`, `ByEntity`; `CommandQueue.PutFrom`, `Queue.AddFrom`); an `Aimed`
  command is told the `Subject` of the fact it stands under or of the rule's moment. A **fact** is
  what a plugin tells an entity with a `Mind`, and how a plan learns what came of its commands
  (`Until[F]()`, a fact come afresh, or `Until(pred)`). `Mind{Plan, Running, Slot, Since}` holds
  at most `MaxSteps` (128) steps.
- **Markers**: states an entity switches on and off are bits of a plugin's family `States`
  carried for good (`comp.Marks[F]()`, the roster's defaults), never components put on and taken
  off — which moved the entity in memory, some 200 ns a time (`Benchmark_Marker_*`).
  Every effect has its own marker of `effect.States`, "effect.<name>", on while it runs
  (`Effect.Mark()`): what rules of any plugin filter by, `rule.Self(frozen.Mark())`; at most 63
  effects. `effect.Changed` is on for the step after an `Alter` rewrote an entity's components —
  the board learns of a cell's ground so — in place of `Active.Altered`, `effect.Idle`, the
  `effect.Idling` moment and `host.EachHost.RunRows`. `Active` stays on an entity, empty when no
  effect runs; navigation's `CellEntered` is the marker `navigation.Entered` (the cell is the
  unit's `unit.At`); collision's hit is an effect whose marker `HitOverlay` reads:
  `hooks.Hit(w, d)` hands back the `effect.Effect`, `ShowHits(hit)` and `HitOverlay(hit, with)`
  take it. Saves made before this do not load (`Active`'s slots changed).
- `effect.Then(next)` casts the next effect when one's time is up, not when it is dispelled. The
  world fires the clock's moments, in a system of its own just before the effects' pass; the
  `effect` package no longer knows the clock, and `effect.New` takes the namer of the markers.
- **Knobs apart from state**: a plugin gives knobs, components it only reads and an effect's
  `Alter` turns; what its system writes lives beside them, given where missing — an `Alter`
  ending puts back the original, which put back stale speed and heading before. `steering.Steering`
  is the knobs (`TurnRate`, `Reflex`, `MaxSpeed`, `Sprint`, `Accel`, `Brake`, `V0`, and `Halted`,
  standing whatever asked); `steering.Course` the state (`Want`, `Pending`, `Delay`, `Speed`,
  `WantSpeed`), a unit's default; `steering.Helm{*Steering, *Course}` asks (`Request`,
  `RequestSpeed`, `RequestSprint`, `RequestBack`, `Steerable`); `vision.Sighting.Helm` in place of
  `Steering`. `vision.Sight` loses `Seen` to the component `vision.Sighted` and gains `Ahead`, the
  sight looking the way the entity moves (`Sight.Looking`). Saves made before this do not load.
- `rule.Placed` moments stand on places of their own: `m.Here(step)` acts on the cells under the
  entity, `m.Around(rings, step)` on the rings of neighbours too, each cell once — `unit.Standing`
  is one, and `board.Cell`, every cell at every step while a rule of it is hooked, the board's new
  moment. Moments are data alone: their host tells the places round in `plugin.Tick.Around`. The
  unit's cell component is `unit.At{Cell}` (was `board.Cell{ID}`). A `clock.Moment` is the clock's own entity's (`Moment.Clock`): an effect a clock rule
  applies lands there, a phase.
- **A player acts by effects**: `selection.Apply{Effect}` puts an effect on the player's own
  selected units (an ability), `world.Apply{Effect}` on the world itself — its own entity, the
  clock's — a state of the whole game, which rules and plans read with the new step
  `During(e, step)` (`plugin.Tick.World`; `rule.New` takes the world's entity). The new
  `trapdoor-demo` (`make demo-trapdoor`): 1 and 2 pull two levers, each opening its own strip of
  trapdoors under whoever stands on it; J hastens the selected scouts. Cells carry the game's tags
  of places for good, `cell.Tags`, given in the `Layout` (`cell.Entry.Tags`; an entry without a
  `Kind` keeps the default), which rules of a `cell.Now` filter by (`rule.Self`), and
  `Standing.Places` are those of the cell under a unit; the `pressure-plate-demo` (`make
  demo-pressure-plate`) opens each strip while a scout stands on its plate, whoever stands there
  ordering `world.Apply`. The navigation demo's H, opening holes under the units, is gone.
- navigation: a patrol, `navigation.Patrol(pause, cells...)` — a `MoveOrder` with a `Round` walks
  to its goals in turn and round again, for ever, standing the pause on each; reached or given
  up, it goes on. The demos' wanderers are one kind, each with its own round, in place of a kind
  and a plan a row.
- `cell.Occupancy` lets go of whoever left the world: `Release(gone)`, called by the board every
  step. A despawned unit — fallen in, say — kept its holds before, its cell and the one it was
  stepping into, blocking them for good under `SingleOccupancy`.
- A plugin's own hooks and the ready-made ones are written on `plugin/host` (`host.Each`,
  `host.Every`, `host.Pair`): the board's terrain speed, navigation's bumps, vision's
  `ShowViewOf`, `world.Draw.*`, collision's `CountContacts`, `LogContacts`, `HitOverlay`, vision's
  `Flee` and `Chase`, the weathering's clock. New: `plugins/board/hooks.LogFalls()` and
  `vision/hooks.LogSightings()`. The demos: sight follows travel (`Ahead`), a caught prey gives
  itself a `Despawn` (`ForOther`), the witch freezes `Around(1, …)`, a frozen boat is `Halted`.
- Needs goke 3.2.4: 3.2.3 wrote an archetype's values in one order and read them in another when a
  component type was registered before the entities' others but put on them after — values landed
  in other components' columns (a board's cells with markers, from the first save).
- One carrier of commands a stage: the players give theirs to the world's (`Carrier.Put`,
  `Takes`, `Empty`); `players.Plugin` keeps no queues of its own, and its `RunPlan` no longer
  empties every queue at the end of a frame — which dropped an entity's `Despawn` given after the
  world's pass, so a unit that fell in never went. A command waits for its handler's pass;
  `CommandQueue` has `Empty` in place of `Clear`.
- Collision's hit is an effect: `hooks.Hit(fx, d)` defines it, `ShowHits(hit)` casts it on a
  `Struck` that `Hit()`s, `HitOverlay(hit, with)` draws it — in game time; `HitMark` is gone.
- Gone: `plugin.Behavior` (now `plugin.Rule`), `RegisterBehavior` (now `Hook` on every
  plugin), `vision.Between`, `collision.Each`/`Between`/`Every`, `board.Each`/`Every`,
  `world.Each`/`Every`, `effects.Each`/`Every`, `climate.Every`, `effects.Schedule` (a rule of
  the clock's `clock.Moment{Last, Now}` with `clock.At(t)` or `clock.Every(period, offset)`;
  `calendar.Daily`/`Yearly`/`Seasonal` give the period and offset; `weathering.Weathering.Rule`
  in place of `Schedule`). `plugin/host` gains `PairOf`, `EachWith`, `StateOf`, `SideOf`,
  `AnySide`, `ListHost` and `Own`; rules over one component share its column.
- Packages: `plugins/world/kind` is `plugins/world/entity/kind` (and `kind/comp`); the tags
  (`plugin.Tags`, `Tag`, `Any`, `Anything`, `MaxTagsPerFamily`) are the leaf
  `plugins/world/entity/tag`; `plugins/world/effects` is `plugins/world/rule/effect` (package
  `effect`), whose `Define` hands back an `effect.Effect` that casts on its own effects
  (`Cast`, `CastFor`, `Dispel`, `On`); `plugins/collision/behavior` and `plugins/vision/behavior`
  are the packages `hooks`: ready-made rules, whole, for a plugin's Hook —
  `hooks.CountContacts(&stats)`, `ShowHits(hit)`, `LogContacts()`, `HitOverlay(hit, with)` (for the
  world), `Chase(tags, every)`, `NewFlee(tags).Rule()`.
- `world.Despawn`, a command an entity gives itself to leave the world; `unit.Standing` carries
  the entity's `Domain`, `Standing.Fallen()` in place of `Fell(domain)`. The demos' falls and
  drownings are rules ordering `Despawn`; the effect demo's ice is rules keeping `frozen` and
  `slip` with `Keep`.
- collision: a push apart never puts a unit further over ground that does not take it — water to
  a walker, a hole: the side it would put there holds as at a wall and the other goes the whole
  way. `collision.Field` has `Overhang(layers, box)`, the board's field's; ground turning
  to water under a unit still lets it fall in.
- navigation's crowd is rules, as in StarCraft II, its own and unexported: hooked by the plugin
  unless a game gives its own with `WithCrowd`, and a game adds rules beside them with `Hook`. An
  ally standing makes way and stays aside while the one on the move goes on past
  it; one on the move stops on touching one of its order that has arrived (`MoveOrder.Group`, a
  fresh one each `MoveTo`; `LastOrder`, which every unit carries, the group it last came to the end
  of), so a group gathers round its point; one on the goal who does not make way — a stranger, an
  ally with no room — has it stand beside; anyone else
  is gone round, and with no way round it steps aside a while; of two head on the lower id waits.
  The rules are of the moment `navigation.Touch` — two units touching, their boxes or a
  refused step — which navigation hosts (`Plugin.Hook`, tag filters too). Commands `StepAside`,
  `Detour`, `Pass`, `Hold`, `Settle`, `Stop`, carried out for the unit that gives them, the
  engine's rules kept inside: a unit steps aside only where the ground takes it, no steeper than a
  cliff, nobody standing — never into water, a hole or a wall. Facts `Blocked` (with `WaitedOut`
  and `Cornered`) and `Arrived` for units with a plan; a `MoveTo` or `LookAt` a unit gives itself
  orders it alone. Navigation's own
  decisions about others are gone — its last word is the stall: a unit making no headway plans
  afresh, then stands. A step aside draws no goal outline.
- `players/owner.Allies`. `MoveOrder.HitUnit`: a unit struck, told from the ground — entity 0 was
  taken for the ground before.
- The topography demo has 20 of the player's units standing on the plateau, to watch the crowd.
- Saves made before this do not load: `MoveOrder` and `rule.Mind` changed shape, and the
  effects' components are of the package `rule/effect` now.

**Units belong to players**
- `plugins/players/owner`: whose a unit is — `owner.Family`, a tag a player (`owner.Of(id)`, saved
  by `owner.Name`), `owner.Obeys(owners, by)`; a leaf importing only the tags and `control`.
  `players.NewPlugin` registers the family with the world's kinds; `Player.Owner()` is the
  player's tag, given to a kind with `comp.Tagged`.
- A player selects, orders and rides its own units alone: `selection.Select` and `Follow`,
  navigation's `MoveTo` and `LookAt`, the topography's `cameras.Follow` and `LookOut` act only on
  what the issuing player owns; another player's selection stays. A unit nobody owns belongs to
  the virtual player `control.Nobody` (the game's code, a script, an AI run as nobody) and takes
  commands from it alone — a game whose players select units gives them their units.
- The demos give their units to the player at the keyboard; the island has a rival without a
  keyboard (`players.Add`) whose blue walkers start at every other stop; the split-screen demo's
  blocks are their players' by the owner tag (its `Driver` component is gone).

**Packages: the celestial sphere apart, the topography in parts**
- `plugins/atmosphere/celestial` is the celestial sphere, out of `sky` and the atmosphere's root:
  `Place{Latitude, NoonWay}` (`SunPath`, `Sphere`, `MoonAt`, `HeavensAt`), `SiderealTime`,
  `Phase`, `Way` and its values, `Heavens`, the stars (`Star`, `Stars`, `StarSky`,
  `RealStars`, `ScatteredStars`) and their catalogue, the moon's face (`MoonFace`) and the stars
  drawn on the GPU (`StarField`). `sky` imports it: `sky.Config.NoonWay` is a `celestial.Way`,
  `Sky.Heavens` a `celestial.Heavens`; `sky.SunColorAt` is the sun's colour at a height.
- The atmosphere's two renderers left its root, each with its shader: `atmosphere.Backdrop` is
  `backdrop.Renderer` (`backdrop.New`; `WithRunning` is `WithShown`, the stars and the moon
  alone), the flat world's clouds' shadows are `overcast.Renderer` (`overcast.New`).
  `Plugin.Renderer` and `Plugin.Clouds` hand them out as before.
- `plugins/topography` is the plugin over its parts, none of which imports it: `relief` (`Relief`,
  `New`, `Corners`, `Heights`, `HeightsRun`, `MeanOfCells`, `Climbing`, `DefaultClimbing`,
  `Shaping`, `Raise`, `Lower`, `Level`, the `Shaper`, the heights' and the altitudes' systems),
  `painter` (the board's Dressing in relief, the sheets, `Style`), `water` (the materials, `Shore`,
  `Shores`, `Flow`, the water layers, one `Shore` where there were two), `terrain`, `hexes` (the
  prisms), `billboards` (the world's Look in relief, the entities' shadows) and `cameras` (the
  views, the camera commands `View`, `Turn`, `Tilt`, `LookOut`, `Look`, `Follow`, `Drive`,
  `LookFrom`, `LookAt`, `TurnStep`, `TiltStep`, `LookStep`, `Control`, `Maker`, `Switch`). No
  aliases: a game says `relief.MeanOfCells`, `painter.Style`, `cameras.View`.
  `topography.NewPlugin`, `Config` and the Plugin's methods stay; `Config.Shaping` and
  `Config.Climbing` take the relief's types.
- The billboards ask the cameras through contracts (`camera.Rider.FirstPerson`,
  `camera.Eyed.Eye`, `camera.Projection.Sorts`), not their insides.
- `render/gpu` zeroes a texture as it is made (gogpu did not: a partial first draw showed another
  texture's old data) and frees a released resource only after the next submission the GPU has
  finished, not while a submission may still read it.

**The real sky: the stars, the moon, the sun setting**
- The night shows the real stars — the Yale Bright Star Catalogue to magnitude 6.0, 5080 of them
  (`celestial.Stars`), in their colours, twinkling low — turning round the pole over the climate zone's
  latitude with the sidereal time, drawn under the sky as GPU instances (~0.2 ms a frame);
  `sky.Config.Stars` picks `RealStars` (the default) or `ScatteredStars`, the made-up ones.
- The moon goes its own way along the ecliptic, 5° off it, rising and setting with the stars and
  drifting 13° a day eastwards among them; its face is NASA's CGI Moon Kit's, lit by its phase,
  its north towards the pole; it lights the night brighter (`moonStrength` 0.5).
- `celestial.Heavens` has `Sphere` (the celestial sphere as it stands), `Pole`, `Stars`, `OnSky`;
  `Turn` is gone.
- The sun sets behind the horizon as it goes down rather than vanishing; the sun's and the moon's
  discs keep their width on the sky, so they grow as the camera zooms in.
- The clouds hide the stars, the moon and the sun behind them, and take their light from the sky
  and the sun or the moon: dark at night, silver under the moon.

**Clouds: torn shreds and big heaps, looked up in a tile**
- A weather's cloud cover and how heaped its clouds are (`Billow`) are ranges thrown as it comes,
  like its wind (`weather.State.Clouds` is `[2]float32` now; `climate.Weather` has `CloudsTo`,
  `Billow`, `BillowTo`; `air.Weather.Billow`). The default weathers are clear (no clouds), fair
  (a few big heaps far apart), cloudy, rain and storm.
- The heaps are some 3000 world units apart, form whole as the cover reaches their own and grow as
  it grows on; thick ones darker underneath, their edges shining towards the light.
- The clouds' noise is periodic and baked once into a tile with levels (`air.BakeTile`,
  `air.CloudTile`, `air.HeapTile`); the sky and the terrain look it up per pixel — fixed to the
  world as the eye turns, evened out towards the horizon — and the terrain no longer bakes the
  cover every frame. Clouds on the sky cost 3.5–4 ms at 2560x1440 where they took 7.
  `air.CloudsOverhead` is gone: the sky's clouds are its own shader's.

**Switches and the hour**
- `atmosphere.Running` (`Config.Running`, `Plugin.SetRunning`, `Plugin.Running`) switches the day,
  the weather's changes, the wind, the clouds, what falls, the weathering, the stars and the moon;
  all on by default, none saved. Underneath: `climate.Running` (`Climate.SetRunning`),
  `Weathering.SetRunning`, `Sky.SetMoon`.
- `calendar.Config.Start` and `sky.Config.Hour` are hours on the clock, `time.Duration`s
  (`23*time.Hour + 30*time.Minute`), no longer parts of the day.
- The board-topography demo writes out its weathers and every switch to play with.

**The sun goes on smoothly**
- The sky's light goes on tick by tick (`sky.Config.Steps` zero, the default now), or in the
  steps a game asks for; it no longer jumps 3.75° every 96th of a day.
- The terrain bakes its shadows anew a strip a frame as the sun goes on — 0.4 to 0.85 ms of the
  GPU a frame at 2560x1440 where a bake of the whole took 5 to 10 at once — and all of it at once
  when the sun leaps a degree (a frozen light moved) or the ground changes.
- H (`topography.CoarseShadows`, `Plugin.WithCoarseShadows`) bakes the shadows half as fine a
  side: softer edges, about half the work.

**gram on WebGPU: Ebitengine gone, the whole picture on the GPU**
- The engine runs in a gogpu window (pure Go WebGPU over Vulkan) and draws through `render/gpu`,
  WGSL throughout; only `render/gpu` and `internal/engine` import gogpu. `GRAM_FPS_LOG`,
  `GRAM_FULLSCREEN`, `GRAM_VSYNC=off` for measuring; per-frame data goes through mapped staging
  buffers, Wayland's frame gating off.
- `render.Direct` sources draw with shaders of their own at their tier into a `render.Target`: the
  screen and the frame's shared, reversed depth buffer (`render.Depth`); `NewMeshShaderWith`,
  `Shader.Instanced`, `DrawMeshOptions` (depth, a prepass, instances, the frame's depth to read);
  `camera.SceneTransform` and `Transform.Unproject` from a camera's `Rays`.
- The relief's ground is a mesh of its lattice on the GPU (topography/terrain): the board painted
  flat, lit per pixel with shadows baked on the GPU, the clouds' cover, water only on wet cells,
  the grid, fog, and a skirt of level ground to the horizon. The heightfield, G and
  `Config.Heightfield` are retired.
- The world's entities are billboards drawn on the GPU against the ground's depth, their shadows
  draped over the terrain (`sky.Sun.ShadowOf`, `sky.Patch`); `Hides` is retired; from above the
  world's flat look draws its sprites as GPU instances (`world.DirectLook`, `render.Sprites`).
- The sky, the sun, the clouds on the sky and what falls are Direct sources; the views of sight
  are baked per observer and laid over the ground read from its depth, or over level ground along
  the camera's lines of sight, past a wrapping world's seam too (`vision.ShowViewOf` shows only
  the selected entity's); the routes are laid over the ground the same way.

**Hex boards in relief as prisms on the GPU**
- Over a hex grid the topography's ground is a prism a cell, a face down to each lower neighbour,
  coloured from the tiles composed once from above and drawn every frame into an image of the
  world; the board's tiles lay nothing, the units stand on it as GPU billboards, their shadows laid
  over whatever ground the frame drew, read from its depth.
  navigation-vision-hex's CPU drawing went from 1.2 to 0.25 ms a frame.

**The sea to the horizon, a brighter sun, steady waves**
- The sea's waves are noise of five sizes, shaded to and from the light, reflecting the sky by the
  way to the eye per pixel in a perspective, calming far off instead of flickering; they run along
  x — the wind's wander no longer swings the sea to and fro — the wind making them rougher.
- The sun is a white disc in a halo and a wider glare of its light, no longer greyed.

**Removed with the CPU composing of the relief**
- The topography's tile blocks, its tiles' light and shadows per corner, its clouds per corner and
  its parallel dressing workers (the dresser now only paints, in white), the CPU billboards,
  `sky.Sun.Shadow` and `sky.ShadowTier`, `air.Weather.Overcast`, `OvercastOn`, `Shadowed`,
  `OvercastQuad` and `CloudQuad`; `examples/webgpu-island` and `make demo-webgpu`.
  `topography.Plugin.WithShadows` now turns the terrain's GPU shadows off.

**Flat boards and their clouds on the GPU**
- A flat board seen from above is composed once and kept on the GPU (`render.Still`), composed
  anew only when a cell changes, its grid drawn by a shader; the board demo's CPU drawing went
  from 3.2 to 0.3 ms a frame at 2560x1440. A `look.Dressing` lighting every tile alike says so
  with `look.EvenLit`; the ways' and rivers' bands are now lit by the sun as the tiles they lie
  on.
- The clouds' shadows over a flat world are one draw on the GPU, their noise worked out every 8
  pixels: 0.6 ms of the GPU at 2560x1440 where they took about 6.5.
- `gpu.Draw` gains `Place` and `Tint`, `gpu.Kept` keeps triangles on the GPU.
- A flat world's views of sight are drawn on the GPU over level ground, past a wrapping world's
  seam too, for observers without a `vision.SightOutline`; one with it keeps its outline, cut by
  the entities as the scan found it.
- The navigation-vision demos draw their ground again: their composers take the topography's
  renderer.

**Animations move every frame**
- What is drawn goes by `clock.Clock.Shown`: game time run on past the last tick by the real time
  the engine holds toward the next (`Clock.Pending`), at the tempo. The sea, the rain and what
  sways no longer stand and jump when the frames outnumber the ticks (100 Hz against 60).

**The heights in runs of a fixed size**
- `topography.Heights` is a run of `HeightsRun` (1024) heights, `First` and `Count` saying which,
  on as many of the topography's entities as the relief takes, in place of one entity holding a
  slice: goke keeps them in its own memory and no longer logs a dereference at start. They are
  written when the ground changes, not every tick. A save's heights have a new shape.

**A try of WebGPU (examples/webgpu-island)**
- The island as a mesh the GPU raises from a texture of its heights, with a depth buffer, the
  sun's light and the shadows it casts through a 2048-pixel shadow map, a rippling sea, and 400
  units as boxes drawn in one instanced call, on gogpu (pure Go WebGPU over Vulkan): about 3 ms
  of the GPU and 0.2 ms of the CPU a frame at 1024x768 on an Intel UHD 620. The WGSL is one file
  per thing (`shaders/`). Behind the `webgpu` build tag; `make demo-webgpu` runs it.

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
  `unit.Mover.Clearance` over the ground, pushed up where the ground rises to it, and stays under
  `unit.Mover.Ceiling` over sea level; its `Lift` follows, so let go it keeps its height over the
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
  clock the frame set. `look.MinGridCell` is exported; `weather.kage` has `cloudShade`.

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
  as it lies — in place of the tiles, which lay nothing then (`look.Nothing`); what stands on
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
  is a `look.Parallel` and its Look a `look.ParallelLook` (`board.Plugin.WithWorkers`: 0 every
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
  (its cover's `Ready`, the `ground.Readied` contract) rather than cell by cell through the ECS as a ray
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
  `cell.SingleOccupancy.Holder` and `MultipleOccupancy.Holder` tell who holds a cell.
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
- `plugins/board` is the ground: `ground.Heights` (a point's height and the sampling step; the
  Map's, `Map.Heights()`, a topography's relief, nil on a flat map; `Plugin.Heights`),
  `ground.Cover` (the Board; `Plugin.Cover`), and `Plugin.WithCollision(c)` hands collision the
  Solid cells as its `collision.Field`. `vision.Plugin.WithBoard(brd)` (or `WithHeights`,
  `WithCover`) has sight follow them; navigation reads the board's heights. A flat board's tiles
  are drawn as they are; `look.NewRenderer(brd, atlas, m, world.SpaceCfg{})` takes no sun.
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
  board's own atlas of them (`DefaultAtlas`). `look.FlatLook()` is the flat look for another
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
  shadows once over the screen. `look.Tile.Light` without a dressing is the sun's light on level
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
  `Shape{Size, Height}` for `NewUnits`, `Units.Define(name, unit.Mover{Domain, Lift}, …)`. The
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
  Flow by the square root of the slope; `look.Tile.Flow` hands the current at each corner to
  `render.Frame.Stream`, whose shader carries ripples and flecks of foam down with it and turns
  it white where it runs fast: rapids and waterfalls.
- Ways: `board.Way{Kind, Width, Links}`, what runs across a cell over its ground — a stream, a
  river, a road — on every cell entity beside `Plot` and `Ground` (saved, and an effect may alter
  it). Its kind decides who may cross the cell and what it costs (`Way.Over`; `Board.Kind` lays it
  over the ground), the ground keeps the rest. `Board.Way`, `SetWay`, `Layout.Ways`
  (`WayEntry`), `TerrainMap.Ways`; `grid.Link` and `Grid.Toward` name a neighbour by the grid's
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
  (`landscape.NewPlugin(board, world)`), set as the board's `look.Dressing`
  (`board.Plugin.SetDressing`; `Tile.Base`, `Tile.Light`, `Tile.FaceLight` ask it, `Tile.Dress`
  lays what lies on a tile): the sun's light on the relief and the terrain's shadows
  (`landscape.Plugin.WithShadows`), grounds blending, coasts, water glinting and running
  (`landscape.Glint`, `Stream`, `Shore`, `Flow`), ways drawn across the cells, the clouds' shadows
  (`landscape.Overcast`) and less far off, with its materials (`water.kage`, `overcast.kage`). How a
  kind looks is its `landscape.Style{Shine, Flow, Spread, Under}`, set by name
  (`landscape.Plugin.Style`); `CellKind.Shine`, `Flow`, `Spread`, `Under` and
  `board.Plugin.WithShadows` are gone. A board alone draws its sprites in even light, and a game
  that never imports the landscape never compiles its shader. `look.NewRenderer`,
  `Board.Changes`, `Board.Shape`.
- Far off, the landscape dresses the tiles from a ground sheet: under 16 pixels a cell on a square
  grid a tile's blends and ways are painted once (`render.Paint`), 16 pixels a cell below a copy of
  the board's atlas, and drawn as one piece of it (`Frame.SpritePart`); a cell is painted anew when
  it or a cell round it changes. `look.Dressing.Sheet` hands the renderer the tiles' sheet. A
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
  `Footprint` for outlines; `Plugin.SetLook`, `Look`) and `look.Look` (`Cell`, handed a
  `look.Tile` with its box, sprite, `Top` and `Beside`; `Plugin.SetLook`, `Look`). The renderers
  keep their data and ask the Look for geometry only. `selection.HighlightStyle.Compose(frame,
  footprint)`; `NewSelectionSystem` and `NewRenderer` take the world's Look. Selection, navigation,
  the board and the world renderers no longer test for an isometric camera.

**Light**
- `world.Sun` (`Dir`, `Strength`, `Ambient`; `Sun.Light` for a surface's normal), `DefaultSun`,
  `world.Plugin.SetSun`/`Sun`. In a world with heights the board lights every tile per corner from
  the slope of the ground there and at its neighbours (`look.Tile.Light`, `Tile.FaceLight`), so
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
