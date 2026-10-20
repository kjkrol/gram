# ⏱️ gram Benchmarks

[← Back to README](./README.md)

> What each benchmark measures, the numbers, and what they say about a tick.

## Environment

- **CPU:** Intel(R) Core(TM) i5-8265U CPU @ 1.60GHz (4 cores, 8 threads)
- **Go version:** 1.27.1
- **OS:** Linux

Every number below is the **median of 5 runs** (`make bench-save`, `-count=5`) at commit `651d29a`,
2026-09-22. This machine drifts by up to ~10% between runs, so a change is judged by running the
old and the new tree **alternately**, never one run each; see [How to benchmark](#how-to-benchmark).

All benchmarks live in [`bench/`](bench/) and build their Stage the way a game does — plugins
installed through a headless `game.Initializer`, entities spawned from kinds — then tick the ECS
directly, with no window. "0 allocs/op" means a steady tick allocates nothing; the small B/op on
the larger scenes is the amortised growth of buffers kept between ticks. Since etap 1 of
`doc/plans-perf.md` (2026-10-19) the world and the collision ticks allocate nothing; a frame
with text, a frame with UI and the navigation tick still do (their sections say how much).

## World tick — `Benchmark_World_*`

`Benchmark_World_Tick` is one tick of the world and motion: the plans, steering and velocity
folded into each entity's speed, every box moved under the edge rules, and the space rebuilt from
every entity. `Benchmark_World_PositionScan` is the floor under it: reading every
entity's `Base` through a goke query, chunk by chunk. The boxes stand on a 30-unit lattice, so
5,000 of them fill one 2130×2130 corner of the 4000×4000 torus.

| Scene | Tick | Position scan |
|:---|---:|---:|
| 1,000 boxes, 20×20 each, on a 4000×4000 torus | 40 µs | 2.4 µs |
| 5,000 boxes, 20×20 each, on the same torus | 199 µs | 12.1 µs |

After etap 1 of `doc/plans-perf.md` (2026-10-19, medians of 12 runs, the tree before and after
alternately): 1,000 boxes 35 µs and 5,000 boxes 169 µs, 0 allocs/op — `clampStep` compares
squares instead of calling `Hypot` (and inlines now, which the `Hypot` version did not), and every
plugin's simulation block is bound once. With aabbworld's `Translate` keeping a box inside the
world where it is (v1.13.0, gram's go.mod since `perf/plans-perf`): 27 µs and 128 µs, a
tick 40% shorter than at the start of the day. What is left is the copy of every `Base` into the
space (54%), the one etap 3 removes.

### Drawing — `Benchmark_World_Draw`

One frame of the entity renderer gathered with no screen (nothing is drawn). The renderer draws
what the camera's `view.View` contains: the world refreshes that View once a tick after movement,
asking the Space for the entities in the camera's bounds and marking them in an `EntitySet` by
entity index; a View whose bounds cover the whole world is not queried and sees everything. The
frame then walks the entities and does the work — appearance, projection — only for those in
view; the rest cost one bit test each. 5,000 boxes, 20×20 each, spread evenly over a 4000×4000
torus; the camera views all of it, a quarter, or a twentieth. "Before" is the renderer walking
every entity and testing each against the camera; both measured alternately on the same day.

| Camera view | Boxes in view | Before | After | Speedup |
|:---|---:|---:|---:|---:|
| whole world (4000×4000) | 5,000 | 630 µs | 651 µs | 0.97× |
| a quarter (2000×2000) | 1,250 | 309 µs | 179 µs | 1.7× |
| a twentieth (900×900) | 250 | 221 µs | 42 µs | 5.3× |

The View's refresh is part of the world tick, not the frame; with the camera on the whole world
(as in `Benchmark_World_Tick`) it costs nothing measurable (223 µs before and after at 5,000
boxes), and with a quarter in view it is one Space query, ~20 µs. The remaining cost of a frame is
per drawn box, ~110 ns each, almost all of it in the camera: projecting a box on a torus
(`ToScreenQuads`, `Visible`) goes through `math.Mod` several times. That is the next thing to
optimise if drawing ever shows up.

Since the composer (2026-09-26) the frame is composed: the renderer hands its sprites to a
`render.Frame` and the `Composer` keeps them in order before drawing. Gathering costs about 18% more
for it, measured alternately against the renderer that batched its own quads:

| Camera view | Own batch | Composed |
|:---|---:|---:|
| whole world | 599 µs | 707 µs |
| a quarter | 167 µs | 197 µs |
| a twentieth | 44 µs | 50 µs |

What the frame pays back is the drawing: one call per sheet for everything a view shows, instead of
a call or several per cone, shadow, route and outline: whole frames of the island demos are 25–60%
shorter.

Since the order came from Z (2026-10-15) the figures are laid on the screen by the GPU: the
renderer hands `render.Sprites` world rectangles and the shader projects them through the camera's
matrix, a wrapping world's copies in the same call, so the CPU no longer projects, splits at the
seams or culls every figure (`ToScreenQuads`, `Visible`, `math.Mod`). Measured alternately against
the tree before, three runs each, the median:

| Camera view | Before | After |
|:---|---:|---:|
| whole world | 841 µs | 409 µs |
| a quarter | 233 µs | 121 µs |
| a twentieth | 65 µs | 42 µs |

The drawing of a motion/collision frame (8,388 boxes on a torus, `GRAM_FPS_LOG`) went from 2.15 ms
to about 1.5 ms; its presenting grew by some 50 µs, the GPU projecting four copies of every box.

## Collision tick — `Benchmark_Collision_Tick`

One tick of world plus collision at the collision demo's scales: movement, the space rebuilt,
every overlapping pair found, tested, bounced and pushed apart, with a `CountContacts` trigger
counting. The scene is a 1024×1024 torus covered to a share of its area with square boxes of one
side, each heading somewhere at random, after 120 ticks so the boxes have spread.
"Covering 20%" means the boxes' total area is 20% of the world's.

| Scene | Entities | Tick | Contacts per tick |
|:---|---:|---:|---:|
| boxes 20×20, covering 20% | 524 | 134 µs | 16 |
| boxes 20×20, covering 40% | 1,048 | 534 µs | 91 |
| boxes 10×10, covering 20% | 2,097 | 608 µs | 122 |
| boxes 10×10, covering 40% | 4,194 | 2.32 ms | 695 |
| boxes 8×8, covering 20% | 3,276 | 995 µs | 233 |
| boxes 5×5, covering 20% | 8,388 | 2.88 ms | 916 |

After etap 1 (2026-10-19, medians of 8 runs alternately, the same contacts a tick in every scene):
20×20 at 20% 143 µs, at 40% 547 µs; 10×10 at 20% 647 µs, at 40% 2.64 ms; 8×8 1.04 ms; 5×5
3.35 ms — `rest` finds the pairs met this tick in a set instead of scanning `contacts` for each
(19% of the 5×5 tick down to 6%), and the tick allocates nothing (it made 3 allocations a tick:
two simulation closures and a method value in the world's tick, all bound once now).

The cost follows the entity count more than the contact count: at the same coverage, halving
the box side quadruples the population and roughly quadruples the tick, while doubling the
coverage at one size quadruples the contacts and the tick alike.

## Bands — `Benchmark_Sectors_Expanse`

One tick of world, motion and collision at the expanse demo's scale: a 16384×16384 torus, a
hundred thousand boxes 5 a side spread evenly, sectors 512 a side, the first band 750 round the
middle, the second 2000 at a step in four, beyond it nothing; then the same with the bands
switched off (`SetBanded(false)`), every box moving and colliding.

| Bands | Entities | Tick |
|:---|---:|---:|
| on | 100,000 | 2.42 ms |
| off | 100,000 | 19.1 ms |

With the bands on about 1,600 boxes go every step and 4,400 a step in four; the rest lie out of
the space at no cost but the supervisor's pass over the bands near.

## Sight — `Benchmark_Vision_*`

One tick of the vision plugin alone over a 4000×4000 world: every observer, on a 120-unit
lattice with a 60° cone of radius 200 facing right, scans the shared space and fills its `Seen`.
With outlines, every observer also carries a `SightOutline`, so its view's shape is computed for
drawing. The scan is shared out among the CPUs (`vision.Plugin.WithWorkers`); `serial` is one
goroutine. Measured on 2026-09-29, the better of two runs of 300 ticks.

| Observers | Scan | Scan, serial | With outlines | With outlines, serial |
|---:|---:|---:|---:|---:|
| 100 | 70 µs | 107 µs | 120 µs | 264 µs |
| 500 | 256 µs | 540 µs | 495 µs | 1.31 ms |

On one goroutine about 1 µs per observer to know what it sees, 2.6 µs to also know the shape of
its view; both scale linearly with the observers. On eight threads a tick is a half to a third of
that: what stays on one goroutine is walking the ECS's chunks and, with triggers, running them.
Before 2026-09-29 the harness did not replay the world's clock, so these benchmarks had measured
an empty tick since the clock came; the numbers before that agree with today's serial ones.

### Over a relief — `Benchmark_Vision_Relief`

37 observers on a 192×96 board of rolling hills in relief, cells 333 m, each looking 30 cells
ahead in a cone of 72° from 9 units up: the ground sampled every 50 m, every cell, or read along
the relief's profile (`ground.Profile`: the edges and folds of the cells a ray crosses, exact),
scanned every step or 15 times a second (`vision.Plugin.WithFrequency`). Measured on 2026-10-11.

| Ground | A tick |
|:--|---:|
| every 50 m | 7.28 ms |
| every cell | 1.65 ms |
| profile | 1.97 ms |
| profile, 15 a second | 0.81 ms |

The profile reads two or three points a cell where a cell's step reads one, and is exact where a
step of a cell misses a crest between its samples. In board-topography at 2560×1440 a tick went
from 14–95 ms (every 50 m, the engine catching up on several ticks a frame) to 2–3 ms with the
profile at 15 a second, the frame bound by the GPU.

### The views' copy — `Benchmark_Views_Copy`

One frame's copy of the ground and the cover for the views of the observers (the sight overlay
drawn for a selected unit), over a 192×96 board of 32-px cells, in `plugins/vision`
(`go test ./plugins/vision -bench Views_Copy`); medians of 8 runs at `84fde7e1`, 2026-10-19.
"warm" is a frame in which nothing changed, "changed" a frame after a cell's kind changed.

| Ground | Warm, before | Warm, after | Changed |
|:---|---:|---:|---:|
| flat | 925 µs | 16 ns | 2.1 ms |
| relief | 1.57 ms | 21 ns | 2.8 ms |

Before etap 1 the copy was made anew every frame with an observer on screen, because the board's
cover kept no count for the cache's key (`plans-gc.md` task 1); the `Field` now reports the
cells' `Version` and `Changes`, and the copy is kept until a cell or the relief changes. A change
still costs the whole copy, 18,500 allocations in 2.7 MB: the walk's closures and the encoding.
The demos' `GRAM_FPS_LOG` does not show it: board/topography's drawing stayed at 0.9 ms with the
camera following a selected unit, so the overlay's cost was outside the frame's three timers.

## Ground — `Benchmark_Board_GroundAt`

The ground under 64 points a quarter cell apart along a diagonal of a 256×256 square board of
hills, as sight samples it along one ray in a Quasi3D world. Every cell is an entity, and a read
seeks the cell's `Plot` in the ECS. Measured on 2026-09-25 against the raster of altitudes it
replaced, the two trees run alternately six times each.

| Terrain store | 64 reads | per read |
|:--|---:|---:|
| raster of altitudes, before | 2.55 µs | 40 ns |
| cell entities | 1.49 µs | 23 ns |

The seek is the smaller part of a read; the rest is finding the cell, which a square grid now does
straight from the point.

## Standing — `Benchmark_Board_Altitude`, `Benchmark_Gravity_Standing`

A tick of a 64×64 board of hills 4 cells a side, a deck over one row, with walkers standing still
at their heights: the topography's altitude system standing them on the surface under them, or
gravity standing them as bodies (`gravity.Body`), the board's standing pass and the world's
around them. Measured on 2026-10-11, the two run alternately; the baseline is the altitude tick
before decks were stepped onto by height.

| Walkers | altitude, before | altitude | under gravity |
|--:|---:|---:|---:|
| 100 | 38.6 µs | 39.2 µs | 41.3 µs |
| 1000 | 294 µs | 309 µs | 322 µs |

Only a walker on a deck pays for the deck. Gravity's own pass costs what the altitude system's
did — one surface read a body — and a body standing still skips the rest; what is left is noise
and the board's own passes, which run alike in both.

`Benchmark_Board_AltitudeBands` and `Benchmark_Gravity_StandingBands` make the deck a bridge: a
band half a cell wide running east–west, level, railed. Measured on 2026-10-11, the median of
three runs taken alternately with the plain ones (the machine warm, every figure above today's):

| Walkers | altitude | over a bridge | under gravity | over a bridge |
|--:|---:|---:|---:|---:|
| 100 | 52.1 µs | 51.1 µs | 54.0 µs | 54.9 µs |
| 1000 | 414 µs | 393 µs | 404 µs | 410 µs |

A band costs nothing a step: the decks over a point are read only in a cell a band lies over or
reaches into, and the bridges are held to their rules only when what is built or the terrain
changes, so a step allocates as before. The collision, world and standing ticks of a board without
bands measured alongside the commit before them, alternately, are the same within the noise.

## Terrain — `Benchmark_Board_Terrain`

One tick of world, collision, board and vision over an 80×80 board of 16-unit cells, a quarter of
it rough — forest veiling sight by 0.6 and solid, opaque rock, cell by cell in turn — laid out as
one square block or scattered at random; 200 walkers bounce about, each looking ahead with a 60°
cone of radius 200. With felling, one forest cell a tick is cut down, as a woodcutter would. The
board is the world's solid ground and cover, read from the cell entities as collision and sight
ask; before (at `f1d828d`, aabbworld v1.8.0) it spawned merged terrain bodies into the space and
rebuilt them all on every change. Measured on 2026-09-25, the two trees run alternately five
times, 200 ticks each; the medians.

| Rough ground | Terrain bodies, before | Cells as field and cover |
|:--|---:|---:|
| one block | 4.53 ms | 3.51 ms |
| scattered | 11.11 ms | 4.30 ms |
| scattered, one cell felled a tick | 9.63 ms, 331 KB/op | 4.31 ms, 3 B/op |

Almost all of the tick is sight, and two thirds of that is walking the cells along the rays.
What a scattered layout still adds is edges: where the reach jumps between two samples of a cone
the sweep halves the angle to find the edge, so the cost follows the length of the terrain's
outline in view, not how much terrain there is or how many pieces it is in.


## Shadows — `Benchmark_Board_Shadows`

> Stale since the light plugin (`19174ed0`, 2026-10-19): composing plans the shade and the
> painting is a GPU pass, so `Benchmark_Board_Shadows` now measures ~0.1 µs a frame either
> way. The frame's drawing cost is in the demos' `GRAM_FPS_LOG` (below).

The light plugin planning the shade of a 96×64 board of hills 4 cells a side and 20 high under a
sun low in the west (0.3 up): warm, the sun where it was, and after it has moved, the whole shade
planned anew. The bake itself is the GPU's (a strip a frame as the sun turns, the whole at a leap)
and costs the CPU nothing beyond the plan; measured on 2026-10-19 since the bake left the terrain
for `plugins/light`.

| Shadows | Frame planned |
|:--|--:|
| kept | 0.37 µs |
| planned anew | 0.24 µs |

Before the bake went to the GPU (2026-09-26) the board's renderer worked every shadow out on the
CPU: 2.04 ms kept, 4.14 ms anew.

A corner's shadow is a walk towards the sun a quarter cell at a time over the tops of the cells as
the frame read them, stopped as soon as the line to the sun is above the highest top within 16
cells of the view — two or three steps under a high sun. The first version sought every sample's
cell and ground in the ECS and walked the whole 16 cells: 59.5 ms anew.

## Shores — `Benchmark_Board_Shores`

> Stale since the light plugin (`19174ed0`, 2026-10-19): ~0.1 µs a frame either way, the
> painting a GPU pass; see the demos below for a frame's drawing.

The board's renderer composing the whole of a 96×64 board of sea (shine 0.9) round islands 4 cells
a side every 8 cells, from above — every cell of the sea within 3 of a shore, the worst case: warm,
the shores kept from the frame before, after a cell has changed, every shore worked out anew, and
kept under clouds, the weather laid over every tile. Measured on 2026-09-26, three runs, light in
colour.

| Shores | Frame composed |
|:--|--:|
| kept | 2.89 ms |
| worked out anew | 5.00 ms |
| kept, under clouds | 3.28 ms |

The clouds' shadows and the snow are worked out per pixel by the shader; what they cost the CPU is
a quad over every tile — 0.4 ms for these 6,144 tiles — and nothing under a clear sky.

A corner of the grid is worked out once for the four tiles round it, over the cells as the frame
read them, ring by ring outwards until no ring further out can be nearer. Worked out per tile, five
distances a corner for the way to the shore and each cell's kind sought in the ECS, it took 21 ms.

## Island — `Benchmark_Board_Island`

> Stale since the light plugin (`19174ed0`, 2026-10-19): composing is planning only, 0.1 µs a
> frame from above and in isometry, 0.3 µs in perspective (the camera's rays), the terrain
> painted in a GPU pass; a frame's drawing is in the demos below.

The board's renderer composing the whole of a 96×64 island of cells 32 wide, from above and
isometric: a sea lying `Under` it, earth, sand and rock blending, and the brooks, streams and
rivers `water.Drain` works out of its heights laid across it as curving ways, running out to sea —
warm, and after a cell ashore has changed; and the isometric and the perspective island through a
1080p screen, as a player sees it. The tiles are dressed on every CPU (`board.Plugin.WithWorkers`);
`serial` is one goroutine. Measured on 2026-09-29, the better of two runs of 100 frames.

| View | Warm | A cell changed | Warm, serial | A cell changed, serial |
|:--|--:|--:|--:|--:|
| from above | 2.57 ms | 2.55 ms | 4.30 ms | 4.43 ms |
| isometric | 1.80 ms | 1.80 ms | 2.71 ms | 2.72 ms |
| isometric, 1080p | 0.73 ms | 0.73 ms | 0.91 ms | 0.90 ms |
| perspective, 1080p | 3.95 ms | 3.94 ms | 6.83 ms | 6.83 ms |
| from above, far | 1.57 ms | 1.57 ms | 2.49 ms | 2.52 ms |
| isometric, far | 1.72 ms | 1.73 ms | 2.70 ms | 2.73 ms |

Far — the whole island on a screen of 576 by 384, a cell 6 pixels across — a tile's water lays no
overlay (the landscape's detail), and its shore is not worked out. There the tiles are dressed from
the ground sheet: what lies on a tile is one piece of it, painted once, not a piece per blend and
per stretch of a way. The benchmark counts composing alone; handing Ebitengine the pieces costs as
much again, which the sheet saves too: in the island demos at 1024 by 768, fully zoomed out, a
frame took 10.5 ms of the CPU isometric and 5.0 ms from above before the sheet, 8.9 and 3.1 ms
after it (2026-09-26).

On eight threads the tiles take a half to two thirds of the time they took on one: what stays on
the frame's goroutine is culling the cells under the camera (`onScreen`, every cell of the board
in perspective), painting the ground sheet when the board changed, warming the shores, and
appending every worker's vertices to the frame once more (`render.Frame.Append`).

The composer's own share of handing a frame to Ebitengine — `Benchmark_Composer_Render`, the
draw stubbed: each run of quads copied into the call's buffer and indexed, plain colours given
the sheet's white texel — is 0.35 ms for 18 thousand pieces and 1.7 ms for 72 thousand (since a
vertex carries its height, 2026-10-15, some 5% more: 0.43 and 2.07 against 0.41 and 2.0 that
day); the whole island from above is 15.7 thousand pieces (63 thousand vertices), the perspective
through 1080p 8.7 thousand, the isometric view through 1080p 1.5 thousand. The rest of a frame's
cost past composing is Ebitengine's: it converts every vertex it is handed per call, so a frame is
handed in runs, never whole.

Before, every frame read every cell anew from the ECS, worked out every tile's light three times,
and every blend's weights and every way's curve over again. Now the renderer keeps what it read of
a cell while `Board.CellVersion` says the cell is as it was, a tile's blends and way while the
cells round it are (placed as if the tile stood at 0, 0, lit per frame), and a tile's light while
neither the terrain nor the sun changes; while nothing on the board has changed at all, one
comparison stands for all of it. A cell changing works out its bake and its neighbours' anew, and
nothing else: a changed frame costs what a warm one does. What is left is handing the frame its
quads — the tiles, their outlines, the glints and the ways.

* **A tick is the plugins' RunPlan and nothing else.** The engine adds no work of its own per
  entity; what a Stage pays is the sum of the plugins it runs, in the order it runs them.
* **The world tick with motion is the space rebuild plus a walk.** Moving 5,000 entities and
  handing the space every `Base` costs under 200 µs; the query walk under it is 12 µs.
* **Collisions cost by population.** At the demo's default scale (8,388 boxes of 5×5) a tick is
  under 3 ms, well inside the 8.3 ms of a 120 TPS step; the crowd that halves the demo's TPS is the
  40% coverage one, where contacts dominate.
* **A view is a set, not a list.** The world refreshes each `View` once a tick — the Space's hits
  in the view's bounds, marked by entity index — and the renderer masks its walk with it, so a
  frame costs the visible boxes plus one bit test per entity; a view of the whole world skips the
  query. The same shape serves a view per player or per remote client: one query each, one
  sequential walk for all.
* **Drawing pays for the camera's wrap arithmetic.** ~110 ns per drawn box, mostly `math.Mod` in
  projecting a box onto a torus — a camera optimisation waiting for a reason.
* **The terrain lives in the ECS at no cost.** Reading a cell's ground or kind from its entity is
  cheaper than the raster it replaced, and A* across a 128×128 board costs what it did over the map.
* **Terrain costs what the rays and the units touch.** Collision and sight read the cells in
  place, so a scattered forest costs about what a compact one does, and cutting a tree costs a
  write.
* **Zero allocations once warm.** The terrain benchmarks report 0 allocs/op after the first ticks
  have grown the buffers.

## Behaviour — `Benchmark_Plans_Tick`, `Benchmark_Rules_Moving`, `Benchmark_Effects_Tick`

One tick of the world with its units behaving, through ceremony and its goke binding. The units
stand still on a lattice; a sixteenth of them are told a pulse (a fact) every tick and carry it
eight, so plans begin and end as facts come and go. `Benchmark_Plans_Tick`: every unit on a plan
of two loops, one idling until a pulse comes afresh, one begun by the pulse when a word of it
holds, putting an effect on and waiting a step. `Benchmark_Rules_Moving`: every unit playing a
role of two rules of `motion.Moving`, one holding an effect while a word of the moment holds, one
putting another on by chance while under it. `Benchmark_Effects_Tick`: every unit under an
effect lasting three steps with a `Grant` and an `Alter`, cast again by a plan as it ends.

Medians of 3 runs on 2026-10-13, the tree after ceremony's store went behind its contract
(`ceremony/goke`), run alternately with the tree before:

| Scene | Tick before | Tick after | Allocs before / after |
|:---|---:|---:|---:|
| Plans, 1,000 units | 276 µs | 205 µs | 471 / 479 |
| Plans, 5,000 units | 1.53 ms | 1.29 ms | 1680 / 1795 |
| Rules, 1,000 units | 331 µs | 327 µs | 4 / 3 |
| Rules, 5,000 units | 1.82 ms | 1.67 ms | 4 / 3 |
| Effects, 1,000 units | 498 µs | 478 µs | 503 / 502 |
| Effects, 5,000 units | 2.96 ms | 2.62 ms | 2514 / 2513 |

The allocations are the pulse driver's: adding and removing a component moves the entity between
archetypes (see Markers). A rule's tick allocates nothing of its own; the engine's reads go through
the arrays goke's cursor hands out per chunk, and a fact written is staged once in goke's command
buffer (`CmdBuf.Reserve`).

## Markers — `Benchmark_Marker_*`

What it costs to mark an entity for a while — put a state on it, take it off — three ways, on
10,000 entities of a collider's row (`Base`, `Appearance`, `Collider`, `Physics`, a tag family):

- **one** — a marker component put on and taken off entity by entity (`AddOne`, `RemoveCompOne`):
  the entity moves to another archetype, its whole row copied, both ways;
- **batch** — the same, chunk by chunk through goke's editors;
- **flag** — a bit of a tag family the entity carries for good, set and cleared: a write in place.

`Benchmark_Marker_Toggle` marks a share of the entities on one tick and unmarks them on the next;
the op is the two ticks. `Benchmark_Marker_Tick` is those two ticks with nothing marked — the
system still walking every entity — and `Benchmark_Marker_Find` counts the marked: by the
archetype, visiting the marked alone, or by the bit, visiting every entity of the family. Medians
of 3 runs on 2026-09-30, the tree uncommitted after `c983cec`.

| Marked | one | batch | flag |
|---:|---:|---:|---:|
| 1% (100) | 56 µs | 70 µs | 25 µs |
| 10% (1,000) | 359 µs | 520 µs | 25 µs |
| 100% (10,000) | 5.6 ms | 3.2 ms | 47 µs |
| none (`Tick`) | 16 µs | | |

| Finding the marked | by the archetype | by the bit |
|---:|---:|---:|
| 1% | 40 ns | 8.4 µs |
| 10% | 250 ns | 9.2 µs |
| 100% | 2.2 µs | 15.9 µs |

A marker put on and taken off costs about 160–290 ns an entity; a bit, 1–2 ns. Finding the
marked by the bit costs about a nanosecond for every entity of the family, where the archetype
visits the marked alone. So a state that comes and goes often — more than about 50 entities a
tick in 10,000, or read by a pass that walks those entities anyway — is cheaper as a bit of a
family carried for good; a state that lasts, on few entities, which a pass wants alone, is
cheaper as a component of its own. That is the rule of gram's markers (`comp.Marks`, described in
`entity/tag`): effects' `Idle`, navigation's `Entered`, collision's hit.

## Navigation — `Benchmark_Navigation_Tick`

One tick of the world, motion, board, navigation, driving and selection with N walkers under
orders on a 64×64 board of 16-px cells, median of 5 runs at `51da9307`, 2026-10-19. The walkers
stand on the left half; "open" sends each to the cell mirroring its own on the right half,
nothing in the way but each other; "crowded" sends them all through a wall's one-cell gap to one
cell behind it, where they press, step aside and plan again. Every 300 ticks the orders turn them
round, so the tick measured holds the orders' A* amortised (one order per walker per 300 ticks).

| Scene | Walkers | Tick | Allocations |
|:---|---:|---:|---:|
| open | 100 | 863 µs | 487 / 41 KB |
| open | 1,000 | 10.8 ms | 15,108 / 898 KB |
| crowded | 100 | 8.3 ms | 3,189 / 386 KB |
| crowded | 1,000 | 270 ms | 110,765 / 13.1 MB |

A thousand walkers in the open cost 11 ms a tick, a crowd of them a quarter of a second: the
profile (below) has the path finder (`pathfind.Finder.leveledFor`) at 55% of the open tick and
81% of the crowded one, and under it the board's cell lookups (`terrain.Cells.Kind`/`KindAt`,
`grids.Square.Toward`, `boardLevels.Deck`), through maps. The crowd replans every tick it is
blocked. This is the number the plan's navigation work starts from.

**After etap 2a** (2026-10-20, `perf/navigation`): the planner reads a cell once a search into
a table by its ordinal, walks the directions instead of a `Neighbors` list, and its solvers keep
room for every step of a node. Measured at a fixed count (`-benchtime=1000x` open 100, `200x`
open 1,000, `600x` crowded 100, `12x` crowded 1,000): the scene evolves and the ticks right
after an order cost more, so a tick's mean depends on how many ticks the loop runs, and only
runs of one count compare (the table above was taken at the default benchtime). Base
(`main` `e797a753`) and tree alternately, the median of 3 rounds, an idle machine (load 0.9–1.4).

| Scene | Walkers | Tick before → after | Allocations before → after |
|:---|---:|---:|---:|
| open | 100 | 799 → 425 µs | 479 → 197 |
| open | 1,000 | 9.56 → 6.32 ms | 13,341 → 5,004 |
| crowded | 100 | 2.85 → 1.16 ms | 1,239 → 192 |
| crowded | 1,000 | 306 → 104 ms | 120,735 → 8,372 |

What remains in the crowded profile is astar's own dictionary maps (about 12%) and heap (5%),
`leveledFrom` 62% cumulative, and in the open one `stepAside` (25%, etap 2b).

### A route — `Benchmark_Navigation_Path`

One route across a 200×200 board of rocks and walls, the planner alone, `-benchtime=40x`: 94.3 ms
and 14,740 allocations before etap 2a (the part's own base, the benchmark added to it), 18.4 ms
and 19 after (median of 3 on an idle machine). `BenchmarkGrids_Neighbours`
(`plugins/board/internal/grids`): `Toward` over every cell of a square grid 8.0 → 3.3 ms, on a
torus 15–26 → 3.3 ms; the allocating `Neighbors` list costs 4.1–5.6 ms.

## Shots — `Benchmark_Bullet_Flight`

One tick of the bullet, world, motion and collision plugins on a 1024-px square with shots
fired every tick from the left edge, each a second in the air (400 units at 400 a second),
straight or thrown and coming down at its range, where it is despawned; median of 5 runs at
`51da9307`, 2026-10-19. `inflight` is how many are in the air at the end.

| Shots a tick | In flight | Tick, straight | Tick, thrown | Allocations |
|---:|---:|---:|---:|---:|
| 1 | 61 | 18.3 µs | 18.5 µs | 9 / 368 B |
| 5 | 305 | 73.5 µs | 72.8 µs | 19 / 2.6 KB |
| 20 | 1,220 | 363 µs | 356 µs | 38 / 10.9 KB |

About 0.3 µs a shot a tick, thrown as cheap as straight; the allocations grow with the spawns a
tick, not with the shots in the air.

## Text and a frame with UI — `Benchmark_Text_Panel`, `Benchmark_Frame_UI`

Frames drawn on a headless GPU (`gpu.Headless`; skipped where there is none, `GRAM_GPU=software`
for the rasteriser) into a 1280×720 screen, median of 5 runs at `51da9307`, 2026-10-19.
`Benchmark_Text_Panel` is a panel of 20 lines of text changing every frame, as a telemetry
panel's; `Benchmark_Frame_UI` the world of 5,000 boxes shown through a `ui.Image` of a feed over
the whole screen, the same panel and a button over it.

| Frame | Time | Allocations |
|:---|---:|---:|
| the text panel | 261 µs | 486 / 149 KB |
| the world, the panel and a button | 508 µs | 703 / 160 KB |

After etap 1 (2026-10-19, medians of 12 runs alternately): the panel 240 µs and 452 allocations
in 20 KB, the frame with UI 472 µs and 646 in 28 KB — the GPU device keeps its passes, submits,
bind offsets and the present quad, text is drawn without splitting its lines and the pieces are
kept. The allocations left are mostly gogpu's: its Vulkan HAL boxes every `vkCmd*` argument for
the FFI call (`hal/vulkan/vk/commands_gen.go`), about 36 a draw, plus the benchmark's own `fmt`
of the 20 lines; a frame of 4 draws cannot go below about 240 on this backend.

The panel's 261 µs are 28% calls into Vulkan (`runtime.cgocall` under goffi), 28% the glyph
pieces (`gpu.DrawPieces`, `device.triangles`), 6% the font's glyph index (`sfnt`) and the rest
the text's own building; its 486 allocations a frame are the `fmt` of the lines and the pieces.

## Climate — `Benchmark_Climate_Step`

One step of the climate over a 6528×3392 world with heights, median of 5 runs at `51da9307`,
2026-10-19: the wind blowing, a weather forced, the air still, patchy weather.

| Regime | Step | Allocations |
|:---|---:|---:|
| wind | 5.3 µs | 5 / 1.0 KB |
| patchy | 5.6 µs | 5 / 1.0 KB |
| forced | 83 µs | 6 / 1.3 KB |
| still | 123 µs | 7 / 1.1 KB |

## Where the time goes — CPU profiles

`go test ./bench -run xxx -bench <name> -benchtime=3s -cpuprofile p.out`, `go tool pprof -top`,
one run at `51da9307`, 2026-10-19; the share is of the samples, `cum` where a function's
callees are counted with it. The island, the shadows and the shores are not here: since the
light plugin, composing them is planning only, the painting a GPU pass (see their sections).

| Benchmark | Where |
|:---|:---|
| `World_Tick/5000` (212 µs; after etap 1 128 µs: `rebuild` 54%, `Translate` 14%, `inside` 6%, `clampStep` 5%) | `motion.moveSystem.rebuild` (the copy of every `Base` into `aabbworld.Item`, `Grid.Rebuild`, `remember`) 34% cum; `clampStep` 27% cum, of it `math.archHypot` 20%; `aabbworld.Surface.Translate` 27% cum (`fold` 14%, `move`, `wrap` 6%) |
| `Collision_Tick/rect=5,fill=20%` (4.1 ms, 917 contacts; after etap 1 3.35 ms: `Solve` 33%, `visit` 26%, `rest` 6%) | `collide.Solver.Solve` 30% cum (`penetration` 10%); `spatial.pairSweep.visit` 18% cum, `add` 6%, `sort` 2%; `collisionSystem.rest` 19% cum, of it `met` 13% flat; `mark` 5%; motion's `Update` 9% (`rebuild` 3%); `addr.Index.GetUnchecked` (the `Seek` in `moved`) 3% |
| `Vision_Relief/step=50m` (6.6 ms) | `raycast.castElevated` 99% cum; `relief.Relief.GroundAt` 62% cum (31% flat), `index` 10%, `corner` 12%; `vision.scanner.sunkGround` 74% cum; `raycast.ray.ground` 8% |
| `Vision_Relief/profile,15Hz` (0.68 ms) | `raycast.castElevated` 93% cum; `relief.Relief.Along` 56% cum (21% flat), `folded` 13%, `cellCorners` 10%; `field.Field.walkSquare` 18% cum; `ray.ground` 9% |
| `World_Draw/view=100%` (427 µs, 0 allocs) | `renderer.Compose.func1` (the per-entity work) 22% flat; `look.Flat.Sprite` 59% cum, of it `lay` 42% (`render.VisitWrapImages` 36%, `Sprites.Rect` 20%, `piece` 9%); `Frame.At` 9%; map lookups 9% |
| `Navigation_Tick/open,1000` (8.6 ms) | `navigationSystem.Update` 91% cum; the A* `pathfind.Finder.leveledFor.func1` 55% cum, `Price` 19%; `boardLevels.KindAt` 17% cum, `terrain.Cells.Kind` 13% (9% flat), `KindAt` 14%; `grids.Square.Toward` 8%, `DiagonalNeighbors` 9%, `wrapModI64` 4%; `boardLevels.Deck` 6% |
| `Navigation_Tick/crowded,1000` (299 ms) | the A* 81% cum, `Price` 27%, `enterable` 17%; `boardLevels.KindAt` 20% cum, `Cells.Kind` 15% (10% flat), `Cells.KindAt` 17%; `grid.Way` 12%; `Square.Toward` 10%; `runtime.mapaccess2` 6% |
| `Text_Panel` (258 µs; after etap 1 252 µs: `cgocall` 28%, `gather` 18%, `sfnt` 7%) | `runtime.cgocall` 28% (Vulkan through goffi); `render.DrawText` 41% cum, `gpu.DrawPieces` 28%, `device.triangles` 24%; `sfnt` glyph index 6%; `memmove` 5%; `AppendUint32` 4% |
| `Frame_UI` (508 µs; after etap 1 518 µs: `cgocall` 15%, `Mesh` 12%, `Compose` 25% cum) | `runtime.cgocall` 20%; `world.renderer.each` 29% cum; `gpu.Mesh` 13%; `device.triangles` 15%; `gpu.DrawPieces` 14% |

What they say: the world tick is the copy for the space (a third), `Hypot` (a fifth) and the
torus arithmetic (a quarter), not the iteration; the collision tick is the solver and the pairs
sweep, with gram's own resting pairs a seventh; the sight over a relief is the relief's sampling
under the rays; the entity renderer is the assembling of the sprite instances, three fifths; the
navigation tick is the path finder and the board's cell lookups under it; text is half the GPU
calls and half the glyph pieces. No profile has the garbage collector in its first thirty lines.

## The demos — the garbage collector and the frames

Every demo for 60 s in a copy with a probe (`gram-probe` without the tempo change), vsync on at
60 FPS, `GODEBUG=gctrace=1 GRAM_FPS_LOG=1`; then 35 s with `GRAM_VSYNC=off` for the frame costs
alone, medians of the per-second lines after 8 s of warm-up, at `51da9307`, 2026-10-19 on the
machine above (Intel UHD 620, Vulkan). The light demo got Shift+L at the fifth second (the
lamps' shadows), navigation/square its four player units sent together to the bridge's foot at
the tenth and fortieth second and west at the twentieth, motion/bullet the soldier firing a
round every 20 ticks and a grenade every 90. "tick" is a frame's simulation: at 60 FPS a
120-TPS demo (motion/collision) runs two steps in it. "moved" is the share of the entities
whose box changed between two ticks, sampled every second.

| Demo | GC cycles | Live heap, peak | Tick | Drawing | Presenting | Moved |
|:---|---:|---:|---:|---:|---:|---:|
| board/topography | 0.42/s | 92 MB, 272 MB | 2.14 ms | 0.89 ms | 0.24 ms | 5.5% of 122 |
| look/light | 0.45/s | 23 MB, 105 MB | 0.14 ms | 0.46 ms | 0.22 ms | 44% of 6 |
| navigation/square | 0.22/s | 24 MB, 97 MB | 0.45 ms | 0.31 ms | 0.21 ms | 32% of 8 |
| motion/bullet | 0.17/s | 22 MB, 96 MB | 0.15 ms | 0.23 ms | 0.19 ms | 60% of 7 |
| motion/expanse | 0.23/s | 84 MB, 400 MB | 3.24 ms | 1.00 ms | 0.28 ms | 4.8% of 100,000 |
| motion/collision | 0.25/s | 45 MB, 126 MB | 10.5 ms | 1.48 ms | 0.26 ms | 100% of 8,388 |
| look/animation | 0.30/s | 22 MB, 104 MB | 0.14 ms | 0.35 ms | 0.21 ms | 35% of 5 |
| rules/dialog | 0.18/s | 22 MB, 96 MB | 0.13 ms | 0.25 ms | 0.20 ms | 0% of 3 |

After etap 1 the frame's heap allocations in motion/collision (a probe reading
`runtime.MemStats` once a second, vsync on) went from 152 to 119 a frame; the pictures of
board/topography and look/light at frame 120 with a fixed seed are the same before and after
(65–79 of 400–800 thousand pixels differ, the clouds' and the animations' timing).

With vsync off, the frame rate and a frame's costs (the tick is then spread over many frames):

| Demo | FPS | Tick | Drawing | Presenting |
|:---|---:|---:|---:|---:|
| board/topography | 198 | 0.67 ms | 0.88 ms | 0.25 ms |
| look/light | 592 | 0.02 ms | 0.58 ms | 0.37 ms |
| navigation/square | 640 | 0.05 ms | 0.27 ms | 0.19 ms |
| motion/bullet | 1,300 | 0.01 ms | 0.14 ms | 0.14 ms |
| motion/expanse | 596 | 0.32 ms | 0.61 ms | 0.20 ms |
| motion/collision | 160 | 4.2 ms | 1.53 ms | 0.25 ms |
| look/animation | 1,196 | 0.01 ms | 0.26 ms | 0.14 ms |
| rules/dialog | 1,320 | 0.01 ms | 0.16 ms | 0.14 ms |

`GOGC` as an environment variable, the same 60 s with vsync on; a frame's tick and drawing
did not move by more than the drift in any of them:

| Demo | GOGC=100 | GOGC=200 | GOGC=400 |
|:---|---:|---:|---:|
| board/topography | 0.42/s, peak 272 MB | 0.21/s, peak 323 MB | 0.14/s, peak 472 MB |
| look/light | 0.45/s, peak 105 MB | 0.24/s, peak 136 MB | 0.12/s, peak 156 MB |
| motion/expanse | 0.23/s, peak 400 MB | 0.21/s, peak 578 MB | 0.11/s, peak 676 MB |
| motion/bullet | 0.17/s, peak 96 MB | 0.07/s, peak 166 MB | 0.06/s, peak 109 MB |

What they say: the collector runs once every two to six seconds in every demo and gctrace's
cumulative CPU share stays at 0%; at 60 FPS no demo's frame is bound by it, nor by anything but
the simulation of motion/collision (two steps of 8,388 boxes a frame) and the ticks of
board/topography and motion/expanse. Raising `GOGC` halves the cycles and raises the peak heap
by a third to a half for nothing measurable in the frame, so there is no reason to set it in
code. The allocations per frame (`plans-gc.md`) are a tidiness, not a cost, until a cycle lands
on a frame; none did in these minutes. The moved shares are the number behind any change
tracking per chunk: the board demos move 5% of their entities a tick, the expanse 5% of its
hundred thousand (the bands), the motion demos all of them.

## The island's frame — `Benchmark_Frame_Topography`

The island of `Benchmark_Board_Island` drawn through the composer on a headless GPU into a
2560×1440 screen and read back every frame (the readback is the wait for the GPU; "readback"
alone is its cost, taken off below); medians of 4 runs at `b9f2cffe`, 2026-10-19, Intel UHD 620.
The world of sectors (256 units) is cut only for the care cases.

| Frame | Time less the readback (7.7 ms) |
|:---|---:|
| isometric | 8.0 ms |
| perspective | 11.8 ms |
| isometric under a distant sun | 7.8 ms |
| isometric without the water layer | 6.7 ms |
| isometric at 0.7 of the pixels (1792×1008) | 3.8 ms |
| isometric, full care everywhere | 7.6 ms |
| isometric, full care within 2 sectors | 7.6 ms |
| isometric, full care within 1 sector | 6.8 ms |

What it says: the frame is bound by the pixels; 0.7 of them costs half. The water's detail is
the island's whole care cost (full care within one sector lands where the water layer is off);
the ground's shade and the lamps cost nothing measurable here.

The demo, board/topography in a window of 2560×1440 with `PROBE_SEED=7`, `GRAM_VSYNC=off`,
medians of ten seconds after five of warm-up, the keys pressed at the second second:

| Picture | FPS |
|:---|---:|
| all the pixels, care everywhere | 45 |
| 0.72 of the pixels (Alt+- twice) | 70 |
| 0.52 of the pixels (Alt+- four times) | 106 |
| full care within 2 sectors (Alt+Shift+- twice) | 48 |
| full care within 1 sector (Alt+Shift+- three times) | 53 |
| 0.72 of the pixels, full care within 2 sectors | 71 |
| perspective, full care within 1 sector | 51 (46 with care everywhere) |

With vsync on (a 100 Hz screen) and the picture following a frame budget of a sixtieth (Alt+A),
the demo went from 45 FPS to 57 within three seconds and held there: the band of 0.92–0.98 of
the budget is where the adapter rests, and every thirty seconds it tries a finer picture and
backs off (67, 54, 65 FPS in the log). The resolution costs the CPU nothing; the care costs one
draw call more a ring.

**In layers** (etap R2 of `doc/plans-perf.md`, `76c18e9b`, 2026-10-19): the picture drawn in three
layers, the whole frame at a coarse resolution, the central 60% finer, the central 35% at every
pixel, each a centred crop of the same camera laid over the last with a feathered edge. The
layers overlap, so the preset 0.6 / 0.8 / 1.0 draws 0.71 of the frame's pixels and the ladder's
floor 0.4 / 0.6 / 0.7 draws 0.35. Medians of 5 runs of the island, the readback (7.9 ms) taken
off:

| Frame | Time less the readback |
|:---|---:|
| isometric | 7.3 ms |
| isometric in layers (0.5, 0.75, 1: 0.575 of the pixels) | 5.9 ms |
| perspective | 11.1 ms |
| perspective in layers | 7.9 ms |

The demo as above, the baseline `a25a4af2` and this tree run alternately (the machine's
baseline that day was 58 FPS, not 45):

| Picture | FPS |
|:---|---:|
| all the pixels, baseline / this tree | 58 / 58 |
| 0.85 of the pixels (0.72 of them drawn) | 70 |
| in layers, the preset (0.71 drawn) | 67 |
| 0.72 of the pixels (0.52 drawn) | 88 |
| in layers, the outer layer at 0.4 (0.51 drawn) | 85 |
| perspective, 0.85 of the pixels | 67 |
| perspective, in layers, the preset | 64 |
| perspective, 0.72 of the pixels | 83 |
| perspective, in layers, the outer layer at 0.4 | 80 |

What it says: at the same pixels drawn the layers cost 3 FPS (4%), the three composes on the CPU
("drawing" 1.2 → 2.1 ms in isometry, 1.3 → 2.1 ms in perspective) and the two blits, and buy the
centre at every pixel. The layers do not beat a uniform picture on frames; they spend the same
budget where the eye is. Shots of frame 600 in both views: no seam at the 60% and 35% edges,
the water's glint differing a little inside the 20-pixel feather. The adapter in layers was not
timed: the baseline's 58 FPS lies inside its rest band (55–59 of a budget of 60).

**The ground at the camera's resolution, the figures at every pixel** (etap T of
`doc/plans-perf.md`, 2026-10-20, the base `e5ec6dbc`): the ground drawn at 0.7 of the pixels
into its own image and scaled up, then the figures, the clouds, the rain and the marks drawn at
every pixel over it, tested against the ground's depth spread to the screen. Medians of 4 runs,
the readback (7.3 ms) taken off; the base and the tree run alternately:

| Frame | Time less the readback |
|:---|---:|
| isometric (base / tree) | 7.9 / 7.9 ms |
| perspective (base / tree) | 11.6 / 11.6 ms |
| isometric at 0.7 through a feed, the figures at the ground's resolution | 4.7 ms |
| isometric at 0.7 through a feed, the figures at every pixel | 6.4 ms |
| perspective at 0.7 through a feed | 6.7 ms |
| perspective at 0.7 through a feed, the figures at every pixel | 8.5 ms |
| isometric, full care within 1 sector, three cares (base) / five (tree) | 6.9 / 7.3 ms |

The demo as above, the base and the tree alternately:

| Picture | FPS |
|:---|---:|
| all the pixels, base / tree | 57 / 57 |
| 0.72 of the pixels: base / the figures at every pixel / the figures soft | 86 / 76 / 86 |
| 0.52 of the pixels: base / sharp / soft | 128 / 108 / 128 |
| perspective, 0.72: base / sharp / soft | 82 / 73 / 82 |

What it says: the pass over the ground costs a fixed 1.5–1.8 ms a frame, whatever the ground's
resolution (the clouds are drawn at the screen's size there, the figures and the marks are few
pixels), so at 0.52 of the pixels the picture runs at 108 FPS with the units' edges crisp
against 128 with them blurred and 57 at every pixel. The five cares cost 0.4 ms more than the
three at R = 1 (two more draws a frame); the far water goes flat two sectors sooner.

**The picture in layers from one composition, footed in a perspective, the figures sharp over
it** (etap U of `doc/plans-perf.md`, 2026-10-20, the base `b6ecc82c`, etap T merged): a picture
in layers composed once in the frame's pixels and drawn a layer at a time with the pieces placed
into each; in a tactical perspective the inner layers stand on the frame's bottom edge (footed);
the figures at every pixel drawn over layers too, tested against the layers' depths placed into
the screen's. Medians of 4 runs, the readback (7.8 ms) taken off; the base and the tree run
alternately. The preset's layers: 0.6 of the pixels whole, the middle 60% at 0.8, 35% at all.

| Frame | Time less the readback |
|:---|---:|
| isometric (base / tree) | 10.0 / 10.6 ms |
| perspective (base / tree) | 14.1 / 12.8 ms |
| isometric in layers: three composes (base) / one (tree) | 8.0 / 7.0 ms |
| perspective in layers, inset: base / tree | 10.4 / 9.4 ms |
| perspective in layers, footed | 8.9 ms |
| isometric in layers, the figures at every pixel | 10.2 ms |
| perspective in layers, footed, the figures at every pixel | 11.6 ms |
| isometric at 0.7 through a feed: soft / sharp (base) | 6.0 / 8.2 ms |
| isometric at 0.7 through a feed: soft / sharp (tree) | 5.2 / 7.5 ms |
| perspective at 0.7 through a feed: soft / sharp (base) | 8.2 / 10.8 ms |
| perspective at 0.7 through a feed: soft / sharp (tree) | 7.5 / 10.1 ms |

The demo at 2560×1440 without vsync, 35 s a run, the medians after 8 s, the base and the tree
alternately, both profiles still (no adapter):

| Picture | FPS | drawing |
|:---|---:|---:|
| all the pixels: base / tree | 63 / 63 | 1.1 / 1.1 ms |
| the preset's layers, the figures soft: base / tree | 70 / 70 | 2.0 / 1.7 ms |
| the preset's layers, the figures at every pixel (tree) | 64 | 1.6 ms |

What it says: one composition takes a millisecond off a frame in layers on the island (8.0 →
7.0 ms, 10.4 → 9.4) and 0.3 ms off the demo's drawing (2.0 → 1.7 ms; the Directs still draw a
layer each), which the GPU-bound demo does not turn into frames; a footed layout in perspective
is half a millisecond cheaper than inset (less of the frame drawn twice). The figures at every
pixel over layers cost 2.7–3.1 ms on the island (the over pass at every pixel, three depths
placed) and 6 FPS in the demo. The whole picture and the 0.7 cases are within the machine's
drift of the base.

## How to benchmark

```bash
make bench        # the whole suite once, with allocations
make bench-save   # 5 repeats, raw output under bench_results/ (ignored by git)
```

A profile beside a benchmark, for where a tick goes rather than how long it takes:

```bash
go test ./bench -run xxx -bench 'Benchmark_World_Tick/entities=5000' -benchtime=3s \
  -cpuprofile p.out -o bench.test && go tool pprof -top -nodecount=30 bench.test p.out
```

The benchmarks link Ebitengine but open no window; in CI they run under `xvfb-run` like the
tests. To judge a change on this or any drifting machine, keep a copy of the baseline tree and
run the two alternately, then compare medians:

```bash
cp -r . /tmp/before          # before the change
for i in 1 2; do
  (cd /tmp/before && go test -run xxx -bench . -count 4 ./bench/... | sed 's/^/BEFORE /')
  go test -run xxx -bench . -count 4 ./bench/... | sed 's/^/AFTER  /'
done
```
