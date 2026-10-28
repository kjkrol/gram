---
name: gram-plugins
description: Reference of gram's built-in plugins, one entry each with its current API and rules — world, motion, board (cells, units, entities on the board, bridges, maps as files, gramap), topography, atmosphere, collision, navigation, entity, steering, view, clock, selection, dialog, players and owner, cameras, driving, gravity, vision, bridges, minimap, bullet. Use when working in or with a specific plugin, to learn what it keeps, hosts, handles and depends on before reading its code.
---

# gram-plugins

Every plugin's `doc.go` ends with `# Vocabulary`: its moments (with their words and subjects),
facts for plans, knobs and commands (who may give each). That section is the authoritative list;
`go doc ./plugins/<name> | tail` shows it. The entries below are the prose round it.

- **`world`** — the foundation a Stage makes from `world.Config` (`stage.World(cfg)`;
  `ctx.UseWorld(cfg)` underneath, once): the space's size and edges (`SpaceCfg.Edges`:
  `aabbworld.Torus`, `WrapX`/`WrapY`, `OpenX`/`OpenY`, closed by default — a box stops whole at a
  closed edge, wraps at a wrapping one, may leave by an open one: what becomes of it then is
  motion's),
  entity bounds, `Heights`, `Scale{Metres, Figures}` (what a world unit is; `Scale.Units` metres;
  `Figures` how many times bigger than life the figures are drawn, `Scale.Figure(m)` their boxes,
  heights and eyes; speeds and sight stay real; with it the ground sinks under an eye's level,
  `Scale.Drop`, and the air hazes far off), `Seed` (which `Chance` draws from), `Sectors`
  (`SectorsCfg{Buckets, Bands}`: square sectors of `Buckets` space buckets a side, 4 by default,
  and up to `sector.MaxBands` bands `sector.Band{Reach, Pace}` round the anchors, nearest first,
  reaches growing, the first `Pace` 1; none, the whole world in one band and no supervisor's
  cost). The supervisor (`sectors.go`, once a tick after the clock, before the views) lays each
  sector in the band its nearest anchor reaches (`sector.Grid.Place`: nearer at once, farther
  only half a sector past the reach; every sector begins beyond, so the first tick and a load
  wake everything near), wakes the sleepers of a sector come back (its list of `sector.Off`
  entities, kept by sector, never a space query: **the space holds no `Off` entity** — the
  world's index, motion's rebuild and collision's `all` exclude them — so asking the space for
  them would never find them again; lists laid anew at the first tick and after a load,
  `restart`), and moves the mark of every entity whose sector's band is not its own over the
  band queries; a move to or from `Off` sets `unindexed`; marks `sector.Band2`…`Band5`, `Off`
  (zero-size, `CmdBuf` `AddOne`/`RemoveCompOne`, saved), the first band unmarked. Anchors are
  `sector.Anchors` handed to `Plugin.Anchor` (the cameras plugin's, the `camera.Focused` point
  of each camera but a `Whole` one); `Plugin.Sectors()` is a `render.Reporter` ("Sectors",
  "Entities by band", ", bands off" while they are); board/topography and motion/expanse have
  them. **Bands off** (`SwitchBands{On}`, the settings screen's; `SetBanded`, `Banded`;
  `Res.Settings.Unbanded`, saved): the supervisor
  leaves the anchors out, so `Grid.Place` lays every sector in the first band at once, the
  sleepers wake and the marks go — the whole world as if without bands but for the supervisor's
  pass; on again, the sectors go back past the margin. **The sectors shown** (`SwitchSectors{Shown}`,
  the settings screen's, `SectorsShown`, `Res.Settings.Sectors`: `SectorBands`, `SectorsSeen` — the camera's `view.Level` —
  `NoSectors`; `ShowSectors` from a scene; not saved): the world's `sectorsView`, in a world with
  heights through a camera with `Rays` on the GPU over the ground's depth
  (`internal/sectorview`: a texel a sector a camera, `ReadGround`, `OnGround(0.2)`, nothing past
  an open edge, the edges fading where a pixel spans much of a sector), in a flat one quads from
  `Look().Footprint` (copies at a seam) at 0.2 under the figures. Both settings are there only
  in a world cut into sectors, neither has a key. **Systems go in the bands by a policy**
  (`sector.Policy`: `Everywhere` every step but `Off`, `Paced` a step every `Pace` in the
  farther bands over `Pace·d` — all at once, on the steps where `Clock.Steps() % Pace ==
  Pace−1` — `Near` the first band alone): `world.Plugin.Pace(policy)` is a goke query option,
  `goke.PacedBy(clock.Steps, rules…)` (`Every[BandK](Pace_k)`, `Never[Off]`; without bands no
  rules), added with `qb.With(pace)`, the loop walking `q.Paced(d)` and taking `q.Dt()` (goke
  works out a block's period once, as its archetype is matched). Who goes how: motion
  (steering, velocity, move, exit), gravity along Z, the world's plans and effects `Paced`;
  navigation `Everywhere`; collision (far boxes `Plain`, contacts cleared), bullet's flight,
  vision (far observers' `Sighted` cleared), gravity across `Near`.
  Per-step quantities scale by the steps given (steering's `TurnRate`, `Delay`; move's
  `MaxStep`). `world.Roster()` is
  what the plugins ask of the kinds of each template: a `kind.Template` is a sort of kind a plugin
  brings in by name under a parent (`kind.NewTemplate(name, parent)`; world: `EntityTemplate`;
  motion: `UnitTemplate` under it; board: `StandingTemplate` under the entity's, `CellTemplate`;
  `entity/kind` knows none). A plugin writes its asks in `KindTemplates(r *kind.Roster)`
  (`plugin.KindTemplater`, asserted beside the type) and nowhere else: the world calls its own and
  every made plugin's, in the order made, as the roster is first read, and refuses a write elsewhere
  and a plugin asking made after (make every plugin before defining a kind).
  `kind.Require[T](r, t, by, why)` names what the game must supply (world: `Position` of every
  entity; board: `At`, `Mover` of a unit; navigation, driving: `steering.Steering`),
  `r.Default(t, comp.Const(v))` what a plugin brings itself (world: `Velocity{}`, effect marks;
  light: `Shadow{Patch}` of every entity; motion: `steering.Course` of a unit; board: `Lie{}`,
  `light.Shadow{Baked}` of what stands; collision: `Collider{}`, `Physics{}` of a unit; dropped with
  `comp.Without[T]()`, a template's
  own taking the place of its parent's); `r.Spec(t, own...)` builds a kind's Spec
  (`board.NewUnits` from the unit's, `Entities.Define` from the standing one's with its shape's `Z`,
  the cells from the cell's), a missing requirement panicking by plugin and reason. `world.Layers`
  are the planes an entity is on (none: every plane); collision and vision read it.
  `world.Config{Heights: true}` gives the world heights: entities carry `world.Z{Altitude, Height,
  Step}` (`Z.Reach()`: how high it steps up), written by the board in relief or by gravity;
  collision and sight follow Z; a flat world asks none of it, and each plugin refuses the other
  mode's facts where it first meets them. `Base` is the one component every entity carries
  (`Position`, `Velocity`, `TypeID`, `Caps`). Entities are spawned before the game
  (`Seed`/`Populate`) and during it (`world.Spawn{Entry}`, `Plugin.Spawn`: the world's own system
  makes the entity at the next step after the plans, refusing with a log line an unknown kind, a
  wrong row, a full world, a bad size, a box past an open edge) and go by `world.Despawn{}` (an
  entity's own, `act.Do`, carried out in the world's step; `Plugin.Despawn(cb, id)` a plugin's).
  The world's step: the plans, the spawns and despawns, the commands about effects, the clock's
  moments, a Sync, the space reading every held entity's box where goke keeps it (runs over the
  `Base` columns, `world.RunOf`, `Space.RebuildRuns`: motion hands it the runs after moving,
  collision where Caps change or sweeps stretch, and the world gathers them anew after every
  Sync of the Stage's Update and of what the clock replays, `Clock.OnSync`, the engine handing
  Update `Clock.Settling(rc)`, and as its tick begins, once a chunk came, went, grew or shrank or
  a table's `TableVer` changed: spawns, despawns and marks alike, nothing else tells it, the
  `unindexed` flag gone; 2026-10-21, etap 3 and wave 3), the effects' pass. `Query`, `Scan` and collisions read that grid
  until the next rebuild. At the start of the world's tick the `view.System` refreshes every
  `view.View` (`Plugin.View()` the camera's); `Read.ViewFor(cam)` is a camera's as the frame
  drawn was published (one copy a frame, 2026-10-23, the live one before the first publish; after
  it nil for a camera the frame holds none of, 2026-10-24),
  `Read.Views()`/`InView(id)` the live ones for what publishes to cull by; the renderer draws
  what the View contains. The world makes no camera. It hosts rules of a `clock.Moment` (every step, just
  before the effects' pass) and handles the commands about effects, `Pause` (P and Shift+P),
  `Faster` (]) and `Slower` ([).
  `Figures(f...)` notes kinds of entity in one piece (`world.Figure`: `Define(w)`, `Look(a, w)`;
  `Stateful`, `Ruled`), `FiguresIn(part)` runs their parts as the Stage reaches Effects, Rules and
  Kinds; `ctx.Looks` declares their looks on the atlas once the Stage is set up. `Picture()` is the
  Stage's one picture: every source its plugins gave `ctx.Draws`. Every `host.Tick` comes from
  `world.Plugin.Tick(d)`: the carrier (`world.Plugin.Carrier()`, the stage's one `command.Carrier`
  for players and entities alike; the engine carries every `plugin.CommandHandler` a stage uses),
  `Time`, `Seed`, `Roles`, `World` (the clock's entity).
- **`motion`** — moves the world's entities (`motion.NewPlugin(w)`, made first in `makePlugins`,
  its `RunPlan` right after the world's; a world without it, look/sky, keeps everything where it
  stands). Every step after the world's: the steering system (`motion/steering`: carries out the
  `Course` asked of each entity as its `Steering` lets it, gives a `Course` where there is none,
  handles `steering.Away{From}`, `Toward{To}` (`host.Aimed`), `Turn{Angle}`, one a step), the
  velocity pass (scales `Base.Vel.Value` by the entity's `steering.Pace`, the board's share, and
  runs the rules of a `motion.Moving`: its words `motion.Still()`, neither walking nor strafing,
  and `motion.Braking()`, slowing down — `Moving` carries the steering's `Course`), the move pass
  (every box by its `Velocity` and `motion.Drift`, gravity's, under the edge rules, capped at half
  its shorter side, `world.StepReach`, `Position.MaxStep`/`MaxSpeed`; a chunk's steps worked out
  first and moved in one call, `aabbworld.Space.MoveRun` over the chunk's run, the leavers marked
  `Outside` from the ids it returns, wave 3; then the space rebuilt), a
  Sync, the leavers (wholly past an open edge an entity carries `motion.Outside`; rules of a
  `motion.Leaving` hear of it every tick, with none it is despawned; one back inside loses the
  mark). Running after the effects' pass, an effect on the steering tells the step it begins; what
  a rule of `Moving` or `Leaving` puts on or takes off lands with the next step's effects.
  `motion.UnitTemplate` (under `world.EntityTemplate`) is the template of what moves.
  Whatever steers or pushes takes motion as it is made — `collision.NewPlugin(w, m)`,
  `bullet.NewPlugin(w, m, sel)`, `driving.NewPlugin(w, m, sel)`,
  `navigation.NewPlugin(brd, w, m, sel, drv)`, `gravity.NewPlugin(w, m, cfg)`; a board's units
  `board.Plugin.WithMotion(m, mobility)` (`NewUnits` panics without either; the mobility
  `cell.DefaultHiking` or the game's own); collision and bullet hand an
  entity they put out to `motion.Plugin.Leave(cb, id)`. A plugin reading a steering component in a
  world that may lack motion lists its `LoadComp` itself (the board `Pace`, the topography
  `Course`, `Driven`, `Drift`), so a save loads either way.
- **`light`** — the light over a world: `light.NewPlugin(world, Config{LampShadows})`, made
  after motion and before the board and the topography (they draw by it; `w.SetLighting(p)` makes
  it the world's `Lighting`), run after the world. What shines is an entity with a `light.Source`
  knob (`Color`, `Strength`, `Ambient`, `Reach`: a lamp reaching from where it stands, from the
  middle of its `Z.Height`; the sun a distant one of no Reach, its way and strength of the hour
  written beside it in `light.Shining` by the atmosphere's `shineRun` on its own entity, or once
  by `Plugin.WithDistant` on the light's own — two panic); `Plugin.Distant()` the working light
  (`Shining × Source`), `Plugin.Lamps()`, `White` the sun of a world without one. `light.Shadow{Cast,
  Take}` is every entity's knob (`KindTemplates`: `Patch` taking all; the board's standing
  template `Baked`): `Casting` `Patch`/`Silhouette`/`Baked`/`NoShadow`, `render.Taking` bits
  `TakesRelief|TakesBlocks|TakesClouds|TakesLamps` (zero all, `render.Shines` none). **The plugin
  declares and publishes; render lights** (2026-10-24): it is a `plugin.Publisher` of
  `render.Lights` once a frame (sun, lamps, `LampShadows`, `Coarse`, policy, the `Baked` blocks
  scanned once a frame with `Exclude[sector.Off]`, the ground they bake over — the relief's lattice
  by its version, or level ground 256 cells a side while a block stands — and the relief's
  `GroundAt` for the live shadows), handed through `world.Write.SetLights` to the world's
  composer (`render.Composer.Light`; `world.Read.Lights()` for what draws: the board's miniature,
  the topography's billboards, the reporter). `world.Lighting` is the simulation's side alone:
  `Join` and `Mark(cur, i) render.Shadowing`. The composer (`render/internal/compose/lighting.go`)
  at the start of each part hands the frame the sun's uniforms (`lightWay`, `U.LightWay`,
  `U.LightStrength`, `U.LightColor`, `U.AmbientColor`, `U.Ambience`, render's library
  `light.wgsl`) and the `MaxLamps` 8 lamps nearest the camera (`lampLightAt(p)`, `U.LampCount`,
  `Lamp{k}At/Color`) — the ground of a sharp-figures frame lit by them too — and plans the bake,
  drawn first of the opaque pass; `render.Frame.Lit(x, y, z, take)` lights what a source draws
  (`render.Illumination{Light, Lamps, Direct, Takes}`), `Frame.Cast(box, alt, height, Shadowing,
  atlas, sprite)` declares a live shadow, laid once every source of the part has composed, through
  the `ShadowPolicy` by the distance from the eye: the sun's (`Distant.ShadowOf`/`SilhouetteOf`,
  an `Umbra`, the sprite's shape flipped to the side it faces) and on level ground the
  `LampShadows` nearest lamps' (`Lamp.ShadowOf`). The bake (`compose/bake.go`): the relief's shade
  from the frame's `GroundHeights` — `groundDrawnAt`, triangle-split as the mesh is drawn — a strip
  a frame as the sun turns, the whole at a leap or as the heights or the blocks change, each band
  fading from its last bake, `bake_shade.wgsl`; then every block, `bake_blocks.wgsl`, `Darkest`, a
  deck's pieces joined, so a flat board gets its towers' and bridges' shadows; `CoarseShadows{On}`
  half as fine (`Res.Settings.Coarse`); nothing to bake over a hex relief or without a sun; a bake
  before the frame holds the heights plans the whole anew. The casts (`compose/cast.go`,
  `cast.wgsl`, the rect each covers from the relief's extent, `f.ReadGround`, at
  `OnGround(castLift)` 0.05 under
  the grid). A ground draws itself in what the frame holds: `render.GroundShade`, `lightWay`,
  `lampLightAt` (the terrain's `mesh.wgsl`, the hexes', the board's tiles, `look/tiles.go`). Tests:
  `plugin_test.go`, `shadows_test.go`, `internal/bake` (GPU: strips, blocks, level ground, reach,
  the library's probe). `Reporter()` the panel's "Light:" line. Depends on `world`.
  The shadows' settings sit under `control.PictureShadows` (2026-10-20): "Shadows follow the
  frame budget" (`AdaptShadows{On}`, `Settings.AdaptiveShadows`: the light a `cameras.Lever`,
  the lamps' shadows dropped first and the shadows baked coarse next as the rate falls under the
  budget, back the other way as it stays over), "Coarse shadows" and "Lamps' shadows"; the
  plugin binds no key.
- **`board`** — optional grid and terrain over `world`, standing only in a world with heights
  (`board.NewPlugin` panics in a flat one; a world without a board — collision, minimal,
  rules/vision, scenes — stays flat); its grids wrap per axis, following the world's `Edges`. The
  root keeps the `Plugin`, the `Board` (`Plugin.Board()`: `At(x, y)`, `Kind`, `KindAt`, `KindFor`,
  `Bare`, `Set`, `SetAll`, `Way`, `SetWay`, `Along`, `DecksOver`, `Reaches`, `Fenced`, `Lane`,
  `CellFor`, `LevelFor`, `CellVersion`, `Changes`, `Version`, `Touch`,
  `Shape`, `Built`, `States`), the `Layout`, the `Map` contract, `NewUnits`, `NewEntities`, `Lay`.
  **The `Map` contract is a customization point for games, not plumbing** (`board.Map`:
  `Look`, `Heights`, `Dressing`, `Top`, `Rise(from, to)` — how steep a step is, the rise between
  the cells' middles over the run, 0 on a flat map — put in by `Plugin.WithMap`;
  `look.Map`, its drawing half: `Look`, `Heights`, `Dressing`, `Top`): a game gives its own
  map through it — topography is one, the board's windy map another. Its methods stay even
  where no plugin of the repo calls one (`look.Renderer` reads only `Look` and `Dressing`
  today); never treat them as dead code — a point with no example gets one under `examples/`
  (the `Map`'s: `examples/board/custom-map`, `ExamplePlugin_WithMap`);
  what draws reads the cells **as published** (2026-10-24): the plugin publishes them once a
  frame (`drawn.go`: two buffers, each brought up to the board where a cell's version differs,
  a cell 32 bytes, its kinds and way interned), read through `board.Read.Drawn()` (a
  `board.Drawn`: `look.Board` + `Way`, `States`, `Version`; the live board before any publish)
  and `Read.DrawnHeights()` (the Map's `DrawnHeights`, the topography's frame relief); the
  renderer publishes its grid's toggles (`look.Renderer.Publish`, `drawn()`)
  (`level.go`), `Spawn[R]`, `Each[R]`, `Sides`, `Shape`. Public subpackages: **`board/cell`**
  (`cell.ID`, `cell.Kind`, `cell.Kinds` through `board.Plugin.CellKinds()`, `cell.Domain` —
  `Land`, `Water`, `Air`, a game's own bits — `cell.Plot`, `cell.Ground`, `cell.Way`/`Links`,
  `cell.On`, `Floor`, `Wall`, `Rail`, `Built`, the moment `cell.Now`, `cell.Entry`, `WayEntry`,
  `cell.Terrain`, `TerrainMap`, `cell.Occupancy`), **`board/unit`** (`unit.At`, `unit.Mover`,
  `unit.DomainAt`, the moment `unit.Standing`, the markers `unit.Entered`, `unit.Airborne`),
  **`board/grid`** (`grid.Grid` — every grid knows its own directions: `Directions`, `Opposite`,
  `Offset`, `Step`, `Corner`, `Flanks`, `Toward`; nobody else writes a table, a bit or a vector
  of them; walking `Directions()` with `Toward` allocates nothing and costs less than the
  `Neighbors` list on both grids (the square divides in 32 bits; `BenchmarkGrids_Neighbours` in
  `internal/grids`), so a hot loop walks the directions and `Neighbors` stays for the rare
  callers; bit i of a `cell.Links` is direction i — `grid.DefaultGrids`, `Link`, `Way`,
  `Straight`, `Direction`, `Nearest`, `Exit`, `Rim`, `Shape`/`ShapeOf`), **`board/look`**
  (`look.Look`: `Flat`, `Swaying`, `Nothing`; `Dressing`, `Laid`, `Overlaid`, `Map`,
  `Renderer`/`NewRenderer`, `RenderState`, `MinGridCell`), **`board/ground`** (`ground.Heights`,
  `Profile`, `Cover`, `Readied`, `Surfaces`, `MeanOfCells`, `Shaper`, `RoadFits`, `RoadJoins`,
  `RoadSlope`), **`board/water`** (brooks, streams, rivers and fords out of a relief:
  `water.Drain`, `Network.Carved`) and **`board/network`** (`Network.Net`, `Route`, `Path`,
  `Ways()`). The machinery is `board/internal`: `terrain` (the cells' state, `Cells`, the entity
  system, the kinds' dictionary, `built.go`), `moments` (`Rules`: the hosts of `unit.Standing` and
  `cell.Now`, their passes, `Around`), `field` (the ground's cover and solid field), `grids`,
  `draw` (the simple map's bands and covers), `occupancy`, `boardtest`.
  A `cell.Kind` says which `cell.Domain`s it admits, whether it is `Solid`, how much it `Veil`s
  sight and whom it `Veils`, the share of its top speed a unit keeps on it (`Pace` in (0, 1] for
  a kind admitting anyone, `Paces` by domain through `Paced(domain, share)`, `PaceFor(domain)`
  the best among the domains admitted and paced, else `Pace`; `Define` refuses a kind admitting
  someone at no pace or a share over 1), whether it is `Graded` (spared the slope), its `Height`
  (with heights), `Shine` (glint per pixel), `Flow`, `Spread` (blending between kinds). **How
  fast a unit goes is the game's `cell.Mobility`** (2026-10-28, the user's design: a step's
  weight a share in [0, 1], 0 refusing the step, 1 full speed and nothing faster, factors
  multiplying, neutral 1): `Pace(cell.Step{From, To, Level, Kind, Domain, Slope}) float64`, given
  beside motion (`WithMotion(m, mobility)`), read through `cell.PaceOf(m, step)` (a Graded kind's
  slope read as 0, a share out of [0, 1] panicking with the kind's name) by the planner
  (`board.Read.Pace/Flat/Sloped`: a step costs its length over the share, h the grid's distance),
  the standing pass (`Pace.Share`, the ground under the unit at the slope of the step ahead the
  way it goes, `grid.Nearest` + `Toward`, so what the planner allowed the unit walks) and the
  driving (a step at 0 not driven, under gravity too). `cell.DefaultHiking` is Tobler's hiking
  function cut at the flat (`Hiking{Rate 3.5, Shift 0.05, Free: Air}`: min(1, exp(−Rate·(|s +
  Shift| − Shift))); half the pace at a rise of 1 in 5, a descent never quicker than the flat,
  Air at 1 over anything); a game's own domain bits tell its classes of unit apart (board/
  topography's `HighlandDomain`, its `madeira` refusing |slope| over 1 to walkers and over 2 to
  highlanders, custom-map's `hillside`). Steep is the relief's (`Map.Rise`), never a kind's;
  what it does to a unit is the mobility's. A save from before 2026-10-28 holds costs where
  shares are read and panics at its first step.
  A `cell.Way` is a second layer on every cell entity, a band through the cell's middle whose kind
  decides who may cross it (`Way.Over`), laid by the painter on the GPU; ways curve round the cell's
  middle, a way's `Fade` has it fade out (a river into the sea). Every cell is an entity for good,
  without a `world.Base`: `Plot`, `Ground` (kept apart so an effect ending restores the kind
  alone), `Way`, `Built`, made at Setup out of the seed or found after a load; `Version` and
  `CellVersion` count every change (writes, effects on cells, `Board.Touch(c)`) — of an effect
  only as it begins or ends, and only one the terrain changes under (`Plugin.mattering`: shown as
  a cover, granting tags or turning `cell.Ground`, `Floor`, `Wall`, `Rail`), never one turning
  another's knob alone (`weathering.Wring`): each change repaints the topography's surface,
  ~2–3 ms a time over Madeira (2026-10-17, the uplands' rules first showed it). A cell plays the
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
  `Lift` — except one carrying a `motion.Drift`, which gravity carries.
  Every tick, after collision's `RunPlan`, `board.RunPlan` reports a `Standing` (cell under the
  centre, its kind on the unit's level, the box, the domain, `Airborne`, `Feet`, `Level`,
  `Reach`) to its rules: `unit.Fallen()` a land unit fallen into water or a hole (never in the
  air nor on a deck), `unit.OnKind(k)`; the reaction is the game's (`act.Self.Do(world.Despawn{})`;
  `board.Plugin.WithLog(log.Default())` writes a line each fall). The same pass writes every
  unit's `steering.Pace{Share}`. Then, while there is a rule of it, every cell as a `cell.Now`.
  The board names its places (`places.go`, kinds in `internal/moments/around.go`):
  `cond.At(board.Around(n)).If(x)`/`act.At(board.Around(n)).Do(cmd)` turn to the cells within
  `n` rings of the box (0: the cells under it) or of a cell and the entities standing on them
  (`cell.On`), each cell once, in a rule of either moment; `board.Ahead(n)` to the cells one to
  `n` steps in front of a `Standing` (`Heading`, its `Vel.Dir`; `grid.Nearest` + `Toward`,
  stopping at the edge), a `cell.Now` refusing it; one kind of places a rule.
  **What stands on the board is an entity**: `board.NewEntities[Row](brd, board.Shape{Size,
  Width, Height}, at)`, `Define(name, extra...)` — a box at the cell's middle, `world.Velocity{}`
  (one among the extra takes its place, its `Dir` the way a ford runs), `cell.On{Cell}`, with
  heights a `world.Z{Height}`, a `world.Lie` for a band; the game gives the rest (a plate's
  `Collider{}` without `Physics`, a sensor; the roles). Its knobs: `cell.Floor{Allows, Height,
  Thickness, Pace, Graded, Links, Width}` (carries the domains over whatever the ground takes: a
  trapdoor over a pit, a ford at its `Pace` (0: the ground's; of several floors the fastest),
  a bridge `Graded` at its deck's `Height`; with a
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
  `board.NewUnits[Row](brd, board.Shape{Size, Width, Height}, at)` defines a game's units, on a
  board given motion and a mobility (`WithMotion(m, mobility)`, else it panics):
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
  `Plugin.World()` is the world under the board. `Surfaces(s...)` (in the Figures link too)
  notes `board.Surface`s, whose `Look(a, b)` runs after the tiles' on the board's atlas and sets
  `a.Grid(board.Line{Color, Width})` and `a.Holds(width)`, a later setting winning; none draws
  `DefaultGridLine(g)` (a square grid black at 115, a hex one grey 20 at 184, a pixel wide) and
  `DefaultHoldWidth` (2). `SetAtlas` hands the line to `Res.Render` (`look.RenderState.Grid()`,
  premultiplied), which the flat grid, the terrain and the hex prisms all read; a foreign atlas
  draws the defaults. The flat grid composed tile by tile draws every side once, in lines. The
  occupancy (`internal/occupancy`, `KeepSystem` after the units' pass) lets go of whoever left the world
  and has every unit hold the cell it stands on, each on its level (a walker on a deck and one
  under it share a cell); one holding nothing lets go of the cell it was `At`, is `At` the one it
  stands on with `unit.Entered` on; one mid-step holds both and is left to navigation.
  `board.Holds{Shown}` (`Plugin.ShowHolds`, `HoldsShown`) outlines every cell held, lying 0.15 over it, in
  its holder's hue, as wide as the surface says. The grid's line and the holds are
  `Res.Settings{Grid, Holds}`, saved; `Grid{Shown}` (`ShowGrid`, `GridShown`); neither has a
  key, the settings screen sets them.
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
  `ground.Cover` (`Plugin.Cover`); both read the cell entities whenever asked. A flat board is lit
  by the light plugin (`look.Renderer.WithLights(world.Read.Lights)`) and drawn on the GPU out of
  its tile map (`look/tiles.go`, 2026-10-23): an image of its cells, a 2×2 block a cell (sprite,
  shade taken, flags, the way's sprite and links, covers, width, fade, sway, height, material),
  and one of the sheet's rectangles; a changed cell (`CellVersion`) rewritten with its
  neighbours, all of them past an eighth. Per pixel through the camera's rays the plain pass
  (`tiles_plain.wgsl`: square cells nothing sways on or beside), the full one (`tiles.wgsl`:
  every hex and the cells near what sways, the 3×3 round in the order the cells are laid, each
  tile with the material over it, `look.Overlaid`, and its covers right after it), then the
  covers of the plain cells and the ways as instances a cell (`tiles_covers.wgsl`,
  `tiles_ways.wgsl`: the quarters, the bands and the hub's fan built in the vertex shader, a copy
  across each seam in view), the clouds' shadows and the cells of a material last
  (`tiles_material.wgsl`); lit per pixel by the frame's light — the ambient, the distant light on
  level ground less the shade the frame holds, the lamps', clamped to 1 — as each cell's
  `cell.Kind.Takes` says, a cell taking none left as drawn; the sectors seen at their levels
  (`seen.go`). Nothing is composed on the CPU: a camera without rays or a board of no world
  draws nothing; `Renderer.Whole` draws the whole board from above into an image),
  and takes the weather through `board.Plugin.WithAtmosphere(atmosphere)` (`lit.go`, reading
  `board.Atmosphere{DrawnAir}`, the weather as published: what sways leaning, the clouds' shadows laid on its ground right after
  the tiles, `draw.Clouds`), called in a scene before the first frame; without either it is drawn
  as it is. The
  board's own Map is the simple map — flat, every kind in its sprite from the game's atlas
  (declared by `ctx.Looks` from the tiles, the sprites the cell's bounds across), the ways as bands
  cut out of their kinds' sprites, a step at its kind's cost; `plugins/topography` is the other.
  Depends on `world`, `motion` and `collision`.
- **`topography`** — a map in relief: `topography.NewPlugin(world, board, light, Config{Shaping,
  Care})`, made right after the world, the light and the board (so a world with heights); the
  board's Map: `Rise(from, to)` from the lattice's corners (a hex grid: the cells' altitudes
  over the step), what a unit makes of it the board's mobility's;
  refuses a wrapping world; hands the light its relief (`light.Plugin.OverRelief`, the heights as
  they stand for the bake, the frame's under the live shadows). It publishes its relief once a
  frame (`relief.CopyInto`, the heights copied as their version moves), its settings and the
  board's grid (2026-10-24); the terrain (`drawnGround`, `terrain.Config.Drawn`), the care's
  lines, the painter (`painter.Board`, a func for the relief), the hexes (`drawnMap`) and the
  billboards draw from them; the terrain encodes the heights as it composes, so the frame's
  `GroundSpan` is theirs. It is the cameras' `cameras.Cage` (`Free(x, y, z)`: over the ground, over the
  kind on its cell to its `Height`, out of a deck's slab; `Span()` the relief's extent; `Grain`
  near decks — a game hands it as `cameras.Config.Cage`), sets the world's Look
  (`billboards.Look`), the board's Map (`look.Nothing`: the ground is drawn on the GPU) and the
  world's Ground (its `Relief`: `At`, `Step`, `Altitude`, `SetHeights`, `Corners`, `Lift`,
  `Flatten`; on a square grid a lattice of corners the neighbouring cells share, on a hex a level
  per cell; living on its own entity as `Heights`, runs of `HeightsRun` 1024, seeded by
  `Plugin.Seed(heights)` at Populate, shaped by `Raise`/`Lower`/`Level` — Shift+=, Shift+-, L-drag —
  `Shaping`). The plugin draws the ground, a `render.Direct` in the opaque pass (`ground` or
  `hexes`, the one it has, given `ctx.Draws`): over a square grid `internal/terrain` (the lattice as
  a mesh, a depth prepass, the board painted flat by the painter on the GPU into albedo and water
  sheets sampled per pixel (`Painter.Surface`: what the shaders read of each cell laid into an
  image, `cells.go`, anew as the cell changes; `paint_ground.wgsl` the grounds running in,
  `paint_ways.wgsl` the ways' pieces in its vertex stage; the cells within two of a changed one
  painted anew, all of them past an eighth; the shores worked out by `paint_coast.wgsl`,
  `Painter.Shores`, and copied into the lattice on the GPU), lit per pixel by the sun and the lamps the frame holds (`lightWay`,
  `lampLightAt`) and shaded as the light baked it (`groundShadeOf`, `render.GroundShade`), each
  cell as its kind's `Takes` says (the bits in the lattice's green beside the wet flag,
  `Painted.Takes`, `takesAt`; how much it sways in the blue, `Painted.Sway`, `swayAmountAt`); a
  kind's `painter.Style.Detail` (2026-10-20) is a texture over its cells: the painter bakes every
  detail into one sheet (`Painter.Details`, `painter.DetailPx` a tile, `painter.MaxDetails` 16
  of them, each tile's `Across` in the uniforms `DetailAcross0..3`, bordered for a seamless
  wrap, its mean scaled to grey and its hue toned down)
  and paints the board's kinds beside the albedo (`Painter.Kinds`, each pixel its kind's detail
  in red, edged hard; the fourth draw image), and the shader (`detail.wgsl`, `grain` in
  `mesh.wgsl`) takes a uniform cell's detail from the flags' alpha (`Painter.CellDetails`,
  `painter.Dressed` where the cell's pixels may differ, by rule: a way on it or beside it, a
  ground running in — the kinds sheet is as big as
  the albedo and reading it every pixel cost ~1.2 ms a frame on an Intel UHD 620), there reads
  the kinds once blended — four alike blend to their own number — and blends the four pixels'
  details only where kinds meet (frame image `GroundDetail`, `U.DetailPx` 0 none) into a tone
  that multiplies the albedo, fading out below `detailNear` pixels a cell (no mipmaps; the
  details cost 1.7 ms a frame on an Intel UHD 620 at 40 px a cell, 265 to 182 FPS, 2026-10-20:
  ~0.4 of it the tile sampled minified, the rest the reads — the flags, the kinds where a cell
  is dressed, the tile; mipmaps in `render/internal/gpu` are the next step, for the far look as much);
  where a cell sways (`sway.wgsl`, `air.Weather.Sway` ported, gust fronts of `noise`, the
  amount and the wave worked out at the lattice's corners in `vs_main` and blended) the grain
  leans with `U.Wind`
  and the wave lightens its crests; a `painter.Shading`
  (`WithShading`: `Bands`, `ShadowTint`, `Warmth`, uniforms of the same names) bands the sun's
  light, tints the shade and warms the sun; it hands the frame the lattice as `render.GroundHeights`, which the
  light's bake reads — the clouds' cover at the corners, water on wet cells, the grid in the
  board's line, fog, a skirt of level ground to the horizon;
  in a world with sectors the indices laid sector by sector, each whole then coarse — every
  second corner within, the edge ring whole and stitched by fans, `sectors.go` `lattice.coarse`,
  tested crack-free — drawn by the ranges the camera's View sees (`runs`, joined where they
  follow, the skirt last) and fogged per pixel past `Sight − Fog` from `View.Fogging()`),
  over a hex grid `internal/hexes` (every cell a prism, its top from the board's tiles drawn
  whole from above, `look.Renderer.Whole`, and the painter's ways over them, `Painter.DrawWays`,
  `paint_ways.wgsl`/`ways_tops.wgsl`, anew as the board changes or every frame while water runs or glints; by
  sector the instances laid in runs, a coarse sector's tops alone, `topVertices` 18). The
  entities are billboards on the GPU (`internal/billboards`, standing upright in the world across
  the screen's level right, so as tall on the screen as their `Height` stands there and inside their
  solid (the boxes), shorter the steeper the view; tested against the ground's depth; lit as the
  world's `Lighting` hands each (`render.Illumination`: the sun's light on a face, `faceLit`, the
  lamps' at the point, the sun's share and the `Takes` bits in the instance, honoured by
  `billboard.wgsl`/`lying.wgsl` — `groundShadeAt`/`standingShadeAt`, the clouds' in the vertex
  stage); their shadows are the light plugin's, laid over the ground's depth); a bridge's deck and
  piers are `Baked` (a deck stands on its beam down its `Z.Height`). The views are the
  cameras plugin's (its entry); `camera.Rays` of its cameras give `camera.SceneTransform` for every
  GPU source here. Its parts are packages none of which imports the plugin: `relief`, `painter` (the
  board's Dressing in relief, the sheets, `Style`), `water` (the materials: the sea five octaves of
  value noise along x, roughened by the wind, a swell within `shoreReach` of the shore, Fresnel
  against the way to the eye; `stream.wgsl` a flow map), `terrain`, `hexes`, `billboards`
  (vectors from gram's `internal/vec`). selection must not import topography, even in tests.
  Everything that is the drawing alone — the billboard, the blocks' shading — is private. `world.Z`,
  relief and `Heights` are the world's heights, not the view: sight
  reads them in a top-down game too. **Layering: the world knows its entities and nothing else; the
  ground is the board's, the sky the atmosphere's, `render` generic.** A relief is lit by the
  light plugin it is made with (without one `light.White`, which the topography hands the
  frame itself) and stands under its `topography.Atmosphere{DrawnAir}` for the weather as
  published (`WithAtmosphere`; `liveSky` relays it to the terrain, the hexes and the billboards).
  Its water reflects the sky's own colours (`skyAt`). "Full care follows the frame budget"
  (`AdaptCare{On}`, `Settings.AdaptiveCare`, 2026-10-20): the plugin is a `cameras.Lever` given
  in `cameras.Config.Levers`, its care's reach stepped down the ladder (`careLadder`) before the
  pictures' resolution as the rate falls under the budget and up after it as it stays over.
- **`atmosphere`** — the time of day, the climate and the weather on the world's clock, over a
  world with heights alone (`NewPlugin` panics in a flat one); its root
  keeps no shaders and draws nothing of the world itself: every part is a package that never
  imports it. Drawn as published (`drawn.go`, 2026-10-24): `Publish` copies the weather
  (`Read.Air()` with the clouds' shadow flags, its dome by `air.Dome.CopyInto`, a patch where
  its `Version` differs from that buffer's), `Read.Sun()`, the heavens, `Running` and the
  Settings; the sky, the low clouds (`lowClouds`, setting `aloft`'s look from the Settings as it
  composes) and the drops read only that (`drawn()`, live before the first swap), the boards
  `Read.DrawnAir()`; `Read.Air` stays live for the weathering and the climate's rules. The
  clouds' setters and the lever write the Settings alone; the sky hands the frame the sun only
  where the composer has not lit it. Its HUD (2026-10-20, etap V, `hud.go`): `TimeOfDay()`, `Date()` (the season
  and the date, the moon's phase an icon beside them while `Running().Moon`, drawn from
  `celestial.MoonFace` with the phase's shade, lit on the right waxing), `Weather()` (the name,
  the temperature, the wind; absent without a running climate; it reads through a drawer of
  its own, so it goes in the tree a scene is made with) and `Bar()` (the three on one panel for
  `ui.TopMiddle`), `*ui.Element`s the scene places, each `Passing` (2026-10-26: the pointer
  passes through to the picture under, so the cameras' zones work under the bar as they are
  drawn over it); the old `HUD()` renderers are gone. Their
  words are read once a frame as the scene publishes, kept (`framed`) till the clock moves or
  the weather's name changes; the moon icon shows the phase the atmosphere published.
  `Reporter()` lists the daylight, the clouds and their shadows alone: the time, the season,
  the moon and the weather are the widgets' (a game without them adds
  `Calendar().Reporter()` and `Climate().Reporter()`). The settings screen's atmosphere section
  has the switch "Frozen light" and the slider "Hour" (the day's time as it goes, the frozen
  light's while frozen; moved, `Hour{At}` freezes the light at that hour, so the slider takes the
  time and the switch lets it go), the choice "Clouds" (the look), the slider "Clouds as bodies out to" (`CloudReach{Share}`,
  `Settings.CloudReach`, a share of the camera's sight from where it looks, `aloft.BodiesTo`:
  the bodies within it, a sheet beyond, the shader deciding per pixel where the line of sight
  enters the layer, `AloftBodiesTo`/`AloftMiddle`), the choice "Clouds drawn at" (a quarter, a
  sixth or an eighth of the pixels each way: `CloudsDrawnAt{Coarse}`, `Settings.CloudCoarse`,
  `DrawCloudsAt`/`CloudsDrawnAt`, `aloft.Renderer.Coarse`, a coarse image a size and coarse,
  `LayCoarse` from it), "Clouds follow the frame budget" (`AdaptClouds{On}`,
  `Settings.AdaptiveClouds`: the atmosphere a `cameras.Lever` down the `ladder` of rungs
  (coarse, reach) (4, 1), (6, 1), (8, 1), (8, 0.5) as the rate falls under the budget and back
  up as it stays over: the pixels first, the bodies' reach last and never under half the sight,
  the look never; the user's word, 2026-10-20: the clouds stand high and far, so dimming them to
  shadows or cutting the bodies near the camera showed too much), the switches "Clouds shade the ground" / "the figures" /
  "High clouds' shadows" (`GroundShade{On}`, `FigureShade{On}`, `HighShade{On}`) and a choice
  "Weather" of the climate's weathers by name (`SetWeather{Name}`; `Climate.Now()`
  the weather of the time, lane 0, as the system last set it) where the Config has any; a weather asked
  for while the clock stands (the settings screen's pause) is entered at once, the climate's
  system run with no time passing (`Climate.Asked`).
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
  the sun's own colour at its height on `Heavens.SunColor`; the sun of the hour is a `light.Distant` (`sky.DefaultSun` the one of a relief under no sky)
  the atmosphere writes each tick into the `light.Shining` beside the `light.Source` on its own
  entity (`shineRun`), the light plugin's to shine;
  `Freeze{On}` freezes the light alone, `Hour{At}` (a slider on the settings screen, no key)
  moves it; both `Res.Settings{Frozen,
  Hour}`, synced from the sky every `carryOut`, saved and re-applied in `Restore`. `atmosphere/celestial` (imported by `sky`, never the other way): `Heavens`
  (`Place{Latitude, NoonWay}.HeavensAt`), the sphere turned by the sidereal time, `SunPath`,
  `MoonAt`, `Phase`, `Stars()` (the Yale Bright Star Catalogue to V 6.0 in `stars.bin`;
  `StarField.Place` takes a catalogue), `MoonFace()` (NASA SVS CGI Moon Kit).
  `atmosphere/backdrop`: the viewport in the sky's colour behind the world (`render.Furthest`,
  `Image.FillBehind`: only where nothing was drawn), through a perspective the sky of the day in
  four parts, a `backdrop.Dome` (2026-10-16): the vault, the sun, the moon (`Size`, `Color`,
  `ByDay`, `Image`), the stars (`Catalogue` drawn as instances behind the sky, `Dots` its shader
  adds); `Earth()` part by part (`EarthVault`/`Sun`/`Moon`/`Stars`, `ScatteredStars` the dots);
  a part is WGSL to `shaders/sky.wgsl` (`SkyVault(s: SkyAt)`, `SkyStars`, `SkyMoon(s, d: SkyDisc)
  -> SkyShine`, `SkySun`) with `Reads` its uniforms and `Feed(v *Values)` setting them (from
  `v.Heavens()`, the heavens drawn under, never the live sky); without
  WGSL it draws nothing (a stub stands in). `Renderer.Dress` assembles the shader as the Stage's
  looks are declared, panicking on a uniform or a module-scope name two parts (or the library,
  `render.SharedUniforms`) declare; compiled at the first draw, cached by source. The clouds'
  layer stays the sky's own. A game's sky: `atmosphere.Sky{Look(d *backdrop.Dome, p)}` in its
  `sky.go`, noted in the Figures link (`a.atmosphere.Skies(dusk{})`), each Look in order, a later
  part replacing an earlier; none is the Earth's (examples/look/sky; board/flat and
  board/topography note `earth{}`). naga (v0.20.3 too) traps for a part: a `var` without an
  initialiser is undefined (start from `SkyShine()`); a module-scope vector `const` in a runtime
  expression, zero under v0.19, works since v0.20. `atmosphere/climate`: a
  `climate.Zone{Latitude, Factors}` (`Equatorial`, `Tropical`, `Mediterranean`, `Temperate`,
  `Cold`, `Polar`; `SeaCurrent`, `DrySummer`) is `Zone.Profile()`
  (`Mean`, `Year`, `Day`, `Wet` per season); the kinds of weather `climate/weather`
  (`weather.State`, `weather.Default`: clear, fair, cloudy, rain, storm); the weather now a
  `climate.Weather` on its own entity (saved with its dice), beginning in `Config.Start` or a state
  thrown by `Often[season]`, going from one of `Config.Weathers` to the next, blending the wind, the
  clouds, what falls and the `Temperature` (snow below 1°C), every step of the simulation under
  `clock.Simulate`; `Climate.Air()` an `air.Weather`. It hosts rules of `Weathering`
  (`host.StepRules`, about the atmosphere's own entity). `Change`, `Set{Name}` (the settings
  screen's "Weather" by name; the plugin binds no key to the weather). The
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
  temperature, `Rain`/`Snow` there). `Change`/`Set`: lane 0 jumps to its targets and the dome
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
  a level is copied out of the dome drawn and written whole as its window moves or a patch
  of it changes, drawn in place by gpu, see gram-render; the copies of a dome,
  `Dome.CopyInto`, share its picture): `U.AirLevelNA` (the block's first
  cell in the world, cell, block across the picture) and `U.AirLevelNC` (the rect read in), the
  finest level holding p, mixed with the next over its 2-cell band, one `textureSampleLevel`
  with `wrapped` a level, as `Dome.At` does on the CPU (a test keeps them within 2.5/255, also
  after the windows moved). Four `textureLoad`s a level with the ring's arithmetic cost the
  terrain, reading it every pixel, ~15% of topography's frame rate. Every cloud function takes
  the mass. **Two layers of clouds** (2026-10-16, stage 7c, reshaped at the user's eye):
  `air.Low` the heaps in **three bands over the ground's relief** (2026-10-17, stage 7c′, the
  user's design: the same show over peaks of 1900 m or 4000 m): under the peaks 0.68–0.79,
  covering them 0.74–1.05, over them 1.05–1.25 of the relief from its low ground (the tenth of
  the board's cells lowest, none under the sea) up to its highest (`atmosphere.Plugin.Over(board)`,
  `WithWeathering` calls it; `climate.Climate.Over`; `air.ReliefOf`: a relief under 1 km taken as
  1 km, so hills never pierce; without Over, 1 km). Each band holds places of its own
  (`Weather.LowBand`, WGSL `cloudLowBand`: noise 2520 wide cut by the weather's shares,
  `weather.State.Ceilings` under/covering/over, eased by the climate like the air, soft 0.08,
  drifting at 0.6 of the clouds' drift, so a cloud flying on rises or sinks over minutes). Bodies
  `cloudHeap(p, z, band, pixel, m)` (air's `clouds.wgsl`: flat base on the band's, top
  `pow(thick, 0.8)·(0.6 + 0.8·lumps)` in the band's deck, lumps two octaves of billow noise, never
  flat — "owieczki"; lumps up to 1.6× the deck, `Weather.Top(Low)` the highest anywhere); the
  marches narrowed to the bands at a line's two ends when they lie within 1260 of each other
  (`cloudLowRay`, `cloudRayBand`: a 200 m deck takes all the steps). A Weather without `Relief`
  (hand-built in tests) has one deck on `Bases[Low]`, 0.45 `cloudSize` deep, as before.
  `air.High` the shreds as thin sheets each at its own height (`cloudHighHeight`,
  `Weather.HighHeight`: from over the highest deck up to `Bases[High]` = max(6000 m, that +
  2500 m), `air.BasesOver`), found along a line by `cloudSheetCross` (8 steps, 4 halvings; the
  low ones' base crossed so for the flat look), never marched. Heights on `air.Weather.Bases`
  (`air.BasesOver`, set by the climate; `Base(Low)` the lowest base, `BaseSpan` the lowest and
  highest; a hand-built Weather without them has its clouds at 0: tests set them). Shares
  (`Mass.Layer`, WGSL `cloudLowMass`/`cloudHighMass`): low `C·(0.35 + 0.65·B)`, high
  `min(2C, 1)·(1 − 0.5·B)` in patches (`cloudHighHere`, `Weather.HighAt`, noise 2520 units
  wide): together or apart, shreds in fair weather too, heaps the more the sky fills. Heaps 4×
  smaller than at first (`heapSize` 787.5; the tile bakes cores at `q·heapTile/CloudTile`), each
  an ellipse (aspect up to 2.2, turned) with five puffs of many sizes on its rim (`heapCores`,
  `heapRoll`, the CPU the same). The shadow: `cloudsShadeAt(p, pixel)`, each layer where the way
  to the sun crosses it (`sun.xy/max(sun.z, 0.25)·(h − p.z)`, the low one's h its band's base
  over the point, corrected once where the way to the sun meets it, the high one's its sheet's
  over the point), none over ground above it, combined `1-(1-a)(1-b)`; CPU `Weather.Shade` within
  2.5/255 on the whole; `FallsAt` from the low clouds alone. The terrain works it out at its
  lattice's corners (`mesh.wgsl` vs_main, a varying; none on the skirt, whose huge far triangles
  showed as angular shadows on the horizon): per pixel it cost the topography ~1.2 ms a frame
  more for the high layer alone; the board's flat ground at its 8-px corners, the hexes
  per vertex. `atmosphere/aloft`: a layer seen from above (`aloft.New(weather, layer)`, a
  Direct at the layer's height; one a layer): the heaps marched 10 steps through their bodies
  (random offsets a pixel: an even pattern showed stripes; slices instead showed as stacked
  discs from the side), the shreds' sheets crossed once; thinning into the ground
  (`groundHeightAt`, checked only under the highest ground); through a perspective faded along
  the horizon and hazed to `U.Fog` by the visibility over the eye; from a perspective eye under
  the layer's `Top` the high ones none (they are the sky's), the low ones as bodies **before the
  ground** (`aloftBelow`, 12 steps, from where the line of sight enters the band as far as it
  stays under `groundHighest()`, stopping where it goes under the ground; the sky's cumulus draws
  those past the highest ground, `backdrop.Clouds.Before`, so neither doubles nor leaves a gap;
  not drawn at all while the frame's ground, `GroundSpan`, stays under the lowest base); a
  `Coarse` (4) of the ground's pixels each way: over a coarse ground in the pass over the
  figures (`render.Target.Coarse`) the image is `ceil(round(screen/Coarse)/coarse)`, kept by the
  screen's size and its own, one a frame; `lay.wgsl` maps the screen onto it exactly each way
  (`LayCoarse` a pair) and lays it at every pixel against the screen's depth (measured
  2026-10-20, R1, 1440p, full cover from above: 6.1 → 3.7 ms over a ground at 0.6; the march
  falls with the ground's pixels, the lay stays at ~1.45 ms, a full screen's fill). The low
  clouds lie as deep as their first dense sample: the coarse
  pass writes `frag_depth` (through the scene's transform, `AloftViewProj`, as the drops) into a
  Readable depth of its own, the lay (`lay.wgsl`) reads it (`depthAt`) and writes `frag_depth`
  tested against the world's (`t.Depth`): the ground or anything nearer hides them, they hide
  what stands behind (a hill in front of the band's entry, which the march does not see, too).
  The high ones are laid by `DrawImage` — but the atmosphere draws only the low layer from above
  (2026-10-17, the user's word: from the tactical view the high sheets veiled the whole map like a
  fog); the high ones show on the sky and as shadows. What greyed the tactical view was the
  ground's haze: the terrain and the hexes took any `camera.Eyed` for an eye, and a camera from
  above, whose `Eye()` says no, hazed from its field's start (`Transform.Eye` = `f.Origin`), the
  map all fog but its corner. Now `air.Weather.Seen(cam, hazed)` decides for both, hazing only
  where `topography.Plugin.WithHaze(true)` asks (`topography.Haze{On}`, no key,
  `Res.Settings.Haze`; off by default
  through every camera, the user's choice, 2026-10-18; the billboards' `Hazer` follows it): a
  camera with an eye (riding, or free in perspective) by its eye's distance, one looking from
  above, with none, by the air over the ground up to the high clouds' base along its lines of
  sight (`HazeUp`, `HazeWay`, `hazeDepth` in `mesh.wgsl` and `hexes.wgsl`). The sky's own fog
  (the backdrop's and the clouds' `Visibility`) is not switched. Seen from a valley, clouds behind hills
  lower than the band are hidden: before slopes they stand only where the slopes reach into it.
  `atmosphere.ShowClouds(look)` in `makePlugins` (the demos start `AllBodies`; `Res.Settings.Clouds`,
  saved), `Clouds{Look}` (a choice on the settings screen, no key):
  `FlatClouds` (flat sheets on the sky, one flat slice a layer from above), `LowBodies`,
  `AllBodies` (the plugin's start, the user's choice, 2026-10-16), `ShadowsAlone` (nothing
  drawn, the discs not dimmed, the fog as it is). From under them the sky heaps the layers
  `backdrop.Clouds.Bodies` names (`backdrop.WithClouds`, `shaders/cumulus.wgsl`, one pass, the
  heaps marched 20 steps, white tops and grey bases by how much heap is over the point, the
  shreds' sheets crossed, thin wisps, low in front, a quarter resolution each way into image2),
  the others flat (`CloudsFlat`); the sky is drawn in pieces whenever clouds show, for
  `cloudsLook` of both layers. `climate.Config.OneLayer` (board/flat): `air.Weather.OneLayer`,
  everything in `Low` at `Bases[High]`, its noise `cloudMix(torn, heap, billow)` as before 7c
  (`cloudLowAt(p, pixel, billow)`), shadows straight under (`cloudSunSlant` 0);
  `Weather.Base(l)`, `Weather.Share(m, l)` answer for either mode. Costs measured at 7c (topography
  iso, paused, UHD 620): the ground's depth read for the clouds from above ~0.6 ms (the opaque
  pass drawn into a readable buffer; replaced by `GroundHeights`), the high layer's shadow on the
  terrain ~0.7 ms (`Weather.HighShade`, on as the game starts at the user's word,
  `HighShade{On}`, the settings screen's, `Res.Settings.HighShade`:
  WGSL `cloudHighOver` none without `U.CloudsHighShade`), the second pass from above ~0.2 ms.
  At 7c′ (2026-10-17, 2560×1440, against 7c): iso and perspective +0.3 ms (the bands +0.1, the
  depth lay +0.2), a walker's eyes and look/sky looking up +0.7–0.9 ms (the clouds before the
  ground, drawn from under them for the first time). A Readable depth read by a target of
  another size was refit to it every draw (two texture allocations a frame, ~2 ms, its content
  lost): `render/gpu` now keeps it the size it was drawn at.
  The sun's and the moon's discs fade by both layers' covers where their
  ways cross them (`coverAlong`). look/sky runs no clouds (`Running` without `Clouds`). The
  clouds' tile (`air.Tile`) keeps a pixel of wrap round every level: reading it clamped half a
  pixel in left a seam where the heaps' cores rise steeply.
  `atmosphere/precipitation`: what falls (only from the low clouds: `FallsAt` reads the low
  layer; WGSL `airAt` without a dome gives the frame's `AirFalls`), drop by drop in the world
  (`Drops`, `NewDrops`, 2026-10-16, the user's word: realistic, with perspective): every drop a
  point of a lattice fixed in the world, in three nested boxes round where the eye looks (one,
  as tall as the air under the clouds, through a camera from above), falling with the clock and
  leaning with the wind, kept where `FallsAt`-like rain falls at the point it left the cloud,
  drawn as a streak a few tenths of a world unit long trailing up, 1.4 px wide, through the
  scene's transform with the depth test (a dot from above, a short line from the side), gone as
  it shrinks under ~0.75 px. Shafts of rain seen from afar were tried and dropped: from the side
  the air between the sea and the clouds' base showed as rectangles over the horizon, and laid
  coarse over the ground they showed through the hills; screen-space streaks before the eye were
  dropped as unreal (the same at any distance).
  `atmosphere/weathering`: what the weather does to a board — `weathering.Config{Snowy, Ice,
  Water, Sway, Swaying, High, Seed}` names the game's kinds, `New` defines three effects
  (`Alter[cell.Ground]`: snow, ice, sway), laid by its own system once a second of game time (snow
  in drifts where it snows in the frost, `FallsAt` on the cell, melting once warm, ice from the
  shore below −3°C, what sways swaying above a wind of 15);
  `atmosphere.Plugin.WithWeathering(board, cfg)` as the atmosphere is made (it calls `Over`).
  **Clouds at the peaks** (2026-10-17, stage 7d, the user's design: the piling is the climate's
  Go, the rain and the gathering the game's rules over the board's cells): the masses carry their
  weather's damp (`weather.State.Damp`, `born` gives it, no new saved field); the weathering walks
  the board's cells a share a step, all of them a second (`weathering/walk.go`, the cells'
  heights kept and worked out anew every 8 s), noting their ground and their `Wring` on an
  `air.Land` (squares of `BoardCell`, the most of each, turned in once all are walked);
  `climate.Climate.OnLand(land)` has the climate run `air.Dome.Lift` every 4 steps over level 0:
  ground reaching 0.7–1 of the low clouds' base there (`LowBand`, read off a grid every 4 cells)
  hands as much of the low layer as the wind carries past in dt to the cells upwind, which keep
  it while they have room (`Mass.WithLow` keeps the high layer): a bank piles up before a ridge,
  spills over it, the lee clears; the land's `Falls` wrings rain out of the low clouds over it,
  draining their damp (a rain shadow past it), easing back to the cell's state's falls past it;
  its `Gathers` turns damp into low clouds, never clouds out of dry air. ~0.2 ms a lift at worst
  (all land high and clouded). The rules: `weathering.Overhead` (`host.About`, no places — the
  board's `AroundPlaces` takes its own moments alone) told each cell once a second (`RunWhere` over
  the block's walked rows), words `Clouded`, `Damp`, `Reaches` (ground over the band's base),
  `Hours`; the knob `weathering.Wring{Falls, Gathers}` given to every cell (`Exclude` + `cb.AddOne`)
  and saved with the atmosphere's components (ceremony saves a knob's originals: one saved under an
  effect goes back to them as the effect ends after a load). Madeira's uplands (earth, rock, forest
  play `UplandRole`): rain on the windward slopes (`Reaches(0.9)`, `Clouded(0.6)`) and clouds
  gathering of an afternoon (12–17 h, `Damp(0.3)`, `Reaches(0.7)`), effects lasting 2 s put anew
  each second: in rain ~1000 cells wring, in fair weather the heaps under the peaks pile up to 0.9
  within a minute and shower over a dozen. **The weather in places** (2026-10-17, stage 7e, the
  user's word: a map wholly under cloud loses the one looking at it): the knob `climate.Spread{Rain,
  Storm, Fair}` on the atmosphere's entity (`comp.Const(climate.DefaultSpread())` in
  `world.NewSelf`, saved with it; the climate lists it among its `LoadComps` too, its query
  registering the type) — rain on a quarter of the world at most (the storm's included), a storm on
  an eighth in its middle, fair breaks on up to a quarter, fewer the cloudier the spell was thrown
  (`Fair·(1−f)`, f where `CloudsTo` fell in its range). `born()` lays a lane's weather in parts by
  the rank of a periodic noise as wide as the world, drifting with the clouds (`climate/spread.go`:
  `places`, ranks from its sorted samples, so any stretch as wide as the world holds exactly the
  shares): storm ⊂ rain ⊂ cloudy, fair at the other end; each part grows out of the one before over
  0.02 of rank inside its own share; its own part the lane's thrown numbers, the others the middles
  of the `weather.Default`-named states (a climate without them: over all, as before); a cell's
  `States` the part's. The places are patches ~1/4 of the world across: on a world only a few of the
  dome's 128-unit cells wide they blur (tests measure over 6400×3200). The panel names the weather
  of the time (lane 0) and, where it differs, the one over the middle ("rain (here cloudy)"). The
  rain the uplands' rules wring out (7d) is beyond the knob. `atmosphere/air`: the weather as drawn
  (`Weather.Frame`, `Sway`, `Cloud(layer)`, `Shade`, `Haze`; the clouds' periodic noise baked once
  into `air.Tile()`, the frame's image `CloudTile` (2026-10-16), which every cloud shader looks up
  per pixel (`cloudTileAt`); `Weather.Frame` hands the frame the tile, `Fog`, the sky at the
  horizon, `SkyOverhead` and the layers' `CloudBases`). The plugin draws the sky and what falls
  itself (`ctx.Draws`); its sky writes the sun of the hour into the light plugin and hands every
  frame the weather; the clouds' shadows are drawn by whoever draws the ground (the board's flat
  ground, the terrain, the hexes) and by the figures, as `atmosphere.CloudShadows{Ground, Figures,
  High}` says (`CastClouds`, set in `makePlugins`; `HighShade` the high ones; uniforms `CloudsShadeGround`,
  `CloudsShadeFigures`).
  `atmosphere.Running` (`Config.Running`, `Plugin.SetRunning`) switches the day, the weather's
  changes, the wind, the clouds, what falls, the weathering, the stars and the moon, all on by
  default, not saved.
- **`collision`** — collision detection over `world`'s space, one `collisionSystem` a tick. An
  entity collides exactly while it carries `Collider`; the system settles every `Collider`'s
  `Base.Caps` (`CanCollide`, `Static` for an immovable `Physics`, `Sensor` for none) and rebuilds
  the space when any changed; two colliders touch only where their `world.Layers` meet and, with
  heights, where their `collision.Band`s overlap. The tick is one `collide.Engine.Tick`
  (`github.com/kjkrol/aabbworld/collide`; the system is its `Handler`): the engine pairs whoever
  may touch within a step, tests the pairs, pushes the overlapping apart in `Base` where the
  boxes lie (aabbworld runs, etap 3: `moved` writes nothing; a push its later passes leave over
  worse ground, `response.Worse`, goes back to the box the tick found, kept at the entity's
  first `Touch`); one pushed out through an open edge, already where `Left` says, is motion's leaver
  (`motion.Plugin.Leave`).
  Overlapping boxes touch: the box is the shape. **A pair parted stays met while it touches**:
  the pairs of the tick before that the engine did not push but whose boxes lie within `restGap`,
  layers and bands meeting, neither swept, get a `Contact` with `Resting` on both sides, its
  `Normal` the face they touch at (`faceOf`, as a struck pair's), no impulse, and a `Meeting`; a
  resting contact is no strike and no count. An entity carrying `Physics` (`Mass`, `Restitution`)
  is pushed out and bounces (an infinite `Mass` is a wall, an unnamed one weighs the box's area,
  `Physics.weight`); one without is only detected (a trigger). The push apart is shared by weight:
  `Touch` hands aabbworld the first side's share (`response.Share` of the inverse masses), and a
  side `Footing` holds over worse ground gets share 0. An impulse that slows a steered
  entity slows its `steering.Course.Speed` in the same share (`response.Side.Speed`), walls too:
  the steering goes on from what is left, the ground's `Pace` kept apart. With a `collision.Field` (the board's solid cells,
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
  `collision.ContactStats` with its `Reporter`; `WithLog`). `Boxes{Shown}` (a
  `Choice` on the settings screen, no key, `Res.Settings.Boxes`, saved) draws every
  entity's box in red, `BoxesShown`: `Bases` (the footprint at its altitude, after its figure in
  the order, under its own sprite where there is a depth), `Solids` (the twelve edges from its
  altitude to its top, `Frame.Edge`, only in a world with heights), `NoBoxes`; the game hands the
  plugin to players for the setting. Made `collision.NewPlugin(world, motion)`; depends on `world`,
  `motion`.
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
  route preview passes it too). A search reads what it asks of a cell, its kind on the ground
  priced for the search's domain, its raised deck, its bare ground, once, into a table by the
  cell's ordinal stamped with the search (`pathfind/reads.go`: nothing may change the board
  while a search runs; every public method of the `Finder` begins one); it walks `Directions()`
  with `Toward`, never `Neighbors`, asks `DiagonalNeighbors` only for a `Corner` direction and
  `Along` only where the bare ground would price a slantwise step otherwise; its solvers have
  room for every step of a node (`astar.WithSuccessorCapacity`) and slice dictionaries by the
  cell's ordinal (`astar.WithIndexedSliceDict`: the plain solver `CellCount()+1` slots, the
  leveled `2·CellCount()+2`, a cell's two levels side by side, then the goal, then a start off
  the grid; astar resets them in O(1) by a stamp); each solver writes its route into a buffer
  the `Finder` keeps (`astar.SolveInto`), copied into the caller's `dst`, so a search allocates
  nothing once they have grown: a route leaves the Finder only through `dst` (a `Path` holds its
  steps in its own array, the kept answers keep Paths by value) (2026-10-20, etap 2a: a route across 200×200
  94 → 18 ms, 14,740 → 19 allocations; a crowded tick of 1,000 walkers 306 → 104 ms). A route
  search asked again is answered as before, unrun (`avoiding.find`, wave 3): the answer kept by
  its question (start, target and level, domain, `pathfind.Body`, the cells gone round as a set
  less the target, round or any way) till the board's `Version` changes (`keptAnswers` 4,096,
  all let go when full; a finder asking the occupancy keeps none); units whose four learned
  cells rotate ask a set again 2–10 ticks later (crowded 1,000: 94.9 → 40.3 ms, then 37.6 with
  the slice dictionaries). Whatever changes what a search reads must count in the board's
  Version before navigation's next search. `TestRoutes_AsPinned` pins every route, step and price on three
  boards (square with bridges and relief, a torus, hex) to `testdata/routes.golden`: a change
  meant to keep the routes keeps it; one meant to change them rewrites it with `-update-routes`
  and says why. The keepings' passes allocate nothing per unit (2026-10-20, etap 2b): the body
  index walks the cells under a box through method values bound once (`bodyIndex.near` hands the
  bodies back as indices, valid till the next call); `takes`, a route round `Avoid` (`avoiding`)
  and `cellKeeping.held` keep their state in fields a method value bound once reads; a `Leg`'s
  `cells()`/`held()` are arrays with a count; `aside` only places the cells (a scratch
  `[8]scored`), whose ground `stepAside` asks in that order, stopping at the first open one. A
  crowded tick is the Detour's searches (every unit refused a held cell plans round what it
  learned, each tick). A search that finds nothing waits (`MoveOrder.Retry`, `Fails`): round the
  cells learned, or for any route when none are, it is not run again from the same cell round the
  same cells for 0.5 s, 1 s, then 2 s; meanwhile the plain route is taken, or with none the unit
  waits, its target still given up at `targetWaitTimeout`; a way found, the next goal, a new
  order or another target start it over, another cell or another cell learnt lets it run at once.
  **Clearance**: a unit is no bigger than a cell's side
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
  slantwise bridges); `price` is what a step costs: its length over the share the board's
  mobility gives it (`pathfind.Pacing`: `board.Read`'s `Pace`, `Flat`, `Sloped`), a share of 0
  the step left out; on a flat board the share is read once a cell and level a search into the
  `read`'s `rate` and multiplied as before, over a relief the strategy is called per ungraded
  step with the ground kind kept by ordinal (`kinds`, allocated at the first sloped search;
  `BenchmarkPathFinder_Terrain/relief` is that cost); h is the grid's distance, admissible since
  no share exceeds 1 (`least` is gone); a slantwise step not `Along` a way paced by the
  destination's `Bare` ground, so roads are followed round their bends. A navigated unit carries a
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
  slop (`control.Context.Dragged`, `DragSlop` 4 shown px) a `LookAt` every move while the player
  has a selection (`selection.Read.Any`; without one the drag is left to the cameras' grab, which
  yields to it; 2026-10-27); Shift appends. `Spacing` (`WithSpacing`; `AutoSpacing`) is how units
  keep apart through the `keeping` seam (`cellKeeping`, `bodyKeeping`), one
  `navigationSystem.Update` loop; both plan routes blind to the others. What a unit does about
  another is rules of the moment `Touch` navigation hosts (a `host.PairRules[Touch]`; words
  `PushedByAlly()`, `Blocks()`, `WaitedLong()`…; `crowd()`, StarCraft II's, added at Install unless
  `WithCrowd(rules...)`): `BodySpacing` the Collider's contacts (a `Resting` or `Sensed` contact
  bumps nobody; every unit touched, struck or at rest, is `bodies.Member.Touching`, and
  `bodyKeeping.Steer`'s `clearOf` never moves the unit into them: facing into them it turns on
  the spot, straight through them it stands, a turn through them goes round their far side), `CellSpacing` a step refused; handed `GivingWay`, `LastGoal`, `Ally`, `Groupmate`
  (`LastOrder.Group`), `OnMyGoal`, `HeadOn`, `Room`, `WaitedOut`/`Cornered`. The rules' commands
  (`StepAside`, `Detour`, `Pass`, `Hold`, `Settle`, `Stop`; `Aimed` at the other) are carried out
  for that unit alone, the engine's rules kept there: `stepAside` goes where the ground takes the
  unit and its slope leaves it `YieldShare` (a third) of its pace on the level (`stepsAside`,
  `asides`: a flat forest is stepped into, a cliff not), nobody standing; the last word stays navigation's
  (`stallAfter`, `maxStalls`, `bumpInterval`). A group gets its spots from `bodyKeeping.place` and
  goes cell centre to cell centre. The board has every unit hold the cell it stands on, navigation
  the cells of the steps in progress on their level (`Path.Decks`, a `Leg`'s `FromDeck`/`ToDeck`); a
  unit standing off its route plans again. The driving plugin gets its `driving.Keeping` from
  navigation (`handKeeping`); a unit whose `steering.Driven.Steers()` gives its order up in
  navigation's pass; navigation's RunPlan comes before the driving's. The renderer of routes
  (`internal/routes`, made in `NewPlugin`): every selected unit's goals as its outline lying 0.8
  over the goal's ground and, on `Routes{Shown}` (the settings screen's, no key; `ShowRoutes`,
  `RoutesShown`, `Res.Settings.Routes`, saved), its routes as thin lines at the ground's
  depth; `WithRouteStyle`.
  Facts told a unit with a tree: `Blocked`, `Arrived`; `MoveOrder.HitUnit` (entity ids start at 0:
  never use 0 as "nobody"). Depends on `board`, `world`, `motion`, `selection`, `driving`.
- **`entity`** (core) — what every entity carries: `Base`, `Position` (`StepReach`, `MaxStep`,
  `MaxSpeed`), `Velocity` (`Dir`, `Value`, `Side`), `Z`, `Layers`, `Drift{X, Y, Z}` (the velocity
  it is carried at beyond its walk, written by gravity alone), `Eye{Height, Angle, Vertical,
  Tilt}` (where one that looks looks from and how wide; `Vertical` the field up and down, 0
  every height; `Tilt` how far the eye is turned up, radians, written by the riding camera;
  `Eye.Level(z)`, `Eye.Within(lo, hi)`; read by vision's cone and the first-person camera
  alike), `Lie`. A leaf; the world re-exports them as type aliases. How an entity casts and takes
  its shadow is the light plugin's `light.Shadow`.
- **`motion/steering`** — `steering.Steering{TurnRate, Reflex, MaxSpeed, Sprint, Accel, Brake,
  V0, Halted}` the knobs, `steering.Course{Want, Pending, Delay, Speed, WantSpeed}` the state
  (given to every unit by the roster and by the system where missing), `steering.Helm{*Steering,
  *Course}` what steers (`Request(heading)`, `RequestSpeed`, `RequestSprint`, `RequestBack`,
  `RequestSide`). `steering.System` runs in motion's every step before it moves: turns `Vel.Dir`
  towards `Want` by at most `TurnRate` a tick after `Reflex` ticks (`TurnRate` 0 turns at once),
  writes `Vel.Value` from the profile (collision lowers `Course.Speed` with the speed an impulse
  takes); `Halted` holds speed at 0. `steering.Driven{Ahead, Turn,
  Sprint, Face, Side, Look, Flown, Climb}` is an entity steered by hand, two writers: the driving
  the hand's (`Ahead`, `Turn`, `Sprint`, `Face`, `Side`), the topography's camera system the eye's
  (`Look`, `Flown`, `Climb`); `Steers()`. `steering.Pace{Share}` the ground's share of the speed,
  the board's mobility's, 0 where it refuses the step ahead.
- **`world/view`** — `view.View{Bounds, Culled, In}`, `view.EntitySet`, `view.System`. In a
  world with sectors `ViewFor` makes `view.NewSeen(cam, sectors)`: every tick each sector's
  `view.Level` — `Hidden` beyond every band (`Off`), past the camera's sight from its
  `camera.Viewpoint` (horizontal distance to the sector's nearest point) or off the picture
  (`cam.Visible(box)`); `Fogged` within the fog band; `Coarse` where `camera.ScaleAt` at the
  nearest point (z 0) is under `Detail.Coarse` (0, behind the eye, counts as near); else
  `Exact` — then the space queried by the boxes of the sectors seen. `View.Level(x, y)`,
  `View.Sectors(each)`, `View.Fog(x, y)` (smoothstep over the band). The renderer's query leaves
  `sector.Off` out, so even an unculled View never draws it. `world.Plugin.Seen()` is the panel
  line "Sectors seen" of the first camera's View.
- **`clock`** (core) — the tactical clock, made and run by the world (`world.Plugin.Clock()`):
  game time is the sum of the simulation's steps, `clock.State{Time, Tempo, Paused}` on the
  clock's own entity, saved. P and Shift+P the tactical pause, ] and [ the tempo (`Config.Tempos`;
  sub-steps of one length, or one longer step with `Config.BiggerStep`). A plugin's `RunPlan` is
  the interface part of its tick; what simulates it hands to `clock.Simulate(c, ctx, d, block)`,
  and the engine's plan wrapper hands the game's `Update` `Clock.Settling(rc)` (so `OnSync`
  settles after every Sync of the interface too) and replays every block after it as many times
  as the tempo says (`Clock.Replay`; a plan written by hand, a test or a bench, does the same); `Clock.Behind` lowers a tempo above 1 after 30 slow frames.
  `clock.Phase` tags on the clock's entity, `Clock.In(phase)`, `Clock.Entity()`, `Reporter()`,
  `HUD()` (a `ui.Drawer` and a `plugin.Publisher`, its line read after the ticks), `Shown` (game time run on past the last tick by the real time held toward the next;
  `Clock.Pending`). Rules of a `clock.Moment{Last, Now}` fire every step (`clock.Every(period,
  offset)`, `clock.At(t)`; by clock time, never again after a load), about the clock's entity.
- **`selection`** — a `Select` command (ids, or a world box, additive or not) sets the
  `Selected` tag on entities carrying `Selectable`, both bits of `selection.Family`
  (`Plugin.Tags()`; every unit carries the family, off, and is told `Allow{}` as it is made;
  `Plugin.IsSelected` a drawing rule's condition). `DefaultBindings()`: a left drag selects
  (Shift adds), the left button held a `Marquee` (drawn by its renderer), C `FollowKey` (a
  `cameras.Follow` of the issuer's one selected unit, `Read.Chosen(by)` — `Read.Any(by)` whether it
  has any selected, navigation's right drag asks — for the camera the player looks through), V
  `RideKey` (a `cameras.Ride{Entity}` of that unit, held `In(camera.Loose)`: the cameras' V takes it
  on from there; 2026-10-27),
  `Hover` every tick (`control.CursorOver`: the entity under the cursor
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
  chord with modifiers fired while it was down: Shift+P is no throw); these three fire with
  Shift, Ctrl or Alt held (the key's own modifier aside) only when the binding is `InChords()`
  (`Binding.Takes(mods)`): the cameras' scroll and the head's tilt leave a game's own Shift+W
  or Shift+R to it, driving's keys and `JumpKey` are in chords (sprint,
  a jump while sprinting, walking while a grenade charges) — `ButtonPress`, `Drag`,
  `Wheel`,
  `ButtonHeld`, `CursorAtEdge`, `CursorOver`, `CursorMove` (the cursor's fire only while the
  window has it, `InputEvents.CursorInWindow`, the engine's from the window's enter and leave;
  a zero `InputEvents` has no cursor, so a test or a probe moving it says `CursorInWindow:
  true`; 2026-10-27), with exact `Mods`
  (`Mods{}.Holding(key)`), `Build[C]` from a `Context` (`World`/`WorldBox` through the camera's
  `Picker`), `Give(trigger, label, cmd)`, `Binding.In(hows...)`, `With(tool)` — lives in
  `control`, so a plugin with commands never imports players. A player holds a tool in hand
  (`control.Tool`: `control.Take`/`Drop`; while it does, a binding `With` that tool takes its
  trigger from those of no tool). `Player.Bind` refuses two bindings on one trigger holding in one
  fastening unless one yields (`Binding.Yielding()`, 2026-10-27: `fire` runs the plain bindings
  first and the yielding ones only when none built — the cameras' grab under navigation's right
  drag); Setup refuses a command nobody defines. Local players through pictures of their own
  split the screen (`ui.Columns`); keys reach every local player, the mouse the one under it. The
  Scene hands input to `players.Plugin.Handle(events, runtime, composition)`: the bindings turn it
  into commands, `ShowControls` (K) and `ShowSettings` (F10) are carried out there (`carry`),
  the game's own `SceneKeys`
  (`Plugin.OwnKeys`: a debug toggle, no command) run; `p.GameBindings()` (K, F10, ~, F5 `game.Save`, F9
  `game.Load`, Shift+Esc `game.Quit`) give the engine's commands. **The settings screen**
  (`players.SettingsScene`, `settings.go`): opened by F10, built afresh of every
  `plugin.Tunable` among the handlers given to `NewPlugin`, in their order, a heading a plugin and
  a line a setting (`control.Switch` a `ui.Checkbox`, `Choice` a `ui.Dropdown`, `Range` a
  `ui.Slider` with its number beside, each named `setting:<section path>/<label>`; twin sections,
  the same settings under each as the cameras' profiles, side by side with a control a twin;
  every top section framed whole (`framed`, a `ui.Panel` bordered a pixel named
  `frame:<section>`, a block the columns never split), the title in the default theme's Large
  (`render.DefaultLargeFont`, Go Bold at 28; the layout and the theme are the controls screen's
  too, `menu.go` `menu`); as
  many columns as the width takes, fewer and taller otherwise, in a `ui.Scroll`; the cursor let
  go while it or the controls screen is up, a player riding or not); its commands
  go through
  `IssueAs(the first local player)`; it forwards input to its `ui.Scene` and holds the game in
  the clock's tactical pause (the ECS pause would stop the plugins' `RunPlan` and their queues),
  unless the players' own `Settings.Running` (`KeepRunning{On}`, a switch on the screen; the
  telemetry panel another, `telemetry.Panel{Shown}`); Esc (with no dropdown open) or F10 closes
  it. The players' `Res.Settings` is kept like any plugin's. **The controls screen**
  (`players.ControlsScene`, `controls.go`, a `game.Scenic` scene made in `NewPlugin`, so K
  works in every game; the shortcuts list till 2026-10-26) is a `ui.Scene` whose tree `Open`
  builds afresh (`ui.Scene.Replace`) as the settings screen's: the title ("Controls", riding
  "Controls: first person, riding in the unit") in its panel, then the groups — "Game" first
  (the Controls section and `SceneKeys`), then the plugins' keys by plugin, then "General" (its
  full-screen line `control.FullscreenKey()`: F11, or what the engine set for the platform —
  Ctrl+Cmd+F on a Mac) — each a framed section (`frame:<group>`) with a `ui.Heading`
  (`section:<group>`) and a line a binding, `controlsRow` 24 px: the trigger (`Written`) in a
  column as wide as the widest of them all, what it does beside, wrapped past `controlsLabel`
  300 px (`wrapped`, by the font's measure); `Props.Description` is shown no more. It holds the
  game in the engine's pause (no command to carry) and forwards input to its `ui.Scene` for the
  scroll. `players.RunPlan` runs last and empties the queues. Depends on `world`.
- **`cameras`** — every camera a game looks through, and all camera behaviour (the contract
  stays in the root `camera`, which `render` and `control` read). Every camera is
  `camera.Snapped` (2026-10-24): `Snap` copies it for the frame drawn (a relief camera's copy
  its own sticks, its ground's extent frozen, no cage read), taken by a feed as it is laid out;
  a copy is for drawing alone, no `Pick`, no command. `cameras.NewPlugin(world,
  cameras.Config{…})` makes no camera; the Config is the game's (in `makePlugins`) and says what
  the keys may do — `Turn` (Q/E), `Tilt` (R steeper, F flatter), `Perspective` (Tab switches to
  it and back, V rides Inside, the mouse looks) — and how a world in relief is drawn (`Cell` required, `TileW`/`TileH` 64×32,
  `HeightUnit`, `Headroom`, `MinPitch`, `FieldOfView`) and where the cameras may be (`Cage`, a
  `cameras.Cage{Free(x, y, z), Span()}` — the topography is one, a game may give its own;
  `Grained{Grain(x, y)}` optional). The zero Config is the plain camera from above
  (`internal/topdown`, wrapping on a wrapping axis, `Whole` for a minimap); any other sees in
  relief (`internal/stick`: a stick whose foot is the point in the middle of the screen, a pan
  sliding it at its height, turned about the upright and leaned from straight down to `MinPitch`;
  isometric is an eighth turned at asin(TileH/TileW); orthographic, `Zoom` screen pixels a world
  unit in every view, or perspective — the eye at the stick's end, `length` the focal length over
  the scale at Tab, the wheel drawing eye and foot towards the cursor (a cell to the world's
  diagonal, stopping where the eye would not be free), the spring lifting the eye straight up
  where the cage is not free at it or a quarter cell under it and letting it down eased (`settle`
  a tick), no ceiling and no narrowing but inside a unit; picking where the cage is not free)
  and refuses a wrapping world and a missing `Cell`. A relief camera's save begins with its
  form, `"stick 2"`: one saved otherwise begins afresh, logged. **Each camera is made where a scene
  shows it**, beside the picture in the Scenes section, `render.NewFeed(s.cameras.Write().New(start,
  camera.Config{...}), picture)` with `cameras.FromAbove` or `cameras.Isometrically` (a plain
  Config has none). `camera.Config` has no pixels (`render.Feed.Resize` every frame): `Zoom`,
  `Whole` (refused in relief), `Follow` (the name of the entity it starts fastened `Centred` over),
  `MinZoom`, `MaxZoom`. `cameras.Config.Detail` (`camera.Detail{Sight, Fog, Coarse}`) is how
  far every camera sees from its `camera.Viewpoint` (the eye, else the focus), the fog band
  before it and the scale (px a world unit) below which it draws coarsely; both cameras are
  `camera.Detailed`, `Sight{Camera, Factor}` (Ctrl+= / Ctrl+-, `Keys.Farther`/`Nearer`,
  `SightStep` 1.25; a sight of 0 has no end and stays so) changes it, saved with the camera
  (forms `"top 1"`, `"stick 3"`: older saves begin afresh). Every camera is saved, in the order
  made. Its commands name their camera,
  so it knows no players: `Pan` (lets go of a fastening), `Zoom`, `Follow{Camera, Entity, On}`
  (fastens `Centred`; from a ride goes `Centred`, from `Centred` lets go),
  `CenterOn{Camera, Entity}` (once, fastened to nothing; players'
  `through.LookAt`), `CenterAt{Camera, X, Y}` (once onto the ground there, the cameras' ground
  height, fastened to nothing; a minimap's click), and in relief `View` (Tab, only with
  `Perspective`), `Turn`, `Tilt`
  (`TurnStep` π/72, eighteen to an eighth; `TiltStep` 1°, positive steeper), `Ride`
  (V: `Centred` → `Behind` → `Inside` → `Centred`; a loose camera given `Ride.Entity` goes
  `Behind` it, the selection's `RideKey`), `Look` (the mouse inside), `LookFrom`,
  `LookAt{Camera, X, Y, Z}`. **The fastening is the camera's** (`camera.Fastening{Entity, How}`,
  `How` `Loose`/`Centred`/`Behind`/`Inside`, `Outside` all but `Inside`, `Free` `Loose|Centred`
  where the hand scrolls, `HowOf(cam)`), all kept
  here: `Centred` over the entity, `Behind` (the perspective's alone, entered from whatever view
  the camera stood in — `viewCamera.enterBehind`, false where the Config reaches none, so a
  camera fastened Behind by hand goes Centred and `Keeper.ride` goes nowhere; turned until the
  unit's `Vel.Dir` runs up the screen, the unit in the middle, on the arm the settings say:
  `cameras.Arm{Pitch, Distance, Aim, Tilts, Zooms}`, `Config.Arm` the start, `Settings.Arm`
  saved, `Hold{Arm}` the settings screen's command, `Read.Arm`/`Write.SetArm` within
  `LeastPitch`..`MostPitch`, `LeastArm`..`MostArm`, `MostAim`, the keeper reading it through
  `stick.Arms` (`arms{p}`, radians) and tilting it with `Tilt` where `Tilts`; the eye on the arm
  at the arm's pitch (`Keeper.hold`), aimed `Aim` of the unit's height up it, shortened at once
  where the cage is not free between, easing back out; nothing zooming it but `Zoom` drawing the
  arm where `Zooms` (the camera system), R/F and Tab unbound there; the view it came from given
  back on letting go — `leaveBehind`: `viewCamera.tactical{scale, length, ortho}`, saved with
  the camera as "stick 4", the orthographic camera untouched meanwhile, the perspective's pitch
  floored: behind is no tactical view, the user's word 2026-10-18, and no isometric one, TPP the
  perspective alone, 2026-10-27; the arm the user's one camera for a Zelda, a racing game or a
  Warcraft, 2026-10-27)
  and `Inside` (first person: the eye where the unit's `world.Eye` stands, the screen as wide as
  `Eye.Angle`, the mouse looks — `Driven.Look`, the cursor captured by players; up and down it
  writes `Eye.Tilt`, given back on letting go — and a flyer rides: `Driven.Flown`, `Climb`; the
  plugin loads `steering.Driven` itself). Bindings `.In(camera.Inside)` fire riding,
  `.In(camera.Outside)` Q/E (behind a unit they turn the camera round it as the mouse does),
  `.In(camera.Free)` R/F, Tab, the scroll keys and the pointer's pans (the edge, the corners,
  the drag, the grab; the zones drawn there alone), the driving's WSAD and the arrows behind; V
  is bound only where the Config reaches the perspective. Behind a unit the mouse orbits the camera
  (`Keeper.orbit`: `Turn(-dx·LookStep)`, `Arms.Tilt(pitch + dy·LookStep)` where the arm `Tilts`,
  the plugin clamping to `LeastPitch` 5°), the easing back behind the unit only while it moves
  (`Target.Moving`) and `orbitHold` 1 s after the mouse; the cursor caught there as inside
  (`players/capture.go` `looksRound`).
  `cameras.Keys{Up, Down, Left, Right, ZoomIn,
  ZoomOut, ScrollSpeed, Wheel, Drag, Edge, CornersTurn, Grab}` are a player's keys, `DefaultKeys()` W/S/A/D, = and -
  (`ZoomStep` about the cursor), the wheel, the middle drag, the edge (a `Pan`; in a corner of the
  picture — `control.Edges.Corner`, a square of `control.CornerReach` 120 px a side, a quarter of
  the shorter side at most (`control.CornerSide`: a 200-px test viewport would be all corner),
  `Edges.Any` true there — slantwise, or with `CornersTurn` a `Turn` as Q in the bottom-left and
  the top-right corners and as E in the bottom-right and the top-left (`sx * sy * CornerStep`:
  the corners of a diagonal alike, the world under the cursor sliding along the top or the bottom
  edge towards the middle; the user's word 2026-10-26, the left corners alike before), a
  `CornerStep` (a third of `TurnStep`) a tick, instead; `DefaultBindings` set it where the Config
  turns, the user's word 2026-10-18; the old turning and bowing at the world's edge are gone).
  `NewPlugin` anchors the world's sectors (`world.Plugin.Anchor`) at each camera's `Focus`
  (`camera.Focused`: the window's middle from above, the stick's foot in either projection, the
  eye inside a unit), a `Whole` camera skipped.
  Tests take a plain camera from `cameras/cameratest` (`NewFromSpace`; `Switch`, `Turn`, `Tilt`
  as a relief camera's keys), which imports
  neither render nor the world, so their own tests use it too: `internal/stick` is arithmetic and
  a `Keeper` over `stick.Entities`, the plugin's `riding` system reading the ECS for it. `Zones{Shown}` (no key,
  `ShowZones`, `ZonesShown`, `Res.Settings.Zones`, saved beside the cameras' own `persisted`;
  the picture's knobs a profile each (2026-10-20, etap U: `Res.Settings{Zones, FrameBudget,
  Tactical, Riding Knobs}`, `Knobs{Strategy, Resolution, SoftFigures, Layers, Inner,
  Middle, OuterRes, MiddleRes, ShowLayers}`, `Config{Tactical, Riding, FrameBudget}`; `ProfileOf(cam)` is
  `Riding` inside an entity, `camera.HowOf == Inside`, else `Tactical`; `dress(cam)` puts the
  profile's knobs on the camera, the layout `camera.Footed` in a tactical perspective (`Eyed`)
  and `Inset` elsewhere, as the camera is made, on `Restore` and in `RunPlan` after the views'
  system wherever a camera's profile or perspective changed; `Knobs.filled()` gives a zero
  float its default, so an old save, which gob decodes to zero, loads the defaults): the
  commands carry the profile, `Adapt{Profile, Strategy}` (`Still`, `Uniform`, `InLayers`),
  `Picture{Profile, Resolution}`,
  `SharpFigures{Profile, On}`, `Layers{Profile, On}`, `LayerShares{Profile, Inner, Middle}`,
  `LayerResolutions{Profile, Outer, Middle}`, `ShowLayers{Profile, Shown}` (the layers' edges, a
  tick a profile); `Budget{Frame}`, `Sampling{Over}` (the window the rate is averaged over before it is
  followed, 1–20 s, 2 by default: coarser once a window falls under, finer once two meet) and
  `Resolution{Camera, Factor}` (the camera's own profile; no key) carry none; `Thresholds{Under, Met}` (the shares of
  the budget's rate the following steps at, 92% and 98% by default, "Coarser under" and "Finer
  over" on the settings screen) carry none; `Config.Levers` (`cameras.Lever{Following,
  Coarser, Finer}`) are turned before the pictures both ways — the first first going coarser,
  the last first coming back — so the cheapest to the eye takes the rate's wavering and the
  pictures move under a lasting change alone; the topography's care is one; the setters take
  the profile first (`SetStrategy(profile, s)`); the adapter, on the frame's barrier
  (`Plugin.Pace`, a `plugin.Paced` told each frame's `plugin.FrameTimes`, 2026-10-24), steps
  coarser only while the drawing's side (drawing, presenting, the wait for the GPU) took longer
  than the simulation over the window, and every camera by its profile's strategy, `note` writes the lead's back into its profile; the settings screen lists "Frame
  budget" under `control.Picture`, then each profile's nine knobs under `control.PictureTactical`
  and `PictureTacticalLayers`, `PictureRiding` and `PictureRidingLayers` (20 settings; the two
  profiles are twin sections, laid side by side by the settings screen; an element is named `setting:<section path>/<label>`); the
  telemetry's Picture line names the lead's profile, a line a part: `tactical` / `0.70 of the
  pixels` / `the figures sharp` / `following a budget of 60 FPS`); `layersView` outlines a
  cropped layer) likewise;
  marks over every other, hidden by default, shown
  in board/topography's `makePlugins`) draws the strips and the corner squares on the glass over
  every picture but one of the whole world (`render.Glass`, `zonesView.DrawGlass`, in shown
  pixels, after everything the scene lays over the picture: the HUD never covers them; the
  user's word, 2026-10-20) in translucent yellow, an `Image.FillQuad` each, clear at the inner
  bound and deepest at the picture's edge, the strips a gap short of the corners (the user's
  sketch, 2026-10-14; the gradient 2026-10-20); the strips `control.EdgeMargin` wide, 60 px,
  reaching the very edge (the dead strip `EdgeDeadZone` went 2026-10-27: the engine reads the
  window's leave, `InputEvents.CursorInWindow`, and the cursor out of the window scrolls nothing),
  and the edge scroll a quarter the speed at the inner bound rising to the whole at the edge
  (`control.Edges.Depth`, 0.25 + 0.75·Depth in `keys.go`). The cameras' own settings section:
  "Scroll zones" (`EdgeScroll{On}`, `Settings.StillEdges` so a zero keeps scrolling,
  `ScrollFromEdges`/`EdgesScroll`: the edge and the corners' turning build nothing while
  still), "Show the zones" (`Zones{Shown}`) with "Zones opacity" beside it
  (`ZonesOpacity{Opacity}`, `Settings.ZonesOpacity` 0.05 to 1, `DefaultZonesOpacity` 40 of 255;
  `control.Range.Beside` lays the slider in the switch's row and `ui.Element.Live` keeps it live
  while the zones are shown, dimmed and passed by else; 2026-10-27) and "Edge scroll speed"
  (`EdgeSpeed{Speed}`,
  `Settings.EdgeSpeed`, 2 to 24 px a tick at the very edge, zero the keys' own) and the arm's
  rows: "Tilt behind the
  unit" with "Pitch behind the unit" beside, "Zoom behind the unit" with "Distance behind the
  unit" beside (each slider live while its switch is on), "Aim behind the unit"
  (`armSettings`); the plugin's `DefaultBindings` read them through `Keys.bindings(p)`, a game's
  own `Keys.Bindings()` does
  not. `CursorAtEdge` fires once an `Engine.Update`, before
  the ticks, not once a tick; a demo
  whose WSAD drive binds `cameras.Keys{Wheel, ZoomIn, ZoomOut}` alone. `Keys.Grab` (`Config.Grab`
  for the defaults; no default, on in board/topography and board/flat; 2026-10-27): the right
  button held past `control.DragSlop` a `Pan` by the delta, the first pass by the whole way from
  the press, `.Yielding()` so navigation's `LookAt` takes the drag with a selection and the grab
  works with none. The engine sizes no camera.
  `SharpFigures{Profile, On}` (`SetSharpFigures`, `SharpFigures()`, `Knobs.SoftFigures` — zero for
  the figures at every pixel, so a save from before it keeps the feature — "Figures at every
  pixel" under the profile's section, no key; 2026-10-20) has every camera but a minimap's
  `camera.Figured` (`dress` applies it): a picture drawn below all its pixels draws the
  `render.Overlying` sources at every pixel over its ground at the resolution, whole or in
  layers, and the telemetry's Picture line says ", the figures sharp".
  Depends on `world`, `motion/steering`.
- **`driving`** — units driven by hand, with or without navigation: `driving.NewPlugin(world,
  motion, selection).WithGround(board)`. Its commands `Ahead{Camera, Sprint}`, `Back`, `Turn{Way}`,
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
  `gravity.Leap` (`driving.JumpKey(key)`; Space `In(Inside | Behind)` among the defaults). Keys
  are
  sets: `driving.Tank{Ahead, Back, Left, Right, In}` (Left and Right turn), `driving.Shooter{…}`
  (Left and Right strafe, the mouse turns) and `driving.Compass{Up, Down, Left, Right, In}`
  (`Bindings()`); `DefaultKeys()` a `Shooter` of WSAD `In(camera.Inside)`, a `Compass` of WSAD
  `In(camera.Behind)` (`Toward.Way` turned by the camera's `Headed.Ahead` in `handSystem`,
  `asScreen`; `Toward.Sprint` with Shift), a `Tank` of the arrows `In(camera.Behind)` (two
  sticks, 2026-10-27). Depends on `world`, `motion`, `selection`, `board`,
  `players/owner`, `gravity`.
- **`gravity`** — bodies falling, standing, leaping (a character controller, not rigid bodies; one
  plugin, two modes, chosen by the pull's axis, not by the world): `gravity.NewPlugin(world, motion,
  Config{Pull})`, its own entity carrying the knob `gravity.Pull{X, Y, Z}` (an effect's `Alter`
  turns it over within its axis; turned from Z to X/Y or back it panics; a pull both ways, or along
  Z in a flat world, panics; zero, the Earth's as the figures feel it). A kind falls carrying
  `gravity.Body{Leap}`; `giveSystem` puts a `motion.Drift` on it where missing. Along Z the
  `fallSystem` (`internal/vertical`) stands a body every step on the surface under its centre
  (`WithSurfaces(board.Footing)`, refused for a pull across) within its reach down, flies it under
  the pull, stops its head at a slab's underside and lands it; across X/Y (a side view, the
  platformer: a world with heights, `Pull{Y}`, Z its depth) the `planeSystem` (`internal/planar`)
  has collision's contacts (not `Sensed`) take the drift running into them, a body borne by one
  facing against the pull, braked across the pull while borne. `unit.Airborne` is on while in the
  air. Commands, an entity's own: `Leap{}` (none in the air), `Away{Speed, Rise}` (`host.Aimed`:
  thrown off from the moment's subject). A `Landing{ID, Speed}` is told the rules hosted once as a
  body lands, its word `gravity.HarderThan(v)`. Run it right after collision's RunPlan.
  board-topography's walkers leap 1.5 m over 1 m railings (a leap no higher than a railing never
  clears it). Depends on `world`, `motion`, `board/ground`, `board/unit`, `collision`.
- **`vision`** — narrowed perception: a `Sight` cone scanned against the space each step fills
  the entity's `Sighted` (who it can see, nearest first; `Sight` is a knob vision only reads,
  `Sight.Ahead`, `Sight.Looking(dir)`), `SightOutline` gets its view's shape drawn. An entity
  carrying `Transparency` dims sight; the world's `Cover` (the board's veiled cells) is walked along
  every ray, dimming or cutting it, a ray spending its radius as a budget; `Sight.Blockers` are
  the `world.Layers` that cut or dim this sight (a hawk with `Air` looks over walls). With heights
  sight has heights instead: the cone's eye is the unit's `world.Eye` (a Sight without an Eye is
  not scanned), every entity spans its `Z`, the ground is read along every ray — off its corners
  inside aabbworld's scan where the ground has them (`Lattice()`, the relief's: `Cone.Heights` over
  the relief's own heights, re-read every scan, wrapping as the world does, with the world's
  `Bend`; 2026-10-26), else exactly through a `ground.Profile` (the relief's `Along`), else sampled
  every `WithGroundStep`, `Blockers` cutting
  as in a flat world; `Eye.Height` and `Eye.Vertical` refused in a flat world; an `Eye.Vertical`
  keeps what the scan found to the field `Tilt ± Vertical/2` by elevation (`scanner.vertical`, after
  aabbworld's cone, which knows no vertical angle), the views drawn the ring of ground the field
  meets (near and far on the CPU from the eye's height, per pixel on the GPU by `atan2`, the
  instance's fifth vec4); the cover they are held back by on the GPU is walked as the observers
  are published (`coverCopy`, anew where the cover or the ground changed; 2026-10-24), the
  drawing only uploading it. Only an observer something reads is scanned (`watched.go`,
  2026-10-26): a hosted `Sighting` rule is for it (`host.PairRules.For` of its tags, ceremony past
  v0.3.1), or its block carries `SightOutline` or `vision.Watched{}`, or the plugin scans all
  (`WithScanAll()`; a `WithLog` reads nobody); another's `Sighted` is cleared once and no
  `Sighting` is dispatched for it; the GPU views read no scan. A read observer is scanned every
  step, or `WithFrequency(perSecond)` with the observers taking turns, shared among goroutines
  (`WithWorkers`). It hosts rules of a `Sighting` inside the scan's pass, once a step per
  observer
  on what it saw at its last scan (`cond.Other.If(b)` everything in view; its `Subject` the nearest
  seen, so an aimed `steering.Away{}`/`Toward{}` is about that one); words `vision.SeesNobody()`,
  `vision.Closing()`. The views drawn are every observer's unless `WithViews(world.Show(...))`
  picks some (given in the scene); `Cones{Shown}` (the settings screen's, no key) shows and
  hides them (`ShowCones`, `HideCones`, `ConesShown`; `Res.Settings.Cones`, saved; the start in `makePlugins`). `WithLog`. Depends on `world`.
- **`bridges`** — bridges built by one set of rules whoever builds them: `bridges.NewPlugin( world,
  board, Config{Rules site.Rules, Builds})`, `WithShaper(ground.Shaper)` (the topography's
  `Flatten`). `bridges/site.Plan(Ground, Rules, from, to, deck)` is where one may stand, pure:
  straight along one of the grid's directions, `Longest` cells, its heads Takes and on no bridge,
  every span's ground `Clearance` under the deck, no higher than `High`, never over or beside
  another bridge's span, something crossed (water, or ground `Under` the deck; a road under it only
  square across), the heads levelled within `Reshape` breaking no road, a pier between every two
  spans; a `Refusal` says why. Kinds: `Plugin.Types().Define(name, Spec{Allows, Width, Thickness,
  Pace, Rail, Pier})` — the name a level's lines call a bridge by (`bridges.Row{From, To}`),
  `<name>.deck` (`board.NewEntities`: a Graded band) and `<name>.pier` (a still collider, drawn
  standing); `Type.Deck()`/`Pier()` the atlas's slots; a kind of bridge in one piece is a
  `bridges.Figure` (`Define(p)`, `Look(a, p)`; `Stateful`, `Ruled`), noted by `Plugin.Figures`
  among the world's, `Plugin.World()` the world. The flow of a bridge (`types.go`,
  `plugin.go`): `Types.Define` registers three kinds — the name itself over `Row{From, To}`,
  never spawned, only the type the level's line `{kind: bridge, from, to}` is read as; the
  deck through `board.NewEntities` over the private `deckRow{At, Links, Height, Dir}` with a
  `cell.Floor` built from the Spec, the `Rail`, the heading, the board's `Baked` shadow; the pier
  (a `Baked` `world.Shadow`) over `pierRow{At,
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
  "Build the bridge?", `Reason()` says why not (a `Passing` label: laid at the bottom, it covers
  no zone); `Build` levels the heads and spawns the parts at the
  next step.
- **`telemetry`** — the panel in the top-left corner: `telemetry.NewPlugin(w)` with the
  standard lines (FPS, TPS, `Frame` from `render.Spent`, `Entities`, the clock's `Game time`),
  `With(reporters...)` (`render.Reporter`), `Line(label, value)`, `Clear()` — all before the
  scenes are made (later panics); a `game.Scenic` scene `telemetry.SceneName` that is never
  focusable; `Show`/`Hide`/`Toggle`/`Shown` on a `game.Composition`. Two modes (2026-10-20,
  etap V): `Basic` shows FPS, TPS, Frame and Entities alone, `Extended` every line (the clock's,
  `With`, `Line`); `Mode`/`SetMode`, `Res.Settings{Extended}` kept with the game
  (`plugin.Keep`, `Restorer`). The panel draws itself (a `ui.Drawer` of the scene, top-left):
  the labels bold, a value beside its label or under it indented, a line wider than
  `panelWrap` (260 px) wrapped with its rest indented, the plate as wide as the lines have run
  since the mode changed, so a panel back to the basics shrinks. The panel is a
  `plugin.Publisher` (2026-10-24) its scene publishes while shown: the lines and the reporters
  are read after the ticks, four times a second and as the mode changes, into labels and
  values; the panel only wraps the last reading, as it changes, and draws it (a reporter runs on
  the ticks' side; the topography's cares are handed over as it swaps). A many-valued line is a value of several lines, each under the
  label indented as Frame's: the light's `sun 0.71` / `lamps 1` / `shadows of 1`, the sky's
  `Daylight` (was "Light"), the cameras' `Picture` (the profile, the pixels, the layers or
  the figures sharp, the following), the topography's `Ground` (the reach, a care a line:
  `everything 10`…`least 0`), the world's `Sectors`, `Entities by band` (a band a line),
  `Sectors seen` (`exact` / `coarse` / `fog`), the calendar's `Season` (the season, the date).
  The Stage's `World(cfg).Plugins(f)` link makes one where `telemetry.Of(w)` finds none, so
  every Stage with a world has it, hidden; `.Shows(MainScene, telemetry.SceneName)` shows it
  from the start. Its command `Panel{Shown, Extended}` is carried by players (`GameBindings`, ~
  cycling hidden → the basics → everything → hidden, "General"; on the settings screen a
  switch "Telemetry panel" and a choice "Telemetry panel shows"), which sets the mode and shows
  or hides it through the Composition in `carry`. Depends on `world`, `ui`.
- **`minimap`** — small pictures round the players' cameras: `minimap.NewPlugin(w, ground,
  Config{Reach, Turning})` after the cameras (a `Reach` past the world's last band panics), in
  the players' handlers, `RunPlan` after the cameras'. `For(cam, issuer) *ui.Element` is a
  camera's minimap for the scene to place and dress (`ui.TopRight(...).Size(220, 220)`,
  `.Masked(ui.Circle)`, `.Fill`, `.Border`). **Its picture is its own, not the Stage's**: a
  `lens` (an inner `camera.Camera` straight down, aimed as drawn: the ticks say where it is to
  look, published with the marks, the picture aimed and the compass turned as published; over `cam`'s `Focus`, turned so `camera.Headed`
  `Ahead` runs up when turning, `min(w, h) / (2·Reach)` pixels a unit, the shorter way round a
  torus; neither in the cameras' list — no anchor, not saved — nor in `ctx.Draws`) and a
  composer of three sources: the ground (`Ground.Miniature() (img, perUnit)`, drawn opaque with
  `GeoM` scale, turn, copies round a torus; none, the element's `Fill` shows), the marks (the
  space's ids in the picture's rectangle, each kind's `Mark{Color, Size}` declared
  `Styled(minimap.Mark{...})` in its figure's `Look`, looked up once a sprite; a square dot
  `OnTop(1)`; a kind without one not drawn) and the outline of `cam`'s screen on the ground
  (`camera.Picker`, past the horizon cut at twice the reach; `OnTop(2)`; picked as the marks are
  published, on the ticks' side, and drawn as published, 2026-10-24; so is the reach, which
  `SetReach` only configures). North is a second
  element, `Compass(cam) *ui.Element` (2026-10-20): a red triangle (18 by 16 px) hung on the
  circle's edge three quarters out, pointing north, turned round the circle and with it as the
  minimap turns, a transparent picture
  the scene lays over the minimap in a box `minimap.Overhang` (20) bigger on every side
  (`ui.Layers(ui.Center(minimap).Size(220, 220), compass)` sized 260). Its marks are published
  at most 30 times a second (`allMarks`, a deadline and a version) and the minimap drawn anew as
  they are (`paced`), the last picture in between. A press on it is its own
  (`ui.Clicked`): `cameras.CenterAt` with the world point under it as the issuer's; the player's
  picture under it is covered (`ui.Hovered`), so the cursor there neither scrolls nor turns.
  The compass is `Passing` and hidden with the minimap (`minimap.compass`, `Write.Show`;
  2026-10-26): laid over the minimap's box, which reaches the screen's corner, it took every
  press (the click never reached `CenterAt`), the wheel and the corner zone, even where the
  circle left it uncovered; the scene test lays it as the demos do.
  `Switch{Shown}` shows or hides them (`Show`, `Shown`), `Turn{On}` turns them with the
  camera or keeps north up (`SetTurning`, `Turning`); both `Res.Settings{Hidden, Turning}`, saved,
  the settings screen's, no key; `SetReach`. Grounds: `board.Plugin.Miniature` (the
  board drawn whole from above by its tile map, `look.Renderer.Whole`, a pixel a unit up to
  2048, anew as the board changes),
  `topography.Plugin.Miniature` (the painter's albedo, `px / cell`; the hexes' `top`). Depends on
  `world`, `cameras` (the command), `ui`, `render`.
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
  game's rules and effects. Depends on `world`, `motion`, `collision`, `selection`,
  `players/owner`, `board/ground`.

Each package has a `doc.go` describing the gameplay capability it adds.
