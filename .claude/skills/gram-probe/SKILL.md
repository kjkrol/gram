---
name: gram-probe
description: Reproduce what a gram demo does, headless, tick by tick: a temporary probe plugin inside the demo that drives it with key events and logs entities, effects, contacts and boxes. Use when the user reports a demo misbehaving (a jump, a lever, a plate, a turn), before guessing at a cause, and to confirm a fix without a window.
---

# gram-probe

`go test` cannot exercise a demo; the GPU window can run headless under `timeout`. A probe is a
throwaway plugin compiled into the demo (`zz_probe.go`, never committed) that plays the keys a
user would and logs what the rules, effects and collision do each tick. Reading the log beats
reasoning about ten systems in the head: three of this session's bugs were found so (a stale
query offset, a lost key press, a lever's box).

## Steps

1. Copy `probe.go.tmpl` to `examples/<group>/<name>/zz_probe.go`, in `package main`, and fill
   the script: which keys when (`ev.AddKeyEvent(key, control.ActionPress/Release)`; a tap is
   both; `ev.CursorDelta` turns a mouse look), what to log and when to `os.Exit(0)`.
2. Hook it into the demo's `demo.go`, three lines: the arena field `probe *probe`,
   `s.probe = newProbe(s)` as the last plugin made in `makePlugins` (after `players`), and
   `defer s.probe.RunPlan(ctx, d)` first in `update` (it then runs after every plugin's RunPlan).
3. Build to the scratchpad and run from the demo's directory (embedded maps are relative):
   ```
   S=<scratchpad>; go build -o $S/demo ./examples/<group>/<name> && \
     (cd examples/<group>/<name> && timeout 40s $S/demo > $S/probe.log 2>&1; echo "exit $?")
   grep 'probe:' $S/probe.log | sed 's/.*probe: //'
   ```
   Exit 0 is the probe's `os.Exit`, 124 the timeout (alive), 2 a panic: look for `panic` in
   the log. A GPU is needed; `GRAM_GPU=software` draws wrong but runs.
4. Compress the log with awk on the fields that matter (a line only when a state changes), or
   log only on change (`p.last`).
5. Before any commit: delete `zz_probe.go`, remove the three hooks, `gofmt -l`, `go build`.

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
  are fine: keep a `.keep` copy in the scratchpad and restore it.
