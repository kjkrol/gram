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
  (zoom × Cell/TileW). From above the board's tiles lie flat (`look.FlatLook()`) and the world's
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
- **Thirtieth round: gram on WebGPU, the whole picture on the GPU (2026-09-29/30).** The user:
  leave Ebitengine altogether, rebuild on WebGPU (gogpu), then draw everything on the GPU — "po to
  była ta cała zmiana" when the first port drew worse than Ebitengine. Targets: 60 FPS in a window
  and fullscreen at 2560x1440 on the UHD 620, the CPU's part of a picture under 2 ms. Phase 1 put
  `render` on `render/gpu` (WGSL programs, textures, mapped staging buffers) and the engine in a
  gogpu window. Two findings made most of the difference: `Queue.WriteBuffer` waits for the GPU to
  finish all it was given (an 11 ms stall a frame; the frame's bytes now go through our own
  MapWrite buffers, a failed map falling back to writing them directly), and gogpu waits for
  Wayland's frame callback after every frame (40 FPS where 100 were possible; the engine turns it
  off). Phase 2 in stages: the relief's ground a mesh of its lattice (`topography/terrain`: a depth
  prepass saved 3 ms at 2560x1440; the heightfield and G retired), the world's entities GPU
  billboards against its depth (`Hides` retired), the sky and the rain Direct, the views of sight
  baked per observer and laid over the ground read from the depth (only the selected entity's,
  `vision.ShowViewOf`), the routes likewise, the water (noise waves faded per pixel, a skirt of sea
  to the horizon, the sky reflected by the way to the eye per pixel), a sun that is not grey.
  Then the flat boards: composed once and kept on the GPU (`render.Still`, `gpu.Kept`, placed and
  lit by `gpu.Draw.Place`/`Tint`; composed anew only when a cell changes — none in 20 s of the board
  demo), the grid by a shader, the flat world's clouds a mesh of noise every 8 pixels (6.5 → 0.6 ms
  of the GPU), the flat look's sprites as instances (`render.Sprites`), the views over level ground
  on the GPU; and a hex board in relief as prisms, coloured from its tiles composed once from
  above. CPU drawing a frame at 2560x1440: the island 0.5 ms, board 3.2 → 0.3, board-atlas
  0.46 → 0.2, navigation-hex 0.69 → 0.14, navigation-vision-hex 1.2 → 0.25, collision 2.8 → 1.4
  (the walk over 14,000 entities). Removed: the topography's tile blocks, its per-corner light,
  shadows and clouds, its parallel dressing workers — the dresser now only paints, in white — the
  CPU billboards, `sky.Sun.Shadow`, the CPU cloud overlays, `examples/webgpu-island`.
  **Two user reports along the way.** "Nothing changed" about the open sea swaying to and fro: my
  first probe froze the uniforms, found the frames smooth and I fixed an unrelated clock step
  (`clock.Clock.Shown`: game time run on between the ticks — right, but not it); the cause was the
  wind's heading wandering every tick and the sea turning its whole field with it about the world's
  corner. The waves run along x now, the wind only roughens them. And the bands of roads and rivers
  on a flat board are now lit by the sun as the tiles are (they were not); said to the user.
  **Decisions.** The composer stays: it orchestrates the tiers, the depth and the Direct sources,
  and paints (the sheets, the stills). The selection's and the goals' outlines and the marquee stay
  2D pieces of the frame on the Marks tier — a handful a frame, drawn by the GPU in one call — like
  the HUD. The board's per-frame tile path stays for cameras not looking from above and dressings
  that light tiles apart (`look.EvenLit` says which). An observer of a world without heights that
  carries a `SightOutline` keeps its outline: the entities cut its view on the CPU only.
  **Found on the way**: the navigation-vision demos drew no ground since the terrain became a
  Direct (their composers lacked the topography's renderer; fixed); on a wrapping flat board the
  per-frame bands jumped across the screen where a cell lay just past the view's edge (the camera's
  `Project` wraps each point alone) — the still does not.
  **Known limits, questions for review.** The hex prisms cast no shadows (the units on them do,
  laid over the prisms from the frame's depth); the GPU views ignore entities as occluders; FPS were not measured live at the end (the screen was
  locked: the compositor held every window at 20 FPS), only headless — to check in the demos.
- **Twenty-eighth round: the WebGPU try (step 0 of the backend plan).** The user asked for
  WebGPU beside Ebitengine, the abstraction in gram, gogpu first, with a gate after a try on the
  UHD 620. `examples/webgpu-island` draws the island as a mesh raised in the vertex shader from
  an R32F texture of the lattice, a Depth32 buffer with the depth reversed and no far plane, a
  2048 shadow map with nine-tap PCF, a sea plane, 400 instanced boxes. Measured (1024x768,
  Vulkan, Mesa): the CPU 0.22 ms a frame; ~95 FPS, which is the external monitor's 100 Hz — the
  compositor paces presentation even with VSync off (an empty frame gave 97); drawing the scene
  4, 8, 16 times a frame puts it at ~3 ms of the GPU. Findings: (1) **Linking.** goffi's
  dynamic-import trampolines do not survive external linking, which any cgo package forces, and
  Ebitengine on Linux pulls `purego/internal/cgo`; a binary holding both builds only with
  `CGO_ENABLED=0 -tags nofakecgo` (goffi then uses purego's fakecgo). Ebitengine runs so —
  `TestShots` passed without cgo, 11 shots — but such a binary has no race detector. The try is
  behind the `webgpu` tag so the default build is untouched. (2) **gogpu bugs worked round.**
  wgpu v0.34.2's Vulkan faulted ending a depth-only pass (fixed in v0.34.5, taken); its Vulkan
  binds a group through the current pipeline's layout, so `SetBindGroup` before `SetPipeline`
  dereferences nil (the pipeline goes first); v0.34.5 refuses a bind group holding a 32-bit
  float texture beside any filtering sampler, even an unrelated one (the heights have a group of
  their own). (3) **The Rust backend** (`-tags rust`, wgpu-native v29.0.0.0 and v29.0.1.1)
  aborts at start inside wgpu-native ("invalid callback"); only the pure Go backend works today.
  (4) gogpu renders on demand unless `WithContinuousRender(true)`; its render thread re-panics
  without the stack, so the try prints it. Not compared like for like with Ebitengine: the
  demo's traced ground draws far more (dressing, clouds, HUD) at 47–61 FPS.
- **Twenty-seventh round: the hawk flown holds its height over the sea.** The user: the hawk's
  height hangs on the ground under it, so in first person it jumps up over a rising range and
  drops where the ground falls, though the rider neither climbed nor dived; ridden, the two should
  come apart, with a least distance kept from the ground. The drive system climbed `Lift`, a
  height over the ground, so a flyer went with every rise and fall of it. Now `Driven.Flown`
  (written with `Climb` while riding) makes `Z.Altitude` the independent state: the altitude
  system — the one that knows the ground, and now the one writer of heights — takes last step's
  altitude, adds the rise over the run the flyer made (its `Speed`, capped at the world's
  `MaxStep`, times rise/run), holds it at least `Mover.Clearance` over the ground and under
  `Ceiling`, and writes `Lift` = altitude − ground back. Out of `Flown` the old rule stands; `Lift`
  written back makes entering and leaving continuous, so let go the flyer follows the ground at
  the height it had. The drive system only slows the run along the ground (`Driven.Slope`, the
  80° cap moved to `steering.Steepest`). The ground is read under the centre, as for every
  altitude; a flyer racing into a cliff is pushed up at once by the clearance, not before it.
  A headless probe through the demo: flown level, the eye stayed at 256.4 while the ground under
  it went 142 to 249, rose to 341 only where a ridge came up to the clearance and held 341 after
  it; flown down, it came to 20 m over the sea and stayed; let go, its lift stayed 20 m.
- **Twenty-sixth round: Shift sprints, the hawk flies along the look.** The user: in first
  person every unit gets a ×4 speed on Shift; the hawk's altitude follows the look, up and down,
  naturally. Chose to keep the world's step cap (half a body a tick, 90 units a second for the
  3-unit giants: a walker reaches about 3.75 times, the hawk about 2.5 times its speed) and a
  hawk from the ground to just under the clouds. (1) **Shift without a binding.** A `KeyHeld`
  context carries the modifiers as the player last held them, so W builds `Drive{Sprint:
  c.Mods.Shift}`; a second, Shift-only binding would clash with W's. The climate's Shift+W
  (`KeyPress`, any mode) would have changed the weather on every sprint, so it holds
  `In(camera.Free)`. (2) **Sprint in the profile.** `Steering.Sprint` says how many times the top
  speed a hand may urge a kind to; `RequestSpeed` keeps its clamp to MaxSpeed for every other
  caller, `RequestSprint` asks past it, and `advance` speeds up past the top at `Accel·Sprint`, so
  the dash comes as quickly as setting off does. (3) **Climbing along the look.** The camera
  writes `Driven.Climb` = −sin(pitch) while riding; the drive system, for a mover with `Air`,
  asks `top·cos` along the ground and changes `Lift` by the run it makes this tick times `tan` —
  the run capped by the world's `MaxStep`, so the flight goes along the look even at the cap.
  The lift floors at 0 there. (4) **The ceiling where the ground is known.** `Mover.Ceiling` is
  over sea level; only the topography knows the ground under the flyer, so its altitude system
  holds `Lift` under `Ceiling − ground` and writes it back. A headless probe through the demo
  (a Drive every tick, as a held key fires) measured 24 → 85–90 units a second for a walker on
  flat earth, 36 → 88–90 for the hawk, and its lift 96 → 310 looking up, down to 0 looking down;
  on the rocky slopes the terrain's slope slows a walker to a fraction whatever it asks.
- **Twenty-fifth round: water, waves, clouds and grid on the GPU ground.** The user asked what of
  the grid and the water's and the sea's effects on the GPU; chose half-resolution tracing (grid
  ~2 px and soft). (1) **One library, two shaders.** The water is Kage functions of the
  composer's one shader; the heightfield's own shader could not call them. compose.kage is split
  into library.kage (header, the composer's uniforms, helpers) and the composer's Fragment;
  `render.ShaderSourceWith(fragment)` puts a caller's fragment after the library and every
  material, so the heightfield calls `SeaGlint`, `RunningWater`, `cloudCover`, `cloudField`,
  `sunWay`, `outlineDark` as they are. Its own Sun/Fog/Visibility went (name clashes); it reads
  the frame's, which `Direct.Draw` now hands it (the composer's boxed uniforms, no allocation);
  it sets its own over them. `Pixel` stays the frame's: scaled by the traced pixel, the rivers'
  ripples and flecks vanished outright rather than blur; so they are drawn as fine as on the
  tiles and the half-size tracing blurs them. (6) **From above nothing showed**: a line straight
  down never meets a column's or a row's boundary, which the walk marks with a boundary
  "never" — `1.0e30` in Kage, which did not come out of the shader's compiling as that number
  (the boundaries read as 0 on the Intel driver, every line ended before its first cell); a
  number past the farthest a line runs (2e9) does. `ShaderSourceWith` seals the
  materials: a late one would be missing from the heightfield silently. The heightfield's compile
  test moved to topography, where the water is registered. (2) **Coverage in a colour, not in
  alpha.** On the tiles what is drawn later covers the water: the coast's grounds over the sea's
  glint, a road and a bridge over a river. Painted with source-over, a black cover at alpha a
  would raise alpha, not lower a coverage; so each layer holds its coverage W in a colour
  channel and every value premultiplied by it (`v' = v·a + dst·(1−a)` for all channels), and the
  shader decodes value/W and multiplies its overlay by W — foam does not scale with shine, so
  scaling the shine alone would not do. Three layers in quadrants of one image: flow (vx, vy,
  Wrun), shine (run shine·Wrun, sea shine·Wsea, Wsea), mouth glint (glint·W, W); painted from a
  white sheet with the data in the vertices' colours, `SpriteBlend` where a piece fades, in the
  albedo's own passes. (3) **The pixel's footprint analytically.** `fwidth` after the early
  return for missed pixels is undefined and wrong at silhouettes; the lines of sight are known
  (`camera.RayField`), so how far the hit moves per screen pixel is worked out on the plane of
  its normal — how many pixels a cell spans (the tiles' Detail, shore and way thresholds) and
  how far to a cell's edge in pixels (the grid). (4) **The coast by shine.** The shore depends
  only on which cells shine; keying it on the board and relief version would rebuild it every
  frame while the ground is shaped. (5) Still apart from the tiles: the grid is drawn over the
  ways (the tiles hide it under ways near); `Toward` and `Pixel` are one per frame, as on the
  tiles.
- **Twenty-fourth round: S brakes, then backs away.** The user: in first person S should brake
  the unit first and then back it away at V0; and the units should walk four times slower. S
  wrote `Driven.Ahead -1`, which the drive system took for "stop at once" (`Speed = 0`). Backing
  away with the view still ahead needs a velocity against the facing, so `Velocity.Value` under
  zero is backing, `Dir` the facing, and `Delta` goes the right way by itself. The steering keeps
  `RequestSpeed`'s clamp to zero for every caller that relies on it (the keepings compute speeds
  that may dip) and gains `RequestBack`; `advance` brakes to a stop before changing way. Two
  readers of the velocity took `Dir` for the way of going: the terrain's slope (now the way it
  goes) and the collision's bounce through `SetDelta` (a backing entity stays backing, so it does
  not turn round). The camera's let-go wrote `Ahead -1` for a stop and then dropped the Driven
  the next tick: under the new rule a standing unit would have been asked back once and backed
  away for ever, nobody asking again; it writes no hand now, and the unit brakes. The speed is
  the demo's (`UnitSpeed`, 3/4 of a cell a second); the gait's Accel, Brake and V0 go with it.
- **Twenty-third round: the GPU ground "a total failure".** The user: no rivers, a faceted
  terrain, and from first person the mountains above the eye vanish. Three causes. (1) The
  colour was one per cell from `Kind.Color`; rivers, roads, bridges and blends are `WayPiece`
  and `BlendPiece` bakes the tiles draw or, from far, paint once on the ground sheet — which
  holds no bases and, under `look.Nothing`, is never repainted. So the dresser paints a second
  sheet, the *albedo*: `groundSheet` with `based`, `newSheet(atlas, based)`, the painter
  (`paint`, `paintAll`, `paintCell`, `lay`, `markWays`) taking the sheet instead of reading
  `l.sheet`, and `layBase` laying `base(c)`'s sprite over the whole cell first, so the sea lies
  under a coast as `dresser.Base` has it. `paint` already repaints only the changed cells by
  the neighbourhood's versions, so shaping costs a few 16-px cells a tick. The atlas reaches
  the topography through `board.Plugin.Atlas` (nobody in the engine calls a plugin's
  `WithRenderer`; games call the board's). The shader blends the albedo between pixels; one
  continuous painting, so a way runs on across cells. (2) The normal was the cell's bilinear
  gradient, discontinuous at cell edges; the tiles light a corner from the slope between the
  neighbours' corners. Now per-corner normals are worked out on the CPU with the same central
  differences (one-sided at the edge) into a third image and blended across the cell: four
  reads a pixel, not sixteen. Shadows keep the smallest clearance over the ground against the
  distance along the ray (`shadowSoft`, a cone of 4.6°) — a penumbra that does not darken
  level ground under a low sun, which a fixed clearance would. (3) The box clip took every
  meeting with the top plane for an entry; an upward ray from under the top got entry = exit
  and marched nothing — the riding eye sits a hair over the ground and every ray to a peak
  goes up. Kage reads every source image in the first one's texture space, so the albedo, a
  different size, is read at `imageSrc0Origin() + pixel` too; images of different sizes are
  allowed in pixel units. (4) **Flat summits.** The shots after (1)–(3) showed the highest peak
  cut flat, 60 px lower than the tiles draw it, and the silhouettes stepped. Not step
  starvation: a CPU port of the march over the demo's own relief, looking at the summit, missed
  179 of 4760 pixels a march of a hundredth of a cell hits — it stepped at least half a cell,
  or six tenths of the height over the ground below, and jumped over tips a few units wide on
  slopes of 5 and more. A walk cell by cell (a grid walk) meeting each cell's two triangles
  exactly missed 1. The shader walks so now, up to 320 cells, passing over a cell whose highest
  corner (a fourth image, rounded up so nothing stands over it) the line stays above with one
  read; four reads where it does not. The shadows still march in steps of three quarters of a
  cell, soft, so a thin tip may cast too little. (5) The harness's first-person shot rides in
  one walker, selected at the start: LookOut takes exactly one selected, and LookFrom lifts the
  eye over the ceiling, so it could not show a low eye. Still missing on the GPU ground: the
  water's materials (rivers are flat bands), kinds' Height (a forest lies flat), grid lines;
  drawn sprites shrink to 16 px.
- **Twenty-second round: the ground traced on the GPU.** Stage 4 of the same order: a
  switchable renderer of the relief on the GPU, in its own package. Ebitengine has fragment
  shaders only, no depth buffer and no retained vertex buffers, so a "GPU terrain" cannot be a
  mesh the CPU does not build every frame; it is a heightfield ray-marched per pixel. (1)
  **Where it draws.** The composer draws the frame's pieces in one shader; a shader of its own
  needs a place in that order: `render.Direct` is a Source drawing itself before the first piece
  of its tier or over — after the sky's quads at −∞ depth, before the tiles' tier. (2) **What it
  reads.** The relief's lattice (`Relief.Lattice`, cols by rows corners, the values to read) as
  an image of 16 bits a corner, written when the relief's version moves; the kinds' Colors a
  cell (`Board.Kind`, so a way's kind colours its cell) written when the board changes; the
  camera's lines of sight as six vectors (`camera.Rays`/`RayField`: origin and direction each
  affine in the screen point — a perspective's from the eye through the picture plane a focal
  length ahead, the isometric and the flat view's parallel, from Headroom over the highest
  ground down along −Toward, which is the way everything projects along); the sun and the air
  as the composer's shader gets them. The shader splits a cell into the two triangles a tile is
  drawn as (`drawnAt`'s rule), so the ground it traces is the ground the tiles draw and the
  units stand on; it sinks the ground by the perspective's Bend as the tiles do. Steps are
  bounded (Kage needs constants): 160 at most, a quarter cell or six tenths of the height above
  the ground, six bisections after a hit; shadows 24 half-cell steps towards the sun, on by
  default in the demo for parity with the tiles. (3) **What hides what.** Without a depth
  buffer a billboard drawn after the ground would show through a hill: `Renderer.Hides` walks
  the line of sight from where it starts to the unit's middle every half cell on the CPU and
  the topography's `worldLook` leaves the billboard out; the cones, routes and shadows are
  overlays on the ground and stay. (4) **The switch.** `Config.Heightfield` makes the renderer
  (`Plugin.Renderer`, nil without) and binds G to the `Heightfield` command; shown, the board's
  Map gives `look.Nothing` for its Look — the renderer readies the dressing for the frame (the
  sun's and the air's uniforms the billboards need) and lays no tile. Tiles stay the default:
  a strategy beside the old, not in its place. (5) **Not done, and doubts.** The colours are
  the kinds' flat colours: no blends, no water glints, no ways' curves — the ground sheet has
  all of them painted and could be the albedo (a cell's rectangle of the sheet), the next step.
  Unmeasured on a GPU: no headless measure exists, and the island's machine has an Intel UHD
  620, where 1080p × 160 steps × several texel reads may well cost more than the 4 ms the CPU
  takes; a half-resolution offscreen image upscaled would quarter it. Compiles and traces right
  by its tests (the shader compiles, the heights round-trip, the occlusion hides what a ridge
  hides and not a hawk, the rays pass through what the screen points unproject to); to be
  looked at in the demo. **Looked at, the morning after**: the user's screenshot of G was a grey
  nothing in the rain. A screenshot harness (`examples/board-topography`, `TestShots`, run with
  `GRAM_SHOTS=dir` on a display) draws the demo's views into PNGs, and showed the tiles, the
  sky and the clouds right and the heightfield a flat black plane, fogged grey in rain. Two
  Kage rules the tests could not catch: `imageSrcNAt` takes a position in the *texture*, so a
  pixel of an image is `imageSrc0Origin()` plus the pixel, and every source image is read at a
  position in the *first* image's texture — `imageSrc1At(imageSrc0Origin() + p)`, never
  `imageSrc1Origin()`, which reads outside the region and gives transparent. The heights were
  read outside the image (all at Low: a plane), then the colours (black). The colours' image is
  the heightmap's size now. Under XWayland the harness's window sometimes never gets its
  VisibilityNotify and GLFW spins; a watchdog kills it after a minute. **The user then**: the
  GPU ground "was to be fast and is slow, under 40 FPS", and the clouds "too low" and "as if
  generated" as the head turns. (1) The march was full-resolution, 160 steps of a quarter cell
  at the finest, 4 texture reads a step, 16 for the normal, 24 shadow steps: on a UHD 620 that
  is the 30 FPS the tiles' perspective also gets. Now half-resolution into an offscreen image
  scaled up linearly (`Downscale`), the normal from the cell's own four corners (bilinear
  gradient, smooth within the cell), 16 shadow steps of ¾ cell, a march no finer than half a
  cell with six bisections after: roughly eight times less work; to be measured by the user's
  FPS counter. (2) The clouds swam because the sky mesh read the noise at 64-pixel corners and
  the shader blended it across each piece: turn the head and the corners land elsewhere on the
  layer, so every piece's blend changes shape. Now the shader looks along each pixel's own line
  of sight to the layer — the camera's RayField as uniforms, the pixel's screen position in the
  overlay's custom — and evaluates the noise there; the ground's cloud shadows evaluate it per
  pixel too. That needed one noise on both sides: the CPU's 64-bit hash cannot be written in
  Kage, so both use value noise over a permutation polynomial mod 289 (`permute(permute(x)+y)`),
  whole numbers under 2²⁴ that floats hold exactly, the lattice shifted off the origin where the
  polynomial is small. The CPU still reads a piece's corners to skip pieces the clouds miss.
  (3) The layer at 3 km looked low from an eye a kilometre up, cloud features 1.3 km wide
  subtending tens of degrees; 6 km now.
- **Twenty-first round: the tiles and the cones on every CPU.** The user asked what of the
  topography's drawing could go to the GPU, or failing that be spread over the CPUs, and ordered
  the whole plan in stages. Ebitengine's Kage is fragment shaders only, so the tiles stay on the
  CPU; what is parallel is composing them. (1) **The renderer, not the dresser, shares the work
  out**, through two contracts: `look.Parallel` (Warm, Ready, Worker) on the Dressing and
  `look.ParallelLook` (Worker) on the Look — both, or the tiles stay on one goroutine, since a
  look such as a test's records what it sees. Each worker draws into its own `render.Frame`,
  appended in order, so the composer's stable sort sees what one goroutine would have handed
  it: piece for piece the same picture (tested vertex for vertex, from above, isometric, a part
  of the island with shadows cast from off the screen, and in perspective). (2) **What a
  worker may write.** The dresser's caches are keyed by cell or by corner. A cell's light, its
  shadows and its bake belong to the one worker drawing the cell, so they are worked out in the
  workers; a corner's shore is met by four tiles, so the shores are warmed on the frame's
  goroutine (cached across frames, so cheap); a corner's cloud noise is per frame, so every
  worker keeps clouds of its own and the corners along a run's edge are worked out twice.
  `topOf` reads a cell anew through the ECS when the board changed — a goke Seek, not for
  several goroutines — so `Ready` reads every top a worker may read (the camera's bounds, plus
  a shadow's reach and a few cells) and the workers are *frozen*: their `topOf` is a plain read,
  a missed cell a stale top rather than a race. `measureHighest` is done in Ready too. (3) **The
  first cut moved too little.** Warming light, shadows, bakes and clouds on the frame's
  goroutine left the parallel part small: a moving sun recomputes every shadow every frame, and
  the cloud noise is 4 octaves a corner. Measured at 10% gains, and a loss on the far view.
  With them in the workers: from above 4.3 → 2.6 ms, isometric 2.7 → 1.8, perspective through
  1080p 6.8 → 3.9, the far views 2.5/2.7 → 1.6/1.7 (8 threads). What stays serial: `eachVisible`
  with `onScreen` culling, the ground sheet's painting, the gather pass, and `Frame.Append`
  copying every vertex once more — the last is stage 3's question. (4) **Vision.** A scanner per
  goroutine (view, lookup query with its own component handles, covering with its closures),
  phase A the scans, phase B the behaviors in order as before. Two lazy readers had to be
  settled first: aabbworld's grid reindexes on the first Query when stale (one query on the
  frame's goroutine), and the board's cover read each cell's kind through a goke Seek as a ray
  met it — `Board.Ready` (the `Readied` contract) now reads every cell's veil into a table when
  the board changed, and `Walk` reads the table. Chunks: an archetype with `SightOutline` holds
  a few entities a chunk (the struct is big), so parallelising within a chunk gained nothing
  with outlines; the runs cut across chunks now (`job` per chunk, a global index). 500
  observers: 0.54 → 0.26 ms, with outlines 1.31 → 0.49. (5) **Found on the way**: the bench
  harness never replayed the world's clock, so the vision benchmark had measured an empty tick
  (300 ns) since the clock came; `headless.start` replays it after the plan now. `trace` zeroed
  the whole shadow buffer per observer; it clears the samples read. (6) **Stage 3, the handing,
  measured and mostly left.** `Benchmark_Composer_Render` stubs the draw: the composer's own
  share — copying each item's vertices into the call's buffer, indexing, the white texel of
  plain colours — is 0.35 ms for 18k pieces (the whole island from above is 15.7k, 63k
  vertices), 0.2 ms for the perspective through 1080p (8.7k), 0.03 ms isometric through 1080p
  (1.5k). A frame handed over without copying would need `DrawTrianglesShader32` with the
  frame's vertices whole per call — and Ebitengine converts every vertex it is handed into its
  own buffer per call (`ensureTmpVertices`), so every sheet switch along the depth order would
  pay for the whole frame again. The depth order scatters the sheets (tiles, then a unit's
  sheet, then tiles) so calls cannot be one per sheet either. Kept: a run of quads appended at
  once (9%). The 4.3 ms measured earlier in the demo as "handing" is Ebitengine's own per-vertex
  work and the driver's, which only fewer vertices cut — the ground sheet's way.
- **Twentieth round: one Eye for the cone and the rider.** The user, in first person: the cones
  are too short, their shadows fall wrong, the eye seemed too low and the width did not match the
  lens. Measured: the cone's eye stood at 18 m (`Sight.Eye`) and the camera's at 20 m (the top);
  the cone spanned 72° and the camera 57.8° across a 1024×768 screen (45° up and down, the width
  following); the cone reached 687 m where the camera shows kilometres, and from an eye 20 m up
  the first 48 m lie under the screen's bottom edge. The shadows themselves were right: a sample
  is hidden when the line from the eye to it dips under a nearer one. Chosen with the user: one
  `world.Eye{Height, Angle}` on the entity, read by the cone (its width and its eye) and by the
  camera riding in it, which now fills the screen's width with the Eye's angle and lets the
  height follow the screen's shape — no distortion, no cropping, the same picture on any screen
  across; the demo's units see 72°, the user's own screen. `MaxSightRadius` 600 as the user
  preferred over 1000 (the outline is a saved component, 2.5 KB against 4). The island's radius
  3 km with the ground sampled every 50 m: about 2000 samples per observer and tick, the whole
  map (9.6 km) would be 7000 and want a scan every few ticks. The renderer drapes the shadows in
  the scan's step, not the relief's cell, so the pieces are as fine as the bands.
- **Nineteenth round: routes as lines, goals as outlines.** The user: the route drawing is ugly,
  in first person it wobbles on straight stretches, the goal should be the entity's outline at
  the point it goes to, routes on Shift+P, goals always, cones hidden by default, rivers half as
  fast. The wobble was the arrow sprites: a raster quad per cell rests on the tile's corners and
  its texture is interpolated affinely, so in perspective a straight line in the raster bends
  (the known limit of the tiles). Lines have no texture: the route is `Frame.Line` pieces between
  ground points every `Heights.Step`, each at the ground's depth, so a hill hides them and every
  camera draws them straight. The goal is `world.Look.Footprint` of the entity's box round its
  spot, on the ground there (`Heights.At`), not on the tile's top as the sprites lay — a unit
  stands on the ground inside a forest too. Gone: `PathSprites`, `Direction` and its atlas
  baking; `WithRenderer` keeps its atlas parameter for the plugin contract and ignores it, as
  vision does. Chosen alone: goals for the selected units only, as RTS markers go; a line 1.5 px
  wide, the outline 2 px like the selection's; `Flow` halved in the island's styles, the engine
  untouched, the speed being the game's.
- **Eighteenth round: cells kept reactively.** The user: fitting in a cell and passing each
  other are orthogonal; cell-sized units should plan blind, correct only on collision and give
  way, like the bodies. Under cells a unit stops at its centre before it can touch anyone, so
  the refused reservation plays the collision: `reserveLeg` says which cell and whether a corner,
  `keeping.blocked` waits, names the holder (`Occupancy.Holder`; entity id 0 is a real unit, so
  `member.pressed` is a flag, not a zero test) and matures a stall into `Bumped`+`Held`; the
  standing are asked through `wanted`/`wanting` maps swapped each tick, one tick's latency
  whatever order the chunks come in. Found on the way: a lingering yielder kept its step's cells
  (the leg never released while `Linger` ran), so the one it gave way to could not pass; a held
  corner of a slantwise step asked its holder off a cell the step never enters — now learnt and
  gone round square; two units both re-routing round each other took the same side every time
  and danced — the greater id goes round, the other waits; a goal someone passes over must not
  be settled beside (the passer's leg holds it for a moment). Test worlds building
  `newNavigationSystem` straight keep a finder with the real occupancy, so their routes stay
  predictive; the plugin's are blind, and the road world (`bump_test`) and the new
  `cell_spacing_test` go through the plugin. The turnaround sweep passed vacuously for a while
  (blue gave up before the sweep began) — traced tick by tick, not trusted.
- **Seventeenth round: a right drag turns the units.** The user: with the right button held,
  moving the mouse turns the selected units to look that way. Chosen with them: each unit looks
  at the ground under the cursor (the `LookAt` command, issued at every move of a `ButtonHeld`
  past a slop of 4 px), a click moves as before and a drag moves nothing on its release — so
  `MoveTo` went from `ButtonPress` to `Drag`, the button up within the slop, as Select is on the
  left button. Not chosen: one heading for all from the press point to the cursor, and Company of
  Heroes' way, the press point the goal and the drag the facing on arrival. The Shift+S right
  click went, at the user's word: the drag does what it did.
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
  board (`WithBoard`: the board's Map and the world's Look). The ground: `ground.Heights` (the name
  `Ground` was the cell component's), `ground.Cover`, `collision.Field` set by
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

### Packages (2026-09-30)

The user: time to group and clean the plugins — the moon, the stars, their images and data apart
(they proposed `ecliptic`), and a proposal for the topography, "a huge set of many elements".
Done in eight steps, each built, vetted, tested and shot against the tree before it.

- **`celestial`, not `ecliptic`.** The ecliptic is only the sun's path; the package holds the
  whole celestial sphere — the sun's path, the moon's orbit 5.14° off the ecliptic, the stars all
  over the sphere, the sidereal time. Renaming is one word if the user prefers `ecliptic`.
- **`sky` keeps the light, `celestial` the bodies.** `sky` imports `celestial` (the sun's path for
  `SunAt`, the moon and its phase for `LightAt`), never the other way round; the observer is a
  `celestial.Place{Latitude, NoonWay}` that `sky.Config` makes from its zone. The sun's colour for
  the disc is the day's light (`sky.SunColorAt`), not the sphere's, so it stayed in `sky`. The
  backdrop in the atmosphere's root stays the sky's composer: it takes `celestial.StarField`,
  `MoonFace` and `Heavens`.
- **The atmosphere's root draws nothing.** The user asked why shaders sat in the atmosphere's
  root: they were the root's own two renderers, the backdrop and the flat world's clouds'
  shadows, left there at first as "the sky's composer". Against the topography's split that was
  inconsistent, so they are `atmosphere/backdrop` and `atmosphere/overcast` now, named as
  `precipitation` is (`Renderer`, `New`). The backdrop read the whole `atmosphere.Running`; it
  takes `WithShown(func() (stars, moon bool))` instead, which the plugin feeds from `Running`.
  The overcast was not put into `air`: `air` is the weather and its materials, not a layer drawn
  on a board.
- **The topography's parts never import the plugin.** `relief`, `painter`, `water`, `terrain`,
  `hexes`, `billboards` and `cameras` take what they need as small interfaces or explicit values
  (`painter.Sky`, `billboards.Sky`, `hexes.Sky`, the relief, the board's Map); the plugin composes
  them, registers their systems (`Relief.HeightsSystem`, `Relief.AltitudeSystem`,
  `Shaper.System`, `Control.System`), gathers their queues and keys, and hands them one `liveSky`.
  `go list -deps` of every part shows no import of the root.
- **No aliases.** Types are named by their package (`relief.Relief`, `relief.New`,
  `painter.Style`, `cameras.View`, `water.Shore`), as the round on aliases decided: aliases only
  for the components every entity carries. Games, demos, the bench and the tests say the new names.
- **The billboards ask the cameras through contracts.** They read `viewCamera`'s and
  `perspCamera`'s insides before; now `camera.Rider.FirstPerson`, `camera.Eyed.Eye` and
  `Projection().Sorts()` (what `viewCamera.relief()` was). `billboards` takes the terrain's
  `*terrain.Renderer` as it is (nil over hex prisms, where the shadows go over the frame's depth).
  The sprites' `heights` switch is gone: the plugin always passed true.
- **`hexes` takes the world, the board, the board's Map (the plugin), the relief and a Sky**
  instead of `*Plugin`; `fromAbove` wraps the Map, not the plugin.
- **Tests went with their code.** Internal tests into their packages; the billboards' tests that
  need the whole plugin into `billboards_test`, which may import the root, with a `Stood` hook in
  the package's `export_test.go`; the root's `export_test.go` is gone. The root's shader-compile
  test is gone too: the painter's compiles the same composer shader with the water's materials.
- **Import groups.** The moves had put module imports above the standard library in many files,
  some from earlier rounds; they are grouped again (standard library, then the rest).
- **Two GPU bugs found on the way** (terrain tests failing together, not alone): gogpu does not
  zero a new texture, so a partial first draw showed another texture's old tiles — `render/gpu`
  zeroes every texture as it is realized; and gogpu frees a resource at once, while a submission
  may still read it — releases are retired and freed after the next submission, the GPU waited
  for. A test (`TestNewTexture_IsTransparent`) holds the first.
- **Saves.** `Heights` is `relief.Heights` now, and goke names a saved component by its type, so a
  game saved on this branch before the split does not load (CHANGELOG).
- **Shots.** The island from above, isometric, and in perspective by day and night came out the
  same after every step, the hex demo from above and in relief drew as before, and the sky (the
  sun under clouds, the pole's stars, the moon) and the flat world's clouds' shadows came out the
  same after the atmosphere's move; the only differences were the water's animation and the
  frame time on the HUD.

### Units belong to players (2026-09-30)

The user's order after the packages: "tag grupujący" — units grouped under a player — before
yielding and avoiding.

- **The owner lives in `players`, in a leaf.** A plan put the family in `plugin`; the user: the
  owner belongs where the player is introduced. `plugins/players/owner` imports only `plugin` and
  `control`, as `world/entity` does for the world's components, so selection, navigation and the
  cameras read it without the players plugin (which pulls `game`, the shortcuts and the carrier —
  the layers would turn over). `players.NewPlugin` registers the family's 64 names with the
  world's kinds, so a player added later is saved too.
- **Own units alone; the ownerless are the virtual player's.** A plan let a player command what
  nobody owns; the user: every player selects its own units alone, the ownerless belong to a
  virtual player, with an AI or without. That player is `control.Nobody`: an ownerless unit obeys
  only commands no player gave, and Nobody commands no player's unit. A named neutral side is a
  player without a keyboard owning its units. Existing tests on ownerless units, issuing as Nobody,
  pass unchanged.
- **One `Selected` tag for everyone.** Each unit obeys one side (or its co-owners), so a player's
  selection is Selected ∧ Obeys(player), and a non-additive Select unselects only the issuer's —
  no per-player selection bits.
- **Each system filters in its own plugin**, an optional `plugin.Tags[owner.Family]` in the query
  it already runs and the issuing player passed down: `SelectionSystem.applySelection`,
  `FollowSystem.toggle`, `moveCommandSystem.selectedMembers`, `cameraSystem.theSelected`.
  `Drive` and `Look` follow the camera already fastened, which only its owner's command fastened.
- **The demos.** Every demo whose player selects units now gives them to that player; the
  island's shots test and probes issue as the player. The island has a rival (`players.Add`)
  owning blue walkers at every other stop — the foreign units of the next stage. The split-screen
  demo's `Driver{Player}` gave way to the owner tag: the established pattern, not a demo's own.
- **Left for later:** the selection outline and `vision.ShowViewOf(Selected)` still show every
  player's selected units in every viewport — on one screen with two selecting players each would
  see the other's; the renderers do not know which player a viewport's camera is.

### Conduct and courtesy (2026-09-30)

The user's third stage: units that talk — "move aside, you block my goal", "I'll stand on
yours" — allies asked, strangers gone round, without deadlocks. After two rounds of design the
user chose behavior trees whose state is component blocks, in a package of the world with
ready-made behaviors, the named-branch notation, and two layers (behaviors stay reflexes).

- **`plugins/world/conduct`**: made and run by the world like the effects, after the behaviors in
  every step, on the world's clock (`Mind.Since` holds clock times, saved). A reactive `First`
  re-runs from the top every tick, `Then` remembers its step; nodes that ran and are not running
  now are halted, so a lost branch's action comes off at once. `On` latches for one-tick facts.
  `Parallel` and `Limit` from the plan were left out: nothing needs them yet.
- **Trees registered globally by the root's name hashed** (a `Mind` keeps the hash; a save knows
  the name, not a pointer), laid out afresh for each world, so their steps bind that world's goke
  columns. Two different trees under one name panic.
- **Fields exported, arrays fixed**: goke refuses unexported fields and reads only fixed sizes.
  goke registered at most 128 component types (before goke 3.3.0; 512 since); every fact, action, `Asked[W]` and `Replied[W]` is
  one — worth watching as trees grow.
- **Conversation**: delivered a tick later through the command buffer (no recursion, no chunk
  order); asks reach only an entity with a mind (goke errors adding to a gone entity); a relay
  keeps its ask until it answers itself, passes refusals back and turns a yes further on into a
  "wait" for the asker, who then waits up to `AskLife`.
- **Entity 0 is an entity.** `uid.UID64` has no nil; the first conversation code used 0 as
  nobody and a unit with id 0 could not be asked. Subjects are `(id, ok)`, `Room.Beside` and
  `blocked`'s `known` say presence. The old `MoveOrder.Hit == 0` meant the ground and also struck
  entity 0: `HitUnit` now tells them apart, and bump no longer looks up entity 0 for the ground.
- **Navigation for a conducted unit**: it keeps the aside reflex and the stall safety, but not its
  own decisions (asking holders off, learning cells, `placeAgain`, the press-based `yield`); a
  refused step falls back to navigation's own going round after `conductGrace` if the tree never
  moves it. `Blocked` holds `blockedHold` past the last contact, so a body standing still while it
  waits keeps its fact; `WaitFor` probes `mayStep` and re-notes the blocker while held.
- **Deadlock rules**, each found by a test: two groupmates asked each other to swap at once — the
  one that waits first no longer asks; a detour that must still pass the blocker's cell fails, and
  the unit steps aside (a corridor's passing place) instead of rerouting into it each tick; a unit
  giving way itself only waits or goes round, else its step aside nested into others' and allies
  in a crowd never got home. The corridor scene takes about 6.5 s: the one stepping aside comes
  back once before the other passes.
- **Bodies are nudged while they talk**: under BodySpacing the ask takes a few ticks, and the
  collision pushes the one struck a couple of units meanwhile; the test allows its side.
- **Not measured**: the island's walkers meet on long crossings; a quick probe cannot run them
  there. Watch it in the demo.

### One behaviour: triggers, trees, effects, commands (2026-09-30)

Reading the conduct stage, the user found three mechanisms saying one thing: a `plugin.Behavior`
was a trigger that mostly cast an effect or did something, an effect was state, and a tree
decided. They asked for one vocabulary in `conduct`, with every "do" a command queued as a
player's is. The decisions were theirs: `conduct.Trigger` with optional filters (no `plugin.Any`
in calls), pairs kept; the hosts' method `Hook`; triggers take instant nodes alone; memory
through an effect; `Issue` fire and forget, the tree waiting on facts; effects under conduct;
`kind` and the tags under `world/entity`.

- **Triggers reuse the tree's steps** in an instant pass (`ctx.instant`): a trigger's `fired`
  holds the moment by pointer and a scratch `Mind`, so a firing allocates nothing. The hosts are
  still `plugin/host`'s, erased (`PairOf`, `EachWith`, `ListHost`); the bench of collision, vision
  and the world tick showed no regression (a single-run baseline, the new runs 0–20% faster —
  noise, not a gain).
- **Filters decide the host**: `Having` or `RunOn` → `EachWith` over that component; a `Self`
  alone on a moment that is not a pair reads the entity's tags as state; `Self`/`Other` or a
  `Met` moment → a pair; none → every entity. Two triggers over one component, or a host that
  reads it itself (the board's `Mover` for `Standing.Domain`), panicked goke with a column added
  twice: the host's columns are shared by type now (`host.Own`).
- **The schedule is a trigger of the clock**: `clock.Moment{Last, Now}` every step, `clock.At` and
  `clock.Every` its conditions, fired by the effects' pass where the schedule ran. A moment of no
  entity: a node acting on an entity fails on it — the test caught a cast landing on entity 0,
  the clock's own.
- **Commands from entities**: `control.Issued` names its entity (`ByEntity`), the world keeps a
  `control.Carrier` the engine fills with every `CommandHandler` used — before the world too —
  and hosts hand it to triggers in `plugin.Tick`. An unknown command panics: a game's mistake. The
  players keep their own carrier for now.
- **Fire and forget changes the trees**: no action succeeds or fails; navigation tells outcomes as
  facts (`Blocked.Cornered` after a `Detour` with no way round, `Blocked.WaitedOut` after a
  `Hold`, `Arrived` once an order is over, until the next — a state, so `Until` sees it at any
  tempo). A reactive branch that issues and succeeds would issue again every tick: it is
  followed by `Until` or `Idle`, and the reaction to the outcome sits earlier in the `First`.
  `Until` counts a fact come afresh only, or a patrol's second leg would take the first leg's
  `Arrived`. The eight courtesy scenarios pass unchanged; a patrol test orders a unit alone while
  another of its player's stays selected.
- **`MaxNodes` 128**: `Courteous` grew to 73 nodes with the `Issue`/`Until` pairs; `Mind.Running`
  is a two-word bit set (`Nodes`) and a `Mind` about 1.2 kB.
- **Left for later**: collision's `HitMark` is a timer that could be an effect; the players'
  carrier and the world's could become one; a command issued in the step a game is saved is lost
  (drained in the same frame in practice).

### act: builders with methods, the hit an effect, one carrier (2026-09-30)

The user read `WhenBlocked` and asked for a builder: a variable `c` whose methods make the nodes,
`When` a constructor, the package named `act` (chosen over `action`, which in behaviour trees is a
leaf and was just what commands replaced), and the three loose ends done with it.

- **Builders**: `act.When[F]`, `act.On[F]`, `act.Named` make a `Branch`; `act.Trigger[P]` a
  `Reaction[P]`; `Do` closes them. Go 1.27's methods with type parameters carry `c.Issue(cmd)`,
  `c.Ask[W](…)`, `c.Until[F]()` and `Command.Until[F]()`, also on the generic `Reaction[P]`
  (checked before the move). Only the root is named, so the `""` of every inner `First` and `Then`
  went; `Issue(x).Until(p)` and `.Stay()` replace `Then("", Issue(x), Until(p))`; branches are
  values (`hold`, `goRound`), each use laid out on its own. The old package functions are gone —
  one way to write a node.
- **Instant by type**: a Reaction's methods hand out `Instant` nodes (a wrapper marking them), so a
  lasting node in a trigger no longer compiles; the runtime check stays as an internal guard, and
  its test went, the compiler being the test now.
- **The hit an effect**: `trigger.Hit(fx, d)`, `ShowHits(hit)` on a `Struck` that `Hit()`s,
  `HitOverlay(hit, with)` through `IfUnder`. It lasts in game time now (the mark counted wall
  time) and any collider may carry it; a kind opts out by narrowing the trigger.
- **One carrier**: the players give their commands to the world's carrier. Writing it turned up a
  bug from the step before: `players.RunPlan` emptied every queue at the end of a frame, the
  world's `Despawn` among them, and a board trigger's `Despawn` comes after the world's pass — so
  a drowned unit never went. Nothing clears a queue now; a command waits for its handler's pass,
  given after it for the next frame's. Two engine tests: a trigger's late `Despawn` is carried out
  (it fails with the old clearing), and a tree's commands leave no queue holding one at the end of
  a frame, so a save between frames loses none a tree gave. A trigger's late command can still be
  in a queue at a save; triggers fire again on the state that caused it, so it is given again.
- Names: the world's `conduct` field is `trees` (`act.Trees`), navigation's `conducted` is
  `minded`, `conductGrace` `treeGrace`; `doc/conduct.md` is `doc/act.md`.

### Markers, and goke's save order (2026-09-30)

The hit as an effect cost the collision demo ~7.6 ms a frame: effects put `Active` on at the first
cast, took it off at the last end and put `Idle` on for a step — each a move of the entity in
memory. The user asked for a valuation, then for markers as a part of the kind: states switched by
a bit of a family carried for good. Tags stay groups (a built-in tag column in every goke archetype
was weighed and rejected: tags would lose their worth to queries).

- `Benchmark_Marker_*`: a component put on and off costs ~160–290 ns an entity, a bit 1–2 ns;
  finding the marked by the bit ~1 ns an entity of the family.
- Effects: `Active` stays, empty when idle (a prototype: 28.9 → 26.0 ms); `Idle` a marker; the
  `touched` map is reused instead of made for every entity every step. Navigation: `Entered`.
  Collision: the hit grants a marker, the overlay reads it.
- `world.Outside` stayed a component: rare, and the exit system walks the outside alone.
- Writing it turned up a goke bug: Save wrote an archetype's values in its own order and listed its
  components by type number; Load read in the list's order. A type registered early and put on
  later swapped values with another (a two-component example in goke showed A and B exchanged). The
  user: base both on the archetype's order — goke's directory now lists it (3.2.4, unreleased; gram
  tested against the local goke through a go.work outside the repo).
- `TestBodySpacing_CrowdsStandRoundThePointWithoutPushing`: 25 units from the north-east strike
  2082 ticks against a bound of 2000. Navigation is unchanged; the order the units are walked in
  changed (they no longer move between archetypes each step), and the crowd's contacts swing with
  it — with markers from spawn another case, a column, reached 2364. Settled by the crowd round
  below: the test measures the longest touch of a pair.

### The crowd as StarCraft II's rules, written in act (2026-09-30)

The goal outlines overlapped when a player kept changing a group's target: one standing, bumped,
got a step aside and home again, and the step aside was drawn as a goal. The user judged the
eight ways of giving way (the `Courteous` tree, asks, relays, swaps, step aside and back, and
navigation's own reflexes) overcomplicated, chose StarCraft II's few rules, the old ones removed
for good ("to defaulty, nie ma sensu trzymać dwóch"), on one condition — no unit pushed where it
falls or into a wall — and then that the rules be written in act: a plugin perceives and carries
out, act says what to do when; gram is a library of layers (the rule is in CLAUDE.md, "Behaviour
goes through act").

- Navigation hosts the moment `Touch` (a pair: two units touching — boxes met, or a step refused),
  with what each is doing, and carries out the commands `StepAside`, `Detour`, `Pass`, `Hold`,
  `Settle`, `Stop`; the engine's rules live in the handlers (`open`/`aside`: ground that takes the
  unit, no steeper than `yieldClimb`, nobody there). `Crowd` = `MakeWay`, `JoinTheGroup`,
  `GoRound`. `LastOrder` tells which group a standing unit came to the end of.
- Found on the way and settled as rules or perception, not branches: collision records a contact
  on the one that struck only (each pair is felt once and handed to both); head on under cells had
  to be seen from this tick's refusals; a mover going round an ally that steps aside met it on the
  same side (it now goes on past it, `Pass`, and waits under cells, where the step waits anyway);
  an ally with no room was pushed along a lane (the Touch now says `Room`); a unit stepping aside
  was gone round under cells, re-routing into it again (cells do not route round one giving way);
  in a corridor with a passing place the cornered one steps aside a while (`NoWayRound`).
- The crowd test now measures the longest touch of a pair (under `stallAfter`; 57 ticks at worst)
  instead of the sum of contacts, which swung with the storage order.
- The demo probe (20 plateau units, clicks every 15–300 frames) counts no overlapping outlines
  (before: up to 2 pairs).
- The collision's push of two boxes (an even split) looked at no ground: with no rules a pushed
  unit was shoved into water; with `Crowd` one with no room at the water's edge was nudged ~6 px,
  its box 5 px over the water. The user chose the guard in collision (2026-10-01): a push apart
  never puts a unit further over ground that does not take it (`Field.Overhang`); the side held
  bounces as off the ground, the other goes the whole way. The engine (aabbworld) asks the
  handler only on a pair's first pass, so the box written back is checked too. A unit whose goal
  such a one stands on stands beside it (`Touch.GoalTaken`). The navigation test field now wires
  the board into collision, as the demos do.

### rule: the names (2026-10-01)

The user found the effect demo's rules hard to read and "act" a poor name, and that
`plugin.Trigger` and `act.Trigger` were two things of one name. Talked through, without code, to:

- the package is `plugins/world/rule`; what is done at a moment is a **rule** (`plugin.Rule`, which
  `Hook` takes), what an entity does over time a **plan** (the trees);
- two constructors, each taking a function that writes the steps for a builder — the user's idea
  to drop `Do` for a function, and to give the function's parameter a type of its own per
  context, so good names need not clash: `rule.On(name, filter, func(m *rule.Moment[P])
  rule.Step)` and `rule.Plan(name, func(a *rule.Actor) rule.Step)`; the Actor's `When[F]` and
  `On[F]` open branches on a fact, and `If` is the condition everywhere;
- the filter is the second argument, so whom a rule concerns is read before its body (Go allows an
  optional argument only last: `rule.All` stands for none); `Self`, `Between` (one filter for a
  pair, in place of `Self`+`Other`), `Having` (kept, as `CallOn` needs it);
- steps renamed: `OneOf` (First), `Steps` (Then), `Not` (Invert), `Keep` (While), `Under`
  (IfUnder), `Order` (Issue), `ForOther` (ToOther), `Call`/`CallOn` (Run/RunOn); `Runs`/`RunsOn`,
  `Named`, `Do` and `act.Tree` gone — one way to write each thing; `Node`/`Instant` are one
  `Step`, a lasting step in a rule refused as it is made; `Mind.Tree` is `Mind.Plan`, `MaxNodes`
  `MaxSteps`;
- a library shows a game only what it needs: navigation's crowd rules are unexported (CLAUDE.md
  says so now; the earlier "ready parts" sentence was mine and wrong).

The packages `collision/trigger` and `vision/trigger` are `collision/hooks` and `vision/hooks`
(the user's name): they hold ready-made rules, whole, for a plugin's Hook —
`collision.Hook(chooks.CountContacts(&stats), chooks.ShowHits(hit))`, `vhooks.Chase(tags, every)`,
`vhooks.NewFlee(tags).Rule()` — where they held steps a game had to wrap in `rule.On` itself.
Saves made before this do not load: `rule.Mind` is a new type path.

### Effects, step 1: a game written as states (2026-10-01)

Talking through an imagined fire that touches many plugins, the user set the direction: a game's
states are effects, its rules connect the plugins' moments to them, the plugins give knobs that
effects alter, a plugin's own effects stay private. `Call` turned out a trap that hid what was
missing; it goes in step 2. Step 1, done:

- every effect has its own marker, "effect.<name>" of `effect.States`, granted by `Define` before
  the Spec's traits (`Effect.Mark()`); the user preferred this to bundling rules with an effect,
  which would keep another plugin from adding its own rules for fire. Collision's hit and the
  effect demo's `frozen` lost their own tag families.
- `Dispel` as a step; a cast after a `Dispel` in the same step takes the slot back (the slot
  remembers it was dispelled, so `Then` is not cast). Agreed: a rule keeping an effect has it back
  the step after someone's `Dispel`; a shield checked with `Unless` lets the dispeller win; a
  plan's `Keep` gives way. The plan's `Keep` notes when it first saw its effect on (`keepCast`,
  `keepOn`), since a cast on an entity without `Active` lands only after the next sync.
- `effect.Then(next)`: the next effect queued in the same pass, beginning the step after.
- `Chance(p, step)`: a hash of the world's seed, the game time, the entity and the step's place;
  `plugin.Tick` carries `Time` and `Seed`, every host takes its tick from the world
  (`world.Plugin.Tick`, `plugin.TickSource`), and the world's renderer gives its Drawing rules the
  last step's time.
- the clock's moments are fired by the world, in their own system before the effects' pass.
- one marker, `effect.Changed`, in place of `Active.Altered`, `effect.Idle`, the `Idling` moment
  and `host.EachHost.RunRows`: the user asked to keep to the established pattern.
- a slot waiting for missing tag families attaches them all at once (it attached one a step).
- At most 63 effects (one bit each beside `Changed`); the 64th panics naming it.

Fire does not yet spread over the ground: it needs a step turning to the cell a unit stands on and
a board moment of a cell with its neighbours — for step 2's audit of moments and knobs.

Committed as 58b5003.

### Step 2: no Call, knobs apart, cells in rules (2026-10-01)

The user chose: knobs in components of their own (as `board.Ground`), ready-made log hooks in
place of the demos' logs, the names `Here`/`Around`, and the board's cell moment now. Done:

- `Call` and `CallOn` are gone from `rule`; `Having` stays, a filter alone. Every use found its
  place: a plugin's own hooks and the ready-made ones on `plugin/host` (`host.Each`, `Every`,
  `Pair`), game behaviour in steps, and what was missing added.
- What was missing, found on the way: the steps `Here`/`Around` (`rule.Placed`, the board's
  `Standing` and new cell moment); `steering.Steering.Halted`; `vision.Sight.Ahead`; and the clock's
  moment as its own entity's (`clock.Moment.Clock`) — rewriting the moments' test showed a clock
  rule could cast a phase only through `Call`.
- Knobs apart: `steering.Steering` the knobs, `steering.Course` the state, `steering.Helm` pairing
  them for the requests (navigation, vision's hooks and the split-screen demo ask through it);
  `vision.Sight` the knobs, `vision.Sighted` what the scan found. `Course` is a unit's default in
  the roster and given by the steering where missing; `Sighted` only given by vision at the first
  scan, so units without sight do not carry its ~100 bytes. Tests building raw kinds add them.
- The cell moment was first `board.Lying` (`look.Tile` was taken); it is `board.Cell` now, the
  unit's cell component renamed `unit.At` to free the name (below).
- `Around(n)` takes in the places stood on (ring 0); a spreading rule keeps off what already
  burns with `Unless`, or a cell keeps itself burning (doc/rule.md says so).
- `LogFalls` logs once an entity, `LogSightings` once a pair (each keeps a map).
- The navigation demo's H, opening holes under the units, went the same day to a demo of its own
  (below): finding them through the occupancy also caught the cells units were only stepping into.
- Entity 0 is a valid id (the clock is often it); a guard I added against it was wrong and is out.
- The host tests (collision, board, world) now probe with `host.Pair/Each/Every`; the rule
  filters are tested through the world's rule tests with effects, which need no `Call`.

### A player acts by effects (2026-10-01)

Rewriting the trapdoors showed a player could not change the game's state but through Go code.
The user set it apart: some effects a player puts on its own units, others on the board — the
trapdoors take whoever stands on them — and asked for a demo of its own on a flat board, the
trapdoors and the key defined in the game. Done: `selection.Apply{Effect}` (own selected units),
`world.Apply{Effect}` (the world's own entity, the clock's; drained by the moments' system before
the clock's rules), the step `During(e, step)` for rules and plans (`plugin.Tick.World`,
`rule.New(now, world, …)`), and `examples/trapdoor-demo`: a strip of trapdoor cells a cell rule
keeps open `During` the lever, wanderers with patrol plans, the player's scouts hastened by J.
The user then asked how the effect knows its trapdoors, and why the rule's filter was
`rule.All`: a cell's kind is a value, not a tag, so no filter could narrow to it. Now cells carry
the game's tags of places (`board.Places`, given in the `Layout`), the rule is
`rule.Self(trapdoor)`, and `open` turns the ground into a real pit. Many levers: a lever an effect
each, its trapdoors a tag each, a rule a pair, made by a function in a loop (1 and 2 in the demo);
tens fit the limits (63 effects, 64 tags a family), hundreds would want levers as entities. A unit
pulling a lever where it stands — a pressure plate — is a demo of its own. I first had the cell
moment count who stands on each cell (`Lying.On`, `Trodden`), a second record of "who is where"
beside the occupancy; the user caught it, having asked earlier whether to extend the occupancy.
Looking again: the occupancy is navigation's bookings (the cell stood on and the one stepped into),
and it kept the holds of despawned units for good — four fallen wanderers left eight cells held,
blocking navigation. So: `Occupancy.Release(gone)`, called by the board's standing pass every step;
the plate goes through the unit's own moment (`Standing.Places`, the tags of the cell under it), no
count of the cells; the cell moment is `board.Cell` (the unit's component renamed `unit.At`,
`gopls rename`), data alone — the user objected to the moments carrying a pointer to the board,
so the board's neighbourhood comes in `plugin.Tick.Around` and `rule.Placed` is a marker.

The user found the wanderers unreadable: a kind and a plan a row, the route written into the
plan. Data and behaviour apart now: `navigation.Patrol(pause, cells...)` (StarCraft II's patrol),
a `Round` kept beside the order's goals, which the crowd's commands do not touch; an order with a
round never ends — reached or given up, it goes on to the next goal — and stands its pause (a
second in the demos, the user's wish) on each goal reached. One wanderer kind, its round loaded
per unit. Any effect may be applied today; a list of a player's effects waits for a game over a
network. The user also asked that a rule's short condition be written inside the rule, not as a
method of the stage; the effect demo's three were moved in.

### board/cell (2026-10-01)

The user asked for everything of a cell in a subpackage. Their choices: `cell` (singular), the
tags as `cell.Family` with `cell.Tag`/`cell.Tags` as other families are, `cell.Domain` and its
bits in `cell`, the moment of a cell in `cell` as `cell.Now`, the dictionary `cell.Kinds`
(`board.Plugin.CellKinds()`, not `Kinds()`, which would read as the world's kinds of units).
Done by a script over the qualified names (it also hit fields named `board`, `d.board.Way`,
put back), the compiler's "undefined" positions inside `board`, and `gopls rename` for local
variables named `cell` that shadowed the package (`cellAt` for the demos' helpers, `at` for
`goke.Comp[unit.At]`, `here` for the rest); `water`'s queue item type `cell` is `flooded`.

Then the user had the cell systems moved out of `board` too, with what they work on: the system
making the cells' entities is `cell.EntitySystem` (it was `cellSystem`), the board reading them
through the `cell.Store` it hands over (was `cellStore`); `cell.Terrain`, `cell.TerrainMap`,
`cell.Occupancy` with `SingleOccupancy` and `MultipleOccupancy`, the Layout's `cell.Entry` (was
`CellEntry`) and `cell.WayEntry` went with it. The dictionary's implementation stays in `board`.
The rules of a cell got a subpackage of their own at the user's word, `plugins/board/rule`
(`rule.CellSystem`, was `cellRuleSystem` in `cell_rules.go`, a file without the System suffix).
`cell` must not import `board`: the system takes the grid as a small `cell.Grid` (`CellCount`,
`Ordinal`, `EachCell`), the seed, and two functions of the board — `bind` (the store) and
`changed` (the cells an effect changed, counted once per step as before). The Layout's tags,
kept in `Board.places` till the cells were made, are the seed's now (`TerrainMap.Tags`), so the
seed alone says what every cell starts as. The standing pass stays in `board`: `Standing`, `At`
and `Mover` are the board's, and `board/rule` cannot import `board`. (The same day the cell systems and the rules went to `board/internal`, and `Standing`, `At`, `Mover` to `board/unit`: see the next section.)

### board in parts, API apart from internal (2026-10-01)

The user found the board's root a heap of parts and `board/rule` a package whose system showed no
rules while `terrainSpeed`, a rule, sat in the root; they asked for the parts in packages, an
inventory of what is still needed, and no wide API: what a game uses in normal packages, the
machinery in `plugins/board/internal` (the same for the other plugins later). Their choices:
`board/unit` for `At`, `Mover`, `Standing`; `board/grid`, `board/look`, `board/ground`.

- Public: `board` (Plugin, Board façade, Layout, Map, NewUnits), `cell`, `unit`, `grid`, `look`,
  `ground`. Internal: `terrain` (the cells' state, seed or entities, the entity system, the counts
  of changes, the kinds' dictionary), `rule` (`Rules`: the hosts, `StandingSystem()`,
  `CellSystem()`, `Around`, `TerrainSpeed`), `field` (cover and solid ground), `grids` (the square
  and hex types), `draw` (the simple map's bands), `occupancy` (the release system). Systems
  follow the topography's pattern: a part's method returns a private `goke.System`.
- `Board` first kept `Walk`, `Ready`, `Solid` and `Overhang` as hand-overs to the field, only
  because the board's tests read them. The user had the tests moved to the packages whose code
  they test, and that settled it: the field's own tests build it straight (`field.New` over
  `terrain.Cells`), the ones through the plugin take it from `Plugin.Cover()` as the
  `collision.Field` it also is, and `Board` lost the four. What the tests share — an installer, a
  world of world, collision, board and vision, a game of one stage — is `internal/boardtest`; the
  root keeps the tests of the plugin, `NewUnits`, the simple map and `Along`.
- `Board.Map` stays public: the renderer's tests (now `look_test`, outside `look`) draw a board by
  its simple map. `look.Renderer` reads the board through `look.Board`, public methods of `Board`
  alone (`Shape`, `Kind`, `Bare`, `CellVersion`, `Changes` and the grid); its insides are read by
  the tests through `export_test.go`, as `billboards` does. `grid.SquareShape` became `grid.Shape`,
  square or hex, since the grid lines need a hex's size too.
- `Tile.Sway` no longer asks whether the world has heights: nothing stands at a height in a flat
  world (a kind with a Height is refused there), and a tile of no height leans nowhere.
- Inventory: gone `Board.SetMany`, `TerrainMap.SetMany`, `Plugin.Top`, `Plugin.Slope`,
  `Grid.Contains` (and its test), `CellAABB` (tests' own helpers), `HexCapStrips`,
  `DefaultAtlas` (private), `extras_test.go` (it tested world's `WithEffect` with board props and
  no board); `board.Center` is `Position.Center()`; `NewBoard(grid)`. Kept on purpose:
  `TerrainMap` (navigation's tests use it as a `Terrain`), `NewBoard` (topography's and the
  renderer's tests), `WithWorkers`, `Parallel`/`ParallelLook`. Stale docs fixed: water's
  `board.MeanOfCells`/`board.Relief`, CLAUDE.md's `board.Effect`.
- The demos' row type `unit` clashed with the package: it is `unitRow`.

### topography in parts, and the three layers (2026-10-01)

The user had the topography split as the board was, and set the rule for every plugin while at
it: what a game constructs and drives a plugin with is in the plugin's own package; public
subpackages are for what other plugins need — the vocabulary a game and the plugins both name,
the contracts — and the rest is `internal`. Asked how the board fits, they chose to keep it as it
is: `cell`, `unit`, `grid` are such vocabulary, `look` and `ground` contracts.

- Go's import graph decides much: the plugin's package imports `internal`, so a type the
  machinery needs cannot live there. `relief.Climbing` and `painter.Style` are vocabulary, kept in
  thin public packages; the internal namesakes import them as `public`, and files that import both
  give the internal one an `i` prefix (`irelief`, `ipainter`, `icameras`).
- The commands moved to the root, as `navigation.MoveTo` lives in navigation's: the cameras'
  (`View`, `Turn`, …) and the shaping's (`Raise`, `Lower`, `Level`, `Shaping`). The camera system
  still drains them in its own order, through `icameras.Orders` — one method per command, the
  plugin's `cameraQueues` implementing it; the camera tests keep their own queues of mirror types
  behind a `queued` adapter. The shaping is the root's own system now (`shaping.go`).
- `Plugin.Relief()` was `*relief.Relief`, whose `HeightsSystem()`, `AltitudeSystem()`,
  `Lattice()` and the like were public; it is a small interface now (`At`, `Step`, `Altitude`,
  `SetHeights`). navigation's path renderer test, which built a relief, has heights of its own.
- `billboards`, `hexes`, `terrain`, `water` moved as they were: nothing outside used them.
- Tests: the altitude and heights tests went to `internal/relief`, the GPU ground's to
  `internal/terrain`, the cameras' isometric making and picking to `internal/cameras`, the
  Climbing test to the public `relief`; the root keeps the plugin's (flat or wrapping worlds
  refused, the view switched by its command, the keys, the cover over the relief, the slope, H).
  Shared helpers are `internal/topotest`. 122 tests before and after, by name.

### navigation tidied (2026-10-01)

The same order for navigation. Only games use it, and everything they use — `MoveOrder` and its
`Path`, `Leg`, `Goal`, `Round`, the facts, `Touch`, `Entered`, the commands — is data the
machinery reads and writes every step: an interface such as the topography's `Orders` cannot
stand between them. Asked, the user chose to keep the types and the systems in the plugin's
package (unexported, as they were) rather than move the types to a vocabulary package or alias
them; `internal` took what stands alone: the route finder (`internal/pathfind`: `Find`,
`FindAround`, `NearestFree` into a caller's buffer of steps, the plugin's `pathFinder` laying
them in `Path`s, so its own tests stayed as they were) and the GPU lines of the routes
(`internal/routes`). `PathRenderer`, `NewPathRenderer`, `RouteTier` and `EnteredName` are no
longer public. 121 tests before and after, by name; the finder's moved with it, under helpers of
its own (a mirror `Path`, the old method names).

### collision tidied (2026-10-01)

The collision is small and its system works on the game's own `Collider` and `Physics`, so it
stays in the plugin's package, as the navigation's does. What stands alone is the arithmetic of
the answer to a contact, on numbers alone: it went to `collision/internal/response` — `Normal`,
`Exchange` between two `Side`s (inverse mass, bounce, velocity; was `impactOf` and the root's
`bounce`), and `Footing` and `Worse` over a `Ground` (was `footing`, `overhangs`, `worse`). The
system turns its `contactSide` into a `Side` or a `Body`; `Physics.inverseMass` stays in the root.
The impulse table tests `Exchange` on its numbers; the cases of a default mass and a restitution
past its end test `Physics` in the root; `Footing` has tests of its own. First the contract other
plugins fill — `Field` and `FieldBox` — went to a package of its own, `collision/solid`; the user
turned it down: a package of one interface and an alias is no package. `collision.New` (a public
function returning the unexported `*module`), `CollisionSystem` and `NewCollisionSystem` were
public for tests alone: they are `export_test.go`'s now. The tests outside the package that used
`New` went where they belong: the collision demo's save-and-load cycle, which tested the engine's
index after a load, into collision's tests; the hooks' tests onto the plugin in a world, ticked as
a game ticks it (the impact they log, 10, is the same: it hangs on the velocities alone).

Then the user found `shapes.go` public for tests alone, and every public name of `collision` and
`collision/hooks` was checked for a reader outside the tests (games, demos, plugins, bench):
- `ShapeTest`, `Contactee`, `BoxesTouch`, `Plugin.WithShapeTest`: no reader; `BoxesTouch` was not
  even what the system ran. The user chose to remove the whole hook: overlapping boxes touch.
- `module.Hook`: called by tests alone (the plugin's `Hook` calls `hostAll`); it is
  `export_test.go`'s. The unused `CollisionSystem` alias there went.
- `Physics.Weight`, `Bounce`, `Immovable`: read inside the plugin alone; private now.
- `hooks.ContactStats.Reset`: no caller, not even a test; gone.
- Kept: `Collider.Struck`/`StruckCount` and `MaxContacts` (a saved component's fields stay
  exported, as `vision.Sighted`'s and `effect.Active`'s), `DefaultMass` (what a zero `Mass`
  weighs), `LogContacts` and its options (a ready-made hook, as `LogFalls` and `LogSightings`).
- The tests had three copies of one installer and the same world-with-collision set-up five times:
  both are `collision/internal/collisiontest` now (`InstallCtx`, `Start`, `Step`). `Step` syncs
  after the collision, as a game does; the tunnel and attach tests did not, and pass either way.
- Test names of types long gone: `TestDetector_*` is `TestCollisionSystem_*`, `TestContacts_Update_*`
  is `TestPairs_*` (`candidates_test.go` is `pairs_test.go`).

### The last of the behaviours (2026-10-01)

The user asked what `collision/behavior.go` was, now that there are rules. It was no mechanism:
the two moments the collision hands its rules, `Meeting` and `Struck`, under the old file name, as
`vision/behavior.go` held `Sighting` and `plugin/host/behavior.go` the rules' constructors. Those
are `meeting.go`, `struck.go`, `sighting.go` and `rules.go` now, and the tests that still said
"behavior" of rules say "rule". The one real survivor was `world.Behavior`: a bare goke system
`world.Hook` ran before movement, beside the rules and plans, which no game, demo, plugin or bench
used. It went, with its pass and its tests; the steering test that leant on it runs its asking
system before the world's step itself.

`Struck` came every tick to every collider, and every one of its readers filtered out those that
struck nothing (`Struck.Hit`, `len(s.Contacts) > 0`). It comes now only to an entity that struck
something: the collision walks its colliders with `EachHost.RunWhere`, a skip in the same loop,
and `Struck.Hit` is gone. The user asked whether this should rather be a marker kept on the
entity, so that no component comes and goes: it is so already — the contacts lie in `Collider`
for good, and `Struck` is a view of them. A marker (a bit of `effect.States`, as `Changed`) would
serve a plan or another plugin's rule wanting "struck this step"; nobody wants it yet.

### The world tidied, the core out of it (2026-10-01)

The user asked for the world the same work as the plugins before, wondering whether `rule`,
`entity` and `clock` were not rather gram's core. They were: `plugin` and `plugin/host`, the
framework's spine, imported `plugins/world/entity/tag`; `rule`, `clock` and `entity` never import
the world, and every plugin and game used them (tag 72 files, comp 69, kind 66, effect 27, rule
23, clock 18). They are at the module's top now. The user asked about the components they define:
registering them stays the world's — whoever runs a part registers its components, as the world
registers `steering`'s and collision `Collider` — and only two save names changed, the generic
`tag.Tags[clock.Phase]` and `tag.Tags[effect.States]`, whose argument goke names by full path
(checked by printing the world's `LoadComps` before and after).

The world's root was 20 files in seven groups: the plugin, making entities, motion, drawing, the
cameras and views, the clock's moments, small vocabulary. A separate plugin for making entities
("units") was weighed and left: making an entity writes `Base`, puts its box in the space, counts
it against `MaxCount` and checks its size, and a load remaps its kind and tags — a plugin apart
would need a wide public way into the world. What stands alone went to `world/internal`: the
register of kinds and tags with the remapping of a load (`internal/kinds`) and the flat look
(`internal/look`). Spawning from rows stays in the root, beside the `Appearance` and `Base` it
writes. A place where a kind is defined with its rules and its starting effects is to be designed
on this layout.

The user had `Attach`/`Detach`/`Declare`, `Bodies` with `Kinds.Reserve` and `world.Draw.*`
removed. By the audit rule: `VelocitySystem`, the `Renderer` type, `NewView`/`DropView`,
`View.Refresh`, the clock's setters and getters, `DefaultTempos`, `Steepest` and `MarkerPrefix`
are private; `comp.Template.WithEffect` and `Clock.Now` went (a navigation test that entered its
units' cells at spawn through `WithEffect` enters them in its setup). Kept on purpose:
`MoveSystem`/`NewMoveSystem`, which navigation's and collision's test rigs compose into steps of
their own (moving them onto the world plugin changes the order of systems they count ticks by — a
step of its own), `Drawing.As`/`With` (a Drawing rule's verbs), the plans and asks of `rule` and
the effect traits (the formalism a game writes with; no demo uses plans yet), `Clock.In`.

### The move system private, the tests on the world (2026-10-02)

The user had `MoveSystem` made private: "the tests we have are stupid if they force these systems
public". Navigation's rigs (`newProfiledWorld`, `newLegWorld`, `newEnteredWorld` and two in
`navigation_test.go`, 22 tests with `ground_test`'s and `destination_test`'s) and collision's
`pairs_test` built an ECS of their own: navigation, steering, the world's move. They stand on the
plugins now — navigation's on `newNavWorld` (`world_rig_test.go`: world, board, navigation under
CellSpacing, units from kinds, each holding its cell), collision's on `collisiontest.Start` — and a
step is the game's: the world, then the board and navigation. Six tests changed, none by hiding a
change of behaviour:
- `TurnsBeforeTheBendAndNeverStops`, `BrakesToRestOnTheGoal`, `RunsThroughQueuedGoalsWithoutStopping`,
  `APatrolGoesRoundStandingItsPauseOnEachGoal`: navigation decides after the world's step, so an
  order is carried out from the second tick; the first tick is skipped. `BrakesToRest` also reads
  the rest a tick after the arrival, when the world's step has carried out the stop — and checks the
  unit stood on the goal's centre already at the arrival.
- `BlockedDeparture_RepathsAroundStationaryEntity`, `OccupiedTarget_WaitsThenSettlesNextToIt`: the
  plugin hooks the crowd's rules, and an ownerless unit standing makes way for an ownerless one on
  the move (allies); the old rig ran no rules. The standing unit is another player's now (a
  stranger, `owner: 2`), which is what the tests are about.
The tests of navigation's system alone (no movement) stay as they were.

The ready-made drawing rules came back as `plugins/world/hooks` with `examples/appearance-demo`: a
game cannot write a Drawing rule of its own (it would need plugin/host), and an effect's Alter
changes the Appearance itself, while a drawing rule changes what is drawn this frame. The demo's
test draws through a recording Look. `Leaving` stays, marked to reconsider; `Clock.In` and the
clock's phases stay.

### Rules in one place (2026-10-02, stage 1)

The user found the rule spread over three packages: `plugin.Rule` an `any` in the plugin's
contract with `Tick` and `Marks` beside it, the hosts and a second vocabulary of filters in
`plugin/host`, and `rule` with `On` in `moment.go`, `Plan` in `tree.go` returning a component and
`rule.New` making the plans' engine ("M-A-S-A-K-R-A"). Agreed with the user:
- `rule` holds the rule (`rule.go`: `Rule`, sealed — only `On` makes one — `On`, the filters, the
  errors), the moment (`moment.go`), the `Tick` (`tick.go`) and the hosts; `plugin/host` is gone
  and `plugin` keeps the contract alone. The hosts had to come with the rule: `rule.On` builds
  what a host runs, and a host must know the rule — two packages would import each other.
- Plans are `rule/plan` (`plan.New`, `Actor`, `Command`, the asks, `Mind`, `NewPlans`). The engine
  of both — the nodes run within a rule's pass and over a plan's steps alike — is
  `rule/internal/engine`; the faces alias its types: `rule.Step`, `plan.Mind`, `plan.Asked`,
  `Replied`, `Chain`, `Answer`. A plan reacting to an ask names `plan.Asked[W]`, navigation asks
  whether a unit has a `plan.Mind`: those need public names, and the engine cannot import the
  packages that import it.
- `plugins/board/internal/rule` is `plugins/board/internal/moments`, out of the way of `rule`.
- Tests that hooked a string or a goke system to see it refused are gone: the sealed `Rule` makes
  that a compile error. The engine's `ctx` takes a `Pass` (carrier, step, time, seed, world,
  places round) instead of the `Tick`, which the engine cannot import.

Stage 2 removes the rules written as Go functions (`rule.Each`, `Every`, `Pair`, left public
until then); stage 3 moves render's mechanism into `render`.

### No Go code in a rule (2026-10-02, stages 2 and 3)

The user's standard: everything not of drawing goes through the ordinary mechanism; a rule rides
a pass its plugin makes anyway — one walk, many rules — never a system per behaviour, and holds no
Go code of its own. Each rule with a Go body became something else:
- **The ground's pace** is a fact: the board's units pass (`standingSystem`, already walking every
  unit) writes `steering.Pace{Share}`, and the world's `velocitySystem` multiplies by it. The board
  runs after the world, so the pace of a step is the ground the unit stood on the step before.
- **Navigation's bumps** read the `Collider` contacts in navigation's own pass (`bump()`), the way
  it read them already for the crowd.
- **Logs and stats** are options of the plugins (`WithLog`, `WithStats`), written in their passes.
- **Weathering** is the atmosphere's own system; it was a rule of the clock's moment running Go.
- **Flee and Chase** order steering commands the world carries out: `steering.Away`/`Toward`,
  aimed at the `Sighting`'s subject (the nearest seen), and `Turn`. A behaviour switched for the
  whole game is an effect on the world: `world.Apply` and the new `world.Dispel`, the rule
  running `During` it — the demo's A key. Asked whether that is the way to put an effect on the
  whole game without picking entities, the user agreed. Looking round became its own rule,
  `Search`, `Unless` an effect `Looked` lasting the while: one rule with both would have looked
  round with only bystanders in view and could not be tested as a pure chase.
- An aimed command now **fails** when the moment names nobody (`Sighting` with none in view):
  entity 0 is valid, so an aim at zero could not mean nobody. Only moments that name a subject
  are affected; a plan outside a fact keeps its own aim.
- `climate.Weathering` named no entity, so no rule of it could do anything; it is about the
  world's own now.
- **Tests** watch through components (`Collider.Contacts`, `Pace`) or a rule ordering a test
  command (`heard`) a test queue keeps — who, about whom, which rule. The hosts' own tests are
  internal to `rule`, with makers of Go-bodied rules they alone have.

Stage 3: drawing is the one place rules are Go, and the mechanism is `render`'s:
`render.Appearance`, `render.Rule` (`Over` — `render.Overlay` was taken by the material quad —
`As`, `With`, `Show`, each over a component `T`, `Over`/`As`/`Show` with conditions of `T`),
`render.Rules` to run them (`Own` shares a column the renderer reads itself: goke panics on a
component both required and optional in one query). `world.Plugin.Draw` and `vision.Plugin.Draw`
take them; `world.Facing` is a `render.With` over `Base`; `HitOverlay` is
`render.Over(with, hit.Mark().In)`; `vision.ShowViewOf(t)` is `render.Show(t.In)`, with
`tag.Tag.In` new.

A finding on the way: collision marks a box `Outside` by its pushed place in the index while its
`Pos` keeps the place before the push (it is not written back for a box that left). The world's
exit pass reads `Pos`, so a Leaving rule hears of such a box every tick while the mark comes off
and goes back on. Running the rules only for boxes out by `Pos` broke collision's open-edge test,
so the exit pass was left as it was.

### A box pushed out through an open edge (2026-10-02)

aabbworld's engine listed a box pushed out through an open edge by id alone (`Left`) and did not
report it `Moved`, so collision could not learn where it went: its `Pos` stayed over the wall, the
next Rebuild put it back there, the wall pushed it out in the index again, and the box — marked
`Outside` by collision, unmarked by the world's exit pass that reads `Pos` — was never despawned,
striking the wall every step. Probed: alive and overlapping after 60 steps with no rule hooked.
The user chose to break aabbworld's API rather than add a `Moved` call: `Left` returns
`[]collide.Leaver{ID, Box}` (aabbworld v1.10.0, no v2), and collision writes the box after `Tick`,
asking no ground — leaving the world wins over the board's footing.

## Questions for review


- **`world.Leaving`: a moment nobody uses.** The world hands it to rules every tick an entity is
  past an open edge, and despawns the leaver when no rule is hooked. No game or demo hooks one;
  only collision's test of the open edge does. Kept on 2026-10-02 at the user's word, to be
  thought over: should it stay a moment, or should the world simply despawn whoever leaves?

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
- **The topography's `Heights`** (settled, round twenty-nine): the one entity's `[]float32` made
  goke log at start that it "requires a dereference outside the archetype's chunk memory". The
  heights are runs of a fixed size now (`Heights{First, Count, Values [1024]float32}`), as many
  entities as the relief takes — the island's 97×65 corners on seven — which goke keeps in its
  chunks and gob saves without a codec; `heightsSystem` writes them when `Relief.Version` moves,
  no longer every tick, and a loaded game's are taken back only when they cover the relief.
- **Big groups arriving still strike each other**, now mostly two on the move where the column
  fans out to its spots (see the twelfth round). Lanes halved that for small units; they were
  dropped at the user's word.
- **No ticking test through the demo**: the engine steps by the wall clock and reads ebiten's
  input, so the board-topography test only starts it; the shore and the steps are tested in
  navigation instead.
- **Screenshots of the island in relief were not compared** — I cannot take them unattended. The
  demos run headless for ten seconds without a panic; please eyeball island-isometric (Tab, Q/E,
  R/F, =/-) tomorrow.
