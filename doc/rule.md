# Rule: how entities behave

One vocabulary for every behaviour of an entity, in `rule`: a **rule** is what is
done at a moment a plugin catches, a **plan** is what an entity does over time, an **effect** is a
change that holds, a **command** is what an entity has done, and a **fact** is what a plugin tells
an entity. Written on 2026-09-30 when yielding and avoiding outgrew the reflexes, and unified the
same day: behaviors, effects and trees had grown three ways of saying the same thing. The package
was `conduct`, then `act` (rules were "triggers", plans "trees"); on 2026-10-01 it took the names
it has: two constructors, each taking a function that writes the steps. The same day effects got
their own markers, `Dispel`, `Then` and `Chance`, so that a game is written as states and the
rules connecting them (below, "A game: states as effects"). On 2026-10-02 a Stage came to hook its
rules itself (`ctx.Hook`), entities to obey them by the roles they play, and a lever to be a wire
(below, "Hooking", "Roles", "Wires").

## Two constructors

A rule and a plan are each written by a function. The constructor hands it the one the steps are
for — the **Moment** a rule fires at, the **Actor** a plan is for — and their methods make the
steps it returns, all of one type, `rule.Step`. Go 1.27's methods with type parameters make
`a.Order(cmd)`, `a.When[Blocked](…)` and `.Until[Arrived]()` possible.

```go
// a rule: at a moment, for whom, what to do
rule.On("in the ice", rule.All, func(m *rule.Moment[unit.Standing]) rule.Step {
	return m.OneOf(
		m.If(caughtInIce, m.Keep(frozen)),
		m.If(unit.Standing.Fallen, m.Order(world.Despawn{})),
	)
})

// a plan: what a unit does over time
plan.New("patrol", func(a *plan.Actor) rule.Step {
	return a.Steps(
		a.Order(navigation.MoveTo{Cell: east}).Until[navigation.Arrived](),
		a.Wait(10 * time.Second),
		a.Order(navigation.MoveTo{Cell: west}).Until[navigation.Arrived](),
	)
})
```

- `rule.On(name, filter, func(m *rule.Moment[P]) rule.Step)` is a `rule.Rule` for the plugin that
  catches `P`, hooked through the Stage's `ctx.Hook` (below, "Hooking").
- `plan.New(name, func(a *plan.Actor) rule.Step)` is the component a kind gives its entities:
  `units.Define("unit", …, patrol)`. Its name is what a save knows it by.
- A function writing part of a rule or a plan takes the Moment or the Actor as its own:
  `func whenBlocked(a *plan.Actor) rule.Step`. Ready-made rules come whole, in a plugin's `hooks`
  package: `ctx.Hook(chooks.ShowHits(hit))`.

## Filters

The second argument of `rule.On` says whom the rule fires for, read before its steps run:

- `rule.All` — every entity the plugin shows it, every pair for a moment of two;
- `rule.Self(tag)` — an entity carrying the tag, an effect's marker among them:
  `rule.Self(frozen.Mark())`; a cell carries the game's tags of places (`cell.Tag`, of the family
  `cell.Family`, given in the board's `Layout`) — `rule.Self(harbour)` on a `cell.Now`;
- `rule.Between(a, b)` — a pair whose entity carries `a` and whose other carries `b` (`tag.Any`
  for either side), for a moment that is `plugin.Met`: a collision's `Meeting`, a `Sighting`, a
  navigation `Touch`;
- `rule.Having[T]()` — an entity carrying the component `T`: `rule.Having[witch]()`.

The plugin checks the filter on the tag bits of each entity, so a rule that does not concern one
costs nothing for it. A role narrows a rule further, on top of its filter (below, "Roles").

## Hooking

A Stage hooks its rules through its Initializer, once its plugins are used and before `Init`
returns: `ctx.Hook(rules...)` hands each rule, and every rule of a role, to the plugin in use that
hosts its moment, trying them in the order they were used. The Stage need not know which plugin
hosts what:

```go
func (s *mainStage) Init(ctx game.Initializer) error {
	// … ctx.UseWorld, ctx.Use(s.collision), ctx.Use(s.board), ctx.Use(s.vision)
	return ctx.Hook(fireSpreads, whereItBurns, mortal, hasty)
}
```

A rule no plugin in use hosts is an error wrapping `plugin.ErrUnhosted`; a plugin used after the
`Hook` is not tried. A plugin's own `Hook` takes the rules of its moments too, before or after
`Use`, until the Stage's `ecs.Setup` builds its systems — later is `plugin.ErrHostBuilt` — but not
a role, which goes through `ctx.Hook`. An error names the rule by its `String`, its name, its
moment and what narrowed it: `"fall in" of unit.Standing, for the role mortal`; a role's is
`the role mortal`, a wire's `the wire west`.

## The five words

- **Rule** — a moment a plugin catches: a `unit.Standing` or a `cell.Now`, a `vision.Sighting`, a
  `collision.Meeting` (pairs) or `Struck`, a `world.Moving` or `Leaving`, a `clock.Moment`, a
  `climate.Weathering`, a `navigation.Touch`. A Moment's steps are instant — `OneOf`, `Steps`, `If`
  on the moment, `Not`, `Apply`, `Keep`, `Dispel`, `Chance`, `Unless`, `Under`, `During`, `OnWire`,
  `WhileWire`, `Playing`, `Order`, `ForOther`, `Here`, `Around` — so a step that lasts (`Wait`, `Until`, `Ask`)
  is not to be had in a rule: a Moment has no such method, and one made by an Actor is refused as
  the rule is made. A rule remembers nothing of its own, and runs no Go code of its own: there is no
  step for it. What the steps cannot say is a moment, a step or a knob the plugin still lacks.
- **Plan** — what a kind's entities do over time. It remembers its place (its mind, a component `plan.New` gives: the steps
  running, each one's place and start on the world's clock), waits (`Wait`, `Until`), talks to
  other entities (`Ask`), and is saved with the game. `OneOf` is a reactive choice, `Steps` a
  sequence with memory; the Actor's `When[F]` and `On[F]` open a branch on a fact, `If` reads one.
- **Effect** — `effect.Effect`, defined once from a `Spec` (`Lasts`, `Stacking`, `Then`, `Grant`,
  `Alter`), with its own marker, on while it runs (`Mark()`). `Apply` casts it; `Keep` holds it as
  long as its branch runs, or as long as a rule keeps firing it; `Dispel` takes it off. **An
  effect's presence is state**: `Unless(alarmed, …)` is "at most once a while", a memory for rules
  that keep none, and its marker is what rules of other plugins filter by. Collision's hit is one
  (`hooks.Hit`, cast by `ShowHits`, drawn by `HitOverlay` over `rule.Self(hit.Mark())`).
- **Command** — `Order(cmd)` gives the entity's command, the same one a player gives
  (`navigation.MoveTo`, `world.Despawn`), queued for the plugin that handles its type, and goes on
  at once: **fire and forget**. The handler carries it out for the entity alone
  (`control.Issued.ByEntity`); `world.Spawn{Entry}` is the one that makes another entity, of the
  Entry fixed as the rule is written (a nest laying an egg where it stands). The world keeps a stage's one carrier (`control.Carrier`,
  `world.Plugin.Commands`): the players give it theirs, the entities theirs; the engine carries
  every `plugin.CommandHandler` a stage uses, and a plugin's `plugin.Tick` hands the carrier to its
  rules. Nothing is dropped: a command waits for its handler's pass — given after it, for the next
  frame's. A command that is `plugin.Aimed` is told the subject of the fact it stands under, or of
  the rule's moment: whom the entity touched, who asked.
- **Fact** — a component a plugin writes for entities with a `Mind` alone (navigation's
  `Blocked`, `Arrived`), so entities without a plan pay nothing. Facts are how a plan learns what
  came of its commands.

## Idioms

- **Order, then wait for what comes of it.** A plan's `Order` hands back a `Command`: its
  `Until[F]()` waits for a fact to come *afresh* — a fact left from before does not count — and
  `Until(pred)` for one to come to hold it. `a.Until(pred)` alone waits without a command.
- **In a reactive branch, give the command once.** A branch of `OneOf` that orders and succeeds is
  run again at the next tick, and would order again: end it with `.Until(…)` or `.Stay()` so the
  branch stays while it holds, and put the reaction to the outcome *earlier* in the `OneOf`, so it
  takes over when the fact changes.
- **A branch is a value.** `hold := a.Order(Hold{}).Until(Blocked.WaitedLong)` may stand in
  several places; each is laid out on its own.
- **A command's outcome is a fact.** Fire and forget has no success or failure; navigation says
  what came of a `Detour` with `Cornered` and of a `Hold` with `WaitedOut`, in its `Touch` for
  rules and its `Blocked` for plans.
- **A rule's memory is an effect.** `m.Unless(e, m.Steps(m.Apply(e), …))` runs its steps at most
  once for as long as `e` lasts.
- **The clock's moment is the clock's.** A `clock.Moment` is of the clock's own entity: an effect
  a clock rule applies lands there — `m.If(clock.At(dusk), m.Apply(night))`, a phase that
  `clock.Clock.In` reads. Run once a step, walking no entities, it takes no filter and obeys no role.
- **Where one stands.** On a moment that is `plugin.Placed` — a `unit.Standing`, a cell's
  `cell.Now` — `m.Here(step)` runs the step on the cells under the entity (for a cell, on
  itself) and `m.Around(rings, step)` on those and the rings of neighbours round them, each cell
  once, on square and hex boards alike: an effect applied to the ground.
- **Now and then.** `m.Chance(p, step)` runs the step with likelihood `p`, drawn afresh at every
  step of the game from the world's seed (`world.Config.Seed`), the game time and the entity — no
  state kept, so a load and a replay draw alike.
- **One after another.** `effect.Then(next)` casts `next` when an effect's time is up — burning
  leaves smouldering — but not when it is dispelled: put out, nothing smoulders.

## A player's actions

A player changes the game the same way a rule does: by putting an effect on something, with a
command from a binding:

- `selection.Apply{Effect}` puts it on the player's own selected units — a sprint, a spell;
  another player's units and those nobody owns are never touched. A role's abilities are such
  bindings, reaching only the units playing it (below, "Roles");
- `world.Apply{Effect}` puts it on the world itself — its own entity, the clock's — a state of the
  whole game: an alarm, night called. A rule or a plan may `Order` it too;
- a wire's `Key` and `Switch` put it on the wire's own entity: a lever pulled, a switch flipped
  (below, "Wires");
- `bullet.Shoot{Ammo}` fires a shot from the player's own selected units, the way each faces or
  at a point (`Targeted`); a rule or a plan orders it the same way, for its entity, aimed at its
  moment's subject. What the shot does is rules of collision's `Meeting` and bullet's `Landing`,
  `Resting` and `Blast`: a wound an effect `ForOther`, a fuse an effect on the grenade that
  `Then` bangs, a `Burst` ordered under it.

Rules and plans read the world's states with `During(e, step)`, as they read an entity's with
`Under` and its wire's with `WhileWire`:

```go
alarm := fx.Define("alarm", effect.Spec{effect.Lasts(30 * time.Second)})
alert := fx.Define("alert", effect.Spec{effect.Alter(func(st *steering.Steering) { st.MaxSpeed *= 1.5 })})
s.player.Bind(control.Command(control.KeyPress{Key: control.KeyN}, "Sound the alarm",
	func(control.Context) (world.Apply, bool) { return world.Apply{Effect: alarm}, true }))
guard := rule.Role("guard").Obeys(rule.On("hurry", rule.All, func(m *rule.Moment[unit.Standing]) rule.Step {
	return m.During(alarm, m.Keep(alert))
}))
```

The guards hurry while the alarm sounds and slow down once it is over: `Keep` holds `alert` only
while the rule fires it.

## Roles

A **role** is a behaviour several kinds share, said once: the rules those playing it obey and what
their player can do with them. `rule.Role(name)` makes it; `Obeys(rules...)` adds rules, each
narrowed to the role's players; `Can(effect, trigger, label)` adds an ability. A kind plays its
roles through `rule.Plays(roles...)`, a cell through `cell.Entry.Roles`. A role is hooked like a
rule, and `selection.Plugin.Abilities(roles...)` makes its abilities bindings — each on its
trigger, listed under its label in the keys (K), casting on the player's selected units playing
the role:

```go
fx := s.world.Effects()
haste := fx.Define("haste", effect.Spec{effect.Lasts(3 * time.Second),
	effect.Alter(func(st *steering.Steering) { st.MaxSpeed *= 2 })})

mortal := rule.Role("mortal").Obeys(rule.On("fall in", rule.All, func(m *rule.Moment[unit.Standing]) rule.Step {
	return m.If(unit.Standing.Fallen, m.Order(world.Despawn{}))
}))
hasty := rule.Role("hasty").Can(haste, control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts")

selectable, mine := comp.Tagged(s.selection.Tags().Selectable), comp.Tagged(s.player.Owner())
s.scout = units.Define("scout", land, profile, selectable, mine, rule.Plays(mortal, hasty))
s.porter = units.Define("porter", land, laden, selectable, mine, rule.Plays(mortal))
s.player.Bind(s.selection.Abilities(hasty)...)
return ctx.Hook(mortal, hasty)
```

A scout or a porter fallen into water or a hole is gone; with both selected, J hastens the scouts
alone.

`Obeys` narrows a rule on top of its own filter: a rule of `rule.Self(hungry.Mark())` obeyed by
mortal fires for a mortal entity while it is hungry, one of `rule.Having[T]()` for a mortal
carrying `T`. On a moment that is `plugin.Met` it fires for the pairs whose own entity plays the
role. A collision `Meeting` rule with one tag on both sides, `rule.All` among them, still runs at
most once a pair; a `Touch` belongs to each of the two units, a `Sighting` to each observer. An
entity playing two roles that obey the same rule obeys it once for each. A rule of a moment of the
world as a whole — a `clock.Moment`, a `climate.Weathering`, run once a step walking no entities —
takes no filter and obeys no role: its host refuses it with `plugin.ErrUnhosted`. Say it with
`During`, or in a rule over entities.

`m.Playing(role, step)` runs the step while the entity plays the role and fails while it does not.
Inside `Here` or `Around` it asks the place the step turned to: the wire demo's handy scouts pull
the lever beside them with `m.Under(pull, m.Around(1, m.Playing(lever, m.OnWire(m.Apply(on)))))`,
and leave alone the trapdoors on the same wire. A plan's Actor has it too.

A program names 64 roles at most, one name one tag, which every world saves by the name. Make them
in `Init`: the world names them in its kinds as the Stage's ECS is set up. A kind names
`rule.Plays` once, every role it plays in it; a kind giving a component type twice panics.

## Wires

A **wire** is a connection by name — a lever and its trapdoors, a plate and its gate.
`world.Plugin.Wire(name)` defines one in `Init`, with an entity of its own, made at Setup and
found again by the name after a load. Its state is an effect on that entity, so one effect serves
any number of wires. An entity carrying `rule.Wired` is wired to one: a cell through
`cell.Entry.Wired`, a kind's entities through `comp.Const(west.Wired())` in its Spec. `Wired` holds
the wire's name hashed, so it is given before the wire's entity exists and survives a save. Its
rules follow the wire with two steps:

- `m.OnWire(step)` runs the step on the wire's entity in place of the entity's own:
  `m.OnWire(m.Apply(on))` drives the wire;
- `m.WhileWire(e, step)` runs the step while the entity's wire is under `e`.

Both fail for an entity wired to none; a plan's Actor has both too. A player drives a wire with a
binding: `Wire.Key(e, trigger, label)` puts `e` on it each time the trigger fires, a pulse lasting
as its `Spec` says; `Wire.Switch(e, trigger, label)` puts `e` on or takes it off, and one without
`Lasts` holds until switched off, saved with the game. A plate asks `cell.Now.Stood`: a unit
stands on the cell, its centre on it this step.

```go
fx := s.world.Effects()
open := fx.Define("open", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind = pit })})
on := fx.Define("on", effect.Spec{effect.Lasts(2 * time.Second)})

s.trapdoor = rule.Role("trapdoor").Obeys(rule.On("open while on", rule.All, func(m *rule.Moment[cell.Now]) rule.Step {
	return m.WhileWire(on, m.Keep(open))
}))
s.plate = rule.Role("plate").Obeys(rule.On("press", rule.All, func(m *rule.Moment[cell.Now]) rule.Step {
	return m.If(cell.Now.Stood, m.OnWire(m.Apply(on)))
}))
s.west, s.east = s.world.Wire("west"), s.world.Wire("east")
s.player.Bind(s.west.Key(on, control.KeyPress{Key: control.Key1}, "Pull the west lever"))
return ctx.Hook(mortal, hasty, s.trapdoor, s.plate)

// Spawn: every cell plays its role on its wire
s.board.Seed(board.Layout{Default: "grass", Cells: []cell.Entry{
	{Kind: "boards", Cell: w1, Roles: []*rule.Part{s.trapdoor}, Wired: s.west}, // the west strip, cell by cell
	{Kind: "boards", Cell: e1, Roles: []*rule.Part{s.trapdoor}, Wired: s.east}, // the east strip
	{Kind: "plate", Cell: p, Roles: []*rule.Part{s.plate}, Wired: s.east},
}})
```

1 puts `on` on the west wire and the west strip opens for two seconds; the plate puts it on the
east wire every step someone stands on it, so the east strip stays open while someone stands there
and two seconds after. A mortal on an open trapdoor falls in. The wire demo
(`examples/wire-demo/demo.go`) is the whole program, with a third wire: G flips a gate's switch, an
effect without `Lasts`, and the gate stays open until G again. The west lever stands in the yard
too, a cell playing lever wired to west, and U has a selected scout beside it pull it (the handy
role, `Playing`). An entity is wired to one wire.

### Groups and behaviours

**Roles mean behaviour, wires mean which group.** `on` serves the lever and the plate alike, `open`
every trapdoor, and one rule of each role every cell playing it: a lever costs a wire, an entity —
never a tag, a role or an effect of its own. A hundred levers are a hundred wires and the same
rules. Before wires, a lever was an effect of its own, its trapdoors a tag of places and a rule
hooked for each: 63 effects and 64 tags in a family held a game to tens of levers. Tags stay for
the groups rules filter by — whose a unit is, whether it is selected, a district (`rule.Self`,
`rule.Between`) — and roles for behaviours: tens in a game, never one a unit or a lever.

## Knobs

A plugin gives a game **knobs**: components it only reads, which an effect's `Alter` turns and
puts back when it ends — a cell's `cell.Ground`, `steering.Steering` (how fast an entity goes,
speeds up, brakes; `Halted` holds it), `vision.Sight` (how far, which way; `Ahead` looks the way
it moves), `collision.Physics`, `world.Appearance`. What a plugin's system writes lives beside
the knob in a component of its own — `steering.Course` beside `Steering`, `vision.Sighted` beside
`Sight` — given where it is missing: an `Alter` ending puts back the original, and would put back
stale state with it. A new knob goes into the component a plugin reads, never into its state.

## No Go code in a rule

A rule holds no Go code of its own but the conditions of `If`. What a plugin does of its own is
its own work in a pass it makes anyway, never a rule hooked on another plugin (2026-10-02):

- the ground's pace: the board's pass over the units writes each one's `steering.Pace` — the cost
  and the slope of the cell under it — and the world's velocity pass multiplies the speed by it,
  from the next step;
- navigation's bumps: its pass reads the `collision.Collider` contacts of every unit under orders;
- contacts counted and logged, sightings and falls logged: `collision.Plugin.WithStats`,
  `WithLog`, `vision.Plugin.WithLog`, `board.Plugin.WithLog`;
- the weather on the board: the atmosphere's own `Weathering.System`, once a second of game time.

Steering is commands an entity gives itself: `steering.Away{}` and `steering.Toward{}`, aimed at
the moment's subject — a `Sighting`'s nearest seen; an aimed command fails while the moment
names nobody — and `steering.Turn{Angle}`. `vhooks.Flee`, `Chase` and `Search` are written so:

```go
rule.On("vision.chase", rule.Between(tags.Predator, tags.Prey), func(m *rule.Moment[vision.Sighting]) rule.Step {
	return m.Order(steering.Toward{})
})
```

A switch of a behaviour for the whole game is an effect on the world (`world.Apply`,
`world.Dispel`), the rule running `During` it: `vhooks.Flee(tags, fleeing)`.

How an entity is drawn is the one place rules are Go: `render.Over`, `As`, `With` and `Show`,
given to `world.Plugin.Draw` (and `vision.Plugin.Draw`, which views are drawn), run every frame.
They read a component and decide nothing in the game.

## Dispel and order

Many rules may touch one effect, from several plugins. What happens is fixed:

- A rule that `Keep`s an effect someone `Dispel`led has it back the step after: its cause goes on,
  so the effect does. A dispeller that is to win casts a shield the keeper checks —
  `m.Steps(m.Apply(doused), m.Dispel(burning))` against `m.Unless(doused, m.Keep(burning))`.
- A plan's `Keep` gives way when someone else takes its effect off: the branch fails and the plan
  goes on to what it does next.
- A cast after a `Dispel` in the same step takes the slot back.
- In a step, rules of one moment run in the order they were hooked; moments, in the order of
  their plugins' passes — that is the systems' order, not something a rule picks. Every cast and
  `Dispel` of a step lands together in the effects' pass at its end.

## Conversation and guards

`Ask` → `Asked` on the other unit a tick later → `Agree`, `Refuse` or `Relay` → `Replied` back;
delivery a tick later means no recursion within a tick. Every conversation ends: an ask reaches
only a unit with a mind; unanswered asks and answers are dropped after `AskLife`; a relay chain
holds at most `MaxChain` links and never returns to anyone on it; navigation's stall counter still
gives up an order that makes no headway, whatever the rules and the plans do.

## Writing a new behaviour

1. A moment a plugin already catches, instant: a rule on it; its memory, an effect; what it
   changes, a knob the effect alters — a component the plugin only reads, added where missing.
2. Something that lasts: name the facts the plugin can perceive and write them for units with a
   `Mind` only; name the commands and handle them in the plugin (a `CommandHandler`'s queue),
   carrying them out for the entity that gives them, the engine's own rules inside; tell what came
   of them as facts.
3. Write the plan from short named branches and give it to the kinds. A plan holds at most
   `MaxSteps` (128) steps.
4. Keep the defaults a plugin hooks itself unexported; a game adds its own with `ctx.Hook`, a
   behaviour several kinds share as a role.
5. Test the scenario as ticks: who is where, which facts, which commands, until it ends.

Example — navigation's crowd, the rules of units among others, as in StarCraft II. Navigation
perceives — the moment `Touch`, two units touching, with what each is doing and whether the one
standing has room — and carries out the commands, keeping its own rules inside them (a unit is
never stepped into water, off a cliff or into a wall); the rules decide:

```go
func makeWay() rule.Rule {
	return rule.On("navigation.make way", rule.All, func(m *rule.Moment[Touch]) rule.Step {
		return m.If(Touch.PushedByAlly, m.Order(StepAside{}))
	})
}

func joinTheGroup() rule.Rule {
	return rule.On("navigation.join the group", rule.All, func(m *rule.Moment[Touch]) rule.Step {
		return m.If(Touch.ReachedTheGroup, m.Order(Stop{}))
	})
}

func goRound() rule.Rule {
	return rule.On("navigation.go round", rule.All, func(m *rule.Moment[Touch]) rule.Step {
		return m.OneOf(
			m.If(Touch.GoalTaken, m.Order(Settle{})),
			m.If(Touch.WaitsFirst, m.Order(Hold{})),
			m.If(Touch.NoWayRound, m.Order(StepAside{})),
			m.If(Touch.ClearsTheWay, m.Order(Pass{})),
			m.If(Touch.Blocks, m.Order(Detour{})),
		)
	})
}
```

`StepAside`, `Pass`, `Detour` and `Settle` are `Aimed`: each is told the moment's subject, the
unit touched. A game adds its own rules through `ctx.Hook` — a role's for the units playing it,
`rule.Role("guard").Obeys(rule.On("hold the line", rule.All, …))` — or gives its own set in place
of the crowd's with `WithCrowd`.

## A game: states as effects

A game built on the plugins is written in three parts, and none of them is Go code inside a rule:

1. **States are effects.** Burning, frozen, alarmed, doused, a lever pulled: each defined once,
   for a while or for good, with its own marker on while it runs, on a unit, a cell, the world or
   a wire. One effect may lead to the next (`Then`).
2. **Rules connect.** A rule hooked on a plugin turns its moment — a touch, a sighting, where one
   stands — into effects: `Apply`, `Keep`, `Dispel`, narrowed by a marker (`rule.Self`,
   `rule.Between`) or obeyed by a role, guarded by another (`Unless`), now and then (`Chance`).
3. **Plugins give knobs.** What an effect changes is a component a plugin reads — `Steering`,
   `Sight`, a cell's `Ground`, `collision.Physics`, `Appearance` — turned by the effect's `Alter`
   and put back when it ends. A plugin's own effects stay private; a game reaches it through its
   knobs and moments.

Example — fire, across collision, the board and the world, on units and on the ground alike:

```go
fx := w.Effects()
smouldering := fx.Define("smouldering", effect.Spec{effect.Lasts(20 * time.Second)})
burning := fx.Define("burning", effect.Spec{
	effect.Lasts(8 * time.Second),
	effect.Then(smouldering),                                    // burnt out, it smoulders
	effect.Alter(func(a *world.Appearance) { a.SpriteID = flames }), // a unit drawn burning,
	effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 1.5 }),  // running about;
	effect.Alter(func(g *cell.Ground) { g.Kind = embers }),         // a cell's ground aflame
})
doused := fx.Define("doused", effect.Spec{effect.Lasts(10 * time.Second)})

ctx.Hook(
	// fire spreads to whom a burning one touches, now and then, unless they are wet
	rule.On("fire spreads", rule.Between(burning.Mark(), tag.Any),
		func(m *rule.Moment[collision.Meeting]) rule.Step {
			return m.ForOther(m.Unless(doused, m.Chance(0.3, m.Apply(burning))))
		}),
	// water puts a burning one out — nothing smoulders then — and the wet do not catch fire for a
	// while; one on dry ground sets it alight now and then
	rule.On("where it burns", rule.Self(burning.Mark()), func(m *rule.Moment[unit.Standing]) rule.Step {
		return m.OneOf(
			m.If(inWater, m.Steps(m.Apply(doused), m.Dispel(burning))),
			m.Here(m.Chance(0.1, m.Apply(burning))),
		)
	}),
	// burning ground sets the cells round it alight now and then — not one burning or burnt out
	rule.On("fire spreads over the ground", rule.Self(burning.Mark()), func(m *rule.Moment[cell.Now]) rule.Step {
		return m.Around(1, m.Unless(burning, m.Unless(smouldering, m.Chance(0.05, m.Apply(burning)))))
	}),
)

func inWater(s unit.Standing) bool { return s.Kind.Admits(cell.Water) }
```

`ctx.Hook` hands the first rule to collision and the other two to the board: the Stage names no
plugin. One effect serves units and cells: an `Alter` of a component the entity does not carry is
passed over. `Around` takes in the places stood on too — a cell itself — so the spreading rule
keeps off what burns already, or a cell would keep itself burning for ever.

Another plugin, added later, adds its own rules for fire — vision's units flee from
`rule.Between(tag.Any, burning.Mark())` — without touching these: the marker is the common word.

Where a rule cannot say what the game needs, a moment, a step or a knob is missing; it is added
to the plugin, not written as code in a rule. Fire showed three, added on 2026-10-01: the steps
`Here` and `Around`, a board moment of a cell (`cell.Now`), and the clock's moment as its own
entity's, so that a clock rule casts a phase.

## Later

The same words serve formations and escorts ("follow me"), handing over a load, combat ("cover
me", "fall back"), and an AI player's orders to its units; asks and commands are deterministic
messages, fit to replay and to send over a network.

Not yet there: which effects a player may apply (any, today, on its own units, a wire or the
world — a game over a network will want a list); an effect taking a tag off while it runs (a
`Revoke` beside `Grant`); filters joined (`rule.Self(x)` and `Having[T]` at once); `Here` and
`Around` in plans; a step shared by the moments of several plugins; more than `MaxEffects` (8)
effects on one entity and 63 defined; an entity wired to more than one wire.
