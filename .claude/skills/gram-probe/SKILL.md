---
name: gram-probe
description: Reproduce what a gram demo does, headless, tick by tick, at tempo x4: a throwaway probe plugin compiled into a copy of the demo (the demo's files never touched) that drives it with key events and logs entities, effects, contacts and boxes. Use when the user reports a demo misbehaving (a jump, a lever, a plate, a turn), before guessing at a cause, and to confirm a fix without a window.
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
   (`Clock().Faster()` twice on the first tick), builds and runs it, and prints the `P:` lines.
   Exit 0 is the probe's `os.Exit`, 124 the timeout, 2 a panic (`panic` in `probe.log`); a
   failed build stops it. A GPU is needed; `GRAM_GPU=software` draws wrong but runs.
3. **Time is the game's**: `p.a.world.Clock().Time()`. `update` and the probe run once an engine
   tick, and at x4 the clock replays the simulation four times in it, so ticks count real time.
   End and space the logs by game time (40 s of game take about 10 s at x4); a key script by
   ticks still works, each tick spanning four steps — at tempo 1 (drop the two `Faster`) where a
   key's timing must be exact.
4. Compress the log on the fields that matter (a line only when a state changes, `p.last`).

## What a probe can reach

- Entities: a query over `world.Base` with `Optional` columns — `cgoke.Active` (effects:
  `ac.Left(fx.Named(name))`, false when not under it), `collision.Collider` (`Contacts()`:
  `Other`, `Sensed`, `Resting`, `Terrain`), `cell.Floor`/`cell.On` (a trapdoor's `Allows` by its
  cell), `steering.Driven`/`Course`, `world.Z`, `world.Drift`, `tag.Tags[unit.States]`
  (`unit.Airborne`). Kinds by `p.a.world.Kinds().Lookup(KindName)` → id to match `Base.TypeID`.
- Keys go through `p.a.players.EventHandler().HandleEvents(ev)`, one `InputEvents` a tick, so
  the players' bindings, contexts and cameras are the real ones; `KeyHeld` fires on the ticks
  between press and release.
- Commands bypassing keys: `p.a.world.Carrier().Put(p.a.player.ID, cmd)` (a `selection.Select`
  with `Camera: p.a.cameras.Cameras()[0]`, a `ceremony.Command` from `Commands().Named`),
  `PutFrom(entity, cmd)` for an entity's own.
- Direct state for a setup: move a unit by writing its `Base.Pos.AABB`, set `Vel.Dir`, write
  `steering.Course.Want`; cast an effect with `p.a.world.Effects().CastFor(id, e, d)`; remove a
  `MoveOrder` through a tiny system's `cb.RemoveCompOne`. A demo's walkers may carry orders or
  plans that walk them away: remove or watch for it.
- Cameras: `p.a.cameras.Cameras()[0].(camera.Fastenable).Fastening()` says `How` and `Entity`.

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
