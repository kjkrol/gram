# Refactor notes — decisions taken alone, and questions for review

Written while carrying out [refactor.md](refactor.md) unattended, on 2026-09-28. All five stages
are done and uncommitted: `go vet ./...` and `go test ./...` are clean, every demo runs headless
for ten seconds without a panic (`board`, `board-topography`, `board-atlas`, the navigation and
navigation-vision demos, effect, split-screen, collision, vision, scenes, minimal). Each item
below says what was decided and why, or what needs an answer. Take them out as they are settled.

## Decisions taken alone

### Time (stage 1)

- **`clock.Simulate` is a method on the clock**, which a plugin takes from `world.Plugin.Clock()`
  and keeps: `m.clock.Simulate(ctx, block)`. A package-level `clock.Simulate(c, ctx, step, block)`
  runs the block at once when `c` is nil, so a module built without a world — in a unit test — still
  works.
- **The world's effects are made and installed by the world** (`world.Plugin.Effects()`); the
  `effects` package no longer imports `world`, and `Idling` lost its `Base` pointer (nothing used
  it). Effects' `Each`/`Every` behaviours go through `world.RegisterBehavior`.
- **The clock's State lives on the clock's own entity** (like sky's `Day` did), so it saves; a
  loaded game resumes its time, tempo and pause. The `Phase` tag family sits on the same entity.
- **The schedule's entries are code, not data**: laid at Init, fired by clock time alone (an entry
  in `(last, now]` of a step), so a loaded game does not refire what came before the save. There is
  no saved schedule state. Entries: `At(moment)` and `Every(period, offset)`.
- **"Behaviour on only while a phase holds" is a check, not a wrapper**: a behaviour asks
  `clock.In(phase)` itself. Hosts are generic and closed; wrapping them generically was not worth
  it.
- **The cameras' views refresh in the interface part**, before the simulation, so they lag a tick
  behind the positions. Accepted in the plan; it does not show.
- **Navigation's order changed**: orders are taken before the driving (they used to be taken
  after). An order given this tick is acted on this tick. All navigation tests pass.
- **The players plugin always carries the world's commands** (the clock's): `players.NewPlugin`
  adds the world as a handler itself, so every game with players gets Space, ] and [.
- **Falling behind**: the engine tells the clock when a frame hit its cap of five ticks; after 30
  such frames in a row a tempo above 1 comes down one notch and the report says "held back". It
  never comes down below 1.
- **`Frame.Time` from the clock**: the composer takes the frame's time from the first source that
  is `render.Clocked` — the world's renderer — so animations stand in the pause and hurry with the
  tempo; the composer's own wall clock stays as the fallback for composers without a world.
- **`world.Config` is no longer comparable** (`Clock.Tempos` is a slice); one test compared configs
  by `!=` and now compares fields.
- **`sky`'s default bindings were dropped** at once (they collided with the clock's `]`, `[` and
  `P`) since `atmosphere` replaces the sky in stage 2.

### Atmosphere (stage 2)

- **The calendar keeps no state**: it is the clock at a fixed scale, so a loaded game's clock
  brings its date back and there is nothing to save; `calendar.Config{Day, Start, Year, Season}`
  says where the scale begins. The sky's `Day` entity is gone.
- **The frozen light lives in memory** (`sky.Sky.frozen`, `hour`), not on an entity: it is a look
  at the world, like the camera's turn, and a loaded game's light is the hour's. `Config.Frozen`
  and `Config.Hour` start a game with a frozen light. Only the hour is frozen: the date — the
  season's sun height and the moon — is still the calendar's, so a light frozen for days drifts
  with the season, which did not seem worth freezing the date over.
- **Shift+] / Shift+[ do nothing while the light is not frozen** (the old `]`/`[` changed the
  day's pace then; the pace is the clock's tempo now). Freeze first with P.
- **The sun's latitude is always the climate's zone's**: `sky.New` takes it as an argument and the
  atmosphere hands it `Config.Climate.Zone.Latitude`; there is no sky without a climate in an
  atmosphere. `sky.Latitude` (30°) stays for a sky made alone.
- **The weather's time is the simulation's step**: `Weather.SeenDate/SeenTime` and `passed` are
  gone; every step moves it by the step, the tempo and the pause come from the clock. A behaviour's
  `Tick.Dt` is the step.
- **Weathering is a schedule entry, not a weather behaviour**: `Every(time.Second, 0, …)` on the
  world's schedule, reading `world.Weather()` and `calendar.Now().Season()`. The game makes its
  snowy kinds and its ice itself (their looks are the game's) and names them in
  `weathering.Config`; the weathering only looks them up. Snow's "high ground" is a function
  (`Config.High`), so it does not need the board's heights, which move to topography in stage 3.
  The weathering throws its own dice (`Config.Seed`) rather than `math/rand`, so it is
  deterministic like the weather.
- **The cloud-shadow material moved to `render`** (`render/overcast.kage`, `Frame.Overcast`,
  `OvercastOn`, `OvercastQuad`, `Frame.Material`) because the landscape's water calls its shader
  functions and the flat map needs it without the landscape. The landscape's `Overcast` and
  `OvercastOn` functions are gone.
- **`Sunlit` is a flag set by `SetSun`**: a flat world is lit by the sun once anything set one,
  never before — so a flat game without an atmosphere is drawn exactly as it was. The board's
  `Tile.Light` without a dressing and the world's sprites take the sun's light on level ground
  then; the world's renderer still lays shadows only with a ground (a world with heights).
- **`atmosphere.Plugin.Reporter()` is one reporter** joining the calendar's, the sky's and the
  climate's lines, so a scene adds one; `HUD()` is the calendar's layer. A separate clock HUD
  (`clock.HUD`) shows the game time and the tempo.
- **The two island demos moved to `atmosphere`** for now (their `climate.go` shrank to the snowy
  kinds, the ice and `weathering.Config`); they go away in stage 5.

### Boards and topography (stage 3)

- **The grid stays `board.NewPlugin`'s argument**, not the Map's: the plan listed the grid among
  what a Map gives, but the grid is the board's topology (occupancy, cells, neighbours) and every
  Map draws whatever grid the board has. `board.Map` gives the Look, the Dressing, the tops
  (`Top`), and the costs beyond the kinds' (`Climb`, `Least`, `Slope`). `board.Plugin.WithMap`
  sets one; `SetLook`/`SetDressing` are gone.
- **A kind's look is `CellKind.Color`**, a gob-friendly field, plus `CellKindDict.Draw(name,
  drawer)` for a drawn sprite; `board.Plugin.WithRenderer(nil)` builds the default atlas from
  them (`DefaultAtlas`). A kind of no colour is grey.
- **The simple map's bands are straight**: from the cell's middle to the edge towards each linked
  neighbour, an octagon hub, the kind's Color, faded by `Way.Fade`; crossings the same over them.
  No curves, no water, no light — the topography has those.
- **The heights are a lattice on one entity, not a per-cell component.** `topography.Relief` keeps
  one height per corner of a square grid (per cell on any other), so neighbouring cells share their
  corners by construction and the old "sealing" is gone; `Plot` lost its `Relief`. The values ride
  on the topography's own entity as `topography.Heights` (a `[]float32` with
  `MarshalBinary`/`UnmarshalBinary`, since goke refuses slices otherwise), written every tick like
  the clock's State. An effect can no longer alter one cell's heights (nothing did).
- **`Board.Touch(c)`** is how the topography tells the board a cell's heights changed, so the
  board's `CellVersion`/`Version` keep meaning "anything about the cell changed" and every cache
  (the board renderer's, the dresser's, navigation's) stays right.
- **The board keeps a `Map` reference for the cover's bands**: `Board.Walk` needs a cell's level
  for a veil's band in a Quasi3D world and asks `Map.Top`. The board also keeps `quasi3D`.
- **Navigation takes the slopes from the board plugin** (`board.Plugin.Climb/Least`, which
  delegate to the Map) and the route sprites' heights from `board.Plugin.Top`
  (`PathRenderer.WithTops`); the old type assertions on the grid are gone.
- **`topography.Plugin` is `landscape` + `isometry` + the heights in one**: `NewPlugin(world,
  board, Config{Cell, TileW, TileH, HeightUnit, Headroom, Isometric, Shaping, Climbing})`,
  `Style`, `WithShadows`, `WithSelection`, `Seed(heights)`, `Relief()`. It requires Quasi3D and a
  world that does not wrap (panics otherwise), sets the world's cameras, ground and look, and the
  board's map at once in `NewPlugin`. The shaping commands and bindings moved here from the board.
- **One camera, two views**: the topography's camera holds a `flat` flag; `View` (Tab) flips it,
  keeping the ground point in the middle of the screen and a cell as wide on the screen as it was
  (zoom × Cell/TileW). From above the board's tiles lie flat (`board.FlatLook()`) and the world's
  entities as the world's own flat look draws them (`world.Plugin.FlatLook()`); isometrically
  blocks and billboards. Turn and Tilt do nothing from above; Follow centres without turning. The
  view is saved with the camera. A game begins from above unless `Config.Isometric`.
- **Tilt moved to R and F** (from PageUp/PageDown) as the plan's key table says; the rest of the
  key table is stage 4.
- **The flat `island-demo` is gone already** (stage 5 planned it): it dressed a flat world with the
  landscape, which the topography no longer does (it needs Quasi3D). The navigation-vision demos
  took the topography for their hills (seen from above), so they compile; their hex board under
  Tab would show hex cells as square blocks — stage 5's move should keep them from above or give
  hex blocks a look.
- **The landscape tests kept their dresser-level rig** (`newDresser(brd, relief, sun, quasi3D,
  styles)` with `quasi3D false` for a "flat world with heights"): the dresser still has that
  mode, though no plugin makes it any more.

### Keys and the shortcuts scene (stage 4)

- **WASD are `KeyHeld` bindings of the players' own `Pan`**, at the edge-scroll speed, so they
  work in every view and follow an isometric turn for free (Pan is a screen delta). The arrows
  stay the followed unit's.
- **Scene keys are `players.SceneKeys`**, a list of `{Key, Shift, Label, Do}` the scene runs from
  its HandleEvents; the shortcuts scene lists them under "Game". They are not `control.Binding`s
  because they issue no command and need the runtime and the composition (quit, save, open a
  scene), which bindings never see.
- **The shortcuts scene is `players.Shortcuts`**: bindings grouped by the handler that owns each
  command type (the players plugin keeps `owners`), the handler's heading its plugin name without
  `gram.`; drawn with ebitenutil's debug text over a dimmed screen, in columns when the list is
  long. It pauses the engine on Open and resumes on Close unless the game was paused before.
- **Full screen is F11, the engine's** (Shift+F was the engine's already, `Runtime.ToggleFullscreen`;
  the demos' own Shift+F scene key toggled it a second time in the same tick, so nothing happened,
  and F is the tilt held). The shortcuts scene lists F11 under Game as the engine's key.
- **The demos without players** (collision, vision, minimal, scenes) keep Esc to quit as they were.

### Demos and housekeeping (stage 5)

- **`examples/island` is a package, not `internal`**, as agreed; it exports `Layout`, `Kinds`,
  `Colors`, `Style` and the grid constants (`GridWidth`, `GridHeight`, `CellSize`, `Stops`). The
  snowy kinds and the ice stay in each demo's `climate.go`, since they are the demo's weathering.
- **`board-topography` begins isometric** (`Config.Isometric: true`); Tab shows it from above. The
  flat `board` demo has no hawk — a flat world refuses a Lift — and its units' sight cones are
  flat; its weathering has no "high ground" for the snow to lie on first.
- **`board-atlas` draws its sprites with `ebiten/vector`** (stripes, ripples, cobbles, tree tops):
  procedural, so the demo needs no image files. Its road is laid as a way of the road's kind over
  the grass, so the simple map's bands show.
- **README, CLAUDE.md, Makefile and the plugin/demo tables** name the new packages; the full
  CLAUDE.md architecture text on the board and the topography is refreshed where it was wrong, not
  rewritten. BENCHMARKS.md was not re-run (the numbers there are the landscape's; the code moved,
  the work is the same) — worth a `make bench-save` when convenient.
- **The old binaries in the repo root were deleted** (untracked, ignored by .gitignore).

### After the first manual tests (2026-09-28)

- **Full screen is F11**: see the key note above.
- **Full-screen frame rate.** Measured on the laptop at 1920×1080 with the temporary
  `GRAM_FULLSCREEN=1` log (board-topography, avg FPS): as it was 54.3; without the clouds'
  shadows 57.2; without the water's materials 48.9 (noise: another weather); without both 56.6;
  without the grid 52.4. On the external monitor at twice the resolution the same demo ran at
  30–40 and the flat `board` demo under 50, with the whole island in view in both cases — so the
  cost is per pixel (the fragment shader), not per tile: rendering is at the window's native size
  (`Engine.Layout`), and every pixel of every tile ran the clouds' noise (4 octaves of `sin`
  hashes) under any weather (clear is 0.1 cover), the water's pixels twice. `tps == fps` in the
  log says nothing: ebiten's clock rounds the ticks a frame to the nearest, so at 54–57 FPS it is
  one tick a frame whichever side is slow.
- **The fix**: the clouds' noise is worked out on the CPU at each piece's corners
  (`render.CloudAt`, the dresser's per-frame lattice of corners, the flat map's mesh of 64-px
  pieces) and shaded between them by the shader; pieces the clouds miss skip the pass. The
  shadow's inside is bilinear within a tile (32 wu, the finest octave was 52 wu), its edge still
  per pixel. CPU: the whole island composes in about the same time as before (see the benches).
  The `TEMP-MEASURE` switches (`GRAM_*` variables, the engine's per-second log, the CPU profile in
  the demos' `main`) were taken out once the camera rounds were done; the user's CPU profile at
  zoom 0.5 was never taken, so the composer's sort and the vertex building stay unmeasured.

### Relief hiding the view (2026-09-28)

- **Not a bug**: at a low pitch the near relief hides what lies behind it, in any projection.
  `topography.Config.MinPitch` (30° by default, a game may lower it) keeps the eye from looking
  that flat. A see-through — first the tiles round a selected unit, then the crests and slopes
  turned away from the eye drawn at 0.35 opacity — was tried and withdrawn at the user's word:
  it looked bad. A perspective camera placed at a point of the world is the next thing to talk
  through.
- **Culling**: in a view with depth the board renderer tested every cell under the rectangle round
  the screen's diamond, twice the cells on it; now each cell's screen box (from sea level to its
  top) must meet the viewport. The `Board_Island/iso,screen` benchmark (a 1080p screen) went from
  5.05 to 2.75 ms.

### Perspective camera (2026-09-28)

- **Asked for** once the see-through was withdrawn: a camera with an eye at a point of the world,
  the old cameras kept. Done as a third view of one camera: `viewCamera` switches between the
  untouched `isoCamera` (from above, isometric) and the new `perspCamera`, and Tab reaches the
  perspective only with `Config.Perspective` set; the board-topography demo sets it.
- **Decisions.** The perspective's state is orbital — the ground point in the middle, heading,
  pitch, distance — so Turn, Tilt, Follow and CenterOn work unchanged. `Depth` is how far ahead of
  the eye the middle of the cell lies on the ground, so a tile and what stands on it still tie.
  `Toward` is one direction for the whole screen, the shader takes one. A screen point over the
  horizon unprojects far along its line, so `Bounds` becomes the whole world and the board's
  `onScreen` culling does the rest. A point behind the eye projects far off the screen (not to the
  middle). The eye never goes under the ground: lifted, so steeper and further. `LookFrom`,
  `LookAt` and `LookOut` put the eye exactly where asked, below MinPitch too — the floor is for the
  player's tilting, not for placing; the next Tilt holds it again. Switching keeps the ground point
  in the middle and the screen scale there (screen units a world unit spans across); from the
  perspective back to the view from above goes through the isometric view, so a full round of Tab
  returns to the zoom it began at. `Zoom()` in perspective is the scale in the middle; billboards
  take `ScaleAt` their own point.
- **Follow in perspective** clears the eye before the shoulder pan, looking at the unit itself: the
  ground sampled every half cell from a cell off the unit along the line to the eye, a quarter
  cell of headroom; the eye comes in at once and eases back out (`followEase`); the player's zoom
  (counted in `perspCamera.zooms`) resets how far out. Clearing after the pan fed back on itself —
  the pan's reach depends on the distance — hence before.
- **Known limits**: the painter's sort by cell gives the same artefacts as the isometric view on
  tall relief looked at low; a tile's texture is interpolated affinely (seen on big tiles near the
  eye; splitting them, or 1/w in a vertex attribute, would fix it); the vision cones and the world
  renderer's fades scale by `Zoom()`, the middle's, not their own point's; `Bounds` over the
  horizon is the whole world, so `CellsUnder` walks every cell and `onScreen` rejects most —
  measured in `Board_Island/persp,screen`.
- **Second round, the same day.** The user's screenshot of the perspective low over the ground
  showed the near relief filling the screen as the isometric view does: "the camera is too close,
  it drives into the terrain — move it back". Moving the eye back along the same line of sight
  uncovers nothing over that line (the line is the same), it only shrinks what the near relief
  takes of the screen; what uncovers is the eye going up. So the free eye is lifted straight up
  until the line from the point to it clears the ground — sampled every half cell from a cell off
  the point, a quarter cell over the ground, a whole cell under the eye itself — in the projection
  only: the player's pitch and distance stay and the eye comes down as the way clears. Zooming and
  Follow's clearance keep the eye four cells off at least; a placing (`LookFrom`, coming out of a
  unit) may stand nearer. A zoom while lifted commits the lift into the state, so the anchor stays
  put. A fastened camera does not lift (`perspCamera.lifts`): behind a unit the eye comes in.
- **Inside a unit** (`LookOut`, Shift+V, the user's "FPP"): the eye is the anchor and goes with
  the unit; Turn, Tilt and Pan look round without the pitch floor (Pan turns by pixels over the
  focal length), Zoom narrows the field of view up to eight times. The view turns with the unit
  while the arrows turn it (`Driven.Turn`), free otherwise — the user's choice. Not saved: a load
  comes out, as Follow is not saved either. Coming out puts the point looked at on the ground the
  middle of the screen sees, or `distance` ahead in the air when it looks at the sky.
- **The sun** is drawn by the sky's backdrop through any projection with a vanishing point — a
  point far along the way to the sun drawn where one twice as far is, which only the perspective
  does — over the horizon, dimmed by the clouds, the hills over it. No moon yet: the sky has no
  direction for it.
- **Third round, the same day — the controls, by the user's word.** The orbit-and-lift camera
  was thrown out: the free eye flies at a fixed height, two cells over the relief's highest point
  (`Relief.Highest`, cached by version), and never lower — so it is never in the mountains. WASD
  move it along the ground, Q/E go round the ground point in the middle of the screen (read on
  the relief), R raises the head and F bows it with the eye standing (they were the other way
  round; the swap holds for the isometric view too), the wheel comes in down to the ceiling and
  narrows the field of view from there (`narrow`, 1 to 8), and the other way widens it back and
  lifts the eye — so the whole map can still be seen. Follow behind a unit steepens the pitch (5°
  steps to straight down) where the ground hides the unit, instead of pulling the eye in. Inside a
  unit (Shift+V) Pan no longer looks round: it steers the unit (A/D turn, W on, S stop — the
  cursor at an edge steers too, being a Pan), Q/E do nothing, R/F move the head without a floor.
  A save keeps the eye's place and height, heading, pitch and narrowing; `distance` is gone.
- **Fourth round: V is first person, keys by mode.** The user: V switches one way only, into
  first person; the unit steers with WASD, the camera pinned to the axis of the unit's sight
  cone, raised and lowered — they wrote E and D, but D steers, so R and F, as their previous
  message had it for raising and bowing the head; K must list the first-person keys then. Done
  with binding modes: `camera.Mode` (Free, FirstPerson), `camera.Rider` for a camera that rides,
  `control.Binding.In`; players fire and list only what holds in the camera's mode and accept one
  trigger in modes apart. The free camera's WASD, middle drag and edge scroll are `In(Free)`; the
  topography binds W/S/A/D to `Drive` `In(FirstPerson)`, R/F with first-person labels, V and Tab
  in both modes. Without `Config.Perspective`, V is Follow and the arrows drive, as before; with
  it, Shift+V, the arrows and V-follow are gone. The eye is pinned to `Vel.Dir`, not
  `vision.Sight.Facing`: the topography cannot import vision (vision's internal renderer test
  imports the topography, a cycle), and the demo's `faceTravel` turns the sight with `Vel.Dir`,
  so they are one there. Leaving goes back to the view the camera was in, over the unit (the
  isometric camera untouched, the free perspective as it stood). The ridden unit's billboard is not
  drawn. Question for review: should the eye sit at the sight's `Eye` height rather than on top
  of the billboard? It needs the sight, so the same cycle.
- **Sixteenth round: roads followed round their bends.** The user's screenshots: units cut the
  corners of roads, on the island in relief and on a flat meadow alike; "the road, even angular,
  should be cheaper than cutting across grass or rock". Two causes, measured on the demos'
  layouts. Flat: a road laid as ways in an L has two of its cells diagonal to each other across
  the bend; the planner priced that step by the destination (road 1 × √2 = 1.41), less than the
  two steps round (2), though the chord runs over the grass beside the band. Now a slantwise
  step not along a way (`Board.Along`: the way or crossing links the cells either way round) is
  priced by the ground under the way at the destination (`Board.Bare`), refused where that ground
  does not admit the unit (the water beside a bridge); along a way it is the way's, so slantwise
  roads stay cheap. Tried first and dropped: pricing such a step by the dearest of the two cells
  flanking the corner — it made a slantwise band of cheap cells, snow to the frost-born, as dear
  as the grass beside it, and the chord never enters the flanking cells anyway. In relief: a road step paid the full slope, and the
  island's roads climb grades of 20 to 80% (factors up to 9), which buried the road's 2.5×
  advantage under the slope's variance — a hop off the road onto a gentler cell won. Chosen:
  `CellKind.Graded`, a road cut into the slope, spared the slope in the planner and on the move
  (the island's road and bridge); the alternative, raising the ground's cost, is the demo's
  tuning and no rule. Measured with a throwaway test over every pair of road cells 4 to 20
  apart on the island (2841 routes): the corner rule alone left 1108 off the road, both rules 0.
  The spot search of a group's places prices its steps the same way (`pathFinder.price`).
- **Fifteenth round: the cameras in relief kept over the map.** The user, playing: the wheel does
  not zoom into the point under the cursor, zooming out shows the void beyond the map, WASD drive
  off the map. Diagnosis: `isoCamera.ZoomIn` held the anchor at sea level while the cursor points
  at ground drawn at its height (a 640-unit peak slid 64 px a notch at zoom 1); the perspective
  zooms by narrowing the field of view once the eye is at the ceiling, about the middle of the
  screen, and with 2 km peaks the eye is always at the ceiling; `place` held only the screen's
  middle over the diamond, `minZoom` fitted the diamond into the screen; the perspective clamped
  only the eye's height. Chosen with the user: iso and from above keep the whole screen over the
  world at sea level — the footprint of the screen is a parallelogram (the projection is affine
  at sea level), its rectangle fitted into the world along x and y like `basicCamera.fitAxis`,
  the floor where it just fits; consequences said out loud: the widest view on the island is zoom
  0.66 instead of 0.25 and a rectangular screen never reaches a diamond map's corners (the corner
  cells of the topography's test board were unreachable, so its fixtures have a screen that fits
  the board). The perspective cannot hide the void — from the 704 ceiling at 30° the middle row of
  a 1280 px screen spans 112 cells of a 96-cell map — so it keeps the ground point in the middle
  of the screen over the world (`confine`, one pass on the relief) and caps the eye where the
  middle row shows the world's diagonal at the flattest pitch: one height whatever the heading and
  pitch, so a turn or a tilt never moves the eye — a cap by the map's extent along the screen's
  horizontal, or by the current pitch, would have lowered the eye while turning or raising the
  head. The anchor in perspective: the eye moves along the line to the ground under the cursor
  (the point stays by construction), the ceiling and the cap shorten the step along that line,
  and where the view narrows or widens the head turns until the point is drawn where it was
  (`aim`, small-angle steps); at the pitch floor the head cannot rise, so the eye flies on along
  the ground to the distance that draws the point at its height (`advance`) — zoom in flies
  towards the point, zoom out backs off. Not done: keeping the anchor inside a unit (the wheel
  narrows about the middle there, the user's rule); `LookFrom`/`LookAt` stay unconfined, being
  the game's placing. Verified by tests; the demo is to be eyeballed.
- **Fourteenth round: the world in sub-packages.** The user asked what `plugins/world` could be
  split into ("steering, movement, something else?") and for a better word than `Quasi3D`; chose
  steering and view only, movement staying in the world, and `Heights`. The constraint that shaped
  it: the world's module registers the sub-packages' systems, so the world imports them and none
  may import the world; both systems read `Base`. So the components every entity carries go to a
  leaf, `world/entity`, and the world re-exports them as type aliases — one type for goke, so a
  save written before reads the same, and no plugin or demo changes a line for them. Aliases stop
  there: `steering.Steering`, `steering.Driven`, `view.View` are named by their packages, or the
  split would be a file move. `steering.System` and `view.System` are the packages' names
  (`steering.NewSystem()`, `view.NewSystem(...)`), as `clock` and `effects` name theirs. The
  steering tests that drive the world's ticks stay in `world` (they use its test harness); the one
  reaching an unexported method moved with the code. The view system's loop became
  `View.Refresh`, so the system is the loop and nothing else. Not done, on purpose: `MoveSystem`,
  `VelocitySystem` and the leavers stay — they are the world's core, the user's word. `Quasi3D`
  → `Config.Heights`, `Plugin.HasHeights()`; the board's, the topography's and vision's `quasi3D`
  fields are `heights`; historical mentions in this file and the CHANGELOG stay. Verified: the
  full suite, `go list -deps` on the three new packages showing no import of `world`.
- **Thirteenth round: render generic, the world entities only.** The user: `render` is generic
  and held `sway.go`, `overcast.go`, haze; then, on my plans, twice: no contract for the sky or the
  ground in the world — "world nie może wiedzieć NIC o atmosphere, i najlepiej żeby o ground też
  nic nie wiedział. On wie o ENCJACH; a ground do board, a sky itp to atmosphere." Rejected on the
  way: keeping `Sun`/`Weather` in world as data (its home is the atmosphere), a `world.Atmosphere`
  contract implemented by the atmosphere (the world would still name the sky), leaf packages
  imported by `world` (the core importing a plugin). The import directions decide the rest
  (real imports, tests aside): atmosphere → board → world; topography → board, world; nobody
  imports topography. So: `render` gets generic uniforms (`Frame.Uniform`, the composer zeroing
  what a frame did not set, ebiten ignoring names a shader lacks and taking a slice as long as the
  type) and `Fog`; the sun's Kage and maths go to `atmosphere/sky`, the weather's to a new leaf
  `atmosphere/air` (which imports sky: `Weather.Frame(f, sun)` needs the sky's colour for the
  fog, and `CloudShadow` reads `SunStrength`; one concatenated Kage source, so declaration order
  is free and the compile tests guard the names); the backdrop moves to the atmosphere root, as
  it needs both. The world's renderer draws in white light and the `Look` lights: topography's
  look (it imports the leaves and takes an `Atmosphere`, defaulting to `sky.DefaultSun` in still
  air, so `navigation-vision-demo` shades as before) or the atmosphere's wrappers over a flat
  board (`WithBoard`: the board's Map and the world's Look). The ground: `board.Heights` (the name
  `Ground` was the cell component's), `board.Cover`, `collision.Field` set by
  `board.Plugin.WithCollision` (collision cannot import board); sight takes the board with
  `WithBoard`; `control.Context.Ground` goes, the topography's camera being a `Picker`. Entity
  shadows moved from the world's renderer to `sky.Sun.Shadow`, laid by the topography's look.
  Verified: the full suite, the shader compile tests in render, sky, air and topography, the
  crowd and navigation tests unchanged. Not verified here: the look of the demos — A/B against the
  tree before this round.
- **Twelfth round: no lanes, the standing give way.** The user chose both open points: units go
  cell centre to cell centre, and one standing, struck by one on the move, gives way. It steps
  just off the line of the strike, the two boxes' halves across it and a quarter gap (the circles
  round the boxes stepped it into its neighbours), to the side it stands on or the other where
  the ground does not take it, lingers a second (`MoveOrder.Linger`, stood out at a queued goal)
  and goes home facing as it did; the whole order is `GivingWay`, and nobody gives way to one
  giving way, or two would trade places for ever. The strike's normal gives the line, not the
  mover's velocity: a mover stepping aside when it struck turned the line along a lane. Tried and
  dropped: a mover waiting for the one giving way (worse for larger units). Ticks of contact over
  the 48 crowd runs, per unit side:

  | Side | Lanes, no giving way | No lanes, no giving way | No lanes, giving way |
  |---|---|---|---|
  | 3 | 3476 | 6740 | 6164 |
  | 6 | 2510 | 4646 | 5140 |
  | 10 | 2412 | 3166 | 3758 |

  Without lanes the group funnels through the cells' centres and fans out at the end, where most
  strikes are now, between two on the move; the giving way ends the pushing of those standing
  (a handful of ticks left) but adds a little between the one aside and the one passing.
- **Eleventh round: the click, and units smaller than a cell.** The user: a click on another cell
  sometimes started no route and sometimes picked the wrong cell; and units smaller than a cell
  should stop where clicked, a group round the point, the rest on the cheapest cells round it,
  none where it would fall. The click: `Context.World` unprojected four times at the height of
  the last answer, from sea level — a fixed point that settles only when the slope times the ray's
  run is under 1; on 32-unit cells with 640-unit peaks it diverged, landing on a wrong cell, off
  the board (no command) or on the unit's own cell (it only turned). Now `camera.Picker`: the
  topography walks the line of sight half a cell at a time over the drawn top and halves to
  1e-3; the perspective's middle point too. Units: the user kept the cell model for board games
  and simple games and asked for a choice by the unit-to-cell ratio — `navigation.Spacing`,
  `AutoSpacing` picking boxes at a third of a cell or less (the demos: 22/32 and 24/48 cells,
  3/32 boxes). A first box version planned ahead: each unit saw the others' boxes and velocities
  and steered round them, spots were swapped among the group on arrival, routes went round known
  standing units; tuned from 16 000 ticks of contact over 48 crowd runs down to 98. The user
  stopped it: a unit routes over the ground knowing nothing of the others and corrects when it
  strikes someone; only the destination points of a group are planned. So now: routes over the
  ground alone, lanes from the unit's own spot, and answers to `collision.Struck` — step aside
  towards the goal (never back into them, never onto ground it may not take), note the cell of
  one struck standing for the routes, stand elsewhere round the point when it stands on the spot,
  a repeat strike on a known one or no headway for a second is a stall, five and it stands; by
  its spot, a second one standing struck and it stands there. Group spots: a box's width apart,
  so one of the group walks between two standing; far rows to the units furthest on.
  Measured over the same 48 runs (4 to 25 units, four approaches, 30 s): all settle, nothing
  overlaps once settled, the widest stands 3–4 spacings off the point; ticks of contact per side
  3 / 6 / 10: 3476 / 2510 / 2412, worst 1568 / 696 / 830 (25 in a column from the west). The
  model strikes by design; the remaining pushing is in big groups arriving. Two bugs found on the
  way: a unit giving up kept its speed and drove to the world's edge; a contact whose normals
  cancel read as a stall every tick.
- **Tenth round: giants, the eye under the ground on slopes, blur.** The user: giants, so every
  system reads the same sizes; riding, the view blurred at some angles, and on a steep slope the
  head still went under the ground. Not a lag: the simulation (movement, altitude) all runs after
  the interface part, the camera reads both from the same step. Measured with a throwaway test:
  riding 0.64 up in the middle of a cell rising to the north-west, looking down it, three of the
  cell's corners lie behind the eye and the projection throws them to about ±926 000 px — the
  uphill one far over the top of the screen, so the cell's top is drawn over the whole sky. Nothing
  clipped at the near plane. The fix stays in the topography (no clipping in the frame): a tile
  with a corner not in front is drawn as 16×16 pieces (4×4 when the eye's plane only grazes it off
  to a side), only pieces wholly in front and on the screen, farthest first, each with its water
  and cloud shadow; a tile wholly behind is skipped (its top lies between its corners); blends and
  way pieces not wholly in front are left out; faces of such a tile are left out. The blur was the
  same cause: a tile's pixels were read at its middle, 0 behind the eye, so the tile under the eye
  was drawn from the 16-px ground sheet; now at its nearest corner in front. Giants: `world.Look`
  hands the entity's `Z`, billboards stand as tall as its Height; the demo's units are 3 units
  (9.4 m; sizes are whole units) by 20 m, eyes at 18 m. Still at 300 m a second (`UnitSpeed`, three
  cells a second), fast for a giant.
- **Ninth round: the eye under the ground, 100 m a cell.** Riding low, the eye fell under the
  drawn ground. Measured on the island: a unit's ground was read between its cell's corners
  (bilinear) while the cell's top is drawn as two flat triangles folded along the diagonal whose
  corners stand nearer in height; at a cell's middle the drawn top stood over 2 m above the unit's
  ground on 759 of 6144 cells, up to 267 m. `Relief.GroundAt` now reads the triangles, with the
  fold's own rule (ties split along 1–2, as a quad is drawn), so units, sight's ground, picking
  and shadows stand on what is drawn; the riding eye is also kept over its cell's drawn top (a
  kind's Height). I blamed a forest at first — the island plants none; wrong. The user chose
  100 m a cell (not 10 m: the island would be under a kilometre across, no room for mountains,
  no haze or curve to speak of); peaks to 2 km (`island.Metres` 8). Open: the units' sizes —
  their box is a 69 m map symbol and they are 2 m tall.
- **Eighth round: the Earth under the island.** The user asked whether, a cell being 1 km and a
  unit small, sight's reach and "the bending of rays" should not differ in first person, then
  said the eye belongs on the entity's height, which its component holds, a unit about 2 m
  tall, peaks at most 3 km. One unit system stays: heights and lengths in world units alike, so
  slopes, light, sight and the camera need no conversion; `world.Scale{Metres}` says what a unit
  is and games give metres through `Scale.Units` (the demo: 31.25 m a unit; the island's own
  heights times `island.Metres` 12). With a scale: the ground sinks `Scale.Drop` — the Earth's
  curve with standard refraction k = 0.13 — in the perspective's `view` (its inverse, `cast`,
  solves the quadratic in the stable form; `Bounds` widens the layer down by the drop at twice
  far, conservatively) and in sight (the ground, the cover's bands at a stretch's middle, the
  entities' bands at their centres, all against the observer). The air: `Weather.Visibility`
  from `Scale.Visibility` (40 km clear; cloud to 0.65 of it; rain ÷9, snow ÷21 at their fullest)
  unless the weather sets it; tiles, faces, baked pieces and billboards take `Frame.Haze` in their
  alpha (1 + 0.49·haze, under the overlays' mark) and the shader turns them to the overcast sky;
  blends, water and ways laid over tiles are not hazed — far tiles are baked and their water gone.
  The riding eye is `world.Z.Top()`; `riderLift` is gone; the near plane riding a thousandth of a
  cell (1 m in the demo). Consequences to know: slopes are true now, gentler than the island's own
  units had them (peaks were ~7.8 km at 31 m a unit), so climbing costs and running water are
  milder; the isometric view shows the relief as tall as it is; the units' billboards are still
  map symbols (the box, 690 m) — riding low, another unit near looks a tower. Question for
  review: draw billboards in perspective at their Z.Height? Benchmarks under a load of 6 on the
  machine are not worth recording; the bench's island has no scale, so none of this runs there.
- **Seventh round: white shadows, bare rivers, rain in bands.** From the user's screenshot of
  first person looking far off. (1) White patches: `Frame.Soft` writes 1 plus the distance to each
  edge in fades into the vertex, and the shaders took a last value over 5.5 for a blended sprite
  (mark 10) — any soft piece taller than 4.5 fades was drawn as one. Through a perspective the
  sight's shadow fade came from `Zoom()` in the middle of the screen (tiny looking far) while the
  near pieces are huge. `blendMark` is 100000 now (the shaders test over 50000), fades are capped
  at 10000; the fade is taken where the piece lies, and pieces not in front of the eye are left
  out. (2) Bare river and roads: the dresser picked its detail once a frame from `Zoom()`, so
  looking far off every tile, the near ones too, was drawn from the 16-px ground sheet with no
  water. Now each tile's own pixels (`tile.px`, `camera.ScaleAt` at its middle) choose its
  detail, shore, ways and whether it is baked; the sheet is kept while any tile may be small (the
  top of the screen tells). The same for the grid per cell and the units' soft shadows. (3) Rain:
  the wind's slant came from projecting the world's origin, behind the eye at times; now from the
  ground in the middle of the screen, capped at 45°. Rain and snow were screen overlays already;
  nothing else needed changing. Measured under a load of about 1.5 on the machine (the island
  through 1080p): isometric 3.25–3.61 ms, perspective 8.3–8.8 ms, against 2.8–3.0 and 7.1 earlier
  in the day; a profile shows none of the new paths among the costly ones, and per-tile scale is
  skipped when the camera's scale does not vary — to be measured again on a quiet machine. Also
  found by the new test: the perspective's `Bounds` from the screen's corners missed the ground a
  screen side crossing the horizon sees out to far; the line of sight leaving the layer just at
  far bounds it now.
- **Sixth round: the ground vanished, and the sun.** The user: in first person the sky showed
  under the hills, "a similar effect in the map view, in perspective and without"; the sun,
  riding, seemed to turn with the head when low, and vanished on a small turn when high. Measured
  with a throwaway test: both cameras' `Bounds` — the rectangle the board renderer walks and the
  world's View draws the units of — came from the screen's corners cast on sea level. Isometric:
  high ground drawn up over the screen's lower edge lay outside it. Perspective: the ground
  between the eye and where the lower corners met the sea lay outside it, and looking over the
  horizon it turned inside out, `(0,0)–(40,−460)`. Now both span the layer from
  `Relief.Extent`'s low to its high plus `Headroom`: the isometric corners cast at both heights,
  the perspective's corner rays clipped to the layer (the eye included when it is in it), clamped
  and never inside out; `Visible` uses the same layer. `Board_Island/iso,screen` 2.86 → 2.94 ms,
  `persp,screen` 7.12 → 7.14 ms. The sun came from projecting two finite points behind the ground
  in the middle of the screen and taking them for the vanishing point when a pixel apart — 12 km
  off when looking level, so only near the middle; worked through the code's numbers, it vanished
  off the middle and showed where the sun was not. It is `camera.Vanisher` now: the direction's
  own vanishing point. The "turns with the head" symptom did not come out of the numbers; to be
  checked again in the demo.
- **Fifth round: mouse look, the eye higher.** The user: in first person, the mouse instead of
  R/F, and the eye a little higher — it scraped the ground. The eye rides `riderLift` (a cell) over
  the unit's top. `control.CursorMove` is a new trigger; the players plugin captures the window's
  cursor while a local camera rides (`ebiten.SetCursorMode`, skipping the pass it is caught or let
  go, when the cursor jumps) and fires it for the riding player wherever the cursor is. The
  topography's `Look` turns the view at once and has the unit turn to face it through a new
  `world.Driven.Face`, which navigation's drive system turns towards (at the unit's `TurnRate`),
  held until the unit faces it; the camera stays pinned to the unit's facing otherwise, and A/D
  take over from the mouse. Looking across therefore ends an order the unit walked, like any hand
  on it. Mouse up and down raise and lower the head, without a floor. R/F and Q/E are unbound
  riding. Mouse right turns right; not inverted vertically; `LookStep` 0.0025 rad a pixel.

## Questions for review

- **Determinism across tempos** holds for the simulation; the interface part (orders, selection)
  runs once a tick, so a player acting at ×4 acts every four steps rather than every step. That is
  what the plan asks for, but a replay of recorded commands must record the tick, not the step.
- **Should the frozen light freeze the date too?** Now only the hour freezes (see above). If a
  frozen light should be one fixed sun, the sky should keep the moment it froze at.
- **The frozen light is not saved.** The plan called it a look, like the camera; if it should come
  back after a load, it needs a component (the clock's entity would do).
- **Weathering on a hex board** picks cells from a list of all of them (`EachCell` once); on a
  square board the old code picked by column and row. Same effect, a slice of cell ids in memory.
- **Should heights on a hex board stay per cell (level cells)?** They do, as before; the lattice is
  the square grid's. A hex board's blocks in the isometric view are drawn as square tops, as they
  always were.
- **The topography's `Heights` entity writes every tick** (a slice header, no copy) like the
  clock's State. If saves turn out to need the component only at save time, a `Persisted` hook
  would do instead. goke logs once at start that the component "requires a dereference outside
  the archetype's chunk memory" — true, and harmless for one entity read once a tick; a
  fixed-size component would need one per cell instead.
- **Big groups arriving still strike each other**, now mostly two on the move where the column
  fans out to its spots (see the twelfth round). Lanes halved that for small units; they were
  dropped at the user's word.
- **No ticking test through the demo**: the engine steps by the wall clock and reads ebiten's
  input, so the board-topography test only starts it; the shore and the steps are tested in
  navigation instead.
- **Screenshots of the island in relief were not compared** — I cannot take them unattended. The
  demos run headless for ten seconds without a panic; please eyeball island-isometric (Tab, Q/E,
  R/F, =/-) tomorrow.
