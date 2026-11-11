---
name: gram-plugins
description: Reference of gram's built-in plugins, one entry each — world (with entity, view, clock), motion (with steering), light, board (cells, units, standing entities, bridges, maps as files, gramap), topography, atmosphere, collision, navigation, selection, players and owner, dialog, cameras, driving, gravity, vision, bridges, underground, telemetry, minimap, bullet: how each is wired and in what order, what it rests on, its invariants, the user's decisions, the traps and the ideas dropped. Use when working in or with a specific plugin, before reading its code.
---

# gram-plugins

A plugin's `doc.go` says what it brings and ends with `# Vocabulary`, the authoritative list of
its moments, facts, knobs and commands (`go doc ./plugins/<name>`). The default keys, the
settings without a key and the house rules are `CLAUDE.md`'s; how a plugin is built is
`gram-plugin`'s, how it draws `gram-render`'s, its numbers the `gram-perf-*` skills'. An entry
keeps only what those do not.

## Wiring

- `makePlugins` makes them in the order used: motion first, handed to whatever steers or pushes;
  the light after motion and before the board and the topography, which draw by it; the
  topography after the world, the light and the board; the roads after the board and the
  topography; the minimap after the cameras; the players last, handed every
  `plugin.CommandHandler`, whose `Tunable`s the settings screen lists in that order.
- `Update` as board/topography runs it: bridges, world, light, motion, collision, gravity, board,
  navigation, driving, vision, atmosphere, selection, topography, cameras, minimap, players.
  Fixed: the light and motion after the world; gravity right after collision, the board after it;
  navigation before the driving; the topography once the world has moved, before the cameras;
  the minimap after the cameras; the players last (they fire `KeyHeld` and empty the queues);
  bullet (motion/bullet) before the world, its flight written before the world's step.
- One writer a seam:
  - `world.Z.Altitude`: the topography from the relief; gravity for a body carrying a
    `motion.Drift`, which gravity alone writes.
  - `steering.Pace`: the board's standing pass, from the game's `cell.Mobility`.
  - `steering.Driven`: the hand's part (`Ahead`, `Turn`, `Sprint`, `Face`, `Side`) the
    driving's, the eye's (`Look`, `Flown`, `Climb`) the cameras'.
  - `steering.Course.Speed`: steering; collision lowers it by an impulse's share, walls too.
  - `unit.At.Next`: whoever leads the unit (navigation its route's next cell, the driving the
    cell ahead, its own cell for a step within it), 0 while nobody does.
  - `world.Eye.Tilt`: the riding camera, given back on letting go; vision's cone and the
    first-person camera read one `Eye`.
- A plugin reading a component of a plugin the world may lack lists its `LoadComp` itself (the
  board `steering.Pace`; the topography `Course`, `Driven`, `Drift`), so a save loads either way.
- Kinds' templates: a plugin writes what it asks of them in `KindTemplates(r)` alone; the world
  calls its own and every made plugin's, in the order made, as the roster is first read, and
  refuses a write elsewhere and a plugin made after: make every plugin before defining a kind.
  `world.EntityTemplate` is the parent of `motion.UnitTemplate` and `board.StandingTemplate`;
  `board.CellTemplate` stands apart. Required of the game (`kind.Require`): `Position` of every
  entity (world), `At` and `Mover` of a unit (board), `steering.Steering` (navigation, driving).
  Brought (`r.Default`, dropped with `comp.Without[T]()`): `Velocity` and the effects' marks
  (world), `Shadow{Patch}` (light), `Course` of a unit (motion), `Lie` and `Shadow{Baked}` of
  what stands (board), `Collider` and `Physics` of a unit (collision). A missing requirement
  panics naming the plugin and its reason.
- Levels (`lod.Paces` through `world.Plugin.Pace`, `gram-plugin`), every system's: `Slowing(2,
  4, 8)` motion, gravity along Z, the plans, the effects; `Contact` (through L4) collision,
  vision, gravity across; `Steady` (every level but L6) navigation, a shot's flight (keen at L4
  while it flies), the light's sources, standing, occupancy, altitude, driving, selection,
  dialog, give, routing, shooting. Cells carry no `Base`, so no level; lookups by identity
  (`Seek`, a conversation's partner, a selected unit beyond sight) stay `All()` on purpose.
- What draws reads what was published once a frame after the ticks — the entities and views,
  the board's cells, the relief, the weather, the light, the minimap's marks, the telemetry's
  lines, a camera's `Snap` — never the live state; before the first publish, the live one.

## world, with entity, view and clock

- The ladder (`lod`): three scales of two rungs each, exact and coarse, and the omission: L0–L1
  close (`lod.Close`; a game enters it by setting the first two thresholds, a zero there meaning
  the scale is not used), L2–L3 standard (`lod.Standard`), L4–L5 strategic (`lod.Strategic`),
  L6 omitted (`lod.Omitted`). `Paces` has seven rungs, counted from the finest level any
  observer actually sees; `Plugin.Within(first)` filters a system's entities to one scale (it
  excludes the other levels' marks: goke has no OR), so a system of the close scale ticks only
  where someone looks closely.
- Sectors (`SectorsCfg{Buckets}` cuts the world; `Config.LOD` is its `lod.Ladder`; board/
  topography and motion/expanse have them): the supervisor runs once a tick after the clock,
  before the views. Per observer camera (`sector.Observers`, the cameras' but a `Whole` one's or
  a minimap's) it holds a level a sector from `lod.Ladder.At(camera.ScaleShown(nearest point),
  distance from camera.Viewpoint)`, wrap-aware (`Grid.Nearest`), coarser than a level past its
  `Ladder.Rings[level]` (`m/rings`, 2026-11-08; widened with the Sight by the camera's), finer
  at once and coarser only past `Ladder.Hold`'s hysteresis (a threshold by 10%, the Sight and
  the rings by half a sector), and no finer than Strategic where `cam.Visible` shows no part of
  the sector grown by that margin, but for the viewpoint's sector and its eight neighbours
  (2026-11-09: the perspective's scale falls with the distance all round the eye, a point behind
  it counted as one as far before it, and the orthographic one is one over the frame, so without
  the picture's bound a camera held a disc of fine sectors round its viewpoint, 150–700 on
  Madeira where it showed 25–35, and every forest in them unfolded: 20 000–30 000 trees), then
  grades
  it so neighbours both under L6 differ by one at most (a Chebyshev transform; L6 borders
  anything, else the lift would wake rings beyond the Sight) — the camera's view reads that
  graded level. The level ticked is the finest graded level of any observer, with none the
  ladder's `Unobserved` (zero: L4, a server's middle; a bench says `UnobservedL2`),
  lowered to a `sector.Keen` entity's level in its sector and the eight round (a shot in flight).
  Every sector begins at L6, so the first tick and a load wake everything near. Marks
  `sector.L0`…`L6`, zero-size, saved with the game, one on every awake entity; beside them the
  pace marks `Far1`…`Far5` (2026-11-04): how many levels coarser than the finest level any
  observer sees (`grade` returns it; `Keen` never lowers it; no observer → `Unobserved`) the
  entity's sector is, swapped in `move` with the level mark; `Pace(paces)` builds
  `Every[Far_k]` on them and `Never[L6]`, so time runs at full pace where the viewer looks most
  closely whatever the level, and a world nobody watches at full pace everywhere (the user's
  rule; the aggregates are its saving). A change of the finest level re-marks every awake
  entity (two archetype moves each; batching through goke's `Editor` is open). `Contact`'s
  "none" rungs are relative too: collision and vision run three levels deep from the finest
  seen.
- **The space holds no L6 entity** (the index, motion's rebuild and collision skip them), so
  asking it would never find them again: the sleepers are listed by sector, laid anew at the
  first tick and after a load (`restart`), and woken by sector. A top-down zoom across a
  threshold re-marks every awake entity at once (~200 ns each): known, not spread yet.
- The space reads the `Base` columns where goke keeps them, as runs (`world.RunOf`): motion hands
  it the runs it moved, collision those whose Caps changed or sweeps stretched, and the world
  gathers them anew after every Sync (`Clock.OnSync`; the engine hands `Update`
  `Clock.Settling(rc)`) and as a tick begins where a chunk's table changed (`TableVer`). Nothing
  else tells it of spawns, despawns or marks; queries read that grid till the next rebuild.
  Two layers (`f/space`, 2026-11-08, aabbworld v1.21.0 `RebuildStill`): the indexer runs two
  queries, the still (`Include[Fixed]`) and the moving (`Exclude[Fixed]`), both without L6,
  with runs and `TableVer` checks a layer; the still layer is indexed once and kept across
  `RebuildRuns`, pairs are moving×moving and moving×still, the still never pushed; a still
  box's Capability must not change between calls, so `Write.Restill()` has the layer taken anew
  (the collision calls it the first tick after a `Fixed` collider spawns and when a `Collider`
  is removed).
- Each plugin refuses the other mode's facts (a `Z` in a flat world) where it first meets them.
- `Scale.Figures` draws the figures bigger than life (boxes, heights, eyes); speeds and sight
  stay real.
- A camera riding `Inside` an entity skips that entity's figure in `Compose` by identity
  (`Fastening().Entity`), never by the eye's place in its box (the eye and the published box
  come a step apart while the unit runs); the shadow is cast before the skip. A solid
  (`Slot.Upright()`) or a card is the kind's choice in its `Look`; topography's units are solids.
- `world.Read.Tick(d)` makes every `host.Tick`: the Stage's one carrier (players and entities
  alike), the time, the seed, the roles, the clock's entity.
- clock: a plan written by hand (a test, a bench) does what the engine's wrapper does: `Update`
  under `Clock.Settling(rc)`, then `Clock.Replay` of the simulated blocks as the tempo says.
  `Clock.Behind` lowers a tempo above 1 after 30 slow frames. `clock.At` and `Every` go by clock
  time: a moment passed never fires again after a load.
- view: `View.Level(x, y)` is the picture's level: the observer's graded level shifted coarser
  by the camera's `LevelShift` (the frame budget's; a sector seen at L2 or finer is never
  shifted: what the player looks at stays full, 2026-11-04), a drawn sector to L5 at most, L6
  off the picture or past the sight; the tick never sees the shift. Behind the eye `ScaleAt`
  gives the scale of a point as far before it (`camera.InFront` says which side).
- The sheets of what stands (`sheet.go`, `sheet_gpu.go`, 2026-11-05; `Plugin.Sheets`, asked once
  by a ground's renderer in its constructor: the topography's and the board's; a world none asks
  photographs nothing and records L4–L5 figures as blobs): `world.Fixed`, a zero-size tag goke
  refuses as a `Comp` (read it `OptComp[Fixed].Present` or `Include`), marks a kind standing for
  good: the board's `StandingTemplate` gives it (so `NewEntities` kinds and bridge decks carry it),
  bridge piers get it in their `Define`, units and `SpawnPart` never. Over the whole world lie two
  RGBA8 images a texel `Ladder.Texel` wide (zero: one over `Scales[L4]`, a sector's side over 64
  where that is zero too, raised to 4096 texels a side at most): `Sheet`, premultiplied albedo
  (alpha 0 = nothing stands), and `SheetRelief`, the top's absolute height as f16 in R and G and
  the normal in B and A; and their L5 halves. A sector a view draws at L4 or coarser is signed at
  the publish (count plus xor of a mix of id, L3 sprite, top and place of every `Fixed` in it;
  a thing across a border is signed into every sector it covers), a changed signature bumps its
  content version, and at most `perFrame` (8) sectors a publish whose painted version lags are
  photographed round-robin: the `Fixed` things whose kind declares no L4 look, and the parts a
  folding field snapshotted (`wholes.fold`, before `Fold`; dropped at unfold; point aggregates
  move, so none), each a dome `Altitude + Height·(1 − r²/2)` (a `Lie` flat at its top, turned to
  its heading) drawn into sector-sized scratch images with their own depth in two `Primed` passes
  (albedo, relief; depth = the top over the photo's highest plus `instance_index·1e-6`), copied
  into the L4 sheets with `CopyFrom`, the L5 halves drawn by a shader (mean albedo, mean top and
  normal over the texels something stands on); an empty sector that never held anything is held
  without GPU work. The photos are gathered in `Publish` (all sheet state on the game goroutine),
  cross to the frame through the double buffer, and are developed by `img.Defer` scheduled at
  `Swap` (not `Compose`: the world renderer is `Overlying`, composed after the ground; scheduled
  there every photo was recalled forever); one no frame developed is taken again two publishes
  later. A `Fixed` thing in a held sector loses its record when every view that sees it draws it at
  L4 or coarser (`figure.fine`), keeps it where another view draws it finer (`figure.sheet` skips
  it per view). `Read.Sheet()` (perUnit 0 till a sector holds a photograph: clients skip their
  sample), `Read.Painted(i)` (a held sector holding something, frame-consistent). A unit at L5 is
  its blob as at L4, thinned by `Keep[L5]`. `sector.Grid.Of` truncates, no float64.
  Tiles (`sheet_files.go`, `d/files`, 2026-11-06; `Config.Sheets{Dir, Keep, Version}`): the two
  sheets in 512-texel tiles `albedo-<tx>-<ty>.png` and `relief-<tx>-<ty>.png` beside `sheets.yaml`
  (version, per_unit, grid, sector, tile, space, the painted sectors with their signature and
  inked), read in `open()` with the uploads deferred and the L5 halves by `darkroom.halve`; a
  manifest of another Version, texel or world is ignored with one log line. **PNG is written
  through `image.NRGBA` over the bytes, never `image.RGBA`**: that one is premultiplied and the
  encoder divides every half-covered texel's colour by its alpha. With `Keep` a dirty tile (a
  sector in it photographed) is written 2 s after its last photo, at most one a second
  (`CopyFrom` and `ReadPixels` in the frame, `png.BestSpeed` in a goroutine, `.tmp` then rename,
  the manifest rewritten after each). `Swap` schedules `develop` only when photos wait, so `keep`
  re-arms the Defer fill while tiles are dirty (a workaround; `Swap`'s condition is the cleaner
  lever). `Benchmark_World_SheetFiles` lives in `plugins/world` (it needs the store), so
  `measure.sh` does not see it. The policy is the game's: a static world ships its tiles
  read-only (`Keep` false), a dynamic one writes them back (the user's word, 2026-11-06).
- Seams L3|L4 (`p/seams`, 2026-11-05): once a publish `sheets.begin` marks the sectors a view
  draws at L4–L5 with a neighbour it draws short of L4 (`sheets.ring`, the union over views,
  `tiling.Neighbours`); a `Fixed` thing is `alive` when some view draws it finer than its middle's
  sector (`renderer.alive`: its middle in the ring and its box grown by its longer side covering
  a finer sector, `renderer.drawn` giving the finest level short of L4 it covers; or its middle
  on the finer side with its box crossing into a sector drawn at L4+): it keeps its record,
  skips `kept` (never thinned), is stamped into no sheet, and `Compose` draws it at that finer
  level (`figure.ringed`). The L4|L5 seam is left alone. The plan's `neighbourChanged` hook went:
  the signature counts the things stamped, so a neighbour crossing the seam bumps the version
  once, exactly when the photograph changes; a hook on the ring set would have re-photographed
  unchanged sectors and, a pan moving the ring by a row (more than `perFrame`), flickered them to
  points. A world that never calls `Sheets()` draws a thing by a seam as before.
- Tiers and the handle (`t/tiers`, 2026-11-07): `sheets.begin` builds a view a table of the
  tiers' cells (`sheetFrame.tiered`, in the figures' double buffer; a 2×2 min climb from the
  sectors' levels, 4/3·n entries, no allocation warm) and a `Fixed` thing longer than
  `Read.Small` (a wall, a handle) or a piece (`PieceOf`) is drawn at its cell's level
  (`Grid.Fit` at the publish, O(1), nothing saved; `figure.ringed` now means "drawn at its
  cell's level") and photographed from L4 into every sector it covers; a piece takes its handle's
  cell (`renderer.homes`, a sorted id→slot slice rebuilt lazily at the first piece met; a piece
  whose handle is gone is judged on its own box); a handle (`Whole`) is skipped at chunk level,
  so it is in no figure list and no photograph; nothing living in a cell is thinned by `Keep`
  (its pieces would vanish one by one while the photograph waits); only a world with sheets uses
  the tier rule; the small things keep the ring rule. Trap: `sheets.wants` tests a box's four
  corners alone, so a long thing whose middle sector alone is at L4 gets no photograph there and
  stays alive. `Fit` costs ~27 % of a publish of long things (`Benchmark_World_Seams/long`:
  integer divisions a tier tried; an O(1) `Fit` from the box's first and last sector columns and
  rows is the lever).
- Banners and counts (`banners.go`, `p/far`, 2026-11-05): with `Config.Far` `Banners` (a setting,
  `ShowFar`), the loose units (no field, not `Fixed`) of one kind in one sector a view draws at
  L4–L5 whose kind declares a look at that level (`Atlas.declares`) are one group a view
  (`figures.far`, in the double buffer: a flat id list, a centroid, the mean box and `Z`), drawn by
  `Compose` through `s.point` as that look over the mean box, never thinned (gathered before
  `kept`); a kind without the look stays points; a unit another view draws finer keeps its record
  (`figure.banner`, skipped per view). `Read.Banners(cam, dst)` appends the frame's banners (ids
  alias the frame's buffer: valid till the next frame). Banners of a kind in neighbouring sectors
  whose units stand within `Config.Sectors.Small` of each other are one (`farJoin`, a union-find,
  across the torus's seam; `f/counts`, 2026-11-08). A banner is a badge (`f/badge`, 2026-11-08, the
  user's word: a disc or a diamond in the player's colour with the number inside):
  `world.BannerStyle{Shape, Size, Fill, Rim}` declared on the kind with `Slot.Styled`, each zero its
  default (a disc 14 px across in the kind's far colour, `Atlas.farColour`: the mean colour of its
  L4 look where it declares one, else its blob's; the rim a third of it), read once at `Atlas.Close`
  into a table a sprite (`kindBanner`, `Atlas.bannered`); a style alone makes the kind bannered at
  L4–L5 with no L4 look (the topography's rival walker). The badge stands 2 px over the top of the
  banner's drawn box, drawn through `render.Labels` with the number centred in it, white or black by
  the fill's luma, only with the counts on (`Config.Counts`, `ShowCounts`) and two units or more; it
  grows to hold its digits (`render.BadgeWidth`); `Banner.Reach` is half its width in shown pixels
  and `Banner.BadgeAt(cam)` its middle and reach in the camera's pixels; a folded aggregate with an
  L4 look is a banner of one, a badge without a number. A kind's points two or more in a sector get
  their count over the centroid the same way, no badge. Hover and the pointed commands still hit a
  bannered unit at its real place; only `Select` knows banners.
- Aggregates (`wholes.go`, 2026-11-04): `Aggregate`, `Field{Size}`, `PartOf`; the `wholes` system
  (after the moments', `lod.Steady`) reads each aggregate's level at its centre (`levelAt`, not the
  mark swaps: a spawn mid-tick has no mark yet), keeps a world-made `stretch` entity per unfolded
  (aggregate, sector) with the fold's due time (saved; `Unfold`'s subject, which is how
  `SpawnPart{At}` finds the sector's `Box`), tells `Folding` to the parts whose centre lies in the
  sector, moves a point aggregate to its parts' centroid before `Fold` (`gather`, the space's runs
  re-gathered) and each step while it is `Unfolded` to the middle of its non-`Fixed` parts' joint
  box, its own size kept (`follow`, before `unfold`; `space.Move` only past `f32.Eps`; `f/band`,
  2026-11-08), and checks `Met` among the bodies carrying `sector.Abstract` only when a rule handles
  it (`meeting` entities remember a pair). `Grid.Cover` cuts a box by sector. A field's record keeps
  its own box plus `Size` in a side table; `renderer.over` picks per camera: the level read on the
  field box, drawn over it from L4 (the point path scales the look), over its own box short of it;
  seen wherever the field box meets the view, asleep too (the render query keeps L6 chunks carrying
  `Field`); never thinned, no shadow over the field; a box over 8×8 texels sums its inside a run at
  a time and weighs its rim alone. `Unfolded` (first Unfold → the step after the last Fold): a point
  aggregate publishes no record while it carries it, a field is published as one record per covered
  sector (`renderer.covers`), each at its own sector's level, none for an unfolded sector nor for
  one whose stretch is `Folded` till its parts are gone (the stretch outlives the Fold one step).
  `Keen`/`Calm` commands are carried in the despawn system. **goke's `Query.Seek` ignores a query's
  Exclude and tag filters**: what a Seek must leave out is read as an optional column (the minimap's
  marks, the keeping's release).
- The preview, a dry unfold (`preview.go`, `d/preview`, 2026-11-06): a sector waiting for its
  photo (`sheets.pending`) under a bare field stretch (no snapshot, no stretch entity: never
  unfolded, or not since the load) gets its parts without spawning them: `wholes` opens a
  preview (ids counted from 0xFFFF0000, at most `perFrame` a step), `Unfold` goes to the rules
  through the same RunWhere as a real one, a `SpawnPart` under a preview records a shot (Z from
  the kind's Spec — `ikinds.Kind.Z`, `HasZ` — Altitude `GroundAt(middle)`, the L3 sprite, Lie)
  instead of spawning, `foldPreviews` hands them to `sheet.fold(place, shots)`; a preview lives
  two steps; nothing is saved. **The rules cannot tell a preview from an unfolding**, so under
  `Unfold` only `SpawnPart`: a `Forbid`, an effect or a count there would run for the preview
  too. `Write.SetGround`/`Read.GroundAt` (the topography sets it after `SetLook`; 0 without): the
  world knows no ground and the shot needs the part's altitude.
- A part standing for good is set on the ground as it comes and when the relief's version
  changes, never a step (2026-11-08, `f/still`, `relief/altitude.go`): the parts pass compares
  each chunk's ids pointer, length, archetype and `TableVer` with the last step (the world's
  indexer's check), grounds a grown chunk's tail, a chunk whose table had a removal whole, and
  every part on a new relief version; `GroundAt` a part a step was 1.25–2.2 ms over 31 000
  trees. Pieces of a handle (`PieceOf`) are not parts: a deck stands on its cell, a pier keeps
  the bridges' altitude.

## motion, with steering

- It runs after the effects' pass: an effect on the steering tells the step it begins; what a
  rule of `Moving` or `Leaving` puts on or takes off lands with the next step's effects.
- The velocity pass scales the speed by `steering.Pace`, so a speed is asked before the ground's
  share is applied (navigation's `Trail` divides by the unit's own share).
- The move query excludes `Fixed` and so does `held` (`f/space`, 2026-11-08): what stands for
  good is never stepped, its boxes are the space's still layer.

## light

- The plugin declares and publishes; the lighting is render's (`render/internal/compose`:
  `lighting.go`, `bake.go`, `cast.go`).
- The bake: nothing over a hex relief or without a sun; level ground is baked only while a
  `Baked` block stands, which gives a flat board its towers' and bridges' shadows; a bake asked
  before the frame holds the heights plans the whole anew.
- "Shadows follow the frame budget" (`AdaptShadows`) makes the light a `cameras.Lever`: as the
  rate falls it drops the lamps' shadows first, then bakes coarse; back the other way.
- The blocks go a set a sector (`Lights.Casts`, `d/shade`, 2026-11-07), each with a running hash
  and a version, so the baker bakes anew only the tiles a changed set's shadows reach; a thing
  with a `Lie` is one lying block (a bridge's handle), so the baker joins nothing (`t/whole`).
- Every shadow keeps to `render.ShadowReach` (CLAUDE.md, 2026-11-10): the live ones in
  `Distant.ShadowOf` and `Lamp.ShadowOf`, the baked blocks in `bake_blocks.wgsl` (the quad walks
  the shadow's line to the ground as the relief's march does, never past six heights), the
  clouds' slant in `air`. Before the rule the blocks' quads ran to the map's lowest ground
  (28 km at sunset on Madeira: the bake stalled the frame at 18:30) and the cap lived in three
  places under three names.

## board

- **The `Map` contract is a customization point for games, not plumbing**: a game gives its own
  map through `WithMap` (the topography is one, the board's windy map another). Its methods stay
  where no plugin calls one; never treat them as dead code: a point with no example gets one
  under `examples/` (`examples/board/custom-map`, `ExamplePlugin_WithMap`).
- What draws reads the cells as published (`Read.Drawn`, `Read.DrawnHeights`), a cell copied
  where its version differs.
- `grid.Grid` knows its own directions; nobody else writes a table, a bit or a vector of them.
  Bit i of a `cell.Links` is direction i. A hot loop walks `Directions()` with `Toward`, which
  allocates nothing and beats `Neighbors` on both grids; `Neighbors` is for the rare callers.
- **How fast a unit goes is the game's `cell.Mobility`** (the user's design, 2026-10-28): a
  step's share of top speed in [0, 1], 0 refusing it, 1 full speed and nothing faster, factors
  multiplying, neutral 1. Given beside motion (`WithMotion(m, mobility)`; `NewUnits` panics
  without it), it is read through `cell.PaceOf` by the planner, the standing pass and the driving
  alike, at the slope of the step to the neighbour the unit goes into, so what the planner allows
  the unit walks. Steep is the relief's (`Map.Rise`), never a kind's; what it does to a unit is
  the mobility's.
- No share exceeds 1 (`Define` refuses a kind admitting someone at no pace or above 1, `PaceOf`
  panics naming the kind): the planner's h, the grid's distance, rests on it.
- `cell.DefaultHiking` is Tobler's hiking function cut at the flat: a descent never quicker than
  the flat, `Air` free over anything. A game's own domain bits tell its classes of unit apart
  (board/topography's `HighlandDomain`). A save from before the shares holds costs where shares
  are read and panics at its first step.
- `unit.At.Next` is the cell the leader steps the unit into; the standing pass prices that step's
  slope while the heading lies within a quarter turn of it, else the step toward the heading's
  nearest neighbour. A unit cutting a corner of a slantwise step stands in a cell its route
  never priced, so navigation leads it through that corner as within it (`Next` the corner
  cell, the level pace): priced as the straight step out of the corner, a refused step gave
  pace 0 and no stall, and the unit stood for good (2026-11-02).
- Every cell is an entity for good, with no `world.Base` (out of the space): `Plot`, `Ground`,
  `Way`, `Built`. `Ground` is kept apart from `Plot` so an effect ending restores the kind alone.
  A state of the ground is an adjective (an effect), the kind the noun; what lies on it a cover.
- `Version` and `CellVersion` count every change, an effect's only as it begins or ends and only
  one the terrain changes under (shown as a cover, granting tags, turning `cell.Ground`, `Floor`,
  `Wall`, `Rail`), never one turning another knob alone (`weathering.Wring`): each change
  repaints the topography's surface.
- `Passage` and `CellPassage` count only what a step reads of a cell: its ground's and its way's
  kinds as a step meets them (`Allows`, `Solid`, `Graded`, `Pace`, `Paces`; the ground's own
  apart, since `Bare` prices a slantwise step off a way), the way kind's name, `Width` and
  `Links`, `Built` whole, its heights (`Touch` always counts). `Veil`, `Veils`, `Height`, `Sway`,
  `Takes`, sprites, a ground kind's name, a way's `Fade` and `Mix`, labels, roles and markers
  move the `Version` alone. Whatever changes what a search reads must count in `Passage` before
  navigation's next search.
- Trap: the passage reads `Built` from the store, never through `Read.Built`, which `t.built`
  gates till the gathering ends.
- Its rules run after collision: `unit.Fallen` never in the air nor on a deck; `cell.Now` only
  while some rule of it exists. One kind of places a rule.
- Standing entities: a `Velocity` among the extras gives the way a ford runs (its `Dir`); a
  ford's `Floor.Pace` 0 is the ground's, of several floors the fastest; a fence is a band at the
  ground's level with a `Rail` (slanted walls in collision's field, a barrier for what drives,
  cleared by a leap over its top, never planned across); an effect opens one (`Alter` a
  `Floor`'s `Allows` or a `Wall`'s `Keeps` to 0). `Built.Over` puts the floors' domains on and
  the walls' off in `Kind` and `Bare`, so the planner, the driving, `Fallen` and collision's
  field all see them.
- Limits beyond the kind (`BoardRead.Limits(c, d, deck)`, `Limited`): a `Floor.Load` counts on
  a raised deck's level, and on a lying floor only where the cell's ground and way would not
  carry the domain (a rope's span, not its heads). A `Wall` with a `Gap` leaves its domains in
  `Kind`, so readers of `Kind` alone (`Fallen`, traffic's route re-check, the driving, spots)
  see the gap open to everyone. Of several entities on a cell the strongest floor bears (0
  wins) and a gap stays only where every wall has one, the narrowest. Collision holds a gapped
  wall's cell solid only for a box whose narrower side or height does not fit, so it never
  stops whom the planner sends through.
- `Units.Define` notes its kind (`UnitKinds`, in the order defined): navigation's classes.
- Footprint (`WithFootprint(cells)`, `DefaultFootprint` 3): a box past a cell takes the fewest
  cells a side holding its longer side with room to spare (`unit.Footprint`), slack for
  collision: a box exactly as wide as its cells froze at a passage's mouth. An even side is laid
  from its cell right and down, its unit standing on the shared corner (`unit.Place`,
  `LaidFrom`); the keep, the standing pass and `At` read the cell through `LaidFrom`. Hex has odd
  sides only. `Railed` says whether a railing runs along a cell's own band.
- A bridge's head band runs from the middle of its flat tile into the bridge alone, no railings
  on it. A bridge breaking the rules panics as a fresh board is first gathered (a map's file and
  line); one broken later is not built, said in the log.
- Occupancy: every unit holds the cell it stands on, on its level (a walker on a deck and one
  under it share a cell); one mid-step holds both cells and is left to navigation.
- Maps: a hex row is indented; `relief` is `cells` or `corners`, in metres through `world.Scale`;
  a river flows down in the order its points are given; with a relief every cell of a `Graded`
  road lies level or on an even ramp by its corners on either grid (`ground.RoadFits`; on a hex
  relief by cells the cell where level ground turns into a ramp bends, so a climbing road leads
  onto a bridge or a ford); the shaping (`relief.KeepRoads`) takes back what would break one and
  the bridge tool refuses it; every error names the file and the line.
- `cmd/gramap` (Madeira's maps, from `cmd/gramap/madeira.yaml`): heights from the Terrain Tiles
  on AWS, rivers by `water.Drain`, roads routed by `roads/site`'s Planner (gramap's `Leaps`
  bridge water and valleys, a leap's cost the bridge's length at the recipe's `bridge` plus its
  heads' earth) and levelled together by `site.Plan` over the whole network after every leg; a
  leg stays only if the network still levels and every bridge stands (`bridges/site.Plan` on the
  graded ground), else the refused cell and its ring are made 3× dearer and it is planned anew,
  48 tries a rung, then left out and named; fords where a river is shallow; the recipe's `roads:`
  are `site.Rules` in metres (`cut`, `fill`, `earth`, `step`, `spread`, `slope`, `fords`, `metre`,
  `cubic`, `climb`) with `route: {cut, fill}` the planner's own allowance (Madeira 40 m against
  200 m of levelling: a planner forgiving the levelling's whole allowance sees no slope between
  cells and runs up gorges the levelling cannot fix); a leg is routed up a ladder of
  rungs, forgiving none of the route allowance first, then a quarter, a half and all of it, and
  the planner sees a cell the levelling could flatten as flat at its corners' mean; a cell
  flanking a diagonal bridge's head is barred (its corners are fixed at the deck and on the
  ground); the map is laid on a board and run a step before it is written, the old file's
  `units:` kept — and `units:` alone: a hand-placed row of `entities:` goes with a regeneration.
  With `roads: {source: osm}` (2026-11-11, a map with relief alone; `-flat` plans the routes
  as before) OpenStreetMap leads the same planner (`osm.go`, `guide.go`, `follow.go`,
  `tunnels.go`): the ways' cells thinned (Zhang–Suen, so two carriageways are one line) and cut
  into legs at junctions; a leg planned in the corridor two cells round its trace, cells off it
  twice dearer, leaps offered only over OpenStreetMap's bridges and water (weighing bridges at
  every expanded node was 89% of the old run); legs levelled sixteen at a time with all laid,
  the one through the refused cell taken out and tried alone, then in halves down to four
  cells, so a long road loses a stretch, never itself. Tunnels first: ways joined into chains,
  a twin tube skipped, the line straightened to the grid's ways (`via`) and led out of each
  head across an edge, two cells on before it slants (`squared`), each head cut down and
  the far corners of its first cell raised (the site wants the ground over the first cell's
  middle at the roof: 2 × clear over the head, 90 m at figures 15), drawn back up to four
  cells; their heads, first cells and corners are pinned for every later levelling (`held`
  plans each anew as `hold` does the bridges). What decides the yield is the 10° of
  `ground.RoadSlope` and the room of figures 15 times life, not the planner. `GRAMAP_MADEIRA=1
  go test ./cmd/gramap -run TestMadeira` holds both of Madeira's maps to their SHA-256.
  Fields and settlements (`fields.go`, `settle.go`, `d/gramap`, 2026-11-06): `fields:` rows
  `{cells, kind, grain, least}` lay a field entity a block of `grain` cells from (0, 0) over the
  cells of a kind where at least `least` lie, written as `{kind, from: [x, y], to: [x, y],
  cells: n}` rows of the board's `entities:` (the demo's `fieldRow`); `settlements:` `{cells,
  sizes, default}` lays `cells` of a kind round each road place, as many as its size's word
  names (a place `[x, y]` or `{at, size}`, `none` for a pass), by a BFS roadside first over land
  that is not wet, road, rock, bridge or another settlement; the legend's `@`. The thresholds,
  the words and the sizes are the game's, in its recipe, sizes in cells never in houses (the
  user's word, 2026-11-06: gramap stays general; what stands on a built cell is the game's
  Unfold rules). `check.go` defines `built` like soil and stand-in field kinds so the map lays
  on a board. A field's `cells:` is a kind or a list of kinds (2026-11-11): one field a block
  over all the land cover's kinds, since a field a kind a block tripled the fields and cost the
  world's publish 0.6 ms of every 30 (Madeira: 315 growth fields and 195 settlements). The land
  cover (`cover.go`, 2026-11-11): CORINE Land Cover 2018's shapes and the Natura 2000 sites
  from the EEA's ArcGIS REST services (GeoJSON, no login, the answers kept in `cover/` beside
  the tiles through the same `overpass` cache), rasterised by the cells' middles a row at a
  time, holes by the even-odd rule; `soil()` takes the cover's kind before the slope's rock
  (the laurisilva grows on slopes: with the rock first it lost 40 % of its cells) and after the
  altitude's. CORINE knows no species: the game's stands pick them by height.
- The board's field (collision's `Field`) and cover (sight's) read the cell entities whenever
  asked; nothing copies them.
- The flat board is drawn on the GPU from an image of its cells (`look/tiles.go`): the CPU writes
  the cells' data, the shaders work out the shapes; a changed cell is rewritten with its
  neighbours, all of them past an eighth. Nothing is composed on the CPU: a camera without
  `Rays`, or a board of no world, draws nothing. The tiles' shaders decode the sector's level;
  from L4 they lay the sheet's albedo (`world.Read.Sheet`, `WithSheet`, the method `standing`)
  flat under the tile's own alpha, in the plain pass and the full one alike: one image a draw,
  the L4 albedo while any sector seen is at L4, the L5 half at `perUnit/2` when all are at L5.
- A unit carrying `sector.Abstract` (2026-11-04) gets pace share 1 from the standing pass
  (`moments/standing.go`) and the occupancy's keep (`occupancy/keep.go`) re-enters it into no
  cell; a chunk carrying `sector.Abstract` or `world.Unfolded` gets no standing rules and no fall
  log. `occupancy.PlaceSystem` (before standing) moves a part or aggregate holding no cell yet,
  laid where its domain may not stand or on a held cell, to the nearest cell taking it within
  three rings: the library's answer to parts spawned by offset.

- An unfolded whole holds no cell (`f/band`, 2026-11-08): the occupancy keep's release reads
  `Unfolded` as an optional column (`leaving`; a `Seek` ignores Exclude) and treats such a whole
  as gone; the place system, on the first cell found taken in a step, releases the cells of
  everything gone and tries again (a fold despawns the parts in the step the whole loses
  `Unfolded`, and the placing runs before the keep). Test `unfolded_test.go` through a real stage.

## topography

- Made after the world, the light and the board; refuses a wrapping world; hands the light its
  relief (`light.Plugin.OverRelief`).
- The relief reaches its `Heights` entities as the heights system runs and as each Replay ends
  (`Clock.Replayed`, `HeightsSystem.Fill`, one comparison when nothing moved): a grade made in a
  later plugin's step of the simulation or a shaping command given in the pause is in a save
  taken between the ticks.
- `Write.Grade` (the `ground.Shaper`'s) pins the graded cell's own corners while the MaxStep
  ripple runs, so a ramp steeper than MaxStep along a cell's edge stands; it is false only where
  `KeepRoads` took the grade back, true for a cell already as asked, because the roads tool
  grades every cell of a plan in turn.
- What it draws reads what it published once a frame: the relief copied as its version moves,
  its settings, the board's grid; the terrain encodes the heights as it composes, so the frame's
  `GroundSpan` is theirs.
- The relief: the cells' corners, which the neighbouring cells share, on both grids (the user,
  2026-10-31: a hex is six equilateral triangles from its middle, at the mean of its corners);
  on a hex grid each cell owns its top and top-right corners (`relief.HexCorners`). `GroundAt`,
  `Along` and the hexes' shader share one triangular lattice (`onLattice`): change one, change
  all. The corners' facing lives in an image in the relief's order, the middle's worked out in
  the shader. A hex save of a level a cell loads, every corner at the mean of its cells. It lives
  on the plugin's own entity. `world.Z`, the relief and `Heights` are the world's heights, not
  the view: sight reads them in a top-down game too. The altitude pass has a third query
  (`d/prelude`, 2026-11-06: `Fixed` and `PartOf`, no `cell.On`, no `unit.Mover`) setting a
  standing part's `Altitude` to `GroundAt(middle)` when off by more than `f32.Eps`; `PartOf` is
  required, else the bridge piers (Fixed, their Altitude the bottom they stand at) would be
  regrounded; the plugin hands `relief.GroundAt` to `world.Write.SetGround` right after
  `SetLook`, for the world's previews.
- The painter paints the cells within two of a changed one anew, all of them past an eighth.
- A kind's detail (`painter.Style.Detail`): a uniform cell's detail comes from its flags; the
  kinds sheet is read only where a cell is `painter.Dressed` (a way on it or beside it, a ground
  running in), since reading it at every pixel cost too much on an Intel GPU. The details have no
  mipmaps; mipmaps in `render/internal/gpu` are the next step, for the far look as much.
- The clouds' shadow is worked out at the lattice's corners, not per pixel (the high layer alone
  cost about 1.2 ms a frame so), and not on the skirt, whose huge far triangles showed it as
  angular shadows on the horizon.
- By sector and level, from the camera's View (`gram-plugins` world): a shader a level (`const
  LEVEL`): L2 everything on the whole lattice, L3 without shore, running water or mouth glint,
  L4 on the coarse lattice (every second corner, the edge ring one cell stitched by fans to
  every outer corner, crack-free) with still water a mirror and no lamps or baked shade, the
  painter's albedo at half its size; L5 on the coarser lattice (every fourth corner, the same
  ring; a Madeira sector 94 triangles against 190 and 512; `lattice.coarse(c, step, dst)`, a
  sector too small for the step falling back) with the albedo at a quarter as the only colour,
  lit by ambient and the sun's diffuse alone so it does not glow at night; the albedo's halves
  (`Painter.Shrunk`, `Painted.AlbedoHalf/Quarter`, `DrawImage` at 0.5 after every bake, the
  changed cells' rectangles widened to fours after a partial one) are bound as `Images[1]` by
  level with the uniform `AlbedoPx` (the shader reads the albedo in its pixels; `U.Px` serves the
  kinds image too, so it cannot be halved); L6 nothing; the grid at L2 alone; the skirt always L5
  in a world with sectors, L2 without. Finest first, to prime the depth. Over the ground of a
  sector seen at L4 or L5 whose sheet is painted (`Config.Painted`, the photograph's version),
  the photo mesh (`terrain/photo.go`, `photo.wgsl`, one instanced draw a level after `eachLevel`,
  no index buffer): quads of at most `photoQuad` 8 texels a side (Madeira: 8 a side at L4, 4 at
  L5); a corner stands at the top of what stands nearest it within a quad (the corner's own texel
  sank a thing narrower than a quad; the highest within the reach, till 2026-11-09, made a forest
  on a slope a terrace floating over its downhill trees, sky under its lip from the side), one
  cell under the ground where nothing stands, read from the tops image
  (`tops.wgsl`, a texel a corner: the f16 top and whether anything stands near) worked out anew
  for a sector and its eight neighbours when its photograph's version changes (`photoMesh.watch`),
  never a frame (2026-11-07: searched per vertex every frame, 32 quads a side, it ran Madeira's
  start view at 38 FPS against `main`'s 110); a fragment with alpha 0 discarded, the rest the sheet's albedo
  lit through the relief's normal as that level's ground, clouds at L4, haze and the sight's fog;
  it never binds the ground's albedo. A corner where nothing stands near sinks one cell under
  the ground, no deeper (tried 2026-11-09: sunk under the lowest ground, the quad between a
  standing corner and a sunk one hung as a curtain under a crown on a ridge, against the sky
  where the ground falls away behind). That quad holds nothing (`reach` ≥ a quad's side, so
  every texel with something raises all four corners of its quad). A camera with an eye
  (`eyed`: `camera.Eyed` and `Eye()` ok, the perspective riding or free) gets no sheet at all:
  `drawPhotos` returns, and the world draws what the sheet stands in for as points
  (`renderer.standIn`: the kind's look at the level or its blob, `Atlas.blob`, kept for every
  kind drawn of a look, declared nothing at L4 or not). A sheet is a photograph from above: from
  the side its trees hang as discs in the air, over a slope a terrace at the uphill trees' tops.
  Tried and dropped 2026-11-09 (the user's eye, from the lookout at Paul da Serra's eastern edge):
  walls down the sheet's sides as a second instanced draw, one colour a wall, then cut texel by
  texel into the trees along the edge; a deeper sink of the sunk corners; the sheet solid in the
  perspective (a fragment seen through writes the depth and holds the backdrop, drawn at
  `Furthest` later, out of the hole, which shows the clear colour); every variant left rectangles,
  curtains or confetti nothing like the trees that grow at a zoom. Lesson: a top photograph has
  no side; far trees from the side are points now and billboards of the kinds' Side later (stage
  `v`). A hex board draws tops alone at L3 and L4
  (the sheet's
  albedo flat on the L4 tops) and one quad a sector from its tops' image at L5, the sheet over
  it. No care rings, no care settings (the user's rule: one level a sector).
- The billboards stand upright across the screen's level right, as tall on the screen as their
  `Height` and inside their box, shorter the steeper the view.
- Its parts are packages none of which imports the plugin. selection must not import topography,
  even in tests.
- The haze is off by default through every camera (`topography.Haze`, the user's choice,
  2026-10-18); the camera trap behind it is the atmosphere's.
- A unit bigger than a cell stands on the highest ground under its box, its middle and four
  corners (the ground alone, never a deck): its figure is drawn flat at its altitude, so a slope
  under a corner higher than the middle showed through it. A body within a cell reads the ground
  under its middle alone.

## atmosphere

- Its root keeps no shaders and draws nothing of the world: every part is a package that never
  imports it; `celestial` is imported by `sky`, never the other way.
- **A component never holds a pointer, slice or map, and nothing of the weather is saved but by
  goke** (the user's rule): the weather lives in fixed-size `air.Patch`es on entities.
- The dome (`air.Dome`): levels of cells, the finest over the world, each further one coarser,
  out to `Config.Horizon` past it, every level a ring of patches. Cells entering the coarsest
  level take the mass born there, a finer level's the coarser one's. The climate makes the
  patches in `Init` (from a save it keeps those that fit, lays anew when any is missing) and
  `Hold`s them every step, since their memory may move.
- `climate.Weather.Lanes` are the weathers the horizon brings, lanes across the wind, each its
  own chain, a neighbour's state twice as likely; lane 0 is the weather of the time, `Over` the
  state over the world's middle (the panel, the temperature, rain or snow there).
- A change shows within seconds (the user's word, 2026-10-16): `Change` and `Set` jump lane 0 to
  its targets and settle the dome over and upwind of the world in seconds of game time, all of it
  the first step; without wind the whole dome settles. `Left = max(Lasts, crossing)`: a weather
  lasts till the wind carried it across. A natural front from the horizon reaches Madeira in
  about an hour of game time.
- A weather asked while the clock stands (the settings screen's pause) is entered at once, the
  climate run with no time passing (`Climate.Asked`). `Running` is not saved.
- The sky: noon lies `NorthWest` by default, beyond the isometric view's sea; below the horizon
  the moon lights (`sky.Moon`, a knob: board/topography's blood moon is a rule of
  `sky.Moonrise`).
- A game's sky (`atmosphere.Sky`): `Renderer.Dress` panics on a uniform or module-scope name two
  parts declare, or a part and the library (`render.SharedUniforms`); a part without WGSL draws
  nothing; a later part replaces an earlier. naga's traps are `gram-render`'s.
- GPU and CPU agree: `airAt` reads a level in one filtered sample as `Dome.At` does (a test keeps
  them within 2.5/255, after the windows moved too); `Weather.Shade` matches the shader alike.
  Dropped: four `textureLoad`s a level with the ring's arithmetic, which cost the terrain about
  15% of its frame rate; a level's cells are laid in order so the sampler blends them.
- **Two layers of clouds** (reshaped at the user's eye, 2026-10-16). The low heaps lie in
  **three bands over the relief** (the user's design, 2026-10-17: the same show over peaks of
  1900 m or 4000 m): under the peaks, covering them, over them, as shares of the relief from its
  low ground to its highest (`Plugin.Over`); a relief under 1 km counts as 1 km, so hills never
  pierce. The heaps are lumpy, never flat. The high shreds are sheets, each at its own height,
  crossed, never marched.
- Trap: a hand-built `air.Weather` (tests) without a `Relief` has one deck on `Bases[Low]`, and
  without `Bases` its clouds lie at 0: tests set them. `Weather.Base` and `Share` answer for
  `OneLayer` (board/flat) too.
- From above (`aloft`) the heaps are marched at random offsets a pixel (an even pattern showed
  stripes; slices showed as stacked discs from the side). From an eye under the layer the low
  bodies stand before the ground up to the highest ground (`aloftBelow`) and the sky's cumulus
  draws those past it (`backdrop.Clouds.Before`): neither doubles nor leaves a gap. The coarse
  pass writes a depth of its own, which the lay tests against the world's: the ground or
  anything nearer hides the clouds.
- Only the low layer is drawn from above (the user's word, 2026-10-17: from the tactical view the
  high sheets veiled the whole map like a fog); the high ones show on the sky and as shadows.
- Trap: an isometric camera is `camera.Eyed` but `Eye()` says no. The terrain and the hexes took
  any `Eyed` for an eye and hazed from the field's start, the map all fog but its corner;
  `air.Weather.Seen(cam, hazed)` decides now, `air.Under` likewise for where the weather is read.
  From above the haze is the air over the ground up to the high clouds' base along the lines of
  sight; the sky's own fog is not switched.
- `AllBodies` is the clouds' start (the user's choice, 2026-10-16); the high clouds' shadows are
  on as the game starts (the user's word).
- "Clouds follow the frame budget" (`AdaptClouds`) steps the clouds' pixels first and their
  bodies' reach last, never under half the sight, the look never (the user's word, 2026-10-20:
  the clouds stand high and far, so dimming them to shadows or cutting the bodies near the camera
  showed too much).
- The clouds' tile keeps a pixel of wrap round every level: reading it clamped half a pixel in
  left a seam where the heaps' cores rise steeply.
- Rain and snow fall only from the low clouds, drop by drop in the world (the user's word,
  2026-10-16: realistic, with perspective): a lattice fixed in the world in three nested boxes
  round where the eye looks (one through a camera from above), each drop kept where rain falls
  at the point it left the cloud, a streak through the scene's transform with the depth test.
  Dropped: shafts of rain seen from afar (from the side the air under the clouds showed as
  rectangles over the horizon; laid coarse, they showed through hills); screen-space streaks
  before the eye (unreal: the same at any distance).
- **Clouds at the peaks** (the user's design, 2026-10-17: the piling is the climate's Go, the
  rain and the gathering the game's rules over the board's cells). The masses carry their
  weather's damp; the weathering walks the cells into an `air.Land` of their ground and `Wring`;
  the climate's `Dome.Lift` has ground standing into the low band hold the low layer upwind (a
  bank piles before a ridge, spills over, the lee clears); `Wring.Falls` wrings rain out (a rain
  shadow past it), `Gathers` turns damp into low clouds, never clouds out of dry air.
  `weathering.Overhead` takes no places: the board's `AroundPlaces` takes its own moments alone.
  Madeira's uplands' rules put effects lasting 2 s anew each second.
- **The weather in places** (the user's word, 2026-10-17: a map wholly under cloud loses the one
  looking at it): the knob `climate.Spread` has rain on a quarter of the world at most, a storm
  in its middle, fair breaks fewer the cloudier the spell. Its parts are laid by the rank of a
  periodic noise as wide as the world, so any stretch that wide holds the shares exactly (storm
  ⊂ rain ⊂ cloudy, fair at the other end). The climate lists `Spread` among its `LoadComps` too,
  its query registering the type. On a world only a few of the dome's cells wide the places
  blur: tests measure over 6400×3200. The panel names lane 0's weather and, where it differs,
  the one over the middle.

## collision

- Its tick is aabbworld's `collide.Engine.Tick`, the system its `Handler`; the arithmetic is
  `collision/internal/response`. The box is the shape.
- A push the engine's later passes would leave over worse ground goes back to the box the tick
  found, kept at the entity's first `Touch`.
- What a wall leaves of a speed below a ten-thousandth is a stop, facing on; a unit under orders
  glances along the wall.
- A body carrying `sector.Abstract` (abstract travel, 2026-11-04) is not excluded from the space
  (that would drop it from sight and selection till the world's next gather): the walk reads the
  mark as a column and a marked chunk goes `still()` as a far one does, Caps Plain, contacts
  cleared, a resting pair with a marked side dropped in `rest()`.
- `Fixed` chunks are skipped in `rebuild` (`f/space`, 2026-11-08): a `Fixed` collider's caps are
  constant from its physics (`CanCollide|Static`, `CanCollide|Sensor` for a sensor), never
  changed in place; the first tick after a `Fixed` collider spawns, and when a `Collider` is
  removed, the system calls `Write.Restill()` (`CountRestills` in `export_test.go`).

## navigation

- **Levels** (2026-11-04, second design): a unit's `bodies.Member.Level` is its own sector's
  mark (`bodies.Levels`, once a chunk). At L4 or coarser the unit walks the same refined route of
  cells the climb gives (`Finder.Climb`, 64 steps, re-asked when spent; the hierarchy not ready →
  the ask waits: Madeira's first build ~20 s of game, the runs after load it from
  `navigation.Cache`'s file, 2026-11-10) in the **far mode** (`traffic/far.go`):
  `goFar` releases the Leg, leaves the cells and puts `sector.Abstract`; `tailFar`/`driveFar` is
  pure path-following (waypoint, lookahead, `passed`, `turnFactor`, `approach`, `Index++`,
  `keep.Watch`) without `reserveLeg`, the occupancy, `keep.Steer`, the queue or the formation,
  at `MaxSpeed · stepPace` from the planner's price of the step (`MoveOrder.Pace`/`Paced`
  cache); entry is lazy (the cell path walked out, or the unit off its route with no step in
  flight); `descend` (the mark gone or the level under L4) anchors the route to the cell under
  the unit (its current step, one of the next 8, or a neighbour of the next) and enters it, or
  `Snap`s off a taken one (`Snaps`) and asks anew; a folded aggregate goes far at once; a unit
  told Stop in the far mode stands unmarked, on no cell (open). Groups at L4 march as units
  (`Place 0`, no field). A marked body is `still()` for the collision, out of the occupancy's
  keep and at pace share 1. The straight legs of hpastar's `Search.Abstract` were tried and
  dropped (the user, 2026-11-04: the unit walks the road, not the chord); gram does not call it.
- **Aggregates** (2026-11-04): `MoveOrder.Aim`/`Aimed` hold the aggregate a `To` named; `aims.keep`
  recounts the orders aimed at each target every tick (units marked `order.Aiming`) and keeps
  `sector.Keen{L3}` on it, recording its own Keen in `order.Sought` to restore it; a running count
  would miss orders ending unseen. `Inherit{From}` (`traffic/inherit.go`, at the step's start so a
  fold's parts still exist at tempo > 1) copies the order whole but the walk; zero `From` is the
  giver's `PartOf.Whole`, or for an aggregate the least part with an order. An aggregate ticked
  finer than L4 lands as a unit does (`standFor`, `land` shared with `descend`), drops the mark,
  leaves its cells itself (a `Release` Seek would not) and stands out of the keepings with its
  order kept while `world.Unfolded`, whatever its level, for the parts to take
  (`kind.Entry.Told`); after the fold it is marked and marching. An abstract route inherited
  below L4 is dropped for a cell route. `aims.heed` re-records a target's own Keen given or
  calmed while an aimed order holds it (a game Keen at exactly L3 cannot be told apart and is
  lost when the order ends). `Inherit` copies the `Path` with its `Index` (and Target, goals,
  Aim); a heir not standing on the route keeps it as given (dropped below L4, walked straight to
  its waypoint far).
- The route renderer draws a selected unit's route whole (2026-11-04): past the order's 64-step
  window it computes the rest with the same hierarchy (`previews`, keyed by the window's last
  cell and the passage counter, one computation a frame, 16 units at most) so the line matches
  the roads the unit will take; nothing of it is state.
- **A route once planned is followed**: a better way opening (a road laid, a wall opened)
  changes no route; the unit takes it only on an impulse — a new order, a plan's step, a rule's
  command (rules/guard: the alarm's `MoveTo`, `StepAside` when `Stuck`, `Onward` on the round).
- A goal taken or out of reach is waited on `TargetWaitTimeout`, then the order's target becomes
  the nearest free cell the unit reaches.
- Entity ids start at 0: never use 0 as "nobody" (`MoveOrder.HitUnit`).
- A search reads each cell it asks of once, into a table by ordinal stamped with the search:
  nothing may change the board while a search runs; every public method of the `Finder` begins
  one. Its hot loop walks `Directions()` with `Toward`, never `Neighbors`, asking
  `DiagonalNeighbors` only for a corner and `Along` only where the bare ground would price a
  slantwise step otherwise.
- A route leaves the `Finder` only through `dst`; a `Path` holds its steps in its own array, and
  the kept answers keep Paths by value.
- A search asked again is answered unrun, kept by its question (start, target and level,
  domain, `pathfind.Body`, the cells gone round as a set less the target, round or any way) till
  the board's `Passage` moves, all let go when full; a finder asking the occupancy keeps none.
  The flat steps (`flatStep`) are kept by `Passage` too, the routers' cleared every Update.
- `TestRoutes_AsPinned` pins every route, step and price on three boards (square with bridges and
  relief, a torus, hex) to `testdata/routes.golden`: a change meant to keep the routes keeps it;
  one meant to change them rewrites it with `-update-routes` and says why.
- The keepings allocate nothing per unit: method values bound once, their state in fields.
- A search that finds nothing is not run again from the same cell round the same cells for
  0.5 s, 1 s, then 2 s, the plain route taken meanwhile; a way found, the next goal, a new
  order, another target, cell or cell learnt starts it over.
- It always plans by levels (the plain mode is the planner's tests' alone): the goal is reached on
  either level, the goal's cell met mid-route is a waypoint; a slantwise step on or off a deck
  goes only along it, one on the ground cutting a corner of water only along a deck lying there
  (the flat map's slantwise bridges). A zero `Size` fits any lane.
- A step is refused where the floor it enters does not bear the body (`Body.Weight` from
  `unit.Mover.Weight`) or where either cell's gap does not fit its width across the step or
  its height, a step and its way back alike; a corner is cut past a cell only where it bears
  the body and fits its narrower side; a class stands only on nodes that bear it. Weight is
  part of the body, so another weight is another class even on a board limiting nothing.
  Trap: when the passage moves, traffic re-checks a planned route by kinds alone, so a load
  lowered or a gap narrowed under it does not break it.
- The lookahead is the turning radius, so a turn's corner is cut by about that; at a bridge's
  head the unit lands on the band past its railings' ends, and collision pushes it back. Trap: a
  unit wider than the lane slid along a railing's end over the water and stood there, frozen in.
- `MoveTo.To` names the goal: a cell's `Plot` or a unit's `At`; `Face` turns the unit on arrival
  (a right click on its own cell).
- Crowd: another is `Ahead` within a quarter turn (cos > 0.7) of the way the unit's route leads
  it next (`Body.Way`, from the order's `Way`, which `tail` writes and `arrive` zeroes), or of
  its facing without a route; two units are never ahead of each other. `InTheWay`,
  `ClearsTheWay`, `Blocks` and `GoalTaken` ask `Ahead` of a groupmate alone: one of the group
  pressing from behind or beside is the unit's queue, not its way. Dropped: `Ahead` asked of
  everyone (the crowded tick +28%: strangers stopped veering, the jam at the gap lasted longer).
- A `Resting` or `Sensed` contact bumps nobody; a body never moves into those it touches.
  `StepAside` goes only where the slope leaves a third of the pace on the level (`YieldShare`, a
  ratio: a flat forest is stepped into, a cliff not). A stall is no headway of a quarter of what
  the ground lawfully lets the unit make in a second, at most a quarter of its body.
- `Trail` (the default rule `WalksMyWay && Groupmate`, before `Blocks → Detour`): for
  `TrailMemory` after the last touch the unit asks no more than the trailed unit's ground speed
  over its own share; the trail holds while it touches the trailed or has it within a body's
  width ahead, as long as that one is under way and ahead; headway is judged at the lower of its
  own lawful pace and `TrailPace`, passed down the queue; no wait is a stall while the trailed is
  under way (`maxTrailWait` only a safety); a dead queue unwinds 0.5 s a link.
- A party's routes: the orders of a MoveTo that sent four or more (`MoveOrder.Party`, written as
  it is carried out, kept by `Advance` and a step aside) read their routes off one field of the
  way to the goal, keyed by the goal's cell, level, domain and body class less its feet, so two
  MoveTos to one cell share it. The field is a Dijkstra from the goal through `Finder.step`, the
  one A* uses, so a cell's cost in it is `Find`'s from there. A route asked goes kept answer →
  field walk → climb → search. Searched as before: units with learned cells, parties under four,
  orders giving way with goals queued, boards past `1<<18` cells, finders that see the others,
  units off the field, last legs not found.
- A `March` keeps a `Formation`: `Coordinator.Form` lays every formation out before the units
  move, the members ordered by the field's cost at the body's point, continuous from cell to cell
  (a cost stepped at the cells' borders reversed neighbours). A member asks no more than the
  speed that stops it a room short of the one ahead; one behind in the formation is never
  trailed (no loop of trails); a closed column's head waits while a gap over its slack closes,
  `StallAfter` more once it stops closing; no stall counts while paced or while the head waits;
  a step aside keeps the formation.
- Long routes: ends `From` cells apart or more (64 by default; board/topography 32) climb a
  hierarchy of hpastar's, one a domain serving the classes of its units' bodies: the table comes
  from `board.Plugin.UnitKinds` at Setup (at most 8 a domain, the nearest merged beyond into a
  class no laxer), and a unit is planned everywhere as its body's class or the nearest no laxer
  (fields and kept answers key by class), else by A*. Built steps carry `Refuses` (the body-free
  part read once, a fast path where `bodily` is false); a class refuses every step out of a node
  it does not stand on; `At` lists the nodes any class stands on; a domain of one class builds
  over its body's steps; a query computes only its class's body. Trap: a kind defined by hand,
  not through `board.Units.Define`, has no class, so its units are searched by A* (the test
  rigs class theirs with `asKinds`). It is built and staled by the one finder that made
  it (another panics) and asked by every finder through its own `hpastar.Search`; the `Layout`
  is pure. A `Sweep` before any ask stales each where the passage moved (`CellPassage`: the
  changed cells and a ring of one) and builds the one not ready asked for latest by
  `buildBudget` steps of hpastar's work, the same work on any machine, since readiness decides
  whether a route climbs. `Climb` refuses a pair nearer than two level-1 sides (hpastar's
  `WithNear` alike), a start the class cannot stand on and a hierarchy not staled since the
  passage moved. Trap: `Finder.Build` with a budget of 0 builds nothing, hpastar's `Update(0)`
  builds all. The solver's own skill: `/home/karol/CODE/hpastar/.claude/skills/hpastar/SKILL.md`.
- A body bigger than a cell stands on a node only where every cell of its footprint's window
  takes it on its level (the domain, the limits, no railing along the band, no deck lower than
  it overhead), a slantwise step's flanks too; it steers to the window's middle (a corner for an
  even side), and traffic reads its node from its place less that offset. Each finder keeps its
  footprints by `Passage`, re-read within the window of the changed cells; the hierarchy's stale
  ring widens to the largest extent plus one. Such a body keeps to one level. Known gaps: no side
  check on hex steps (under 1% overhang), neighbours' diagonal bands unseen, its footprint not
  held (one cell), traffic's route re-check and BodySpacing's spots reading kinds alone.

## selection, players and owner

- Every unit carries the selection's and the owners' families, all off, and is told `Allow` and
  `players.Give` as it is made: through its first step it is nobody's and unselectable.
- `KeyReleased` does not fire after a chord with modifiers fired while the key was down (Shift+P
  is no throw); `Context.Held` is real time (`players.ClockWith` in the players' tests).
- The cursor's triggers fire only while the window has the cursor: a zero `InputEvents` has none,
  so a test or a probe moving it says `CursorInWindow: true`.
- The settings screen names its parts for tests and probes: `setting:<section path>/<label>`
  (twin sections, as the cameras' profiles, side by side), `frame:<section>`; the controls
  screen `frame:<group>` and `section:<group>`.
- A click selects a banner's units (2026-11-05): `SelectionSystem.banners` (set in `Install` from
  `world.Read.Banners`, the constructor's signature unchanged) is read in a `Select` with a camera
  and a screen rectangle; every banner whose drawn box (`Drawn(cam, b.Box, b.Z)`) or whose badge
  (`Banner.BadgeAt`: a square of `Reach` round its middle; `f/badge`, 2026-11-08) meets the
  rectangle adds its ids, the units drawn in banners are skipped by the ordinary pick, and
  `applySelection` still enforces `Allow` and the owner. The selection's outlines of such units stay
  at their real places. A banner's `Count` is the units its entities stand for, not the entities
  (2026-11-09): a folded aggregate counts for its `Aggregate.Strength` (`farGroup.heads` beside
  `count`, the entities, which slices `ids`), the world rewriting a declared Strength with the parts
  folded in at each Fold (`wholes.gather`), so the band's banner says 30 from the start and the
  army's its soldiers left; a kind declaring none (the caravan) counts for one.
- A part stands for its whole (2026-11-07, the user's word: an aggregate is selected whole, never
  its parts): `standsFor` maps every id a `Select` or a hover hits — by ids, by a box, by a
  banner, by the pick — to its `world.PartOf` whole where the part carries no `Selectable` and the
  whole does and obeys the issuer; `applySelection` is unchanged, so `Allow` and the owner still
  rule; `drawnIn` is false for an `Unfolded` whole, so it is hit nowhere at the empty spot it
  stands on; the renderer outlines each part of a selected unfolded whole (a `PartOf` query
  walked only while one is selected), the whole alone when folded. `Pointed` keeps the part under
  the cursor. The knob is `Allow` on the part (2026-11-08, the user's word: a group is a thing of
  the coarse levels, one of its units seen is picked itself): a group of individuals tells its
  parts `Allow` as it spawns them (forest-army's soldiers, the topography's band), a unit of
  pieces does not (the caravan's wagon and porters); a whole selected before it unfolded stays
  selected, its parts outlined, till the first pick of a part. Trap: `Chosen` of a selected caravan is the whole, which stands where it unfolded
  while its wagon walks away, so C/V follow an empty spot (roadmap).

## dialog

- What an answer changes is a function of the speaker's component, whose fields are of a fixed
  length, no strings. A `Choice`'s `Do` names commands of `world.Commands()`, a `ui.It` in them
  the speaker. An answer to an undefined node panics as the Stage is set up.

## cameras

- Every camera is `camera.Snapped`: a feed takes a copy for the frame drawn (a relief camera's
  with its own stick, its ground's extent frozen, no cage read), for drawing alone — no `Pick`,
  no command.
- A camera's save begins with its form (`top 1`, `stick 4`): one saved in another form begins
  afresh, logged.
- **Behind a unit** (TPP) is the perspective's alone: no tactical view (the user's word,
  2026-10-18), no isometric one (2026-10-27); the arm (`cameras.Arm`: pitch, distance, aim,
  `Tilts`, `Zooms`) is the user's one camera for a Zelda, a racing game or a Warcraft
  (2026-10-27). Entering behind keeps the view it came from, saved with the camera, and gives it
  back on letting go, the orthographic camera untouched meanwhile; fastened behind where the
  Config reaches no perspective, a camera goes `Centred`.
- Behind, the mouse orbits the camera; it eases back behind the unit only while the unit moves
  and a second after the mouse.
- The corners: `CornersTurn` turns as Q in the bottom-left and top-right, as E in the other two,
  the corners of a diagonal alike, the world under the cursor sliding along the edge towards the
  middle (the user's word, 2026-10-26); `DefaultBindings` set it where the Config turns (the
  user's word, 2026-10-18; nothing turns or bows at the world's edge). A corner's square is
  `control.CornerReach`, at most a quarter of the shorter side (a 200-px test viewport would be
  all corner).
- The zones are drawn on the glass over every picture but one of the whole world, after
  everything the scene lays over it, so the HUD never covers them (the user's word, 2026-10-20):
  translucent yellow, clear at the inner bound, deepest at the edge, the strips a gap short of
  the corners (the user's sketch, 2026-10-14). The strips reach the very edge; dropped: a dead
  strip there (`EdgeDeadZone`), since the engine reads the window's leave and the cursor out of
  the window scrolls nothing. The edge scroll goes a quarter speed at the inner bound, rising to
  the whole at the edge.
- `CursorAtEdge` fires once an `Engine.Update`, before the ticks, not once a tick.
- The plugin's `DefaultBindings` read its settings through `Keys.bindings(p)`; a game's own
  `Keys.Bindings()` does not. A demo whose WSAD drive binds
  `cameras.Keys{Wheel, ZoomIn, ZoomOut}` alone.
- The grab (`Config.Grab`, no default; on in board/topography and board/flat) pans by the whole
  way from the press on its first pass, and yields to navigation's `LookAt` while there is a
  selection.
- A setting's zero is its default, since an older save decodes to zero: `StillEdges` (a zero
  scrolls), `Knobs.SoftFigures` (a zero draws the figures at every pixel), `Knobs.filled()` (a
  zero float takes its default).
- The adapter's ladder: going coarser the `Config.Levers` in order (light, atmosphere), then the
  pictures' level shift +1 (`camera.Shifted`, at most 2, every camera but a minimap's, a sector
  seen at L2 or finer spared; a world without sectors has no rung) while "Detail follows the frame budget" (`Settings.StillDetail`
  zero, `cameras.Detail{Follows}`), then the resolution; finer: levers, resolution, shift. It
  steps coarser once a window falls under the budget, finer only once two windows meet it. A
  measurement stills the shift too (`Detail{Follows: false}`).
- A camera's `Sight()` is a multiple of the world's (`world.Config.LOD.Sight`), 1 as made,
  saved as the multiple; `world.Read.Sight(cam)` the distance; a world sight of 0 has no end.
- `cameras/cameratest` imports neither render nor the world, so their own tests use it too.

## driving

- With no hand a unit brakes and is let go once it stands or `Keeping.Ordered`; held still by a
  wall, it stands at once.

## gravity

- A zero `Pull` is the Earth's as the figures feel it (`world.Scale.Figures`).
- A leap no higher than a railing never clears it: board/topography's walkers leap 1.5 m over
  1 m railings.

## vision

- Over a relief the ground is read inside aabbworld's scan off the relief's lattice
  (`Cone.Heights`), re-read every scan, wrapping as the world does, with its `Bend`.
- aabbworld's cone knows no vertical angle: an `Eye.Vertical` field filters what the scan found
  by elevation afterwards.
- The cover the GPU views are held back by is walked as the observers are published, anew where
  the cover or the ground changed; the drawing only uploads it.
- The same views bit for bit with and without SIMD (`gram-perf-sight`).

## bridges

- `Types.Define` registers three kinds: the name itself over `Row{From, To}`, never spawned, the
  type a level's `{kind: bridge, from, to}` is read as; `<name>.deck` (a `Graded` band through
  `board.NewEntities`); `<name>.pier`, spawned only over a relief (without heights a bridge is a
  deck at the ground, no piers). Each takes its `SpriteID` from the world's registry as it is
  registered; the plugin never calls the atlas: the look is the figure's.
- `Spec.Load` is every deck's `Floor.Load`.
- A level's bridges are planned at `Populate`, an error with the line; a player's `Build` levels
  the heads (`WithShaper`) and spawns the parts at the next step; a player's `Build` is a
  `Res.Orders` entry (its kind by name), saved, so a bridge confirmed in the pause is built after
  a load. The renderer draws the hands alone: a bridge confirmed in the pause is not seen until
  its step builds it.

- A bridge is on a handle (`t/whole`, 2026-11-07): `Types.Define` registers a fourth kind,
  `<name>.whole`, over a private row, with `world.Whole`, `world.Fixed`, the box round the span
  from head middle to head middle `spec.Width` wide (no side under `Entities.MinSize`),
  `Velocity{Dir}` along it, `Z{Altitude: Deck, Height: Thickness}` (the light reads a lying
  thing from its Altitude down, as a deck), `Lie`, `Shadow{Baked}`, `vision.Transparency{1}` (a
  diagonal bridge's box would otherwise cut sight at the deck's height) and no look; `parts()`
  returns the handle and the pieces, `build` spawns them by `SpawnWhole`, a level's bridges by
  `SeedWhole`; the decks cast `NoShadow` (the handle casts the deck whole, the block equal to
  what the old `joined` made), the piers keep `Baked`; the pieces carry `PieceOf` the handle
  and take its cell's level (world), so the deck stays whole across a seam. A save from before
  keeps its decks' `Baked` and has no handle: they cast piece by piece, within a colour step. The
  handle sits in the spatial index with a big box: selection and hover may pick it nearest but
  it carries no marks; vision still sees it (a `Sighted` slot).

## underground

- Made after the topography (its shaper levels a tunnel's heads) and before the players; its
  `RunPlan` beside the bridges', before the world. A world without heights panics.
- **A cell has levels, never "a deck or not"** (the user's word, 2026-11-10: N levels, a
  sensible small number): the ground and three storeys, `cell.Floor.Storey` naming one, raised or
  sunk by the sign of its `Height`. Bridges build on storey 0, the underground from storey 1 up
  (`site.vacant`), so the board's bridge keeping (`keepBridges`, a level deck) never reads a
  tunnel and a bridge stands over one. A sunk floor needs `Clear`: the board panics on one
  without.
- **A level is asked of the board by its feet**: `LevelFor`, `KindOn` and `Surface` put on a sunk
  floor whoever has its feet under its roof and within a step over its floor (`stoodOn`); feet
  at +Inf (a pick from above) never. A sunk band's floor is graded between its pieces
  (`surfaces.graded`), the tube's stretches drawn to the same mean at the edge.
- **A mouth is square to its tunnel** (2026-11-11): `site.Plan` refuses a tunnel out across its
  head's corner, since the tube, wider than the corner, runs under the two cells beside, whose
  ground stands in it or, cut, leaves holes under the frame; `site.Square` leads the tool's
  slanting line out across the edges, two cells on before it slants, for a turn in the first
  cell is refused too: its stretch runs along the slope the mouth opens in and its wall
  stands out of it. The same under the
  ground: beside a corner a tunnel crosses the two cells' ground must close over the roof half
  the width off (`Rules.Width`, the plugin's from the kind), or a wall shows through a slope.
  The frame runs in level as far as the topography cuts the ground (`frameIn` over
  `mouthPast`), as high as the cut (`frameOver`, the plugin's `mouthClimb`): a floor falling in
  under a frame laid along it left a gap one saw under the ground through. A mouth is looked
  at with a probe putting the camera before it (`cameras.LookFrom`, `LookAt`, `GRAM_SHOT`); the
  cage lifts an eye put inside.
- **In the finder a sunk storey is entered and left along its links alone** (`crosses`), by a
  body no taller than its room (`roomy`), from piece to piece whatever lies between their floors
  (they run on as one), and the step on the ground from a head into the cell over its first
  piece is refused (`mouthed`): the way there lies down the mouth, where `Surface` drops whoever
  walks it. A head is plain ground with a mouth standing on it, no floor.
- **The seams a search asserts on the terrain fail without a word**: `built`, `lanes`, `limits`
  and `railings` are found by assertion, so a signature changed on one side alone is a board
  searched as though nothing stood on it (it cost the slantwise bridges twice in this stage);
  `spacing/path_finder.go` asserts `*board.BoardRead` against them all.
- The pieces carry `world.Unseen` (the world draws none of them, the plugin's tube does),
  `vision.Transparency{1}` and no shadow; the mouth is the one part with a look. The tube and the
  marks read every sunk floor of the board, whoever built it.
- The ground is cut over a mouth by the terrain's shader from marks in the lattice's free
  channels (the heights' blue and alpha, the normals' alpha at the cell's corner), found by the
  topography in its Publish as the board's passage moves; `U.Mouths` keeps a board without
  tunnels from reading them.
- The cameras needed no change: the cage is free in the room over a sunk floor
  (`topography.Read.Free`), so the eye inside a unit is the unit's and the arm behind it
  shortens before the walls.

## forest

- A kind's look `Slot.Canopy()` shows a unit standing under the entity through it, at
  `canopyOpacity` 0.75, in a view from above alone (`!cam.Projection().Sorts()`), with the setting
  `Canopies` on (`ShowCanopies`, `Read.Canopies`, `Config.Canopies`; `control.Switch` under
  `control.Game`, a new shared section listed in `players.sharedSections`): Publish marks
  `figure.fixed` from the chunk's `Fixed` and queries the space ±0.5 round every moving figure's
  middle into `figures.veiled`; Compose veils a `Fixed` figure of a `flagCanopy` look in that set
  through `CanopyLook.Veil`. `Slot.Cutout()` has a look drawn in render's cutout pass
  (`flagCutout`, `CutoutLook`), both spread to kin like `Lying`; `renderer.mark(canopies,
  cutouts)` from `SetAtlas`. The user's word (2026-11-09): crowns see-through only over a unit,
  never in the perspective, a Game checkbox.

## forest

- A tree's `Side` is drawn on a square stretched to its crown across and its height down: a
  drawer must squash what it draws by the tree's height over its crown, or a round crown is a
  tall ellipse (the topography's `side` helper counts in crown widths); the sprite is
  `spritePx` times that ratio, up to four, so a tall tree's crown keeps its pixels. A crown
  wider than the world's `MaxSize` is refused as the part spawns, a line in the log, no panic.
  `Types.Entry(name)` is a tree for a rule of another aggregate (the settlement's palms); it is
  checked with the stands' mixes as the looks are declared, since the Rules of a board's figure
  run before the forest's kinds are defined.
- Made over the world and the board (`NewPlugin(w, brd)`), no pass of its own: the trees come and go
  by the world's `Unfold`, `Fold` and `Folding`. `Types.Define(name, Spec)` registers a kind of tree
  on `board.StandingTemplate` (so `Fixed`) with the plugin's `TreeRole` (its rule despawns the tree
  at `Folding`) and the game's `Spec.Roles`; `Spec.Solid` adds a `Collider` of infinite mass; a zero
  `Shadow` is `Baked`. The looks are declared by the plugin's own world figure: `Side` at L2, a
  billboard, or on `Slot.Upright()` where `Spec.Body` says (one sprite on the four sides and the
  top: a top look of its own needs the world; measured 2026-11-08: 26 000 bodies in the frame at
  zoom 1 over Madeira cost 6–7 ms of drawing, 60 → 49–55 FPS, so the body is the kind's choice and
  the topography's trees are billboards; the GPU scene of stage `v` is where 130 000 quads belong),
  `Top` at L3, `.Nothing(L4)` (an L3 look counts as declaring L4, so without it `stand()` would
  refuse the photograph; a tree draws nothing at L4 till its sector's photo is ready, and a world
  without sheets shows no trees from L4); nil drawers are drawn of `Colours`.
- `Stands.Define(name, Stand, rules...)` defines the role a game's forest kind (`Aggregate` +
  `Field`) plays: one `Unfold` rule a plot. The lattice cuts a cell into k×k squares (k the
  smallest even number with k²/2 ≥ `Density`), plots on alternate squares, jittered from a fixed
  PCG(7, 11) table; with `Density` 1.5 and `Jitter` 0.25 the plots are the old topography demo's.
  A plot is sown when it lies in the part, on a listed cell and `lod.Seed(forest, sector·plots+k)
  & 0xFFFF` is under the density's share; its kind is drawn by weight (bits 16–39 of the seed)
  among the `Share`s whose band (`From`, `To`, world height units, zero unbounded) holds the
  ground's height at the jittered spot (the board's heights; 0 on a flat map); no band holding,
  nothing is planted. A single share with no band spawns straight and reads no height. `Felled`
  holds for a `Fold` with fewer parts than the stand plants in that stretch; a `Stand.When` is an
  effect of the forest choosing between stands (forest-army's thinning).
- Every tree's look is `.Canopy().Cutout()` (2026-11-09): whole sprites (the `seeThrough` of
  3/4 opacity baked into the pixels is gone from the plugin's `crown`/`standing` and the
  topography's drawers; it made the trees translucent in every view and in the sheets), seen
  through only over a unit from above, drawn in the cutout pass otherwise.
- A stand tests the spot the tree stands on, shaken (`shaken[k]`, where it spawns and reads the
  height), not the plot: `grows(at)` is the cell's bare kind among `Cells`, no way's band of the
  cell or of a neighbour reaching it (`onWay`: the hub of `Width/2` and a strip to `Step(i)/2` per
  `Links` bit, the geometry of `terrain/decks.go` `inBand`, widened by `Width/4 + widest crown/2`,
  the widest crown of the Mix read lazily from the Types, defined after the Rules section) and no
  deck over it (`DecksOver`). A road built later through a standing forest is issue #57.
- The sector's side comes from the world (`Read.SectorSide`). The demos: topography's `woods`
  (`forest.go`: laurel, pine, heather by height; `tree.go` the specs), forest-army's `forest`
  with its felled role and two stands.

## roads, with site

- Made after the board and the topography (it grades through the topography's `Write`), before
  the cameras; `RunPlan` after theirs. A road is a `cell.Way` laid cell by cell, no entity: a
  bridge is a structure, the road (one day a railway) lies on its deck.
- `Res.Orders` (roads confirmed, waiting for their step) and `Res.Receipts` (built, not read by
  the game yet) are saved with the settings; the hands are not: a load forgets a road in hand.
  The renderer draws the orders as waiting.
- `site.Rules` are what the game allows, zero allowing nothing (the user, 2026-11-02), but for
  `Longest`, a cap, 0 no limit. The price is the game's (`Config.Tariff`), the decision its
  rules': `Estimated` and `Built` are `host.StepRules` on the plugin's own entity, run in the
  hand pass at once (the pause too), a hand at a time, the `Refuse` commands drained right after
  each hand's run being that hand's; a rule of a role the plugin `Plays` reads the game's states
  as effects (`cond.On(world)`, an effect on the plugin's entity). Ceremony's doctrine: a price
  is never a rule's.
- `site.Route` is plain A* on hpastar (no HPA*: the ground changes between routes, leaps have no
  backward, the tool wants the cheapest; gramap, whose Madeira is 640×320, keeps each leg's
  search in a corridor round it instead); a `Planner` re-reads the ground on
  every route and allocates nothing once warm, one per (Rules, Graded, Levelling). A step is
  priced by the slope left after the permitted levelling (Cut + Fill), never by the earth; a
  graded cell that cannot be levelled is searched once per way the road came in and left only
  straight (A* would otherwise settle it from the side and never try the climb); a diagonal step
  or a bend lies level, so both cells must level to one height; no corner of water or of a road
  standing is cut; a deck is entered only along its links, a head from its ground side; a ford
  is judged by `Ford` alone, whatever `Wet` says; `Price` is clamped to 1 at least (the
  heuristic), `Metre` 0 counts a step of the grid's Distance as 1.
- `site.Plan` ports gramap's `grade()` to both grids: corners matched across cells by position
  (1e-3·span), union-find classes (a level cell all its corners, a ramp its entry and exit
  edges), ramps relaxed to `min(Slope, RoadSlope)`, `Fixed` corners frozen, a hex ramp's side
  corners on its plane; the ripple is modelled on `relief.settle` with the road's corners pinned;
  `Estimate.Corners` are what grading the `Ways` cells in order leaves, so the plugin reads every
  cell's targets (`Estimate.Levelled`) before it grades and grades the `Ways` cells in order; a
  standing road's cell whose links the plan extends gets its `SetWay` before its `Grade`
  (`KeepRoads` keeps a road under its old links). No way is laid on a deck or a ford: the road
  leads onto them. Two consecutive cells that are no neighbours (gramap's legs, a bridge's heads)
  get no link and no refusal.
- The plugin's `boardGround`: `Wet` is water without land, `Ford` water and land with no deck,
  `Fixed` what stands on the cell (a village, a bridge piece), water and a Graded road's cell,
  `Held` a unit holding the cell; `Price` 1 everywhere (no hook yet).
- Traps: a diagonal run or a bend on a slope needs a level platform, so on a hillside the planner
  prefers straight ramps and the demo's map keeps its natural trips straight; `Earth` is small
  change (a corner moved a metre on a 32-unit cell is ~256 m³); the hand is forgotten after 30
  ticks without a command unless it asks, as a bridge's.

## telemetry

- A reporter runs on the ticks' side, read after the ticks; the panel only wraps and draws the
  last reading.

## minimap

- The lens of a bounded world holds its centre `Reach` from an edge of an axis that does not
  wrap, at the world's middle on an axis narrower than the circle; a torus is not held
  (2026-11-04). The marks walk a query over `Base` and `Appearance` excluding `world.Unfolded`
  and `sector.L6` (2026-11-08, `f/still`: a space query round the minimap and a `Seek` an entity
  cost 2–5 ms a publish over 31 000 unmarked trees); a sprite→mark table filled lazily, an
  unmarked entity one sprite read and no `Base`; a marked box tested against the covered box, a
  world away round a wrapping axis.

- Its lens is no camera of the cameras' (no anchor, not saved) and nothing of `ctx.Draws`: aimed
  by the ticks, drawn as published.
- The marks are published at most 30 times a second and the minimap drawn anew as they are, the
  last picture in between.
- The compass is `Passing` and hides with the minimap: laid over the minimap's box, which reaches
  the screen's corner, it took every press, the wheel and the corner zone.
- A press on it is its own (`cameras.CenterAt`), and the player's picture under it is covered,
  so the cursor there neither scrolls nor turns.

## bullet

- The next step the nearest contact lands a shot just short of it, else its `Ending`; a shot
  that does not `Lands` lies one more step and is despawned.
- A thrown shot gets the `Climb` that brings it down on the ground `WithGround` gives; untargeted
  it goes `Power` of its `Range` (0: the whole).
- No default bindings: motion/bullet throws by Shift held (`control.KeyReleased`).
