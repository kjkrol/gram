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

`Benchmark_World_Tick` is one tick of the world plugin alone: the decision pass, steering and
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
what the camera's `world.View` contains: the world refreshes that View once a tick after movement,
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
a call or several per cone, shadow, route and outline — see doc/rendering.md §6, where whole frames
of the island demos are 25–60% shorter.

## Collision tick — `Benchmark_Collision_Tick`

One tick of world plus collision at the collision demo's scales: movement, the space rebuilt,
every overlapping pair found, tested, bounced and pushed apart, with a `CountContacts` behavior
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
drawing.

| Observers | Scan | Scan with outlines |
|---:|---:|---:|
| 100 | 101 µs | 237 µs |
| 500 | 503 µs | 1.27 ms |

About 1 µs per observer to know what it sees, 2.5 µs to also know the shape of its view; both
scale linearly with the observers.

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
the sun has moved, every shadow worked out anew. Measured on 2026-09-26, three runs.

| Shadows | Frame composed |
|:--|--:|
| kept | 1.59 ms |
| worked out anew | 3.78 ms |

A corner's shadow is a walk towards the sun a quarter cell at a time over the tops of the cells as
the frame read them, stopped as soon as the line to the sun is above the highest top within 16
cells of the view — two or three steps under a high sun. The first version sought every sample's
cell and ground in the ECS and walked the whole 16 cells: 59.5 ms anew.

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
  sequential walk for all (see doc/views.md).
* **Drawing pays for the camera's wrap arithmetic.** ~110 ns per drawn box, mostly `math.Mod` in
  projecting a box onto a torus — a camera optimisation waiting for a reason.
* **The terrain lives in the ECS at no cost.** Reading a cell's ground or kind from its entity is
  cheaper than the raster it replaced, and A* across a 128×128 board costs what it did over the map.
* **Terrain costs what the rays and the units touch.** Collision and sight read the cells in
  place, so a scattered forest costs about what a compact one does, and cutting a tree costs a
  write.
* **Zero allocations once warm.** Every benchmark reports 0 allocs/op after the first ticks have
  grown the buffers.

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
