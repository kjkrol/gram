---
name: gram-perf
description: Where gram's time goes and what has been done about it — the map of the optimisations landed (the tick, the frame and the picture in layers, navigation, collision, sight, float32, SIMD), each with the bottleneck it answered, the invariant it rests on and its headline number; what was tried and dropped; the protocol for measuring a change in bench/ (a baseline copy against the tree, alternately, on an idle machine, fair siblings, the min of runs, -cpu=1, profiles) with its scripts measure.sh and compare.py; the traps. Use when planning or running a stage of doc/plans-perf.md, measuring a change, chasing a regression or a slow tick or frame, or touching a hot path.
---

# gram-perf

The numbers live in `BENCHMARKS.md` (a section a benchmark, the method, the demos' frames,
measured anew at v0.6.0), the stages' stories in `doc/plans-perf.md` (Polish: a plan and a result
a stage) and the numbers as each stage measured them in `doc/benchmarks-history.md` (the
paragraphs `plans-perf.md` names); `CHANGELOG.md` carries a tag's numbers. This skill is the map
over them and the protocol: read it first, then the section the change touches, never the whole
of any.

## Where the time goes

A tick (`bench/`, package `bench_test`: a Stage built as a game builds one, ticked with no window):
- the world's publish and sectors — `World_Tick`, `World_Publish`, `World_PositionScan`,
  `Sectors_Expanse`; the tick at 5,000 entities is 37 µs, once 259;
- motion and collision — `Collision_Tick`, `Collision_Swept` (aabbworld's broad phase and
  solver: its own `BENCHMARKS.md`, "Key takeaways");
- navigation — `Navigation_Tick` (open and crowded, 100 and 1,000), `Navigation_Path`,
  `Routing_Path`, `Routing_Routes`;
- sight — `Vision_Scan`, `Vision_ScanWithOutlines`, `Vision_Relief` (rungs `profile`, `along`,
  `step=cell`, `step=50m`, `15Hz`), `Views_Copy`;
- behaviour — `Plans_Tick`, `Rules_Moving`, `Effects_Tick`, `Marker_*`;
- the board and what stands on it — `Board_GroundAt`, `Board_Altitude`, `Gravity_Standing`,
  `Board_Terrain`, `Bullet_Flight`, `Climate_Step` (the board's composing benchmarks,
  `Board_Island`, `Board_Shadows` and `Board_Shores`, went on 2026-10-27: composing is planning
  only since the light plugin, ~0.1 µs a frame; the island's frame is `Frame_Topography`).

A frame (a headless GPU): `World_Draw`, `World_Frame`, `Frame_Topography` (the island at 1440p:
the field for every change to `render`), `Frame_Flat`, `Frame_UI`, `Text_Panel`; the painter's
whole bake, `Painter_Bake`, in its own package. A headless frame includes ~10 ms of readback at
1440p: frames compare with frames, never with a frame rate. The demos' frames and their
collector: `gram-probe`, "Measuring a demo's frames". CPU profiles of the main benchmarks:
`BENCHMARKS.md`, "Where the time goes".

## What has been done, and what it rests on

Each: what, the bottleneck it answered, the invariant it leaves (break it and the gain goes), the
number, where the detail is.

- **Etap 1** (2026-10-19, `plans-perf.md` "Etap 1"): the tick without `math.Hypot`
  (`clampStep` inlines only without it), `Simulate` and `RolesOf` bound once, the views' copy
  keyed by the ground's and the cover's versions. World tick 5,000: 215 → 169 µs, 1 → 0 allocs;
  `Views_Copy` 925 µs → 16 ns. Invariant: a steady world and collision tick allocate nothing.
- **R, R2, T, U** (2026-10-19/20): the picture's resolution and the ground's care per ring, then
  the rings as layers (`camera.Layer`, `Inset`/`Footed`), the ground at the camera's resolution
  with the figures sharp over it, one composition for every layer (`Lay`/`DrawLayer`/`DrawOver`)
  and a hole under the inner layers (the preset draws 0.52 of the frame's pixels). The island
  at 1440p, iso: 7.3 → 5.9 ms in rings, 8.0 → 7.0 ms with one composition; the demo at
  2560×1440: 45 FPS at every pixel, 70–106 at 0.72/0.52 of them. Invariant: a Direct draws once
  a layer and keeps what Compose gathered (`gram-render`; the sprites, the routes and the views
  each broke it once). The resolution follows a frame budget (the cameras' adapter): a
  measurement sets it (`cameras.Picture{Resolution}`) or turns the layers off.
- **Waves 1–5** (2026-10-20…22, "Fala 1–5"): A* into kept buffers, answers remembered, the
  group's goals without A* (`Navigation_Tick` open 1,000 9.56 → 6.32 → 4.55 ms, crowded 1,000
  306 ms → under 10; `Navigation_Path` 94 → 18 ms, 14,740 → 19 allocs); aabbworld reading goke's
  chunk columns as `Run`s instead of a copy of `Item` (`World_Tick` 1,000/5,000 23.3/110.9 →
  13.9/58.0 µs: the copy was 52% of the profile) and moving a run in one call (−24…26%); the
  index built before vision's batch (`SpaceRead.Ready`: without it `Vision_Scan` 500 +78%); the
  thread pool (`Vision_Scan` 9 → 0 allocs); ceremony's steps without allocations (open 1,000:
  2,599 → 90 a tick); uniforms packed once, blits without a buffer. Invariant: the space reads
  the chunks' columns, rebuilt after a `Sync`; nothing copies `Base`. The acceptance rule since
  wave 3: the median ≥3% better in the regime aimed at, the two sides' runs not overlapping, no
  regime worse; fewer allocations at a time not worse is a gain too (the user, 2026-10-21).
- **The flat board on the GPU** (2026-10-23, "Płaska plansza na GPU"): the tiles' map in the
  board renderer's shader (a 2×2-pixel block a cell), the painter's bake on the GPU (a layer
  19 → 4 ms), the dead CPU parallel path removed. Invariant: the CPU writes the cells' data, the
  shaders work the shapes out.
- **float32** (2026-10-25, aabbworld v1.17.0/v1.18.0, "aabbworld v1.17.0"): every world number
  float32 (`world.Base` 104 → 52 B): `World_Tick` 5,000 43.6 → 37.3 µs, collision 4–14% less,
  the publish 7–12% less, sight 0–5% more (conversions at its edges). Invariant: the float32
  rule in `CLAUDE.md`, tolerances by `f32.Eps`.
- **Sight in SIMD** (2026-10-26, "SIMD w widzeniu", letter `s`): scans only where something
  reads them (`vision.Watched`, `WithScanAll()`: board/topography's 37 relief scans a tick
  gone), the relief's lattice handed to aabbworld's `Cone.Heights` in place of a callback a
  point, aabbworld's cone query, slabs and lockstep walk (its `simd` skill). `Vision_Scan` 100
  observers 79 → 73 µs (serial 129 → 106), 500: 295 → 249; `Vision_Relief/profile` 1.76 → 1.21
  ms, at 15 Hz 774 → 495 µs. Invariant: the same sightings to the bit in both builds; the
  benchmarks scan all. The sampled-ground rungs (`step=cell`, `step=50m`) are 2–6% slower in
  the SIMD build with the scan in parallel and 2–4% faster on one CPU: AVX2 on every
  hyperthread lowers the clock for the scalar callbacks — documented, not a regression to chase
  again.

**Tried and dropped** (not to be proposed again without a new reason): a mirror of goke's chunks
on the GPU with versions per chunk (every chunk in view is dirtied every tick: ~20% slower; only
the one-move instance copy stayed), a merged reach-grown grid layout in aabbworld (+10–30%),
lanes in navigation (units learn by striking), the flat board through the topography with zero
relief, eight float32 rays in the slabs (ties flipped on cell-aligned boxes), the GPU for the
motion's arithmetic (a dispatch costs as much), `Position` off the CPU.

## Measuring a change

1. **An idle machine, one measurement at a time.** Parts and agents time nothing; `measure.sh`
   refuses a load over 1.5. A number taken beside another run is not a number.
2. **A baseline copy.** `cp -r` of the tree at the base commit into the scratchpad (never
   `git stash` or a checkout of the user's tree; a `git worktree` outside `/home/karol/CODE`
   serves too). `/home/karol/CODE/go.work` governs everything under it; a copy elsewhere builds
   with `GOWORK=off`: `go.mod`'s tags from the cache, or a sibling checkout where `go.mod` has a
   `replace` to it.
3. **Fair siblings.** Both sides build goke, ceremony, aabbworld and astar the same way, except
   the sibling the change is in: a `go.work` of the tree's own in the scratchpad lists the tree
   and that checkout alone (`use (/home/karol/CODE/gram /home/karol/CODE/aabbworld)`),
   `GOWORK=<that file>`; the base takes the tag. A side on the cache against a side on a newer
   checkout of an untouched sibling measures the sibling.
4. **The same build both sides**: `GOEXPERIMENT=simd` (the Makefile, CI) or none; a change in
   SIMD code is measured in both. The first run of a side compiles: a throwaway run first.
5. **Alternately, the min.** `measure.sh <out> <regex> <base dir> <tree dir> [rounds] [count]`
   with `BASE_ENV`/`TREE_ENV` runs the two alternately (3 rounds of `-count=2` is the usual six)
   and `compare.py` prints the min and the median a benchmark with the change in percent; the
   demos' frames take medians of the per-second lines (`gram-probe`).
6. **What counts.** This machine drifts ~10% between runs; ±1–2% inside a set is nothing. A gain
   is taken when it is ≥3% in the regime it aims at, the two sides' runs do not overlap and no
   regime loses; fewer allocations at a time not worse is a gain. A loss in one regime is
   explained before the merge: `EXTRA='-cpu=1'` tells work from scheduling, the plain build
   tells the AVX2 clock from the kernels, a CPU profile (`-cpuprofile`, `go tool pprof -top`)
   before and after tells the hot loop; a change to a hot loop without a profile either side is
   not measured.
7. **Regimes.** A benchmark's rungs are regimes (open and crowded, 100 and 1,000, `profile` and
   `step`); a change is measured in every regime it has, steady and at its hardest, never in the
   one it was written for alone.
8. **Then the record.** The numbers into `BENCHMARKS.md` (the section, the method, the date),
   the stage's result into `doc/plans-perf.md`, the tag's numbers into `CHANGELOG.md`; the
   copies, results and scripts of a session stay in the scratchpad.

## Traps

- A `replace` in `go.mod` (`=> ../aabbworld`) takes the checkout even with `GOWORK=off`: know
  what each side builds (`go list -m all | grep aabbworld` on both).
- `GOEXPERIMENT` is the Makefile's, not the shell's: a plain `go test` is the plain build.
- The vision benchmarks scan every observer (`WithScanAll()`), a game only those read: a scan's
  share in a game is smaller than its benchmark says.
- A headless frame's readback; the telemetry panel, a random weather and a random scene in a
  picture: noise, not a difference (`gram-probe`, "Checking a change").
- A parallel regime on a laptop: four cores, eight threads, the clock falling under AVX2 or
  heat; a serial number (`-cpu=1`) is the steadier one to compare work by.
- A long run warms the machine: in the 15-minute suite of 2026-10-27 the benchmarks late in its
  order (`Text_Panel`, `Frame_UI`, `Vision_Relief`) came out 25–150% slower than apart, the
  ticks before them within 5%. The record (`make bench-save`) is checked by measuring the late
  ones again apart; a comparison alternates its sides anyway.

## Pointers

- `BENCHMARKS.md`: "How to benchmark", "Where the time goes", "The demos", "The island's frame";
  `doc/benchmarks-history.md`: the same sections as the stages measured them, to v0.6.0.
- `doc/plans-perf.md`: "Co wiadomo na starcie", "Wspólne dla każdego etapu", a stage's "wynik".
- `gram-probe`: "Checking a change", "Measuring a demo's frames and its garbage collector".
- aabbworld: `BENCHMARKS.md` ("Key takeaways", "Wide pair sweep", "Sight in SIMD") and its `simd`
  skill, `/home/karol/CODE/aabbworld/.claude/skills/simd/SKILL.md` (a session in that repo
  loads it; from here, read it by path).
- `CLAUDE.md`, "A stage in parts": who measures (the integrator alone, after the parts) and when.
