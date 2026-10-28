---
name: gram-probe
description: Reproduce what a gram demo does, headless, tick by tick, at tempo x4: a throwaway probe plugin compiled into a copy of the demo (the demo's files never touched) that drives it with key events and logs entities, effects, contacts and boxes; and check a change on the demos that use what it changed — pictures and frame rates of a baseline copy against the working tree. Use when the user reports a demo misbehaving (a jump, a lever, a plate, a turn), before guessing at a cause, to confirm a fix without a window, and before reporting a change done.
---

# gram-probe

`go test` cannot exercise a demo; the GPU window can run headless under `timeout`. A probe is a
throwaway plugin compiled into **a copy of the demo** that plays the keys a user would and logs
what the rules, effects and collision do each tick. Reading the log beats reasoning about ten
systems in the head: three bugs were found so (a stale query offset, a lost key press, a lever's
box). **The demo's own files are never touched**, not even for a moment: the user is changing
them too, and a probe left in by an interrupted run is a broken demo.

## Steps

1. Write the probe in the scratchpad from `probe.go.tmpl`, in `package main`: which keys when
   (`ev.AddKeyEvent(key, control.ActionPress/Release)`; a tap is both; `ev.CursorDelta` turns a
   mouse look), what to log (every line starts `P:`) and when to `os.Exit(0)`.
2. Run it with `probe.sh <demo dir> <probe.go> <real seconds> <scratchpad>`: it copies the demo
   into a module of its own with its own `go.work` (gram and the modules gram's `go.work`
   joins), adds the arena field `probe *probe`, `a.probe = newProbe(a)` after `players` in
   `makePlugins` and `defer a.probe.RunPlan(ctx, d)` first in `update`, sets the clock to x4
   (`Write().Clock().Faster()` twice on the first tick), builds and runs it, and prints the `P:` lines.
   Exit 0 is the probe's `os.Exit`, 124 the timeout, 2 a panic (`panic` in `probe.log`); a
   failed build stops it. A GPU is needed; `GRAM_GPU=software` draws wrong but runs.
3. **Time is the game's**: `p.a.world.Read(0).Clock().Time()`. `update` and the probe run once an engine
   tick, and at x4 the clock replays the simulation four times in it, so ticks count real time.
   End and space the logs by game time (40 s of game take about 10 s at x4); a key script by
   ticks still works, each tick spanning four steps — at tempo 1 (drop the two `Faster`) where a
   key's timing must be exact.
4. Compress the log on the fields that matter (a line only when a state changes, `p.last`).

## Checking a change

1. **The demos that use what changed, and only those**: `grep -rl
   '"github.com/kjkrol/gram/plugins/<pkg>"' examples` (the atmosphere: board/topography,
   board/flat, look/sky). A demo without the package shows nothing of the change: motion/collision
   has no atmosphere. Every demo is the field only for a change to `render` (its device too), the
   engine or the world, which they all go through; there motion/collision is the one without a
   board or a sky.
2. **A baseline**: a copy of the tree from before the change in the scratchpad (`cp -r`, never
   `git stash` or a checkout of the user's tree), run with `PROBE_TREE=<copy>`. Run the copy
   and the working tree alternately, twice: this machine drifts ~10% between runs.
3. **The same moment in both**: `PROBE_SEED` for a demo's random seed, `PROBE_SED` to fix what
   the change depends on in the copied demo (`Start: weather.Cloudy`), the probe freezing the
   light (P) and pressing what the change is about (Tab for the perspective, Q/E, a command put
   through the carrier).
4. **Pictures**: `GRAM_SHOT=<png> GRAM_SHOT_FRAME=<n>` with the probe; compare per pixel (max
   difference, pixels differing). A steady state gives the same picture, ±1; every other
   difference is named in the report. Shoot each view the change touches (from above, in
   perspective, the head raised to the sky).
5. **Frame rate**: `GRAM_FPS_LOG=1 GRAM_VSYNC=off`, 35 s a run; the median FPS, tick, drawing and
   presenting from the per-second lines, after ~8 s of warm-up for a steady state, over the
   seconds it lasts for a passing one (a weather set settles a minute of game time, ~15 s at
   x4). Measure each regime the change has: steady, and the moment it works hardest. A slow
   frame runs up to 5 steps of simulation, so a slow draw drags the tick with it: look for
   the first of the two to go.
6. **Where the time goes**: a probe calling `pprof.StartCPUProfile`/`StopCPUProfile` at two
   ticks, `go tool pprof -top <probe_bin> <prof>`. A plugin's own pass costs in `bench/`
   (`go test ./bench -bench <Name>`), run on the copy and the tree alike.
7. **Slow game time** (a weather crossing the sky in an hour of game) without a window: a
   scratch program over the plugin (`climate.New`, `System`, a goke ECS ticked in a loop)
   simulates hours in a second.

## What a probe can reach

- Entities: a query over `world.Base` with `Optional` columns — `cgoke.Active` (effects:
  `ac.Left(fx.Named(name))`, false when not under it), `collision.Collider` (`Contacts()`:
  `Other`, `Sensed`, `Resting`, `Terrain`), `cell.Floor`/`cell.On` (a trapdoor's `Allows` by its
  cell), `steering.Driven`/`Course`, `world.Z`, `motion.Drift`, `tag.Tags[unit.States]`
  (`unit.Airborne`). Kinds by `p.a.world.Kinds().Lookup(KindName)` → id to match `Base.TypeID`.
- Keys go through `p.a.players.EventHandler().HandleEvents(ev)`, one `InputEvents` a tick, so
  the players' bindings, contexts and cameras are the real ones; `KeyHeld` fires on the ticks
  between press and release. A zero `InputEvents` has no cursor (`CursorInWindow` false): the
  scroll zones, the wheel and the hover fire nothing; a probe moving the cursor sets
  `CursorInWindow: true` with `MousePos`.
- Commands bypassing keys: `p.a.world.Write().Carrier().Put(p.a.player.ID, cmd)` (a `selection.Select`
  with `Camera: p.a.cameras.Read(0).Cameras()[0]`, a `ceremony.Command` from `Commands().Named`),
  `PutFrom(entity, cmd)` for an entity's own.
- Direct state for a setup: move a unit by writing its `Base.Pos.AABB`, set `Vel.Dir`, write
  `steering.Course.Want`; cast an effect with `p.a.world.Write().Effects().CastFor(id, e, d)`; remove a
  `MoveOrder` through a tiny system's `cb.RemoveCompOne`. A demo's walkers may carry orders or
  plans that walk them away: remove or watch for it.
- Cameras: `p.a.cameras.Read(0).Cameras()[0].(camera.Fastenable).Fastening()` says `How` and `Entity`.

## Traps

- `OptComp.At(cur)` after `q.All()` reads the chunk's first row: index the `Slice` by `i`.
- A nil `*cgoke.Active` on an entity never cast on: guard before `Left`.
- A panic inside the probe surfaces as a gogpu render-thread nil dereference; the real cause is
  the probe's line.
- Ticks are per `Update`; a slow frame runs several simulation steps per Update, so a key held
  for N ticks is not exactly N steps.
- Temporary diagnostics in the library (`println` in `effect_commands.go` found the Seek bug)
  are fine: keep a `.keep` copy in the scratchpad and restore it. A demo's file never.
- A query over `world.Base` alone catches the entities standing on the board too (a bridge's
  decks): add `unit.At` to keep to the units.

## Measuring a demo's frames and its garbage collector

`BENCHMARKS.md`, "The demos", was measured this way (etap 0 of `doc/plans-perf.md`); a change to
a frame's cost or to allocations is judged on the same demos the same way, before and after.

1. `run_demo.sh build <demo dir> <probe> <work dir>` is `probe.sh` without the tempo change
   and without running: the copy, the hook, the binary at `<work dir>/bin`. `<probe>` is one
   `.go` file or a directory of them: `probes/generic` logs every second how many entities
   moved their box since the tick before (`P:t<tick> moved <m> of <n>`); `probes/light`,
   `probes/square` and `probes/bullet` add that demo's script (the lamps' shadows off, the
   command through the carrier; the units sent to the bridge and back; the soldier firing);
   `probes/keys` presses the keys `PROBE_KEYS` names from the second second on, one a tick,
   with `Shift+`, `Alt+` and `Ctrl+` (e.g. `PROBE_KEYS="Tab,V"`). A setting on the settings
   screen has no key (2026-10-20): a probe sets it by its command through the carrier
   (`cameras.Picture{Profile, Resolution}`, `topography.Care{Rings}`), the way the picture's
   resolution and the ground's care are measured: `probes/picture` sets board/topography's
   tactical picture at the second second (`PROBE_PICTURE=whole` at `PROBE_RES` of the pixels,
   or `layers`, both still) and presses Tab a tick later with `PROBE_TAB=1`. A new script is
   `zz_probe_script.go` beside a copy of `zz_probe_common.go`: `play()` runs once a tick.
   `PROBE_SED` edits the copied demo's `.go` files (`ScreenWidth  = 1920`), `PROBE_SEED` fixes
   its seed, `GRAM_FULLSCREEN=1` starts full.
2. `run_demo.sh run <work dir> <seconds> <log>` runs it under `timeout` with the environment
   given: `GODEBUG=gctrace=1 GRAM_FPS_LOG=1` for 60 s at 60 FPS (vsync on) for the collector
   and a frame's tick (the simulation and the publishing), barrier, drawing, presenting and
   waiting (for the GPU's next image); `GRAM_FPS_LOG=1 GRAM_VSYNC=off` for 35 s for the
   frame rate; `GOGC=200` the same for the knob. Exit 124 is the timeout, as meant. With vsync
   on a window behind the desktop's active one (a new window opens behind the terminal) is
   paced at 20 FPS by KWin, a frame then holding several steps: the 60 FPS regime runs
   fullscreen, `GRAM_FULLSCREEN=1`. With vsync off a window behind is not paced, and a
   fullscreen one is bound by the GPU, not the screen (2026-10-28).
3. `gcstats.sh <log>` sums the gctrace lines (cycles a second, live and peak heap, the goal,
   the cumulative CPU share); `fpsstats.sh <log> [skip]` the medians of the per-second lines
   after the first `skip` (8) of them; `medians.py <go test -bench output>` the medians of
   `-count=N` runs, the world's log lines between them skipped.
