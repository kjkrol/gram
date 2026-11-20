---
name: gram-probe
description: Reproduce what a gram demo does, headless, tick by tick, at tempo x4, with a throwaway probe compiled into a copy of the demo (its files never touched) that presses keys and logs entities, effects and contacts; and check a change on the demos that use what it changed — pictures, frame rates and the collector of a baseline copy against the working tree. Use when the user reports a demo misbehaving (a jump, a lever, a plate, a turn), before guessing at a cause, to confirm a fix without a window, and before reporting a change done.
---

# gram-probe

A probe is a throwaway plugin compiled into **a copy of the demo** that plays the keys a user
would and logs what the rules, effects and collision do each tick: reading the log beats
reasoning about ten systems in the head. **The demo's own files are never touched**, not even
for a moment: the user is changing them too, and a probe left in by an interrupted run is a
broken demo. Temporary diagnostics in the library are fine, restored from a `.keep` copy in the
scratchpad.

## Scripts

Each script's header says its arguments and variables.

- `probe.go.tmpl`: the probe to copy into the scratchpad: a key script by tick, a query over
  `world.Base` with optional columns, a kind, an effect, contacts, a line logged (`P:`) only
  when it changes, `os.Exit(0)` at the end.
- `probe.sh <demo dir> <probe.go> <real seconds> <work dir>`: the demo copied into a module of
  its own, the probe hooked into `arena` (after `players` in `makePlugins`, `RunPlan` first in
  `update`), the clock at x4, run under `timeout`, the `P:` lines printed. Exit 0 is the probe's,
  124 the timeout, 2 a panic (`probe.log`). `PROBE_TREE` builds another copy of gram,
  `PROBE_SEED` fixes the seed, `PROBE_SED` is a sed expression over the copied `demo.go`. A GPU
  is needed (`GRAM_GPU=software` draws wrong but runs).
- `run_demo.sh build|run`: `probe.sh` without the tempo, built and run apart, for frames;
  `PROBE_TREE` and `PROBE_SEED` as there, `PROBE_SED` over every copied `.go` file.
  `PROBE_DEMOS=<a repository>` takes the demo dir from there, for either script: the relief and
  the flat demo of gram-madeira-square, whose own `.claude/probes/picture` sets the tactical
  picture.
  `probes/`: `generic` (entities moved a second), `keys` (`PROBE_KEYS`), `light`, `square`,
  `bullet`; a new one is
  `zz_probe_script.go` beside a copy of `zz_probe_common.go`.
- `fpsstats.sh`, `gcstats.sh`: medians of the per-second lines, the gctrace lines summed;
  `medians.py`: medians of `-count=N` benchmark runs.

## Time

- Time is the game's (`p.a.world.Read(0).Clock().Time()`): at x4 an engine tick replays four
  steps, so logs are spaced and ended by game time (40 s of game take ~10 s). Where a key's
  timing must be exact, run at tempo 1 (drop the two `Faster`).
- A slow frame runs several steps in one Update: a key held N ticks is not N steps.

## What a probe reaches

- Entities: add `Optional` columns to the template's query (`cell.Floor`/`cell.On`,
  `steering.Driven`/`Course`, `world.Z`, `motion.Drift`, `tag.Tags[unit.States]`). `world.Base`
  alone catches what stands on the board too (a bridge's decks): add `unit.At` for units.
- Keys go through the players' `EventHandler`, so bindings, contexts and cameras are the real
  ones; `KeyHeld` fires on the ticks between press and release. A zero `InputEvents` has no
  cursor: a probe moving it sets `CursorInWindow: true` with `MousePos`.
- Commands: `p.a.world.Write().Carrier().Put(p.a.player.ID, cmd)`, `PutFrom(entity, cmd)` for an
  entity's own. A setting on the settings screen has no key (the user's word, 2026-10-20): a
  probe sets it by its command.
- State: write `Base.Pos.AABB`, `Vel.Dir`, `steering.Course.Want`; cast with
  `Write().Effects().CastFor`; remove a `MoveOrder` through a tiny system's `cb.RemoveCompOne`.
  A demo's walkers may carry orders or plans that walk them away.
- Cameras: `p.a.cameras.Read(0).Cameras()[0]` (a `selection.Select`'s `Camera`), its fastening
  through `camera.Fastenable`. Routes: `navigation.Read(0).Counts()` once a game second.

## Checking a change

1. **Only the demos that import what changed, never any other** (the user's word, 2026-11-20).
   `grep -rl '"github.com/kjkrol/gram/plugins/<pkg>"' examples` lists them, with
   gram-madeira-square's `relief` and `flat` where they import it. A demo that does not import
   the changed package cannot show anything of the change: it is not built, run, shot or
   measured, and no sweep of every demo is run for a plugin's change. Every demo is the field
   only for a change to `render`, the engine or the world. A plan or a brief saying "every
   demo" for a plugin's change is wrong: this rule wins. The tests likewise: the changed
   package's while working.
2. **A baseline**: a copy of the tree from before the change in the scratchpad (`cp -r`, never
   `git stash` or a checkout of the user's tree), run with `PROBE_TREE`, alternately with the
   tree, twice.
3. **The same moment in both**: `PROBE_SEED`, `PROBE_SED` fixing what the change depends on
   (`Start: weather.Cloudy`), the probe freezing the light (P) and pressing what the change is
   about.
4. **Pictures**: `GRAM_SHOT` with the probe, compared per pixel (max difference, pixels
   differing): a steady state gives the same picture ±1, every other difference is named in the
   report. Each view the change touches. A probe placing the camera reads `cam.Zoom()` from the
   60th tick on and puts `cameras.Zoom` and `CenterAt` again till it holds; what moves (a wind,
   a wave) is judged by two frames of one run, a scratch copy's shot saving a second frame, never
   by two runs, whose cameras settle a few pixels apart.
5. **Frame rate**: `GRAM_FPS_LOG=1 GRAM_VSYNC=off`, 35 s a run, after the warm-up or over the
   seconds a passing regime lasts; steady and the moment it works hardest. The warm-up is not a
   fixed 8 s: on a board with far routes the navigation builds its hierarchy in budget on its own
   goroutine (~2.5 ms a tick till it is done, Madeira 100 m ~33 s of game) the first time, and loads
   it from the game's cache after (`navigation.Cache`, 2026-11-10; the relief demo's,
   gram-madeira-square's, in `~/.cache/gram-madeira-square/relief/hierarchy`): a probe of a demo
   keeping one points `XDG_CACHE_HOME` at the scratchpad, runs once to fill it and measures the runs
   after, or reads `navigation.Read.Counts().Builds` (the "Routes" line's `builds`) and starts once
   it stops growing; a comparison of two trees over the same window is fair meanwhile, the absolute
   numbers are not. A slow draw drags the tick with it: look for the first of the two to go. The
   probe stills the adapter whole: the resolution (`Adapt{Still}`, `Picture`), the levers and the
   level shift (`cameras.Detail{Follows: false}`), else the picture moves under the budget as it
   runs. The "Sectors" and "Sectors seen" lines of the world's reporters say the levels a camera
   state gives; a zoom across a threshold re-marks every awake entity at once (a spike to look at).
   Read a run's raw lines before believing its median (2026-11-18): a window is paced by the
   compositor unless it is the active one on a display that takes its rate, the first seconds
   standing at the display's 60.0 and creeping up (so it went with the outer monitor off); full
   screen (`GRAM_FULLSCREEN=1`) is bound by the GPU. Runs back to back heat this machine till
   its package throttles, the ticks doubling and the rates scattering: each run waits for
   `x86_pkg_temp` to fall under some 60 °C, with the throttle count
   (`thermal_throttle/package_throttle_count`) beside its medians. Nothing compiles or tests
   while a run measures. "presenting" trades with "drawing", so the frame rate is the measure,
   neither alone. A source the probe adds that asks the frame for a pass changes the frame it
   measures: a diagnostic one is registered only when its variable is set. An adapter is
   watched unpaced too (2026-11-18): under vsync a window behind the active one stands at
   20 FPS and everything following the budget goes to its floor, on any tree; the probe leaves
   the following on, sets the budget between the rates of the two states in question
   (`cameras.Budget`) and logs what was turned twice a second for a few minutes, a try burnt
   by the adapter coming back every half minute.
6. **Where the time goes**: `pprof.StartCPUProfile`/`StopCPUProfile` at two ticks of a probe;
   a plugin's own pass in `bench/`, on the copy and the tree alike. Hours of game time (a
   weather crossing the sky): a scratch program ticking the plugin's goke ECS in a loop.

## Measuring a demo's frames and its garbage collector

`BENCHMARKS.md`, "The demos", was measured so; a change to a frame's cost or its allocations is
judged on the same demos the same way, before and after (`run_demo.sh build`, then `run`):
`GODEBUG=gctrace=1 GRAM_FPS_LOG=1` 60 s with vsync, fullscreen (KWin paces a window behind the
active one at 20 FPS), `GOGC=200` the same for the knob; the frame rate as above. Without vsync
a window behind is not paced, and a fullscreen one is bound by the GPU.

## Traps

- `OptComp.At(cur)` after `q.All()` reads the chunk's first row: index the `Slice` by `i`, as
  the template does, and guard a nil `*cgoke.Active`.
- A panic in the probe surfaces as a gogpu render-thread nil dereference: the cause is the
  probe's line.
- A demo's window opened while the user works takes their keys and clicks: a shot whose view
  is not the probe's is taken again.
- A probe that saves and loads (F5/F9, `eng.Save`/`Load`) must not register its system with
  `ctx.RegSys` in Install (goke: "Load called after other components were already registered"):
  register it in `ctx.Setup`, as board/forest-army's probe does (2026-11-04).
- **A crowd's outcome is not reproducible by a seed**: the steps an Update replays follow real
  time, so a loaded machine changes what a crowd does. Judge a crowd change by several runs on a
  quiet machine, base and tree alike.
- In zsh a variable does not split into words (`set -- $v` passes nothing): a probe's
  parameters go on each command line written out, or through `${=v}`.
- Within a tick navigation takes a `MoveTo` before selection takes a `Select`: put the `MoveTo`
  a tick later.
