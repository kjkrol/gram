# Roadmap

[← Back to README](../README.md)

What is left to do. Take an item out when it lands.

## Engine
- **A dictionary of idioms** — `Once` (a thing done at most once a while, an effect the memory),
  `Either` (a coin), the patrol (`Onward`, `Until[Arrived]`, `Wait`), the asker's "no answer"
  (`Until` under a `Timeout` in a `OneOf`): named shapes written out of the bricks, in
  `doc/behaviour.md` or as functions a game copies, never steps of the engine.
- **Effects, what is left** — an effect taking a tag off while it runs (a `Revoke` beside
  `Grant`); places (`cond.At`) in plans; a step shared by the moments of several plugins; more than
  8 effects on one entity and 63 defined; an entity in more than one group; which effects a player
  may apply (any, today, on whatever a command names — a game over a network will want a list).
- **Messages passed on** — a reply to a message passed on goes to the one who passed it on, not to
  the first asker; passing the answer back is the game's branch (ceremony's `plan/message_test.go`,
  the giver). A loop (A asks B, B asks A) ends by `MaxChain` and by a branch of a fact firing once
  per coming; whether `Hops` should count across types is open.
- **The minimap, further** — marks picked by what a player's units see (`vision.Seen`), the
  other players' views outlined; beyond the bands a picture kept of what was last seen in place
  of the live world.
- **ui, what is left** — `Dialog`, `Toast`, `MenuBar`/`Menu`/`ContextMenu`, `Tabs`, `List`,
  `Radio`; `Canvas` (a tech tree), `Tooltip`, drag and drop, focus moved by keys and pads
  over the controls too; the UI drawn into one `render.Frame` in place of a draw an element, once
  a profile asks for it. Settings of a player's own (its camera's resolution and sight) on the
  settings screen, beside the plugins'.
- **Canals and building on shaped ground** — a cell lowered to the sea turns to water; a preview of
  a shaping drag (lost with the players' marquee); the costs of shaping.
- **A smooth route** — an opt-in `RouteStyle` drawing the line the unit will follow: arcs of
  radius `Speed / TurnRate` through the lookahead points, the queued waypoints marked; computed
  when the route or the waypoints change and kept beside the `Path`, sharing the lookahead's code
  with navigation so what is drawn and what is driven cannot drift.
- **The wind in the pricing** — sailing ships and flyers priced by the wind (`cell.Mobility` by
  domain and way against the wind's), a change of wind counted as one change of the board's
  passage, never a gust: the hierarchy and the kept answers follow it.
- **Why a 16-byte component slows the crowd's tick** — with `unit.Mover` at 16 B, `world.moved`
  and `motion.moveSystem` ran about ten times longer than at 20 B: find the mechanism (a
  column's stride, alignment) and whether other components pay it.
- **The hierarchy, further** — the queued goals' preview planned over it as the walked route is;
  a unit standing where its class cannot stand climbs instead of searching; `Reaches` exact on
  one-way steps (a level-1 label by directed reachability); its rebuilds allocating nothing.
- **Covers** — a state of the ground drawn over its cells (`board.Plugin.Covering`) is the simple
  map's on a square grid: in relief (the topography's painter and its sheets), on a hex grid
  along a line of its own, a ragged edge (noise on the threshold), a cover narrowed to the cells
  that can take it (a lake's shore), one that moves (a storm, a whirlpool). What stands on the
  ground and moves — fire, smoke — is sprites over it, not a cover.
- **Shots and decks** — a shot's ground (`bullet.Plugin.WithGround`) is the heights alone: it flies
  through a deck and falls by its own `Gravity`, which gravity turned over does not touch; both
  through `ground.Surfaces` and the world's `gravity.Pull`.
- **Leaps, climbing and swimming planned** — the planner never plans a leap (a railing, a gap),
  a climb or a swim; a unit under gravity does them only by hand.
- **Bridges that turn** — a bridge runs straight; a knee (two links not opposite) is refused by
  the board's rules and not made by `cmd/gramap`.
- **Bridges and roads taken down** — a player builds both and takes neither down.
- **The road planner's reach** — `roads/site.Route` prices a step by the slope left after the
  permitted levelling and knows no turn cost, so down a slope it lays a staircase of bends,
  which the levelling must flatten into one chain; a bend or a diagonal step is judged a cell or
  a pair at a time, so a chain of level cells building up across many is seen only by `Plan`;
  a road standing costs its length whatever the ground's `Price`, so a dearer ground never
  steers a route off it. cmd/gramap works round them with its ladder of allowances and its
  dearer refused ground; a planner carrying the road's height, as gramap's old one did, would
  see the climb whole.
- **A save in the tactical pause** — what waits for a step is lost by a save taken while the
  clock stands: the world's effect, spawn and despawn commands given in the pause, collision's
  pairs resting since the last tick, traffic's memory of a tick; weathering's dice restart from
  the seed after a load. Each would be a resource or a component.
- **The road tool as the game runs** — a road's levelling under standing units waits for the game
  to move them; taking the tool in hand pauses the game only where `roads.Config.InPause` says,
  and nothing unpauses it as the tool is put down.
- **Baked shadows over a hex relief** — the light bakes over a square lattice of heights alone,
  so a hex grid's relief casts no shade of its own, what casts a Baked shadow (a bridge's deck
  and its piers) casts none there, and what stands there receives only the clouds'.
- **The lamps' shadows in relief** — a lamp casts shadows on level ground alone; over a relief
  its cast would have to follow the ground as the sun's bake does.
- **Clouds shading themselves** — the cover's own shade is the one look of a cloud's thickness;
  a cloud lit by the sun from above and dark below is not drawn.
- **Gamepads** — a trigger vocabulary for pads, so split screen is not only a keyboard's.
- **Networking** — `plugins/netview` over players; the server is one engine, a remote client a
  player whose translator decodes frames:
  - deltas from the player's `View`: entered, updated, left; a client that joins gets all as
    entered;
  - one walk for all clients: a `uint64` mask per entity for up to 64 clients;
  - a binary frame a tick per client: the tick, the camera state echoed, entered
    `[id, kind, sprite, box]`, updated `[id, box]`, left `[id]`;
  - a client without an ECS: a camera, an atlas, a `render.Composer` over the frames, commands
    going out; the transport behind an interface, tests through memory;
  - open: the server's tick against the client's frame rate (interpolation), joining mid-game (a
    snapshot), trust (a LAN to begin with);
  - commands that carry a camera today (`Select`, `Follow`, `Marquee`) carry the player instead.
- **Turn-based movement** — every unit at one tempo and a say in who moves when: a layer above
  navigation setting the tempo for a move and issuing `MoveTo` one unit at a time, waiting for
  each arrival; navigation need not know.
- **Arbitration** — the planner and a reaction (`Flee`) steering one unit in one tick: to start
  with, the reaction wins the tick and the planner re-plans; summed weighted requests only if that
  fails somewhere real.
- **A wall on a steep slope** — collision stops an entity in a solid cell whose band, from below up
  to its kind's `Height` over the cell's level, meets the entity's; on a slope steeper than that
  `Height` over half a cell a unit coming downhill may be partly inside the cell before its band
  meets, and the push out, through the shallowest open side, may put it on the far side. No demo has
  a solid cell on a slope; a wall's band from the lowest corner, or a sealed side, when one does.
- **A sweep across the seam** — a swept entity (`collision.Sweep`) in a wrapping world: its
  stretch in the space as the fragments the seam cuts it into, the segment refined per image.
  Collision refuses one today, with a panic at the first it meets.
- **Shots that go on** — a shot through its target (`Body.Pierces`: the nearest contact a Landing
  that does not end the flight), a trail drawn behind it, a mine that feels a tread (a landed
  shot touching), a wounded unit slowed by its own Z in `collision.Field.Overhang`.
- **Live hydrology** — the water worked out as the game goes: rivers swelling after rain, drying
  in summer, courses changing with the weather and the season.
- **Sight's cost** — over a relief the scan hands aabbworld the lattice (`Cone.Heights`) and
  walks every ray through it, the relief the larger half of a scan (`BENCHMARKS.md`,
  `Benchmark_Vision_Relief`). Ways: skipping ground that cannot raise a ray's horizon (a
  max-height mip of the relief, the walk told the angle to beat; aabbworld's, since the scan
  walks `Cone.Heights` itself); fewer rays or sampling far off (with the LOD); 128-bit lanes for
  the scan's AVX2 kernels, which on every hyperthread lower the clock for the scalar callbacks a
  sampled ground is made of (2–6% on `Vision_Relief/step=*` in parallel); on the profile's path
  (`relief.Along`) clamping with comparisons and folding a cell's corners once, measured −3–4%
  of the scan with the same points, not landed.
- **Aggregates, what is left** — the hierarchy level an abstract route keeps to (hpastar takes
  the highest the distance allows, so a leg may span several sectors; a `level` parameter); the
  first abstract routes wait for the hierarchy's first build (Madeira ~20 s of game, the units
  stand): a faster build or a cell route meanwhile; a field split across levels is drawn whole at
  one camera's level (sector by sector would fix it); a standing aggregate come off the graph
  keeps `sector.Abstract`, so `Met` may fire with a traveller; a game's Keen changed while
  navigation keeps a target keen is overwritten on restore; abstract routes drawn
  (`routes/renderer.go` reads `Path`); re-checking an abstract route after a passage change; a
  right click sending `MoveTo{To}` at the entity under the cursor (navigation's click sends a
  cell, selection exposes no hovered entity); a Fold the moment an unfolded aggregate's last part
  goes, or a `MoveTo` aimed at the nearest of a group, so a charge can last till the foe is gone;
  a unit told Stop in the far mode stands unmarked on no cell; a change of the finest level seen
  re-marks every awake entity in two archetype moves (goke's `Editor` would make it one); the
  zero rungs of a system's `Paces` count from the finest level seen as the others do, so
  collision and vision run two levels past it; a process that exists only up to a scale is cut
  by its query, the aggregate lacking the components the fine entities carry (an army has no
  `Collider`, a forest is no unit), which costs nothing (the user's word, 2026-11-04).
- **The minimap by the ladder** (the user's word, 2026-11-04, the stage after the aggregates):
  a minimap draws at its camera's level +1, its figures at L4 at most (the kind's version at that
  level: the blob, a declared `At(L4, …)`, a `Mark` overriding it), its ground at L5 (the sheet),
  `Keep` thinning the dots in a crowd, and refreshed far more seldom than the main picture, as
  today (the user's word, 2026-11-04); the expanse's minimap, with no ground and marks only
  within the Sight, leaves the circle empty past it: to fill the whole widget.
- **A far figure's cost** — a point at L4 costs ~40 ns drawn (the target 20) and keeping a share
  (`Ladder.Keep`) ~7 ns an entity published: both the level read for every entity, at the
  publishing and again at the drawing. Planned: `doc/plans-perf.md`, "Daleka figura taniej"
  (its parts on the L5 sum are moot since the sheets, 2026-11-05).
- **The sheets, what is left** (2026-11-05) — the sheet casts no shadow on the ground at L4 (the
  light's bake reads `GroundHeights` alone); a look changed under an effect, or an animated look
  on a `Fixed` kind, re-photographs its sector (the sprite is in the signature: a frame of
  animation a frame); a thing across a wrapping world's seam
  is clamped in the photo, not wrapped; the minimap's `Miniature` could read the albedo's
  quarter; `lay` builds a third index set at every relief change (~12 % more indices on
  Madeira); the uniform `AlbedoSize` is read by no shader; `surface.wgsl`'s `albedo()` could read
  `U.AlbedoPx` and `levelAlbedo` go; the tick and the drawing of Madeira jump by ~50 % at about
  two minutes (seen in v0.19.1, cause unknown); the photo mesh costs the island's L4 frame
  ~1.4 ms more than the bare ground (the raised domes' walls rasterised: culling the triangles
  with a sunk corner would bring it to ~0.3 ms, changing the fringe under each top), a round of
  photographs ~0.8 ms a sector over the benchmark's floor (a copy blend in `DrawMeshOptions`
  would spare three draws a round; the albedo and the relief in one image another pass); hover
  and the pointed commands do not know banners, only a click does; a banner's Z is its units'
  mean, so it may sink under a hill between valley units.
- **Madeira with aggregates, what is left** (2026-11-06) — the topography's trees carry no
  `Collider` (`forest.Spec.Solid` gives one; forest-army's have it) and hold no cell, so units walk
  through a forest (a decision: a collider costs the L2 tick, an occupancy the planner); a
  settlement's lamp is in the sheet without its light (the sheet carries no light sources); the flat
  demo reads no fields and its map is not renewed; a change to the rules planting a field is a new
  `SheetsVersion` by hand (a sector's signature knows the kinds in it, not the rules); `Swap`
  schedules the developing only when photos wait, so the dirty tiles re-arm the Defer themselves
  (the condition in `Swap` is the cleaner lever); a sector off a camera's picture is Strategic since
  2026-11-09, so a zoom to L2 unfolds the forests shown and a ring of sectors round them, not every
  field within the Sight (~30 000 trees on Madeira before); the ladder's `Rings` stay unset in the
  topography, a decision of the picture (L3 within a radius of the viewpoint, the sheet beyond); the
  unfolded caravan stands at its goal till it folds (the whole learns its parts arrived only at
  `Fold`: an effect laid under `Unfold`, safe for a point aggregate, or a moment); houses as prisms
  at noon are dark boxes with a roof (a billboard with a facade); only two settlements get a lamp
  (the blocks of 16 cut the big towns into fields under 16 cells; a settlement as one field over its
  cells); a hand-placed entity row goes when gramap regenerates the map (it keeps `units:` alone);
  the caravans stand ~20 s of game at the first start till the hierarchy is built, the runs after
  load it from the cache. Measured and open
  (2026-11-06, `doc/plans-perf.md` "Wynik etapu `d`"): the island at L4 costs 22 ms of GPU a frame
  on an Intel UHD 620 (the whole island in the sheets, 173 sectors with the photo mesh) and a steady
  tick of 8.7 ms where the old map at L2 had 2.1 ms (a CPU profile of the probe is the next step);
  the forest at L2 is 24,000 trees and a tick of 20–30 ms before the still layer and the altitude
  pass set once (`f/still`, `f/space`; the round's result in `doc/plans-perf.md`); a tile is saved
  in 29 ms (`png.BestSpeed`, in a goroutine; the target was 10) and loaded in 2.5 ms (the target 2).
- **The forests in view, what is left** (2026-11-09) — the trees' depth before the ground's
  colour pass, so the ground under a forest is not shaded where a crown hides it (the terrain's
  prime and colour phases split across passes without polluting the ground's readable depth: the
  bigger lever the cutout pass left); the board's standing things (houses) as cutouts, measured;
  what stands far off through a camera with an eye as billboards of the kinds' Side sprites in
  place of points (the GPU scene of stage `v`); a road built through a standing forest felling
  its trees (#57).
- **The shade in tiles, what is left** (2026-11-07) — the trees' `Baked` shadow is a look
  decision (`NoShadow` on the demo's trees is the other); a tile fades not, and overwrites the
  band's "before" where a band of the same frame overlaps it; the frame budget's `Coarse`
  switch still bakes the whole shade (halving or doubling the old image would do);
  `Benchmark_Light_Publish` 0.6 ms (the target 0.4: a reciprocal in `sector.Grid.Of`, or no walk
  when no chunk changed).
- **Tiers and the handle, what is left** (2026-11-07) — only bridges stand on a handle: gramap's
  walls and a settlement's long houses still live piece by piece; `sector.Grid.Fit` costs ~27 % of
  a publish of long things (`Benchmark_World_Seams/long`), an O(1) `Fit` from the box's first
  and last sector columns and rows would take it off; `sheets.wants` tests a box's four corners,
  so a long thing whose middle sector alone is at L4 is photographed nowhere and stays alive;
  the handle is seen by vision (it can take a `Sighted` slot) and may be the nearest thing a
  hover meets, carrying no marks; a save from before the handle keeps its bridges' decks casting
  piece by piece; a point aggregate waiting its turn to unfold is walked as a unit for those
  steps and a `Keen`-held one gets no priority; the longest step at a zoom over Madeira (~170–250
  ms) is navigation: the units leaving the abstract graph ask their routes at once; a persistent
  scene on the GPU (instances in buckets a tier's cell, compute culling, `DrawIndexedIndirect`)
  is stage `v` in `doc/plans-perf.md`; an unfolded whole asleep at L6 whose parts
  are seen is picked but not selected; the sheets' tops pass reads up to 17×17×2 + 11×11 texels
  a corner (a max a quad would be 4–7× fewer) and the photo mesh is drawn in every camera layer
  (culling a layer's slice belongs to render, the ground's too); a `Fixed` collider with a
  finite mass is still judged in `side()` as a body (the still layer never pushes it); the
  weathering walks every cell of the board a second (2 ms a step at
  640×320); the hierarchy's first build takes ~33 s of game (main's 25); each band of the shade
  stretches every block's shadow down to the lowest ground (~11 000 px a tree).

## Landscape and Madeira

- **The surf follows the cells** — the line of breaking waves (`water.Shore`) runs along the
  cells' edges, not the rounded coast.
- **Roads in the isometric view** — dark and thin, covered by the routes: their colour and width.
- **Mipmaps for the ground's details** — `gpu.Texture` holds one level, so a kind's
  `painter.Detail` fades out below 40 px a cell instead of growing coarse, and sampled minified
  it costs ~0.4 ms a frame on an Intel UHD 620; levels baked once (a blit a level) and
  `textureSampleGrad` by the derivatives before `fract` would let it show at every distance and
  under a low camera.
- **Two mouths side by side** — two rivers reaching the sea next to each other look like a "U" at
  the water: the drainage joins them by the shore.
- **The forest plugin, what is left** (2026-11-08) — `Slot.Upright` puts one sprite on the four
  sides and the top, so a pine's top is its side (an Upright taking a top look of its own); a kind
  declaring an L3 look counts as declaring L4 (`.Nothing(L4)` needed, a tree drawing nothing at L4
  till its sector's photo, no trees from L4 in a world without sheets); the flat look (forest-army
  at L2) draws the side sprite from above as an icon; a forest's composition from the map or
  gramap's recipe (today the game's `Stand` by the ground's height alone); the far look of a forest
  from a low eye (a field's L4 piece as a canopy card of the trees' height in the relief's Look: a
  forest has no silhouette on the horizon).
- **Underground, what is left** (2026-11-10) — the tool digs a straight tunnel alone (a level's file turns one at
  `via`); a mouth is a plain box, no frame round the hole, and the sheets photograph the ground
  over it uncut; a tube's stretches leave a wedge open at the outer side of a turn and a
  chamber's wall whole beside a tunnel coming in; its lamps are the shader's own, no
  `light.Source`; on a hex board no mouth is cut and no chamber walled (`topography.mouths`,
  `tubes.room` read a square grid); `cell.Built` grew by two floors a cell (to measure against a
  component apart for the storeys past the first); the ride through a tunnel (V) was checked by
  tests and from above, not yet looked at from inside in a demo.
- **Winter without the slowdown** — snow and ice on Madeira (`weathering`) slow the game down
  badly; a lead: `weathering` asks `air.FallsAt` of every cell once a second of game time, and
  since the clouds came in two layers it works out both layers' noise on the CPU.
- **More knobs of the sky** — the sun and the weather as knobs on the atmosphere's entity, as
  the moon is (`sky.Moon`), so an eclipse or a spell of fog is an effect.
- **Evaporation and fog** — the air masses carry their weather's damp and the rules of the
  board's cells gather it into clouds, but none rises from water (evaporation) and no fog lies
  low at night and at dawn.
- **The weather over a unit, for rules** — the board's cells hear the air over them
  (`weathering.Overhead`); a rule about the rain over a unit (taking shelter) needs it too.

## Open questions

- **`motion.Leaving`** — a moment no game uses: motion tells its rules of an entity past an open
  edge and despawns it with none. Should it stay a moment, or should motion simply despawn
  whoever leaves?
- **The crowd at the goal** — big groups arriving still strike each other where they fan out to
  their spots, a column too; a column through a packed mouth is reordered there and re-forms
  past it; a walk off the field and its last leg are not kept answers (a jam of a group still
  searches round its learned cells as before) — `doc/plans-perf.md` „Kolejka i pole grupy:
  wynik", etap 4.

## Housekeeping

- **A thumbnail in a save** — the frame at the moment of saving, for a load screen.
