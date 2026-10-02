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

`Benchmark_World_Tick` is one tick of the world plugin alone: the plans, steering and
velocity folded into each entity's speed, every box moved under the edge rules, and the space
rebuilt from every entity. `Benchmark_World_PositionScan` is the floor under it: reading every
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

The board's renderer composing the whole of a 96×64 board of hills 4 cells a side and 20 high, from
above, under a sun low in the west (0.3 up): warm, the shadows kept from the frame before, and after
the sun has moved, every shadow worked out anew. Measured on 2026-09-26, three runs, after light
took colour (each corner a red, green and blue light, the sun worked out once a frame as a
`world.Lamp`).

| Shadows | Frame composed |
|:--|--:|
| kept | 2.04 ms |
| worked out anew | 4.14 ms |

Before light had colour: 1.59 and 3.78 ms. Coloured, with the sun worked out again at every corner
as at first, it was 2.35 ms kept; the lamp and taking a corner's light as it is when a piece is not
split brought most of that back.

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
the sheet's white texel — is 0.35 ms for 18 thousand pieces and 1.7 ms for 72 thousand; the whole
island from above is 15.7 thousand pieces (63 thousand vertices), the perspective through 1080p
8.7 thousand, the isometric view through 1080p 1.5 thousand. The rest of a frame's cost past
composing is Ebitengine's: it converts every vertex it is handed per call, so a frame is handed
in runs, never whole.

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
* **The world tick is the space rebuild plus a walk.** Moving 5,000 entities and handing the space
  every `Base` costs under 200 µs; the query walk under it is 12 µs.
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
