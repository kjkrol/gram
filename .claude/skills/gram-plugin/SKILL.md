---
name: gram-plugin
description: How a gram plugin is built and how to write one — the plugin contract, the world's registers, the delivery of rules to hosts, the commands about effects, the store behind ceremony's contract (cgoke columns, command buffers), roles, a plugin as an entity (world.Self), tags and markers, kinds and the roster, a plugin's package layout (entries, vocabulary, internal), the module and System conventions, and how behaviour goes through rules. Use when reading or changing any package under plugins/, adding a moment, fact, word, knob or command, or wiring a new plugin into a Stage.
---

# gram-plugin

## Plugin system

`plugin.Plugin` — `Name`, `Install(ctx plugin.Installer) error`, `RunPlan`, `WithRenderer`,
`Renderer`, `EventHandler`, `Serializable` — is the one extension point. `ctx.Use(p)` registers
`p.Serializable()` (if any) and calls `p.Install`, which only queues ECS wiring
(`ctx.UseModule`/`ctx.Setup`), flushed once by `ecs.Setup()` after `Stage.Init` returns — which
lets `Persistence.Load` decide fresh-spawn vs. restore before the ECS commits. `ctx.Track(s)`
registers non-`Plugin` state (a `Composition`, a ui scene) into the same save/load machinery.

**How entities behave is ceremony's**: a rule is an `act.Branch[P]{Name, When, Then}` run inside
the pass of the plugin that catches its moment `P`; a plan `plan.New(name, root)` of
`plan.Loop[P]{Name, When, Then, Until, Timeout, Cooldown}` on the entity's mind; an effect an
`effect.Effect` with its marker; a role a `*ceremony.Part`; a command about an effect
`ceremony.Put(e).On(whom)`. A plugin runs the rules of its moment with `host.Rules[P]`/
`host.PairRules[P]`/`host.StepRules[P]` inside a pass it makes anyway — one walk, many rules,
never a system per behaviour — handed a `host.Tick`.

**What gram keeps** is the store's side and the perceiving:

- The world's registers: `world.Effects()` (`Define(name, Spec{Lasts, Stacking, Then, Grant,
  Alter})`, `Named`, `Cast`, `Has`; the register and the `effect.Runtime` the world's
  `*cgoke.Effects` is), `world.Roles()` (`Define(name, rules...)`, `Named` a `*ceremony.Part`
  over a tag of `cond.Roles` from `Kinds.DefineTag`, 64 a Stage, noted with `Kinds.Play` for the
  delivery), `world.Commands()` (`Define(name, cmd)`, `Named`), the kinds (`kind.Define[P]`,
  `kind.Named[P]`, `board.Units.Define`/`Named`, `bullet.Shots.Define`/`Named`); the plans a kind
  carries in its Spec (`plan.Planned`), registered on the world's `plan.Plans` by `Kinds.Register`.
  A name defined twice or asked for unknown panics by name, in `Init`. Rules are not registered.
  `ceremony.NewPart(name, tag)` is for plugins and tests without a world.
- The delivery: a plugin tells the engine its hosts in `Install` (`ctx.Hosts(&p.pairs,
  &p.entities)`); once `Init` returns the engine hands every rule of every role played
  (`world.Kinds.Played`) to the first host, in `Use` order, that takes it (`ceremony.Deliver`;
  a host refuses a moment made for another with `host.ErrUnhosted`); one none takes fails `Init`
  naming the rule by its `String()`. A plugin's own rules (navigation's crowd) it adds to its host
  itself.
- The commands about effects carried out (`plugins/world/effect_commands.go`, the first system
  of a step): `ceremony.Put/Take/Toggle(e).On(target).For(d).By(source)`, a target
  `ceremony.Named(names...)` (one entity a name), `Group(names...)`, `World` (the world's own
  entity, the clock's), `ID(id)`, or a plugin's `ceremony.Router` (`selection.Plugin.Selected(
  roles...)`, `Pointed()`, a `world.Self`). Entities are found by their `ceremony.Label{Name,
  Group}` through one query; a step's commands of one effect on one entity are netted (the last
  wins). `act.Self.Do(ceremony.Trigger{})` gives every command whose `By` names the entity; a
  command with a `By` is kept in the register, and a name nobody bears, or two bear, panics as the
  first step begins. A command given to a key must come from the register (`control.Give` panics
  otherwise). A `ceremony.Command` without an `On` is no step: `act.Self.Put(e)`/`Take(e)`
  (`Other`, `Around(n)` the same) are the steps on effects.
- The perceiving, each marked and asserted beside its type: a moment by `Who()` (`var _
  host.About = Standing{}`; `host.Met` a pair, `host.Placed` one standing on places), a fact by
  `Fact()` (`navigation.Blocked`, `Arrived`, `bullet.Flight`), a knob by `Knob()`
  (`steering.Steering`, `cell.Ground`, `Floor`, `Wall`, `collision.Physics`, `gravity.Pull`,
  `sky.Moon`, `vision.Sight`, `render.Appearance`), a command by `Command()`, a word by
  `cond.NewWord(name, func(P) bool)` (`unit.Fallen()`, `vision.SeesNobody()`, `clock.At(t)`).
  The moment's type — `collision.Meeting`, `collision.Struck`, `vision.Sighting`,
  `unit.Standing`, `cell.Now`, `world.Moving`, `clock.Moment` — says whose it is.

**The store is behind a contract**: the engine reads a `store.Row` a block at a time (an array a
column out of goke's own accessors, `cgoke.Columns`, indexed as `Slice(cur)[i]`) and stages what
it writes in goke's command buffer (`store.Facts.Stage` over `CmdBuf.Reserve`), so nothing costs
more than typed code — `bench/behaviour_test.go` measures plans, rules and effects a tick. A pass
binds its hosts with `cols := cgoke.NewColumns(qb); host.Bind(cols)` (`cols.Own(&s.col)` shares a
column the pass reads itself) and runs them over each block with `cols.Row(cursor)`. `host.Tick`
carries no command buffer; the binding's systems write through their own (`ECS.CmdBuf(runnable)`).
The tag families join the host's queries as optional components, so a rule costs no query.

**Roles** say who obeys: a role is a behaviour (mortal, hasty, a trapdoor), not a group.
`Part.Obeys(rules...)` narrows each rule to the role's players on top of its own filter (a pair
rule: the pairs whose own entity plays it; a same-sided `collision.Meeting` rule still once a
pair, `DispatchEitherWay`; a `Touch` each unit's own, a `Sighting` each observer's; an entity
playing two roles that obey one rule obeys it once per role). A kind plays roles through
`ceremony.Plays(roles...)` in its Spec (one component; a kind carrying a component type twice
panics), a cell through its kind (`cell.Kinds.Define(name, kind, roles...)`) or its Layout entry
(`cell.Entry.Plays`), a plugin through `world.Self.Plays`. A rule of a moment of the world as a
whole (`clock.Moment`, `climate.Weathering`; `host.StepRules`, once a step, no entities walked)
fires while the entity the moment is about plays its role. A role under `cond.Around(n)` holds
for the place turned to.

**A plugin is an entity** (`plugins/world/self.go`): `p.Self = world.NewSelf(w, p, knobs...)` in
`NewPlugin`, which notes `p` as made over the world (`world.Plugin.Made()`, the order the Stage
uses) and calls its entity by `p.Name()`. Embedded as `*world.Self` in every plugin (the world's
own is the clock's entity, the clock's `State` its knob), it is one entity called by the plugin's
name (`ceremony.Label`), carrying its knobs (`comp.Comp`s, listed for the saves), the effects'
markers, `tag.Tags[cond.Roles]` and a `cgoke.Wide` (a slot for every definable effect, where a
unit's `Active` holds 8). The world's first system (`selves.system`) finds them by name in a loaded
game. `Self` is a `ceremony.Router` (`ceremony.Put(e).On(s.atmosphere)`), `Self.Entity()`,
`Self.Changed()` (its `effect.Changed`: the convention by which a plugin works costly things
anew), `Self.Plays(roles...)`; `cond.On(s.atmosphere).If(e)` reads it back. One thing of a kind a
plugin.

**Tags** are bits of a family, not component types (`ceremony/tag`): `tag.Tags[F]` holds up to
64 tags of family `F` (an empty type a plugin names the family by; tags are the plugins'
technique, a game says roles, names, groups and commands), `kinds.DefineTag[F](name)` hands out
the bits by name through `world.Kinds` (saved by name, remapped on load), `comp.Tagged(tags...)`
gives them to a kind, a query over the family's `Tags` narrows to those carrying any, a bit flip
is seen the same tick; `host.Marks.Carries(tag)`. **Tags** are groups a kind gives; **markers**
are states switched on and off — a plugin's family `States`, carried for good (`comp.Marks[F]()`,
`Roster().Unit.Default`), each a constant bit (`effect.Changed`, every effect's `Mark()`,
`unit.Entered`). Putting a component on or off moves the entity (~200 ns) against 1–2 ns for a
bit: a state that changes often or lasts a step is a marker; one that lasts, on few entities,
walked alone (`MoveOrder`, `Mind`, the facts, `world.Outside`) keeps its own component. Data that
comes and goes keeps its component, empty when off (`cgoke.Active`).

**Kinds** (`entity/kind`): `kind.Define[P](world.Kinds(), name, kind.Spec{...})` — a `Spec` is
the list of a kind's components, each made in `ceremony/comp`: `comp.Const(v)` or
`comp.Load(func(row P) T)`, `world.Position` and `world.Velocity` among them (one of each);
`kind.Named[P](reg, name)` is the `kind.Of[P]`: `prey.Entry(row)` builds a roster entry for
`world.Plugin.Seed`, `prey.ID()`/`SpriteID()` what its entities carry and are drawn from;
`kind.Entry.Named(name)`/`InGroup(group)` give the entity its `Label`, `Entry.Told(cmds...)` the
commands it gives itself as it is made. `kind` never imports `world`: the registry is
`world.Kinds` behind `kind.Registry`, which also issues atlas slots no kind owns (`NewSprite`),
lists every component type its kinds carry for `Persistence.Load` (`Kinds.LoadComps`) and gives
`Lookup` (a kind's id and row type). Initial state: `Stage.Spawn` declares it through each
plugin's typed `Seed`, and the engine calls `Populate()` on every tracked `plugin.Populator` for a
Stage entered afresh.

## Package layout

`render` (root) — `Renderer`/`AtlasSource`/`Atlas`/`Composer`/`Frame`/`Source`/`Tier`/
`CachedRenderer`/`SolidBackground`/`TelemetryRenderer`/`Feed`/`Still`/`Direct`/`Font`, drawing
with no knowledge of Stages, Scenes or plugins. `plugin` (root) — the extension contract
(`Plugin`/`Installer`/`Serializable`/`PostLoader`/`Populator`/`CommandHandler`/`Made`,
`plugin/section`); imports `render`, `control`, `ceremony/host`, `ceremony/command`. `game`
(root) — what a Stage implements and receives, all in one package; imports `plugin` and `render`.
`internal/engine` — the `Engine` plus the unexported `initializer`/`persistence`/`storage`/
`stageRuntime`. Dependency direction: `render` ← `plugin` ← `game` ← `internal/engine` ← `gram`.
Built-in plugins (`plugins/*`) import `plugin`/`render` directly, as a third-party plugin would.

gram's core beside them: `ceremony` (`tag`, `comp`, `effect`, `fact`, `cond`, `act`, `plan`,
`host`, `command`, the root), `entity` (with `entity/kind`), `clock`, `control`, `camera`. None
imports a plugin; the world makes and runs their systems and registers their components for the
saves.

**A plugin's packages**: its root holds the entries (`NewPlugin`, `Config`, `With…`, `Seed`,
`Plays`, the commands it handles); public subpackages hold the vocabulary and contracts a game and
other plugins both name (`cell.Kind`, `unit.Mover`, `grid.Grid`, `board/look`, `board/ground`) and
nothing else; `plugins/<plugin>/internal/...` the machinery, importable only inside the plugin,
never in a public signature. A type the machinery needs goes to a vocabulary package; a command
the machinery carries out is defined in the root, its queue kept there, reaching the machinery
through an interface the plugin implements (`icameras.Orders`). Where a file imports a public
package and its internal namesake, the internal one gets an `i` prefix (`irelief`, `ipainter`);
inside the internal package its public namesake is `public`. Systems in `internal` are a part
type whose method returns a private `goke.System` (`terrain.Cells.System()`). What several
packages' tests share goes to `internal/<plugin>test`; a test reading a package's insides uses
`export_test.go`. A public name only tests read is no API: it goes, or moves to
`export_test.go`. Where the machinery reads the game's own types every step (navigation's
systems), it stays unexported in the root. A package of one interface and an alias is no package.

Every package under `plugins/` implementing `goke.Module` names that type `module` (unexported);
the exported `Plugin` is a facade. A `module` never implements `goke.System`; per-tick logic has
its own type with the `System` suffix (`ScanSystem`, `altitudeSystem`), never leaking out of the
plugin: no `Plugin` method hands out a system's state, no plugin takes a callback reaching into
another's system. A system keeps no table beside the ECS: what it knows about an entity is a
component on it. Each `plugin.go`/`module.go` groups contract methods first.

## Behaviour goes through rules

A plugin **perceives** — moments for rules (`unit.Standing`, `collision.Meeting`,
`navigation.Touch`), facts for plans — and **carries out** commands (`navigation.StepAside`,
`world.Despawn`), keeping the engine's own rules inside the handler (a unit is never stepped
into water, off a cliff or into a wall). **Effects** hold state and a rule's memory. A plugin
gives its host its own defaults unexported (navigation's crowd); a game adds its own as roles its
kinds play or replaces them where the plugin offers it (`navigation.Plugin.WithCrowd`). Export
only what a game needs.

A game is written the same way: its **states are effects** (burning, frozen, alarmed, bored),
each with its own marker; its **rules connect** the plugins' moments to those effects
(`act.Self.Put`, `Take`, `Hold`, `Do`; `cond.Chance`, `cond.Around(n).If`); the plugins give
**knobs** — components an effect's `Alter` turns. **A knob is a component its plugin only
reads**; what a system writes lives beside it (`steering.Course` beside `Steering`,
`vision.Sighted` beside `Sight`, `Ground` apart from the heights), given where missing. A
**player acts** by giving a command: `ceremony.Put(e).On(sel.Selected(role))`, `.On(sel.Pointed())`,
`.On(ceremony.World)` (read by `cond.On(s.world).If(e)`), `.On(ceremony.Group(name))`.

**A thing used by touch** (the wire demo's lever): a post (`Collider`, `Physics{Mass: +Inf}`)
the hero (`Physics{Mass: 1}`) walks up to and rests against; the hero's rules of a
`collision.Meeting` under `cond.Self.If(collision.Facing(angle))` hold `InReachEf` while a lever
is at hand (a ui prompt pinned `Under` it) and, under the player's `ReachingEf` (the intent, a
second, put by Space), put the lever's transition on it and give it `Trigger`; the lever's lower
position is its own effect held for good (`LeverPulledEf`, the transition's `Then`), the
trapdoors' command a `Toggle`. An intent and an animation are two effects: `ReachingEf` the
intent, `PullingEf` the hands on the lever for the swing (the Phased look); a press during a
swing waits for its end. **Boredom is a state too**: a rule of `world.Moving` under
`world.Still()` puts `StillEf`, whose `Then` casts `BoredEf` after three seconds; a step takes
both off.
