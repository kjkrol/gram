---
name: ceremony
description: ceremony, the language of behaviour gram builds on (github.com/kjkrol/ceremony, locally /home/karol/CODE/ceremony through go.work) — rules, plans, effects, roles, commands, facts, and its goke binding cgoke. Use when writing or reading rules, plans or effects in gram or a demo, when an effect or a rule misbehaves (fires once, fires too often, lingers, flickers), or when touching ceremony itself.
---

# ceremony

Grammar and layout: `/home/karol/CODE/ceremony/doc/ceremony.md` (the reference, read it for the
clauses) and `CLAUDE.md` there; gram's side (its plugins' moments, words, facts, knobs and commands,
where a game defines things) is gram's `doc/behaviour.md`. Nothing of a game goes in: an "obrzęd" is
prescribed steps with roles, so `command.Issuer`, never a player; a sprite, a cell, a unit are
gram's.

## What is what

- **Rule**: `act.Branch[P]{Name, When, Then}` of a moment `P` a plugin's pass catches; runs its
  `Then` **every time** the moment comes while `When` holds — no memory of its own. A rule's
  memory is an effect (`When: cond.Self.If(cond.Not(e))` with `Put(e)` = at most once a while).
- **Plan**: `plan.New(name, root)` of `plan.Loop[P]{Name, When, Then, Until, Timeout,
  Cooldown}`, a mind on the entity; a loop of a fact runs once as the fact comes afresh.
  `Around` is a rule's: a plan has no places.
- **Effect**: `Effects.Define(name, Spec{Lasts, Stacking, Then, Grant, Alter})`, `Named`; its
  marker `Mark()` is what rules ask (`cond.Self.If(e)`); `Alter[T Knob]` turns a knob (a
  component its owner only reads) and puts it back.
- **Command**: `act.Self.Do(cmd)`, fire and forget, carried to the handler of its type;
  `ceremony.Put/Take/Toggle(e).On(whom).By(source).For(d)` are the commands about effects;
  `Trigger{}` gives every command whose `By` names the entity.
- **Fact**: a component told to entities with a mind (`fact.Fact` mark); `Subject` for `Aimed`.

## How an effect runs (the table in doc/ceremony.md, "How an effect runs")

- A `Put` on a running, non-stacking effect **refreshes** its time (restart, not a sum, not
  ignored) and does not re-run its begin; `Stacking` adds a slot (8 an entity, 63 a plugin's).
- Running out puts the knobs back and begins `Then`'s next **in the same pass** (since v0.2.1;
  before there was a step with neither marker, a false "not under" window). A `Take` casts no
  `Then`.
- `Hold` in a rule = a cast of two steps renewed each firing: it lapses two steps after the rule
  stops. In a plan, held while the loop runs.
- `Has` is true from the cast (pending); the marker from the begin, the next pass.
- gram nets a step's commands of one effect on one entity (`effect_commands.go`): the last given
  wins; a `Put` every tick keeps a `Lasts` effect open until a tick after the last.

## Hosts and the binding

- A plugin hosts rules of its moment: `host.Rules[P]` (one), `host.PairRules[P]` (a pair,
  `DispatchEitherWay` once a pair for a same-sided rule), `host.StepRules[P]` (the whole, once
  a step); bound to columns with `cols := cgoke.NewColumns(qb); host.Bind(cols)`, run over a
  chunk with `cols.Row(cursor)`; `host.Tick` from `world.Plugin.Tick(d)` (carrier, time, seed,
  `Around` from the board's hosts, `Roles`).
- A moment is a struct with `Who()` (`host.About`), `Whom` (`host.Met`), `Placed`; a word is
  `cond.NewWord(name, func(P) bool)` beside it; a knob `Knob()`; a fact `Fact()`; a command
  `Command()` — each asserted beside its type (`var _ host.About = Standing{}`).
- `cgoke`: `Effects` (register + runtime, `Cast`, `CastFor`, `Dispel`, `Has`), `Active`/`Wide`
  (`Left(e)`), `Plans`, `Columns`, `Spawner`. The effect system casts through its own `CmdBuf`
  (`ECS.CmdBuf`) and stages facts with `Reserve`.

## How a subject kind runs (Around, and adding one like it)

- ceremony knows no geometry: `Around(n)` is a subject kind (`cond/cond.go` `who{kind, rings}`,
  `act/subject.go`), lowered to `steps.OfTarget` with `Lowering.Rings`/`Places` (`cond.go`
  `whose`), and the host's callback does the walking: `host.Tick.Around(moment any, rings,
  each)` copied into `steps.Pass` by `act/rule.go` `fire`; the callback gets a **pointer** to
  the moment (`&f.current`), so it switches on `*P`.
- The exec `steps.around` (`internal/steps/instant.go`) wraps the **smallest part of the rule
  holding every place clause** (`act/step.go` `build.place`: each leaf, group and Branch asks;
  `placed{rings, count, mixed}`), runs it once a place with `c.id` the place, succeeds when any
  place does; `refuses(payload)` at build (`NewInstant`) when the moment is not `Placed`.
- "One ring of places a rule": `cond.go` `whose` fails on two rings, `act/branch.go` `build`
  on `placed.mixed`; a plan refuses places (`act/lasting.go` `loop.node`, `branch.go` `topNode`).
- A role under a place is `GuardPlaying(bit, OfTarget)` read through `Tick.Roles` (the host's;
  gram: `world.effectCmds.RolesOf`); a word under a place is refused (words are of Self/Other).
- Adding a kind beside it (as `Ahead(n)`): `host/moments.go` (a marker interface), `host/tick.go`
  (a callback), `steps.Pass` + the exec + `refuses`, `cond.go` (`whoKind`, constructor,
  `String`, `whose`), `act/subject.go` (kind, constructor, `places`), `act/step.go`
  (`placed`, `build`, the wrap), `act/branch.go`/`lasting.go` (the plan refusal), `act/rule.go`
  (the Pass copy), then `doc/ceremony.md` grammar lines, README, `CLAUDE.md`, `cond/doc.go`,
  `act/doc.go`. Tests: `act/clause_test.go` (`spot` fixture with a hand-written `Tick.Around`),
  `act/steps_test.go`, `cond/cond_test.go`, `plan/loop_test.go`.

What each gram plugin perceives and carries out is the `# Vocabulary` section at the end of its
`doc.go` (moments with words and subjects, facts for plans, knobs, commands).

## Shapes found for periodic and random behaviour

- A repeating cycle of effects (A `Then` B `Then` A) cannot be defined: `Named` before `Define`
  panics. Periodic cosmetics (a blink every 3 s) are a look (`render.Timed` under the stage's
  effect), not effects.
- A plan's loop runs its `Then` once; repetition is the plan finishing and beginning again at
  the top (`plan.OneOf` of loops gated on effects). Two periods at once (3 s and 4 s) do not fit
  one plan: one of them is a look.
- `act.Branch[any]` inside a plan takes an `act.Step` in its `Then`: `plan.Self.Do`/`Put` yes,
  `plan.Self.Wait` no (a part, not a step). A random wait is a lasting effect: a coin `Put`s it
  (`Lasts` the long time), the short wait passes, then `plan.Loop[any]{Then: Idle(), Until:
  cond.Self.If(cond.Not(e))}` idles while it lasts. A `Put` lands at the step's end: a clause
  reading its marker in the same step sees nothing yet, so a wait must lie between.
- A plan's `Hold` runs as long as its loop: in a `plan.Steps` nothing after it ever comes.
  A stage with a length is `Steps(Put(e), Wait(d), Take(e))`, the interrupting rule taking `e`
  off too. A round is the plan done and beginning again at the top.
- `cond.Chance` draws per guard (seed, time, entity, the guard's place), so two coins in one
  step are independent.
- `cond.Chance` is drawn afresh every step: never the `When` of a loop that should last (the
  coin flips and the loop stops); use it in an instant `act.Branch[any]` inside the steps.

## Pitfalls met

- A state meant to end a rule's firing must be taken off in the rule's own `Then` (or the rule
  must ask for it to be absent): a quarter-second effect as a "press" fired for fifteen ticks.
- An intent and an animation are two effects (`ReachingEf` a second, `PullingEf` the swing):
  one effect for both loses presses during the swing.
- Two effects' looks on one entity: gram draws the look declared later (`Under` order).
- A pair rule of a sensor fires every tick the boxes overlap; a parted pair at rest keeps
  meeting (gram's collision `Resting`), navigation ignores those.
- ceremony's conventions: tests in the package they test (`goke/*_test.go` external, importing
  `cgoke`), comments of two sentences, the user tags and publishes; gram's `go.mod` follows a
  tag, go.work joins the working trees meanwhile.
