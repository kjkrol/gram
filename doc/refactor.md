# Refactor: time, atmosphere, boards and maps, demos

[← Back to README](../README.md) · [Roadmap](roadmap.md)

Two days went into drawing an isometric map and adding systems on top of it — the landscape, the
sky, the climate, rivers, roads. Before the roadmap moves on, what is there gets put in order:

- three island demos test the same thing;
- `board` gives no simple map out of the box: no terrain types with a look, no drawing of them, no
  costs;
- `isometry` and `landscape` are two halves of one thing, a map with heights and its drawing;
- `sky` and `climate` are a split pair;
- the clock, the calendar and a schedule of events have no place of their own;
- today's pause stops orders along with everything else.

This is the plan agreed on; nothing of it is built yet. The API and saves break freely — no aliases,
no compatibility layers; the CHANGELOG notes that saves from before do not load.

## Where things end up

| Package | What it is |
|:--|:--|
| `world` | the world, entities, movement, drawing entities; installs its two sub-packages itself |
| `world/clock` | the tactical clock: game time, the tactical pause, the tempo, `Simulate` |
| `world/effects` | effects (today's `plugins/effects`) and the schedule |
| `board` | cells (entities, grid, occupancy, `Standing`, routes) and the `board.Map` contract; the simple flat map is the default `Map`; `board/water` and `board/network` stay |
| `topography` | new: the map with heights, the second `Map` — today's `landscape`, `isometry` and the heights taken out of `board` |
| `atmosphere` | new: today's `sky` and `climate` — `calendar`, `sky`, `climate`, `precipitation`, `weathering` |

Gone: `plugins/effects`, `plugins/sky`, `plugins/climate`, `plugins/landscape`, `plugins/isometry`;
`island-demo`, `island-25-demo`, `island-isometric-demo`.

## 1. Time — `world/clock`, `world/effects`

### The tactical clock

- **What it is.** A thing of its own in `world/clock`, saved with the game. Game time is the sum of
  the simulation's steps. The systems that simulate read it; selecting, the camera and orders do not.
- **Two pauses.**
  - **The engine's pause** (`Runtime.Pause`) is for a menu only. The ECS does not tick while it
    holds, and `Draw` goes on drawing: that is how it works today.
  - **The tactical pause (Space)**, as in Baldur's Gate. The simulation stands still. The player
    still:
    - selects;
    - gives and queues orders;
    - plans routes and sees them drawn;
    - shapes the ground, which takes effect at once.
- **Tempo ½, 1, 2, 4.** `]` faster, `[` slower.
  - By default in sub-steps: a fixed simulation step, ×N being N steps a tick and ½ a step every
    other tick. Deterministic, which replays and networking need.
  - A flag in the clock's config switches to one bigger step instead: cheaper, less exact.
  - When a frame cannot keep up the tempo drops by itself, and the telemetry says so.
- **What the clock governs:**
  - movement, steering and collisions;
  - effects, the schedule, the calendar and the weather;
  - sight (the cones);
  - animations while drawing: water, currents, swaying, clouds, rain and snow. The composer's
    `Frame.Time` comes from the tactical clock, not the wall clock.
- **Showing it.**
  - A reporter of game time: a stopwatch, the pause, the tempo.
  - A HUD element for the stopwatch and the tempo, as a screen layer.
  - A game picks what it shows; a platformer may show a stopwatch alone.

### Interface and simulation

- **A game calls its plugins' `RunPlan`, as today.** The contracts stay as they are:
  `plugin.Plugin.RunPlan` and `game.Stage.Update`. gram's plugins come set up right.
- **A plugin marks in its own `RunPlan` what is simulation**, with `Simulate` from the clock's API.
  Any plugin author has it, and so does a game with systems of its own:

  ```go
  func (m *module) RunPlan(ctx goke.RunCtx, d time.Duration) {
      ctx.Run(m.commands, d)            // interface: at once, once a tick
      ctx.Sync()
      clock.Simulate(ctx, func(ctx goke.RunCtx, step time.Duration) {
          ctx.Run(m.move, step)         // simulation: as many times as the clock says
          ctx.Sync()
      })
  }
  ```

- **`Simulate` does not run the block, it appends it** to the tick's list of simulation, in the
  order of the calls.
  - Once the game's `Update` is done — every plugin's `RunPlan` — the clock replays the whole list as
    many times as the tempo says.
  - The order between plugins stays the one the game laid out, and every step is the same: at ×2,
    "movement → collisions → …" runs twice over. Collisions do not come apart as with a bigger step.
  - The engine gets one small wrapper round the plan: the game's `Update`, then the replay of the
    list. Today that line is `host.ecs.SetPlan(stage.Update)` in
    `internal/engine/stage_runtime.go`; the engine already knows the stage's world.
- **The number of steps.**
  - A counter: `counter += tempo`, `steps = its whole part`, the rest carried on.
  - That gives 0 in the tactical pause, 0, 1, 0, 1… at ½, and 1, 2, 4 at ×1, ×2, ×4.
  - Game time a real second: 0 / 0.5 / 1 / 2 / 4 s.
- **The interface part never speeds up.** Selecting, the camera and orders run once a tick at any
  tempo and in the pause. Only the number of replays of the simulation grows.
- **With the bigger-step flag** the list is replayed once, with the step times the tempo.
- **To check when building it:** replaying the same goke systems several times in one plan, with
  the command buffer synced between the steps.
- **Behaviours run where the system hosting them runs**, and every host there is today is in the
  simulation:
  - `world.Each` / `Every`;
  - collision's `Meeting` / `Struck`;
  - board's `Standing`;
  - vision's `Sighting`;
  - effects' `Idling`;
  - the weather's `Weathering`;
  - the schedule's points.

  A game author can take it that a behaviour is simulation: it stands in the pause and speeds up
  with the tempo.

### Which systems go where

Today's systems, once moved:

| Plugin | In `RunPlan` at once — interface, once a tick | In `RunPlan` under `clock.Simulate` — simulation, the clock's steps |
|:--|:--|:--|
| `world` | `ViewSystem` (the cameras' views; a camera moves in the pause too) | the world's behaviours, `SteeringSystem`, `VelocitySystem`, `MoveSystem`, `ExitSystem` |
| `world/clock` | its commands (tactical pause, tempo); the number of steps this tick | game time moving on a step |
| `world/effects` | — | effects counting down, the schedule |
| `collision` | — | `CollisionSystem` with `Meeting` / `Struck` behaviours |
| `board` | shaping the ground (`Raise`/`Lower`/`Level`, works in the pause) — split out of `cellSystem` | cells an effect altered (touch, seal), `Standing` with its behaviours |
| `navigation` | `moveCommandSystem`: `MoveTo` / `LookAt` orders, planning routes | `navigationSystem` (driving along the route, reserving cells, re-planning), `driveSystem` (a unit steered by hand) |
| `selection` | selecting, the camera's `followSystem` | — |
| `vision` | — | `ScanSystem` (the cones) with `Sighting` behaviours |
| `players` | input → commands, the shortcuts scene | — |
| `topography` | the cameras (turn, tilt, follow, `Tab`), steering orders | `altitudeSystem` (units' altitude) |
| `atmosphere` | its commands (`P`, `Shift+]` / `Shift+[`, `Shift+W`); the sky's light | the calendar, the weather (`weatherSystem` with `Weathering` behaviours), weathering of cells |

The frozen light's hour is a setting of how things look, as a camera's turn is, so it changes at
once in the tactical pause while game time stands. A tick runs everything interface first, then the
simulation's replays. The cameras' views are therefore worked out on last tick's positions — a
tick late, which does not show.

### Effects and the schedule — `world/effects`

- **Effects stay as they are**, counted in tactical time: `Define`, `Grant`, `Alter`, `Lasts`,
  `Dispel`. Speeding the game up shortens them in real time; the pause holds them.
- **The schedule is one queue of entries stamped with tactical time.**
  - An entry comes at a moment, or every period.
  - Calendar entries — "every day at 22:00", "at the start of winter" — come from
    `atmosphere/calendar`, which turns them into clock stamps. The calendar is the clock at a fixed
    scale, with no jumps, so the translation is plain.
- **A point of the schedule** casts effects on entities or runs a behaviour (`plugin.Behavior`).
- **Switching behaviours on and off** uses one gram tag family, `plugin.Tags[Phase]`: 64 bits, one
  component, on the clock's entity.
  - The schedule casts an effect granting a tag (say `night`), and the effect takes it off itself.
  - The behaviour is registered for good, with a condition: only while the tag is on.
  - Hosts stay closed after install, as today.

## 2. Atmosphere — `atmosphere`

- **`atmosphere/calendar`**:
  - a day is N minutes of game time (config), plus seasons, the moon and date formats;
  - calendar entries for the schedule;
  - a reporter of the hour and the date, and a HUD element: a clock with the date and the moon.
- **`atmosphere/sky`**: the sun and the moon from the latitude (the climate zone's) and the time;
  the sky's colours and backdrop → `world.SetSun`.
- **Frozen light**: the game goes on, and the day's light stands still.
  - Only the light stands: the sun, the moon, the sky's colours. The calendar, the weather and the
    calendar entries go on.
  - The hour is set in the config by default. In the game `P` freezes and thaws it, and `Shift+]` /
    `Shift+[` move the frozen hour by half an hour.
  - Once thawed, the light is at once the light of the moment.
- **`atmosphere/climate`**: zones, weather states, temperature, wind and clouds →
  `world.SetWeather`; `Shift+W` changes the weather.
- **`atmosphere/precipitation`**: rain and snow drawn.
- **`atmosphere/weathering`**, ready to switch on.
  - Snow settles, ice grows from the shore, trees sway in the wind — effects cast from the schedule.
  - A game says which terrain types have a snowy and an icy version.
  - The demos' `climate.go` goes.
- **On the simple flat map**:
  - the day colours the light on tiles and units: night darker and colder, dawn and dusk warmer;
  - clouds' shadows and rain and snow;
  - no terrain shadows.

## 3. Boards and maps — `board`, `board.Map`, `topography`

- **The `board.Map` contract.**
  - `board` keeps the cells and draws through a `Map`.
  - A `Map` gives:
    - the grid;
    - the terrain types — what they mean and how they look;
    - the cost function — a planner's step and the speed on the move;
    - the way the cells are drawn.
  - Today's `Look` and `Dressing` go into `Map`.
- **The simple map is `board`'s default `Map`.**
  - A terrain type carries its look, a colour or a sprite drawer, and the board builds a default
    atlas itself. A game may hand its own atlas.
  - It draws tiles, and roads, bridges and watercourses as plain bands in their type's colour — no
    water, no light.
  - Its cost is the type's cost times the distance.
  - Its world is always flat.
- **`board` becomes purely 2D.** Moving out to `topography`:
  - the heights at the corners (`Relief`, `SetHeights`, `GroundAt`, `world.Ground`);
  - shaping (`Shaping`);
  - slopes (`Climbing`);
  - units' altitude (`altitudeSystem`).
- **`board/water` stays in `board`** (drainage, courses). It takes its heights from whoever has them
  — `topography` — as it does today through its `heights` function.
- **`board/network` stays in `board`** (graph, routes, bridges): it makes sense on any map.
- **`topography` (`plugins/topography`) is the second `Map`**, always a world with heights; it needs
  `world.Config{Quasi3D}`. It brings:
  - the heights, shaping (at once, in the pause too) and the cost of a slope (`Climbing` with
    `Ease` and `Steep`);
  - light and terrain shadows, water (glint, current), grounds blending, coasts;
  - roads and bridges over the relief, the ground sheet far off;
  - a default atlas from the types' looks, or the game's own;
  - a relief style per type by name: `Shine`, `Flow`, `Spread`, `Under`, `MixWith`;
  - views from above and isometric — cameras, blocks, billboards, commands to turn, tilt, follow
    and drive — switched with `Tab` during the game.
- **Isometry is `topography`'s only.** The simple map is always seen from above.

## 4. Keys and the shortcuts scene

- **Camera** (every view unless said otherwise):
  - **WASD** moves the camera across the screen (in isometry as the camera is turned); the wheel
    (zoom), the middle button and the screen's edge stay;
  - **Q / E** turn by 45° (isometry);
  - **R / F** tilt (isometry), instead of PageUp and PageDown;
  - **Tab** switches between the view from above and the isometric one (`topography`);
  - **C** follows the selected unit with the camera (today F);
  - **V** puts the camera behind the unit (isometry), and the **arrows** steer it (as today).
- **Time**:
  - **Space**: tactical pause;
  - **] / [**: tempo ½, 1, 2, 4;
  - **P**: freeze or thaw the day's light;
  - **Shift+] / Shift+[**: the frozen light's hour ±½ h.
- **Weather**: **Shift+W** changes it (today W).
- **Orders**:
  - left click or drag: select; with Shift: add;
  - right click: go; Shift + right click: a waypoint;
  - **Shift+S + right click**: look there (today S).
- **Ground** (`topography`): `=` raise, `-` lower, L + drag to level.
- **Game**:
  - **K** opens the shortcuts scene, **Esc** closes it;
  - **Shift+Esc** quits the game (today Esc); Esc alone outside the scene does nothing;
  - **F5** saves, **B** toggles the grid, **Shift+F** toggles fullscreen.
- **The shortcuts scene** is a ready scene of `players`.
  - It holds the game with the engine's pause, as a menu does.
  - It shows every binding with its label, grouped by plugin.
  - The scene's own keys — Space, Tab, B, F5, Shift+F, K, Shift+Esc — become labelled bindings
    too, so the list is whole.

## 5. Demos and housekeeping

- **`examples/island`** is a public island generator: the coast's shape, a height function,
  grounds, rivers (drainage) and roads between the stops. The layout's tests move here. It is not
  `internal`, since the demos are there to be copied.
- **Three demos in place of the three islands:**
  - **`board`**: the island on the simple map — tiles, plain bands for roads, rivers and bridges,
    the flat atmosphere, the default atlas;
  - **`board-topography`**: the same island with relief — from above and isometric (`Tab`), the full
    atmosphere (day, weather, snow, ice), the default atlas;
  - **`board-atlas`**: a small board with the game's own atlas.
- **Staying:** `navigation-demo` (+ hex), `navigation-vision-demo` (+ hex, moving to `topography`),
  `effect-demo` and `split-screen-demo`, moved onto the new time and effects.
- **Housekeeping:**
  - the Makefile's duplicated `run-island-isometric` target;
  - old binaries in the repo's root;
  - the demos table in README and the list in CLAUDE.md;
  - the isometric demo's doc comment and its code disagree (an eight-day year from winter against
    `EarthYear` from spring) — that goes with the demo.

## Order

1. **Time.**
   - `world/clock`: the tactical clock, the two pauses, the tempo in sub-steps, the reporter, the
     HUD.
   - `clock.Simulate` (the tick's simulation list) and the engine's wrapper round the plan.
   - Every plugin split as the table says.
   - `world/effects`: effects on the clock, the schedule, the phase switches.
2. **Atmosphere** on the new time: calendar, the sky with frozen light, climate, precipitation,
   weathering; the flat version.
3. **Boards.**
   - `board.Map` and the simple map: the types' looks, the default atlas, plain bands.
   - The heights out of `board` into `topography`, which takes in the landscape and isometry
     (`Tab`).
4. **Keys and the shortcuts scene**: the new keys, labelled bindings for the scene's keys, the K
   scene in `players`.
5. **Demos and housekeeping**: `examples/island`; the `board`, `board-topography` and `board-atlas`
   demos; the other demos moved; README, CLAUDE.md, CHANGELOG, Makefile.

Every stage ends with tests passing and screenshots taken; nothing is committed until asked.

## How each stage is checked

- `go vet ./...` and `go test ./...`.
- **Time.**
  - In the tactical pause movement, effects and the schedule stand, while orders reach the queues
    and routes are planned.
  - Tempo ½, 2 and 4 in sub-steps give the same state as tempo 1 after the same game time.
  - A calendar entry comes on time at any tempo.
  - An effect granting a tag switches a behaviour on and off.
- **Atmosphere.** Frozen light stands while the weather goes on; thawed, the light is right; the
  flat version colours the tiles.
- **Boards.**
  - The simple map draws from the default atlas and from a game's own.
  - Steps and speeds are priced by the `Map`.
  - `topography` draws the island as today's isometric demo does, the screenshots compared.
- **Demos.**
  - `board`, `board-topography` (with `Tab`) and `board-atlas` run and are photographed.
  - The other demos run.
  - The island's benchmarks do not regress.
