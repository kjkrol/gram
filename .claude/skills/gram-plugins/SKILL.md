---
name: gram-plugins
description: Reference of gram's built-in plugins, one entry each — world (with entity, view, clock), motion (with steering), light, board (cells, units, standing entities, bridges, maps as files, gramap), topography, atmosphere, collision, navigation, selection, players and owner, dialog, cameras, driving, gravity, vision, bridges, telemetry, minimap, bullet: how each is wired and in what order, what it rests on, its invariants, the user's decisions, the traps and the ideas dropped. Use when working in or with a specific plugin, before reading its code.
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
  distance from camera.Viewpoint)`, wrap-aware (`Grid.Nearest`), finer at once and coarser only
  past `Ladder.Hold`'s hysteresis (a threshold by 10%, the Sight by half a sector), then grades
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
- The figures' colour at L5 (`Plugin.Aggregates`, asked once by a ground's renderer in its
  constructor: the topography's and the board's; a world none asks records L5 figures as before
  and spends nothing): the publisher sums a figure some camera sees at L5 — one with a look of
  its own, no material, nothing declared at any level — as its blob's mean colour
  (`render.MeanColour`, kept at `Close` for every sprite in the kind's place) premultiplied, by
  how much of each texel (`Ladder.Texel`) its box covers, into tiles of 64×64 texels in a
  `render.Buffered` pair: only the tiles a frame wrote exist (a sector left is clear the next
  frame, a still world writes nothing), the world's `Compose` uploads once a frame the tiles whose
  signature changed, and a tile emptied is written as zeros. A figure seen at L5 by one camera and
  finer by another is both summed and recorded; `Keep[L5]` thins the sum as the records. The frame
  image is `world.AggregateImage`, bound per draw through `opts.Uniforms` (seven of the eight named
  images are in use now). `sector.Grid.Of` truncates, no float64.
- Aggregates (`wholes.go`, 2026-11-04): `Aggregate`, `Field{Size}`, `PartOf`; the `wholes` system
  (after the moments', `lod.Steady`) reads each aggregate's level at its centre (`levelAt`, not the
  mark swaps: a spawn mid-tick has no mark yet), keeps a world-made `stretch` entity per
  unfolded (aggregate, sector) with the fold's due time (saved; `Unfold`'s subject, which is how
  `SpawnPart{At}` finds the sector's `Box`), tells `Folding` to the parts whose centre lies in
  the sector, moves a point aggregate to its parts' centroid before `Fold` (`gather`, the space's
  runs re-gathered), and checks `Met` among the bodies carrying `sector.Abstract` only when a rule
  handles it (`meeting` entities remember a pair). `Grid.Cover` cuts a box by sector. A field's
  record keeps its own box plus `Size` in a side table; `renderer.over` picks per camera: the
  level read on the field box, drawn over it from L4 (the point path scales the look), over its
  own box short of it; seen wherever the field box meets the view, asleep too (the render query
  keeps L6 chunks carrying `Field`); never thinned, no shadow over the field; a box over 8×8
  texels sums its inside a run at a time and weighs its rim alone. `Unfolded` (first Unfold →
  the step after the last Fold): a point aggregate publishes no record while it carries it, a
  field is published as one record per covered sector (`renderer.covers`), each at its own
  sector's level, none for an unfolded sector nor for one whose stretch is `Folded` till its
  parts are gone (the stretch outlives the Fold one step). `Keen`/`Calm` commands are carried in
  the despawn system. **goke's `Query.Seek` ignores a query's Exclude and tag filters**: what a
  Seek must leave out is read as an optional column (the minimap's marks, the keeping's release).

## motion, with steering

- It runs after the effects' pass: an effect on the steering tells the step it begins; what a
  rule of `Moving` or `Leaving` puts on or takes off lands with the next step's effects.
- The velocity pass scales the speed by `steering.Pace`, so a speed is asked before the ground's
  share is applied (navigation's `Trail` divides by the unit's own share).

## light

- The plugin declares and publishes; the lighting is render's (`render/internal/compose`:
  `lighting.go`, `bake.go`, `cast.go`).
- The bake: nothing over a hex relief or without a sun; level ground is baked only while a
  `Baked` block stands, which gives a flat board its towers' and bridges' shadows; a bake asked
  before the frame holds the heights plans the whole anew.
- "Shadows follow the frame budget" (`AdaptShadows`) makes the light a `cameras.Lever`: as the
  rate falls it drops the lamps' shadows first, then bakes coarse; back the other way.

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
  `units:` kept.
- The board's field (collision's `Field`) and cover (sight's) read the cell entities whenever
  asked; nothing copies them.
- The flat board is drawn on the GPU from an image of its cells (`look/tiles.go`): the CPU writes
  the cells' data, the shaders work out the shapes; a changed cell is rewritten with its
  neighbours, all of them past an eighth. Nothing is composed on the CPU: a camera without
  `Rays`, or a board of no world, draws nothing. The tiles' shaders decode the sector's level;
  at L5 they lay the figures' colour the world sums (`world.Read.Aggregate`) under the tile's own
  alpha, in the plain pass and the full one alike, bound only while a sector seen is at L5.
- A unit carrying `sector.Abstract` (2026-11-04) gets pace share 1 from the standing pass
  (`moments/standing.go`) and the occupancy's keep (`occupancy/keep.go`) re-enters it into no
  cell; a chunk carrying `sector.Abstract` or `world.Unfolded` gets no standing rules and no fall
  log. `occupancy.PlaceSystem` (before standing) moves a part or aggregate holding no cell yet,
  laid where its domain may not stand or on a held cell, to the nearest cell taking it within
  three rings: the library's answer to parts spawned by offset.

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
  the view: sight reads them in a top-down game too.
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
  L4 on the coarse lattice (every second corner, the edge ring stitched by fans, crack-free)
  with still water a mirror and no lamps or baked shade, L5 the sheet: the painter's albedo as
  the only colour on the coarse lattice (a texture, not a fan: the L4|L5 edge shares vertices),
  under the figures' colour the world sums (`world.Read.Aggregate`, laid over the albedo before
  the light and the fog, the skirt bare), lit by ambient and the sun's diffuse alone so it does
  not glow at night; L6 nothing; the grid at L2 alone; the skirt always L5 in a world with
  sectors, L2 without. Finest first, to prime the depth. A hex board draws tops alone at L3 and
  L4 and one quad a sector from its tops' image at L5, the summed colour over it. No care rings,
  no care settings (the user's rule: one level a sector).
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

## navigation

- **Levels** (2026-11-04, second design): a unit's `bodies.Member.Level` is its own sector's
  mark (`bodies.Levels`, once a chunk). At L4 or coarser the unit walks the same refined route of
  cells the climb gives (`Finder.Climb`, 64 steps, re-asked when spent; the hierarchy not ready →
  the ask waits: Madeira's first build ~20 s of game) in the **far mode** (`traffic/far.go`):
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
  (2026-11-04). An aggregate carrying `world.Unfolded` draws no mark (read as an optional
  column: `Seek` ignores Exclude).

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
