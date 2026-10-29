# ⏱️ gram Benchmarks

[← Back to README](./README.md)

> What each benchmark measures, the numbers at v0.6.0, and what they say about a tick and a frame.

## Environment

- **CPU:** Intel(R) Core(TM) i5-8265U CPU @ 1.60GHz (4 cores, 8 threads)
- **GPU:** Intel UHD Graphics 620 over Vulkan; the screen 1920×1080 (the demos)
- **Go:** 1.27.1 with `GOEXPERIMENT=simd` (the Makefile's and CI's build), Linux

Every number below is the **median of 5 runs** (`make bench-save`: `-count=5 -benchmem`) at
`c1f2d419` (v0.6.0), 2026-10-27, one benchmark after another on an idle machine (the load 0.4 as
it began, 1.3 as it ended, the desktop up); the navigation's, the routing's and the standing
sections were measured anew the same way at `feat/pace` (astar v1.2.0) on 2026-10-28, and say
so. Within a set the runs lie within 1–5% of the median
for most benchmarks; a scene whose first ticks vary or a frame the GPU schedules spreads 10–25%,
said where it does. A long run warms the machine and its later benchmarks come out slower:
`Vision_Relief`, `Text_Panel` and `Frame_UI`, late in the suite's order, had come out 25–150%
higher than ever before, so they were measured again apart, 5 runs each on a cool machine, and
are given so (the ticks measured again beside them agreed with the suite within 5%, the island's
frames were 6–7% quicker). This machine drifts by up to ~10% between runs, so a change is judged by
running the old and the new tree **alternately**, never one run each
([How to benchmark](#how-to-benchmark)). The numbers of the stages of `doc/plans-perf.md` as
each measured them, with their methods and dates, are kept in
[`doc/benchmarks-history.md`](doc/benchmarks-history.md).

All benchmarks live in [`bench/`](bench/) and build their Stage the way a game does — plugins
installed through a headless `game.Initializer`, entities spawned from kinds — then tick the ECS
directly, with no window; the frames draw on a headless GPU device. "0 allocs/op" means a steady
tick allocates nothing; a small B/op on the larger scenes is the amortised growth of buffers kept
between ticks. A scene that evolves over its ticks (the navigation's crowd, the expanse) measures
a mean over the ticks it ran, so only runs of one `-benchtime` compare (the default's here). A few
benchmarks live beside what they measure (`plugins/vision`, `plugins/navigation`, the painter, the
composer, the grids); their sections say where.

## World tick — `Benchmark_World_Tick`, `Benchmark_World_PositionScan`

`Benchmark_World_Tick` is one tick of the world and motion plugins: the velocities folded into
each entity's step, every box moved under the edge rules (a chunk's boxes in one call, aabbworld's
`TranslateRun`), the space reading every `Base` where goke keeps it. `Benchmark_World_PositionScan`
is the floor under it: every entity's `Base` read through a goke query, chunk by chunk. The boxes
are 20×20 on a lattice over a 4000×4000 torus.

| Scene | Tick | Position scan |
|:---|---:|---:|
| 1,000 boxes | 8.7 µs | 2.4 µs |
| 5,000 boxes | 35.6 µs | 11.9 µs |

0 allocs/op. The tick is the moving itself: of the 5,000 (the profile below) aabbworld's
`TranslateRun` 37%, `clampStep` 13%, `inside` 11%, `MaxStep` 9%; the walk of the chunks under 10%.

### Drawing, publishing and both — `Benchmark_World_Draw`, `_Publish`, `_Frame`

The entities are published once a frame, after its ticks, and drawn from what was published:
`Benchmark_World_Publish` is the publishing alone (the entity renderer's record of every entity in
some camera's view: the box, heading, speed and `Z`), `Benchmark_World_Draw` composes the frame
from the records (the look's sprite laid for every box in view), `Benchmark_World_Frame` both.
Worlds of 5,000 boxes 20×20 on a 4000×4000 torus (and of 50,000) spaced on a grid, drawn through a
camera viewing all of it, a quarter, a twentieth or a hundredth, through two cameras a quarter
each, or a twentieth and a hundredth of 50,000; the cases are named by the boxes in view.

| In view | Publishing | Drawing | Both |
|:---|---:|---:|---:|
| 5,000 of 5,000 | 57.6 µs | 431 µs | 480 µs |
| 1,296 of 5,000 | 22.2 µs | 112 µs | 135 µs |
| 256 of 5,000 | 10.8 µs | 22.2 µs | 33.3 µs |
| 49 of 5,000 | 8.4 µs | 4.6 µs | 13.0 µs |
| 1,296 and 1,190 of 5,000, two cameras | 39.2 µs | 220 µs | 262 µs |
| 2,809 of 50,000 | 110 µs | 242 µs | 353 µs |
| 529 of 50,000 | 82.2 µs | 45.5 µs | 128 µs |

0 allocs/op. The publishing walks the world and writes a record an entity in view: under 2 ns an
entity walked and some 10 ns a record. The drawing costs the boxes in view, about 85 ns a box,
three fifths of it the look laying the sprite (`Flat.Sprite`, `VisitWrapImages` for a torus's
copies), the rest the frame's placing (`Frame.At`, `Lifted`) and the atlas's lookups.

## Collision tick — `Benchmark_Collision_Tick`, `Benchmark_Collision_Swept`

One tick of world, motion and collision at the collision demo's scales: movement, the pairs
swept, every overlapping pair tested, bounced and pushed apart, with a `CountContacts` trigger
counting. The scene is a 1024×1024 torus covered to a share of its area with square boxes of one
side, each heading somewhere at random, after 120 ticks so the boxes have spread; "covering 20%"
means the boxes' total area is 20% of the world's. `Benchmark_Collision_Swept` is the 10×10 scene
at 20% with swept shots (`collision.Sweep`) crossing it.

| Scene | Entities | Tick | Contacts a tick |
|:---|---:|---:|---:|
| boxes 20×20, covering 20% | 524 | 124 µs | 16 |
| boxes 20×20, covering 40% | 1,048 | 371 µs | 91 |
| boxes 10×10, covering 20% | 2,097 | 523 µs | 122 |
| boxes 10×10, covering 40% | 4,194 | 2.06 ms | 696 |
| boxes 8×8, covering 20% | 3,276 | 890 µs | 233 |
| boxes 5×5, covering 20% | 8,388 | 2.72 ms | 914 |

| Swept shots crossing the 10×10 scene | Tick | Contacts a tick |
|:---|---:|---:|
| none | 532 µs | 122 |
| 1 | 540 µs | 123 |
| 16 | 559 µs | 137 |
| 64 | 614 µs | 182 |

0 allocs/op. The cost follows the population more than the contacts: at one coverage, halving the
box side quadruples the population and roughly quadruples the tick; doubling the coverage at one
size quadruples the contacts and the tick alike. Of the 5×5 tick (the profile): aabbworld's pair
sweep 27% (`visit`, `listItems`, `sort`, `add`) and its solver 34% (`Solve`, `penetration`), gram's
resting pairs (`rest`) 7%, its contacts and resolving 9%.

## Bands — `Benchmark_Sectors_Expanse`

One tick of world, motion and collision at the expanse demo's scale: a 16384×16384 torus, a
hundred thousand boxes 5 a side spread evenly, the world in sectors with the first band 750 round
the middle, the second 2000 at a step in four, beyond it nothing; then the same with the bands
switched off, every box moving and colliding.

| Bands | Entities | Tick |
|:---|---:|---:|
| on | 100,000 | 1.86 ms |
| off | 100,000 | 13.6 ms |

With the bands on about 1,600 boxes go every step and 4,400 a step in four; the rest lie out of
the space at no cost but the supervisor's pass over the bands near. Of the banded tick (the
profile): the collision's walk over the boxes at rest (`still`) 32%, its marking 41% cumulative,
the pair sweep 26%, the sectors' update 8%.

## Sight — `Benchmark_Vision_Scan`, `Benchmark_Vision_ScanWithOutlines`

One tick of the vision plugin alone over a 4000×4000 world: every observer, on a 120-unit lattice
with a 60° cone of radius 200 facing right, scans the shared space and fills its `Seen`; with
outlines every observer also carries a `SightOutline`, so its view's shape is computed for
drawing. The scan is shared out on the game's pool, every CPU; `serial` is one goroutine. The
benchmarks scan every observer (`WithScanAll()`); a game scans only the observers something reads
(`vision.Watched`), so a scan's share in a game is smaller than its benchmark says.

| Observers | Scan | Scan, serial | With outlines | With outlines, serial |
|---:|---:|---:|---:|---:|
| 100 | 70.1 µs | 89.3 µs | 125 µs | 246 µs |
| 500 | 232 µs | 450 µs | 535 µs | 1.24 ms |

0 allocs/op. On one goroutine about 0.9 µs an observer to know what it sees, 2.5 µs to know the
shape of its view too, both linear in the observers; on eight threads a half. Of the scan (the
profile, 500 observers): aabbworld's walk of the rays 52% — the slabs laid 29%, the cone query
15% — the depths gathered 32% cumulative, and `math.Sincos` 18%, the rays aimed.

### Over a relief — `Benchmark_Vision_Relief`

37 observers on a 192×96 board of rolling hills in relief, a cell 333 m (32 units), each looking
30 cells ahead in a cone of 72° from 9 units up, scanned every step or 15 times a second
(`vision.Plugin.WithFrequency`). The ground reaches the scan three ways: sampled every 50 m or
every cell through a callback a point (`step`), read off the relief's corners inside the scan
(`profile`: the lattice handed to aabbworld's `Cone.Heights`, four rays at a time in the SIMD
build), or handed over ray by ray through the relief's profile (`along`, the path before the
lattice, kept to compare).

| Ground | A tick |
|:--|---:|
| sampled every 50 m | 6.88 ms |
| sampled every cell | 1.39 ms |
| profile | 1.13 ms |
| profile, 15 a second | 441 µs |
| along | 1.71 ms |

0 allocs/op. The profile reads two or three points a cell where a cell's step reads one, and is
exact where a step of a cell misses a crest between its samples. Of the sampled scan (the profile
below): the relief's `GroundAt` 61% under the scanner's `sunkGround`; of the lattice's: aabbworld's
walk (`profileBlock.walkWide` 25%, `lattice.walk` 14%) and the board's cover walked beside it
(`Field.walkSquare` 23%, `covers` 6%). In board/topography nothing reads the walkers' sight, so
they are not scanned at all.

### The views' copy — `Benchmark_Views_Copy` (`plugins/vision`)

One frame's copy of the ground and the cover for the views of the observers (the sight overlay
drawn for a selected unit), published and drawn, over a 192×96 board of 32-px cells
(`go test ./plugins/vision -bench Views_Copy`): warm, a frame in which nothing changed, and with
a cell changed every frame.

| Ground | Warm | A cell changed |
|:---|---:|---:|
| flat | 27 ns | 517 µs |
| relief | 33 ns | 1.07 ms |

The copy is kept until a cell or the relief changes (the `Field`'s `Version` and `Changes`); a
change costs the whole copy, 3 allocations.

## Ground — `Benchmark_Board_GroundAt`

The ground under 64 points a quarter cell apart along a diagonal of a 256×256 board of hills, as
sight samples it along one ray: 2.2 µs the 64 reads, 34 ns a read, 0 allocs/op.

## Standing — `Benchmark_Board_Altitude`, `Benchmark_Gravity_Standing`

A tick of a 64×64 board of hills with a deck over one row, with walkers standing still at their
heights: the topography's altitude system standing them on the surface under them (altitude), or
gravity standing them as bodies (`gravity.Body`), the board's standing pass and the world's around
them; the `Bands` variants make the deck a railed bridge, a band half a cell wide.

| Walkers | altitude | altitude, over a bridge | under gravity | gravity, over a bridge |
|--:|---:|---:|---:|---:|
| 100 | 43.4 µs | 45.6 µs | 45.6 µs | 47.6 µs |
| 1,000 | 412 µs | 419 µs | 415 µs | 418 µs |

Measured anew at `feat/pace` (2026-10-28, the median of 6). 0 allocs/op (2 B/op the buffers'
growth). A band costs nothing a step: the decks over a point are read only in a cell a band lies
over or reaches into. Gravity's own pass costs what the altitude system's does — one surface read
a body — and a body standing still skips the rest. Since v0.6.0 a walker's share of its top
speed is the game's mobility over the rise to the cell ahead (`cell.Mobility`, the default
hiking function's `exp`; `Map.Rise` between the two cells' middles) where it was a slope read at
a point under a factor of 1: against v0.6.0 alternately (the lower of 6 at the default
benchtime) the four scenes cost +9…+10% at 1,000 walkers (altitude 367 → 404 µs), within
noise at 100 — the strategy's price, 40 ns a walker a step. At `fix/stall-pace` (v0.7.1,
2026-10-28) the pass reads `unit.At.Next` to price the slope of the step the unit is led into
(`At` 8 → 16 B): against v0.7.0 alternately (the lower of 6 at `BENCHTIME=200x`, plain and
`-cpu=1` alike) +1.5…+2.3% at 1,000 walkers standing, within noise at 100.

## Terrain — `Benchmark_Board_Terrain`

One tick of world, collision, board and vision over an 80×80 board of 16-unit cells, a quarter of
it rough — forest veiling sight by 0.6 and solid, opaque rock, cell by cell in turn — laid out as
one square block or scattered at random; 200 walkers bounce about, each looking ahead with a 60°
cone of radius 200, every one scanned (`WithScanAll()`). With felling, one forest cell a tick is
cut down, as a woodcutter would.

| Rough ground | Tick |
|:--|---:|
| one block | 3.74 ms |
| scattered | 3.73 ms |
| scattered, one cell felled a tick | 3.74 ms |

0 allocs/op. Almost all of the tick is the sight, and a scattered layout costs what a compact one
does: collision and sight read the cells in place, and cutting a tree costs a write.

## Behaviour — `Benchmark_Plans_Tick`, `Benchmark_Rules_Moving`, `Benchmark_Effects_Tick`

One tick of the world with its units behaving, through ceremony and its goke binding. The units
stand still on a lattice; a sixteenth of them are told a pulse (a fact) every tick and carry it
eight, so plans begin and end as facts come and go. `Benchmark_Plans_Tick`: every unit on a plan
of two loops, one idling until a pulse comes afresh, one begun by the pulse when a word of it
holds, putting an effect on and waiting a step. `Benchmark_Rules_Moving`: every unit playing a
role of two rules of `motion.Moving`, one holding an effect while a word of the moment holds, one
putting another on by chance while under it. `Benchmark_Effects_Tick`: every unit under an
effect lasting three steps with a `Grant` and an `Alter`, cast again by a plan as it ends, so a
third of them begin or end a step.

| Scene | Tick | Allocations |
|:---|---:|---:|
| Plans, 1,000 units | 200 µs (±25%) | 480 |
| Plans, 5,000 units | 877 µs | 2,048 |
| Rules, 1,000 units | 245 µs | 0 |
| Rules, 5,000 units | 1.15 ms | 0 |
| Effects, 1,000 units | 346 µs | 500 |
| Effects, 5,000 units | 2.02 ms | 2,500 |

The allocations are the pulse driver's: adding and removing a component moves the entity between
archetypes (see Markers). A rule's tick allocates nothing of its own; the engine's reads go
through the arrays goke's cursor hands out per chunk, and a fact written is staged once in goke's
command buffer. Of the effects' tick (the profile): the effect system's step 54% cumulative (its
`begin` 11%, `end` 6%), the plans' tick 22%, map lookups 12%.

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
archetype, visiting the marked alone, or by the bit, visiting every entity of the family.

| Marked | one | batch | flag |
|---:|---:|---:|---:|
| 1% (100) | 43.3 µs | 54.7 µs | 19.5 µs |
| 10% (1,000) | 279 µs | 339 µs | 21.9 µs |
| 100% (10,000) | 4.48 ms | 2.15 ms | 43.9 µs |
| none (`Tick`) | 13.8 µs | | |

| Finding the marked | by the archetype | by the bit |
|---:|---:|---:|
| 1% | 39 ns | 7.1 µs |
| 10% | 181 ns | 7.8 µs |
| 100% | 1.6 µs | 14.7 µs |

A marker put on and taken off costs about 110–270 ns an entity; a bit, a nanosecond or two once
the walk over the family (the two ticks' 14 µs) is paid. Finding the marked by the bit costs
about a nanosecond for every entity of the family, where the archetype visits the marked alone. So
a state that comes and goes often — more than about 50 entities a tick in 10,000, or read by a
pass that walks those entities anyway — is cheaper as a bit of a family carried for good; a state
that lasts, on few entities, which a pass wants alone, is cheaper as a component of its own. That
is the rule of gram's markers (`comp.Marks`, described in `entity/tag`): effects' `Idle`,
navigation's `Entered`, collision's hit.

## Navigation — `Benchmark_Navigation_Tick`, `Benchmark_Navigation_Path`

One tick of the world, motion, board and navigation with N walkers under orders on a 64×64 board
of 16-px cells. The walkers stand on the left half; "open" sends each to the cell mirroring its
own on the right half, nothing in the way but each other; "crowded" sends them all through a
wall's one-cell gap to one cell behind it, where they press, step aside and plan again. Every 300
ticks the orders turn them round, so the tick measured holds the orders' routing amortised; the
crowd replans every tick it is blocked.

| Scene | Walkers | Tick | Allocations |
|:---|---:|---:|---:|
| open | 100 | 357 µs | 1 |
| open | 1,000 | 2.81 ms | 15 / 20 KB |
| crowded | 100 | 503 µs | 1 |
| crowded | 1,000 | 7.66 ms | 2 / 28 KB |
| grouped | 1,000 | 9.7 ms (`-cpu=1`) | 2 |
| column | 1,000 | 2.8 ms (`-cpu=1`) | 0 |

`column` (v0.10.0) is the grouped scene sent by one `March` in a closed column (`ColumnOf(1,
true, 1)`): the formation laid out every tick off the party's field, every walker paced behind
the one ahead, the head waiting on the widest gap. At `feat/column` against v0.9.0 alternately
(the lower of 6 at `BENCHTIME=200x`, plain and `-cpu=1`; the base with its own bench file, four
rungs): the column's tick 2.84 ms plain and 2.77 ms at `-cpu=1`, 0 allocations an op, against
the same tree's `grouped` 4.78 and 8.17 ms: the crowd paced behind its head presses the wall's
gap less than a free crowd, so it searches round its learned cells less (not profiled); the
older rungs within noise (−1.0…+3.4% min, the +3.4% of open 1,000 plain reading −1.0% at
`-cpu=1`; the lay-out costs a formation member a seek, a key, a level, a few field lookups and
one sort a formation, and an order without a formation one byte compare). `Routing_Group`
unchanged (+0.0% and −1.1% min, `-cpu=1`, the lower of 6 at `-benchtime=10x`).

`grouped` (v0.9.0) is the crowded scene with every walker selected and sent by one MoveTo: a
party, so its routes come off one field; on v0.8.0 the same bench is a search a route. Against
v0.8.0 alternately at `feat/field` (`-cpu=1`, the lower of 6 at `BENCHTIME=200x`, the base with
the branch's bench files): `grouped` +3.9% (the fields' bookkeeping and the walks' last legs,
where the jam itself searches round the cells its units learned by striking as before: over 200
ticks 3,615 such searches against the base's 3,613, 15 walks), the four older rungs within
noise (+0.2…+1.7% min; a party counted every Update by a map over the living orders had cost
them +2% before `MoveOrder.Party`), `Navigation_Path` within noise.

Measured anew at `feat/pace` (2026-10-28, astar v1.2.0, the median of 5). Against v0.6.0 with
astar v1.1.3, alternately at `BENCHTIME=200x` (the lower of three rounds): crowded 1,000 8.83 →
7.93 ms (−10%), crowded 100 −11%, open 100 −8%, open 1,000 within noise — astar's states in
arrays by an integer id and the planner's share of a cell on the level kept across searches
(`flatStep`, by the board's version), where the open tick of a thousand is the traffic along the
routes. Of the crowded tick (the profile at v0.6.0): the routing's A* three quarters (astar's
solver, the finder's `leveledFrom` 53%, its `price` 13%) and the board's cell lookups under it
(`dxdy` 9%, `NeighborCost` 7%, `Distance` 7%); in the open the A* two fifths, the traffic moving
the units along their routes the rest. At `fix/stall-pace` (v0.7.1, 2026-10-28) the traffic
reads an optional `steering.Pace` column for the keepings' lawful headway and writes
`unit.At.Next` a tick: against v0.7.0 alternately (the lower of 6 at `BENCHTIME=200x`, plain and
`-cpu=1`) every rung within ±1.5%, `Benchmark_Navigation_Path` too. At `feat/trail` (v0.8.0,
2026-10-28) the traffic gathers two more columns a body, copies the order's `Way`, bears every
touch and renews the trails: against v0.7.1 alternately (the lower of 6 at `BENCHTIME=200x`,
`-cpu=1` and plain) every rung within ±2% (open 1,000 +0.7…+1.9% min); the bench's walkers
trail nobody (each its own MoveTo), so the trailing's cost is the plateau probe's to show. On
the way there a `Way` worked out a body a tick (CellCenter, unwrap, hypot) cost open 1,000 +3%
and was replaced by the order's, written as the traffic drives; `InTheWay` asking `Ahead` of
strangers cost the crowded tick +28% through the jam's dynamics (strangers no longer veering),
not code, and was kept to groupmates.

`Benchmark_Navigation_Path` is one route across a 200×200 board of rocks and walls planned a
tick: the walker sent to the far corner and the next tick to the other. The routing keeps its
answers, so after the first two ticks every route is a kept answer: 564 µs a tick, 4 allocations.
The A* itself is `Benchmark_Routing_Path`.

### The routing alone — `Benchmark_Routing_Path`, `Benchmark_Routing_Routes` (`plugins/navigation`)

The same board laid through the board and navigation plugins: every iteration each unit asks for
its route anew, nothing kept between two, and only the routing's work is timed
(`go test ./plugins/navigation -bench Routing`). `Routing_Path` is the A* of one route across the
board, a unit in the north-west corner to the far corner and then to the south-west one;
`Routing_Routes` is 32 routes at once, units on the west edge each to the east edge, answered by
the plugin's routers on one goroutine or on the pool of every CPU, 4 routers; `Routing_Group`
(v0.9.0) the same 32 units sent to one goal by one MoveTo: a party, so one field of the way to
the goal filled by the first router and 32 routes walked off it (on `main` before v0.9.0 the
same bench is 32 searches).

| Benchmark | Routes | A tick | Allocations |
|:---|---:|---:|---:|
| `Routing_Path` | 1 | 13.2 ms | 0 |
| `Routing_Routes/workers=1` | 32 | 189 ms | 10 |
| `Routing_Routes/workers=all` | 32 | 64.7 ms | 0 |
| `Routing_Group/workers=1` | 32, one goal | 57.7 ms | 6 |
| `Routing_Group/workers=all` | 32, one goal | 57.1 ms | 6 |

`Routing_Group` at `feat/field` (2026-10-28, `-cpu=1`, the lower of 6 at `-benchtime=10x`)
against v0.8.0 alternately: 347 → 57.7 ms on one goroutine (−83%) and 356 → 57.1 on the pool
(−84%; the fill is one router's work, the walks are microseconds, so the pool gains nothing
here where it gave 32 searches 3×; 6 allocations an op as `Routing_Routes`, a field filled
again taking the place the last one held); `Routing_Path` +0.1% and `Routing_Routes` +1…+2.5% (min;
within noise, the medians of that run drifted up to +13% under a rising load). Under the
field: `BenchmarkPathFinder_Field/{map,relief}`, one fill of the 128×128 board from its far
corner (every cell settled), 19.6 ms over the seed and 31.2 ms over the rolling hills (the
medians of 6), about three times one `PathFinder_Terrain` search of the same board (6.5 and
11.5 ms), as a Dijkstra with no heuristic over the whole board should; `PathFinder_Terrain`
itself within noise after the `step` factoring (−2.3…+1.8%).

Measured anew at `feat/pace` (2026-10-28, astar v1.2.0, the median of 5); against v0.6.0 with
astar v1.1.3, alternately (the lower of three rounds), `Routing_Path` −4%, `Routing_Routes` −17%
on one goroutine and −14% on the pool. On the pool the 4 routers plan 32 routes 2.9 times as fast
as one goroutine. Under the A*: `BenchmarkPathFinder_Terrain` (`plugins/navigation/internal/
pathfind`), one route corner to corner over a 128×128 board with a quarter of its cells walls,
4.6–4.9 ms reading the cells with 0 allocations (4.8–5.0 ms and 4,392 allocations through a
map; −20…−22% against v0.6.0, `-cpu=1` the same); its `relief` rung, the same board over rolling
hills with the default hiking function asked for every step (the heights by ordinal, as a relief
keeps them; 2026-10-28, new), about 9 ms (two sets of 5: medians 9.03 and 9.37 ms; 5,080
allocations, the map's reads): the strategy's price a step, nearly as much again as the search
itself. `BenchmarkGrids_Neighbours`
(`plugins/board/internal/grids`), every cell's neighbours on a 200×200 grid, the walk over the
directions (`Toward`) 1.64 ms on a square grid, 1.66 ms on a torus and 1.28 ms on a hex grid with
0 allocations, against a list 2.33, 2.41 and 2.17 ms with 40,000.

## Shots — `Benchmark_Bullet_Flight`

One tick of the bullet, world, motion and collision plugins on a 1024-px square with shots fired
every tick from the left edge, each a second in the air (400 units at 400 a second), straight or
thrown and coming down at its range, where it is despawned; "in flight" is how many are in the
air at the end.

| Shots a tick | In flight | Tick, straight | Tick, thrown | Allocations |
|---:|---:|---:|---:|---:|
| 1 | 61 | 16.6 µs (±26%) | 15.7 µs | 2 |
| 5 | 305 | 54.0 µs | 54.4 µs | 6 |
| 20 | 1,220 | 277 µs | 279 µs | 21 |

About 0.2 µs a shot a tick, thrown as cheap as straight; the allocations grow with the spawns a
tick, not with the shots in the air.

## Climate — `Benchmark_Climate_Step`

One step of the climate over a world as big as Madeira's (6528×3392) with heights, its sky's air
masses out to the horizon: carried by the wind; patchy, the horizon's lanes each their own;
settling over the world as a weather set now is forced; settling all over without wind (still).

| Regime | Step | Allocations |
|:---|---:|---:|
| wind | 5.1 µs | 4 / 1.0 KB |
| patchy | 5.3 µs | 4 / 1.0 KB |
| forced | 77.5 µs (±14%) | 5 / 1.2 KB |
| still | 121 µs | 6 / 1.1 KB |

## Text and a frame with UI — `Benchmark_Text_Panel`, `Benchmark_Frame_UI`

Frames drawn into a 1280×720 screen on a headless GPU (`gputest.Need`; skipped without one,
`GRAM_GPU=software` for the rasteriser), submitted and not waited for: the CPU's side of a frame.
`Benchmark_Text_Panel` is a panel of 20 lines of text changing every frame, as a telemetry
panel's; `Benchmark_Frame_UI` the world of 5,000 boxes shown through a `ui.Image` of a feed over
the whole screen, the same panel and a button over it.

| Frame | Time | Allocations |
|:---|---:|---:|
| the text panel | 244 µs (±11%) | 371 / 16 KB |
| the world, the panel and a button | 477 µs | 567 / 25 KB |

Of the panel (the profile): the glyph pieces (`DrawPieces`, the device's `gather`) 37%, the calls
into Vulkan (`runtime.cgocall` under goffi) 29%, the font's glyph index 6%; of the frame with UI
the entity renderer's compose 28% beside them. The allocations are the `fmt` of the 20 lines and
gogpu's: its Vulkan HAL boxes every `vkCmd*` argument for the FFI call, about 36 a draw.

## The island's frame — `Benchmark_Frame_Topography`

The island (96×64 cells of 32: a sea lying `Under` it, earth, sand and rock blending, and the
brooks, streams and rivers `water.Drain` works out of its heights laid across it as curving ways)
drawn through the composer on a headless GPU into a 2560×1440 screen and read back every frame,
so the GPU is waited for; "readback" alone is the wait's cost, taken off in the last column. In
isometry and perspective; under a distant sun with the ground's shade baked (lit); without the
water's shine (no water layer); at 0.7 of the pixels straight into a screen of that size (its
readback smaller too); cut into sectors of 256 shaded with full care everywhere, within two of
the screen's middle and within one; in layers (the whole frame at 0.5 of the pixels, the central
60% at 0.75, the central 35% at all of them) inset or footed; and through a feed at 0.7 of the
pixels scaled up to the screen; in layers and through the feed the figures at the ground's
resolution or at every pixel (sharp).

| Frame | Time | Less the readback |
|:---|---:|---:|
| readback alone | 9.25 ms | |
| isometric | 16.5 ms | 7.3 ms |
| perspective | 20.6 ms | 11.3 ms |
| isometric, lit | 17.7 ms | 8.4 ms |
| isometric, no water layer | 16.0 ms | 6.7 ms |
| isometric at 0.7 of the pixels (1792×1008) | 8.4 ms | ~3.8 ms |
| isometric, full care everywhere | 16.5 ms | 7.2 ms |
| isometric, full care within 2 sectors | 16.7 ms | 7.4 ms |
| isometric, full care within 1 sector | 16.4 ms | 7.2 ms |
| isometric in layers | 13.9 ms | 4.7 ms |
| perspective in layers, inset | 15.0 ms | 5.7 ms |
| perspective in layers, footed | 14.5 ms | 5.3 ms |
| isometric in layers, the figures sharp | 16.2 ms | 7.0 ms |
| perspective in layers, footed, the figures sharp | 17.2 ms | 7.9 ms |
| isometric at 0.7 through a feed | 12.9 ms | 3.7 ms |
| isometric at 0.7 through a feed, the figures sharp | 15.2 ms | 5.9 ms |
| perspective at 0.7 through a feed | 15.3 ms | 6.1 ms |
| perspective at 0.7 through a feed, the figures sharp | 17.4 ms | 8.2 ms |

What it says: the frame is bound by the pixels — 0.7 of them costs half, the layers (0.52 of the
pixels drawn) a third less than the whole; the figures at every pixel over a coarse ground cost
2–2.3 ms, the pass over the ground at the screen's size; the care within a sector or two and the
baked shade cost nothing measurable here. The CPU is busy a fifth of the frame (the profile), the
readback's copy and the Vulkan calls, and waits the rest.

## The flat board's frame — `Benchmark_Frame_Flat`

A flat board of 96×64 cells 32 wide (grass, sand round lakes of water, roads every 12 cells both
ways, the grid over them) drawn through the board's own renderer on a headless GPU into a
2560×1440 screen and read back every frame, after a first frame that builds the shaders: from
above, from above with a cell changed every frame, and isometric.

| Frame | Time | Allocations |
|:---|---:|---:|
| from above | 13.9 ms | 542 |
| from above, a cell changed every frame | 14.2 ms | 906 / 81 KB |
| isometric | 13.9 ms | 553 |

With the readback (~9.3 ms) taken off a frame is 4.6–4.9 ms; a changed cell writes its block of
the tiles' map and its neighbours', not the board.

### The painter's bake — `Benchmark_Painter_Bake` (`plugins/topography/internal/painter`)

The island painted anew on the GPU and waited for: whole, a Style changed every time, and round a
cell changed every time.

| Bake | Time | Allocations |
|:---|---:|---:|
| whole | 18.1 ms | 3,272 |
| round a cell | 4.5 ms | 4,671 |

### The composer's share — `Benchmark_Composer_Render` (`render/internal/compose`)

The composer's own share of handing a frame the size of the island's to the GPU, the draw
stubbed: each run of pieces copied into the call's buffer and indexed.

| Pieces | Time | Calls |
|---:|---:|---:|
| 18,120 | 353 µs | 241 |
| 72,480 | 1.82 ms | 961 |

0 allocs/op.

## Where the time goes — CPU profiles

`go test -c -o bench.test ./bench`, then `bench.test -test.run xxx -test.bench <name>
-test.benchtime=3s -test.cpuprofile p.out` and `go tool pprof -top bench.test p.out`, one run each
at `c1f2d419`, 2026-10-27; the share is of the samples, "cum" where a function's callees count
with it. A scene that evolves (the crowd, the expanse) measures lower here than at the default
`-benchtime`.

- `World_Tick/entities=5000` (38 µs): `moveSystem.Update` 86% cum — aabbworld's `TranslateRun`
  37% cum (26% flat), `clampStep` 13%, `plane.inside` 11%, `Position.MaxStep` 9%,
  `Velocity.Delta` 3%; goke's chunk walk under 10%.
- `Collision_Tick/rect=5,fill=20%` (2.8 ms): the pair sweep's `visit` 27% cum (16% flat;
  `listItems` 10%, `sort` 5%, `add` 4%), the solver's `Solve` 34% cum (`penetration` 10%,
  `AABB.Penetration` 4%), gram's `rest` 7% cum, `contact` 5%, `resolve` 4%.
- `Sectors_Expanse/banded` (1.5 ms): the collision's `still` 32% flat (the boxes at rest walked),
  its `mark` 41% cum; the pair sweep (`visit`, `listItems`, `sort`, `add`) 26%; the sectors'
  update 8%; motion's `Update` 6%.
- `Vision_Scan/observers=500` (226 µs, on every CPU): aabbworld's `walk` 52% cum — the slabs laid
  29% (`lay`), the cone query 15% (`walkConeWide`, `newConeQuery`), `castFlat` 11%, the depths
  gathered 32% cum; `math.Sincos` 18% flat, the rays aimed.
- `Vision_Relief/step=50m` (6.9 ms): `castElevated` 99% cum, of it the relief's `GroundAt` 61%
  (`index` 9%, `corner` 12%, `drawnAt` 7%) under the scanner's `sunkGround` 73%, `ray.ground` 7%.
- `Vision_Relief/profile,15Hz` (446 µs): aabbworld's `walk` 92% cum — the lattice's
  `profileBlock.walkWide` 25% and `lattice.walk` 14%, `ray.ground` 13%; the board's cover walked
  beside it, `Field.walkSquare` 23% and `covers` 6%, the grid's `Fold` 5%.
- `World_Publish/5000 of 5000` (60 µs): `renderer.publish` 97% cum (67% flat), `progress` 12%,
  the views' `Contains` 5%, the records' slice grown 5%.
- `World_Draw/5000 of 5000` (430 µs): `renderer.Compose` 28% flat; the look's `Flat.Sprite` 55%
  cum (`lay` 39%, `VisitWrapImages` 31%, `Sprites.Rect` 16%, `piece` 6%); `Frame.Lifted` 10% cum,
  `Frame.At` 7%; map lookups 10%.
- `Navigation_Tick/open,1000` (3.1 ms): the traffic's `head` 40% cum, astar's solver 39% cum
  under the routing (`leveledFrom` 29%); the board's `Kind`, `dxdy` and `Deck` a few percent
  each; ceremony's `Instant.Fire` 5%.
- `Navigation_Tick/crowded,1000` (8.8 ms): astar's solver 75% cum, `leveledFrom` 53%, the
  `price` 13%, `groundOf` 8%; the grid's `dxdy` 9%, `NeighborCost` 7%, `Distance` 7%; astar's
  dictionary 7%.
- `Effects_Tick/entities=5000` (2.0 ms): the effect system's `step` 54% cum (`begin` 11%, `end`
  6%), the plans' tick 22% cum, map lookups 12%, `memmove` 5%.
- `Text_Panel` (236 µs): `Font.Draw` 52% cum — `DrawPieces` 37% (`gather` 16%, `appendVertex`
  7%), the glyphs 6%, the font's index 6%; `runtime.cgocall` 29%, Vulkan through goffi.
- `Frame_UI` (441 µs): `renderer.Compose` 28% cum (the look's `Sprite` 14%, `lay` 9%);
  `Font.Draw` 26% cum; `runtime.cgocall` 23%; `DrawPieces` 18%.
- `Frame_Topography/iso` (16.4 ms): the CPU busy 19% of the time — the readback's copy
  (`memmove`, 71% of the samples) and the Vulkan calls (15%) — and waiting the rest.

What they say: the world tick is the moving (aabbworld's `TranslateRun`, the step's clamp), not
the iteration; the collision tick is aabbworld's sweep and solver, gram's resting pairs a tenth;
a world in bands pays most for the boxes at rest being walked; the flat sight is aabbworld's
slabs and cone query, a fifth of it aiming the rays; the sight over a relief is the ground under
the rays — through a callback the relief's reads, off the lattice the walk and the cover beside
it; the entity renderer is the look assembling the sprite instances, three fifths; the navigation
tick is the A* and the board's cell lookups under it; text is half the GPU calls and half the
glyph pieces; the island's frame is the GPU's, the CPU waiting. No profile has the garbage
collector in its first thirty lines.

## The demos — the garbage collector and the frames

Every demo in a copy with a probe (`gram-probe`'s `run_demo.sh`, no tempo change, `PROBE_SEED=7`),
at `c1f2d419` on 2026-10-28, the machine above (Intel UHD 620, Vulkan, KDE on Wayland, the
screen 1920×1080 at 60 Hz): 60 s fullscreen with vsync on, the compositor pacing the frames at
60 FPS, `GODEBUG=gctrace=1 GRAM_FPS_LOG=1`, for the collector and a frame's costs at that pace;
then 35 s in the demo's own window with `GRAM_VSYNC=off` for the frame rate and the costs alone;
the medians of the per-second lines after 8 s of warm-up. navigation/square has its four player
units sent together to the bridge's foot at the tenth and seventieth second and west at the
fortieth; motion/bullet's soldier fires a round every 20 ticks and a grenade every 90 from the
second second; the rest run as they start. "tick" is a frame's simulation: at 60 FPS a 120-TPS
demo (motion/collision) runs two steps in it. "moved" is the share of the entities whose box
changed between two ticks, sampled every second. Fullscreen because a window behind the
desktop's active one, or any window once the screen saver is up, is paced at 20 FPS by the
compositor with vsync on, a frame then holding up to five steps.

| Demo | GC cycles | Live heap, peak | Tick | Drawing | Presenting | Moved |
|:---|---:|---:|---:|---:|---:|---:|
| board/topography | 0.27/s | 75 MB, 269 MB | 1.18 ms | 2.49 ms | 0.66 ms | 6% of 122 |
| look/light | 0.26/s | 23 MB, 87 MB | 0.13 ms | 0.49 ms | 0.24 ms | 50% of 6 |
| navigation/square | 0.16/s | 25 MB, 60 MB | 0.14 ms | 0.29 ms | 0.20 ms | 38% of 8 |
| motion/bullet | 0.15/s | 23 MB, 65 MB | 0.14 ms | 0.26 ms | 0.20 ms | 57% of 7 |
| motion/expanse | 0.19/s | 50 MB, 290 MB | 3.02 ms | 1.30 ms | 1.18 ms | 5.3% of 100,000 |
| motion/collision | 0.21/s | 43 MB, 87 MB | 14.3 ms | 1.16 ms | 0.32 ms | 100% of 10,485 |
| look/animation | 0.19/s | 23 MB, 60 MB | 0.15 ms | 0.36 ms | 0.22 ms | 40% of 5 |
| rules/dialog | 0.16/s | 22 MB, 61 MB | 0.13 ms | 0.29 ms | 0.21 ms | 0% of 3 |

With vsync off, in the demo's own window, the frame rate and a frame's costs (the tick is then
spread over many frames):

| Demo | Window | FPS | Tick | Drawing | Presenting |
|:---|:---|---:|---:|---:|---:|
| board/topography | 1024×768 | 142 | 0.47 ms | 2.01 ms | 0.80 ms |
| look/light | 768×512 | 626 | 0.06 ms | 0.87 ms | 0.33 ms |
| navigation/square | 768×512 | 708 | 0.03 ms | 0.26 ms | 0.18 ms |
| motion/bullet | 768×512 | 1,163 | 0.03 ms | 0.16 ms | 0.15 ms |
| motion/expanse | 1024×768 | 786 | 0.31 ms | 0.56 ms | 0.30 ms |
| motion/collision | 1024×1024 | 173 | 5.14 ms | 1.49 ms | 0.59 ms |
| look/animation | 768×512 | 1,090 | 0.04 ms | 0.25 ms | 0.16 ms |
| rules/dialog | 768×480 | 1,208 | 0.02 ms | 0.17 ms | 0.15 ms |

What they say: the collector runs once every four to seven seconds in every demo and gctrace's
cumulative CPU share stays at 0%; at 60 FPS no demo's frame is bound by it, nor by anything but
the simulation of motion/collision (two steps of 10,485 boxes a frame, some 7 ms a step with the
rules and the light's patches) and the ticks of motion/expanse (3 ms, its bands) and
board/topography (1.2 ms). The moved shares are the number behind any change tracking per chunk:
the board demos move 5% of their entities a tick, the expanse 5% of its hundred thousand (the
bands), the motion demos all of them.

## The island demo's picture

board/topography at 1920×1080 with `GRAM_VSYNC=off`, the tactical picture set through the carrier
at the second second (`probes/picture`: whole at a resolution, or in the preset's layers — the
whole frame at 0.6 of the pixels, the middle 60% at 0.8, the inner 35% at all of them, 0.52 of
the frame drawn — both still, the figures at every pixel) and Tab a tick later for the
perspective; in a window of the screen's size and fullscreen; the medians of 27 s after 8 of
warm-up, 2026-10-28:

| Picture | In a window | Fullscreen |
|:---|---:|---:|
| isometric, all the pixels | 66 FPS | 57 FPS |
| isometric, the preset's layers | 81 FPS | 70 FPS |
| isometric, 0.72 of the pixels | 91 FPS | 79 FPS |
| perspective, all the pixels | 66 FPS | 57 FPS |
| perspective, the preset's layers, footed | 84 FPS | 74 FPS |
| perspective, 0.72 of the pixels | 90 FPS | 80 FPS |
| as it starts: the adapter following a budget of 60 FPS, in layers | 61 FPS | 56 FPS |

What it says: the picture is bound by the pixels — 0.72 of them gives a third more frames, the
layers a quarter more, the perspective costs what the isometry does — and the adapter rests just
under its budget (the band 0.92–0.98 of 60 FPS), the drawing 2.5 ms a frame in layers against
1.5 whole. Fullscreen costs a sixth on this desktop, the compositor's fullscreen path on the same
GPU.

## What they say

- **A tick is the plugins' RunPlan and nothing else.** The engine adds no work of its own per
  entity; what a Stage pays is the sum of the plugins it runs, in the order it runs them.
- **The world tick is the moving.** 5,000 entities moved and read in place by the space in 36 µs,
  the query walk under it 12 µs; it allocates nothing.
- **Collisions cost by population.** At the demo's default scale (8,388 boxes of 5×5) a tick is
  2.7 ms, well inside the 8.3 ms of a 120 TPS step; the crowd that halves the demo's TPS is the
  40% coverage one, where contacts dominate.
- **A view is a set, not a list.** The publishing walks the world once a frame and writes what
  some camera sees; the drawing pays the boxes in view, about 85 ns each, and a view of the whole
  world skips the query. The same shape serves a view per player or per remote client.
- **Sight is the space's.** 500 flat observers scanned in 232 µs on every CPU; 37 over a relief
  at 15 a second in 0.44 ms; a game scans only the observers something reads.
- **The terrain lives in the ECS at no cost.** A ground read is 34 ns; a scattered forest costs
  what a compact one does, and cutting a tree costs a write.
- **Rules allocate nothing; markers move archetypes.** 5,000 units on plans tick in 0.9 ms,
  under effects in 2 ms; what allocates is a component put on and taken off, which a bit of a
  family avoids.
- **Navigation is the A*.** 1,000 walkers in the open 3 ms a tick, through one gap 10.5 ms, the
  routing answering questions asked before from memory and planning on the pool.
- **A frame is the GPU's.** The island at 1440p is 7.3 ms past the readback in isometry, 4.7 ms
  in layers; the CPU's side of a frame with text and UI stays under a millisecond.
- **Zero allocations once warm** in the world, collision, sight, terrain, standing and rules
  ticks; a navigation tick a few; a frame with text a few hundred, gogpu's.

## How to benchmark

```bash
make bench        # the whole suite once, with allocations
make bench-save   # 5 repeats, the raw output under bench_results/ (ignored by git)
python3 .claude/skills/gram-probe/medians.py bench_results/<file>.txt   # the medians
```

A profile beside a benchmark, for where a tick goes rather than how long it takes:

```bash
go test ./bench -run xxx -bench 'Benchmark_World_Tick/entities=5000' -benchtime=3s \
  -cpuprofile p.out -o bench.test && go tool pprof -top -nodecount=30 bench.test p.out
```

The benchmarks link gogpu but open no window; in CI they run under `xvfb-run` like the tests. To
judge a change on this or any drifting machine, keep a copy of the baseline tree in the
scratchpad (never a stash or a checkout of the working tree) and run the two alternately, on an
idle machine, the same build both sides (`GOEXPERIMENT=simd` or none); `measure.sh` and
`compare.py` in `.claude/skills/gram-perf` do it and print the min and the median a benchmark
with the change in percent:

```bash
.claude/skills/gram-perf/measure.sh <out dir> '<bench regex>' <base dir> <tree dir> 3 2
```

A gain is taken when it is at least 3% in the regime it aims at, the two sides' runs do not
overlap and no regime loses; fewer allocations at a time not worse is a gain too. The demos'
frames and their collector are measured as the `gram-probe` skill says ("Measuring a demo's
frames and its garbage collector": `run_demo.sh`, `fpsstats.sh`, `gcstats.sh`).
