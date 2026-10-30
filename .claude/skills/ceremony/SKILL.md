---
name: ceremony
description: ceremony, the language of behaviour gram builds on (github.com/kjkrol/ceremony, locally /home/karol/CODE/ceremony through go.work) — rules, plans, effects, roles, commands, facts, and its goke binding cgoke. Use when writing or reading rules, plans or effects in gram or a demo, when an effect or a rule misbehaves (fires once, fires too often, lingers, flickers), or when touching ceremony itself.
---

# ceremony

The language: `/home/karol/CODE/ceremony/doc/ceremony.md` (clauses, how an effect runs, idioms,
places, roles, knobs) and ceremony's `CLAUDE.md` (layout, what is refused at build, the store's
zero cost). gram's side, what its plugins perceive and carry out and where a game defines
things: gram's `doc/behaviour.md` and each plugin's `doc.go` `# Vocabulary`. Hosts, moments and
the `cgoke` binding in a plugin: `gram-plugin`.

## Decisions

- Nothing of a game goes into ceremony's code, comments or examples: `command.Issuer`, never a
  player (`CLAUDE.md`).
- **Ceremony keeps no rhythm of its own** (the user's word, 2026-10-18): a schedule is the
  executor's, the pace lives in goke. `NewPlans` and `NewEffects` take the caller's goke query
  options (gram's `world.Plugin.Pace` is a `goke.PacedBy`) and walk each block over its `Dt`.
- **Ceremony names no places** (the user's word, 2026-10-18): a library names its kinds with
  `host.NewPlacing(name, takes)`, `takes` handed a **pointer** to the moment; gram's are the
  board's `board.Around(n)` and `board.Ahead(n)`.
- The user tags and publishes ceremony; gram's `go.mod` follows a tag, `go.work` joins the
  working trees meanwhile.

## The model

- A rule (`act.Branch[P]`) runs its `Then` **every time** its moment comes while `When` holds,
  with no memory of its own: its memory is an effect (`When: cond.Self.If(cond.Not(e))` with
  `Put(e)`: at most once a while). A plan (`plan.New` of `plan.Loop[P]`s) is a mind on the
  entity, a loop of a fact running once as the fact comes afresh; places are a rule's alone.
- An effect misbehaving (fires once, too often, lingers, flickers) is answered by the table in
  doc/ceremony.md, "How an effect runs": a `Put` on a running, non-stacking effect restarts its
  time, `Then` begins in the same pass, `Hold` in a rule lapses two steps after it stops firing,
  `Has` holds from the cast but the marker only from the next pass.
- gram nets a step's commands of one effect on one entity (`effect_commands.go`): the last given
  wins, and a `Put` every tick keeps a `Lasts` effect open until a tick after the last.

## Shapes for periodic and random behaviour

- A cycle of effects (A `Then` B `Then` A) cannot be defined: `Named` before `Define` panics.
  Periodic cosmetics are a look (`render.Timed` under the stage's effect), not effects.
- A loop runs its `Then` once; repetition is the plan finishing and beginning again at the top
  (`plan.OneOf` of loops gated on effects). Two periods at once do not fit one plan: one is a
  look.
- `act.Branch[any]` inside a plan takes steps: `plan.Self.Do`/`Put` yes, `plan.Self.Wait` no. A
  random wait is a lasting effect a coin `Put`s, a short wait (its marker lands a pass later),
  then a loop idling `Until` the effect is gone.
- A plan's `Hold` lasts as long as its loop: in `plan.Steps` nothing after it comes. A stage
  with a length is `Steps(Put(e), Wait(d), Take(e))`, the interrupting rule taking `e` off too.
- `cond.Chance` is drawn afresh every step, per guard: two coins in one step are independent,
  and a coin is never the `When` of a loop meant to last (use it in an instant branch inside).

## Pitfalls met

- A state meant to end a rule's firing must be taken off in the rule's own `Then`, or the rule
  must ask for its absence: a quarter-second "press" effect fired for fifteen ticks.
- An intent and an animation are two effects (`ReachingEf`, `PullingEf`): one for both loses
  presses during the swing.
- Two effects' looks on one entity: gram draws the look declared later (`Under` order).
- A pair rule of a sensor fires every tick the boxes overlap; a parted pair at rest keeps
  meeting (collision's `Resting`), which navigation ignores.
