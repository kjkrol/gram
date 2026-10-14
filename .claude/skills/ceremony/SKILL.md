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
