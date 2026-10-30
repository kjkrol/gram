---
name: gram-plugin
description: How a gram plugin is built — its install and what it draws, its settings saved, a system in the world's bands, rules hosted in its own pass, the store behind ceremony's contract, roles, a plugin as an entity (world.Self), tags against markers, its packages, module and systems, where the marks sit, patterns of touch and state. Use when reading or changing any package under plugins/, adding a moment, fact, word, knob or command, or wiring a new plugin into a Stage.
---

# gram-plugin

The contract is `plugin/doc.go`; the house rules (behaviour in ceremony, no hooking, registers,
a plugin's root and commands, settings, float32) are CLAUDE.md's; the game's side of ceremony is
`doc/behaviour.md`, the language the `ceremony` skill; the registers, `Self` and the bands
`plugins/world/doc.go`; each built-in plugin `gram-plugins`; drawing `gram-render`.

## Install and drawing

- `Install` only queues the ECS wiring, flushed by one `ecs.Setup` after `Stage.Init`, so
  `Persistence.Load` decides between a fresh spawn and a restore before the ECS commits.
- The sources are made in `NewPlugin` or `Install` and given `ctx.Draws` there, existing from then
  on even where what they draw comes later (an atlas in `SetAtlas`). A typed nil passes `Draws`'
  nil check: a plugin with one of two renderers names the one it has. A `Draws` after the world's
  `Picture()` panics. Tests take the sources from a recording Installer (`topotest.Drawn`, the
  bench's `drawnAs`, a `drawsCtx`) and hand `SetAtlas` a foreign `render.AtlasSource`.
- A plugin's style for a kind is a `render.Style` type (`Style()`, asserted); `render.Appearance`
  is optional in its query, as a bare test entity has none. A colour per entity on the GPU is an
  instance attribute, never a uniform.

## Settings saved

- `plugin.Keep` encodes `Res.Settings` whole because gob sends no zero field and zeroes none on
  decode: a saved `false` over a `true` start would come back `true`. `Restore()` calls
  `kept.Restore()`, then applies what mirrors must (a baker's flag, a renderer's switch). A plugin
  with a `persisted` of its own (cameras) appends the kept's value; `decodeGroup` leaves what an
  older save lacks as it is.

## Systems and rules

- **One walk, many rules**: a plugin runs the rules of its moment (`host.Rules`, `PairRules`,
  `StepRules`) inside a pass it makes anyway, never a system per behaviour. Rules are not
  registered: they ride on roles. A plugin's own rules (navigation's crowd) it adds to its host.
- **A system goes in the world's bands by its plugin's policy, one query and one system, never
  one a band**: it holds `w.Pace(policy)` (nil in a test building it bare: `if s.pace != nil {
  qb.With(s.pace) }`), walks `q.Paced(d)` scaling what is "a tick" by `q.Dt()/d`, and walks
  `All()` asking `q.Due()` where it must touch blocks out of turn. An exported system offers
  `Paced(pace goke.Opt)`.
- **The store is behind a contract, at no cost over typed code** (`bench/behaviour_test.go`): rows
  read a block at a time (`cgoke.Columns`), writes staged in goke's command buffer. `host.Tick`
  carries no command buffer; the binding's systems write through their own. The tag families join
  a host's queries as optional components, so a rule costs no query.
- A knob is saved with its original, so one under an effect at a save goes back as the effect
  ends after the load.
- A `ceremony.Put(e)` without an `On` is no step: a rule's steps on effects are `act.Self.Put(e)`,
  `Take(e)` (`Other`, `At(...)` alike).

## Roles, Self, tags and markers

- **A role is a behaviour (mortal, hasty, a trapdoor), not a group.** An entity playing two roles
  that obey one rule obeys it once per role. A kind carrying a component type twice panics, and
  `ceremony.Plays` is one. A `StepRules` rule fires while the entity its moment is about plays the
  role; a role under `cond.At(...)` holds for the place turned to.
- `world.NewSelf(w, p, knobs...)` in `NewPlugin` also notes `p` as made (`world.Plugin.Made()`,
  the order the Stage uses). Its entity carries a `cgoke.Wide` (a slot for every definable effect,
  where a unit's `Active` holds 8) and is found by its name in a loaded game. `Self.Changed()` is
  the convention by which a plugin works costly things anew.
- **Tags are the plugins' technique; a game says roles, names, groups and commands.** A tag is a
  bit of a family (`tag.Tags[F]`, 64 a family), handed out and saved by name, a flip seen the same
  tick; `host.PairRules.For(marks)` says whether any rule is for its carrier (vision scans only
  those).
- **A marker or a component**: putting a component on or off moves the entity (~200 ns against
  1–2 ns for a bit). A state that changes often or lasts a step is a marker (a family `States`,
  carried for good, asked of the kinds by `r.Default` in `KindTemplates`, the one place a plugin
  writes them); one that lasts, on few entities, walked alone (`MoveOrder`, `Mind`, the facts)
  keeps its own component; data that comes and goes keeps its component, empty when off.

## Packages

- **A strategy a game writes is a contract** of a vocabulary package, given as the plugin is made
  and read by every pass that needs it, never two copies of the logic: `cell.Mobility`
  (`WithMotion(m, mobility)`, 2026-10-28) paces the standing pass, the planner and the driving
  alike; the library gathers the facts (`cell.Step`), the strategy composes them (`cell.PaceOf`
  checks the range), a default stands beside it (`cell.DefaultHiking`).
- A public subpackage holds the vocabulary and contracts a game and other plugins both name, and
  nothing else; `internal` never shows in a public signature.
- A type the machinery needs goes to a vocabulary package; a command it carries out is defined in
  the root with its queue, reaching the machinery through an interface the plugin implements
  (`icameras.Orders`). Machinery reading the game's own types every step (navigation's systems)
  stays unexported in the root.
- An internal package imported beside its public namesake takes an `i` (`irelief`); inside it the
  public one is `public`. Test helpers shared go to `internal/<plugin>test`, a package's insides
  through `export_test.go`. A public name only tests read is no API; a package of one interface
  and an alias is no package.
- A `goke.Module` is the unexported `module`, the `Plugin` its facade; it never implements
  `goke.System`: per-tick logic is a type with the `System` suffix (in `internal` a part type whose
  method returns a private `goke.System`). No `Plugin` method hands out a system's state, no
  plugin takes a callback into another's system, a system keeps no table beside the ECS.
  `plugin.go` and `module.go` group the contract's methods first.
- An interface asserted at run time (`x.(interface{ Proportions() (float32, float32, bool) })`)
  misses an implementer of another float type without a word: a seam is changed on both sides.

## Where the marks sit

- `# Vocabulary` lists moments (host kind, subject, words), facts, knobs, commands (an entity's
  own, a player's, both; `host.Aimed`) in that order, a sentence each, every name a doc link.
- A moment's assertions in a grouped `var (…)` at the bottom of its own file (bullet and gravity:
  `moments.go`); a fact's and a knob's beside the type; commands and their `host.Aimed` in
  `commands.go`'s block. A word's name is spaced lowercase (`"sees nobody"`): errors show it.
- The board's `ringsRound` (the places round a box) is not reentrant.

## Patterns

- A thing used by touch (the mechanisms' lever): a post (`Collider`, `Physics{Mass: +Inf}`) the
  hero rests against; the hero's rules of a `collision.Meeting` under `collision.Facing(angle)`
  hold `InReachEf` (a prompt pinned `Under` it) and, under the player's `ReachingEf` (the intent,
  put by Space), put the lever's transition and `Trigger` it; its lower position is its own effect
  held for good. An intent and an animation are two effects: one for both loses presses mid-swing.
- Boredom is a state: a rule of `motion.Moving` under `motion.Still()` puts `StillEf`, whose `Then`
  casts `BoredEf` after three seconds; a step takes both off.
