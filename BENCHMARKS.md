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
the larger scenes is the amortised growth of buffers kept between ticks.

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
* **Zero allocations once warm.** Every benchmark reports 0 allocs/op after the first ticks have
  grown the buffers.

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

## How to benchmark

```bash
make bench        # the whole suite once, with allocations
make bench-save   # 5 repeats, raw output under bench_results/ (ignored by git)
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
