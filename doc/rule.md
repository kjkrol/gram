# Rule: how entities behave

One vocabulary for every behaviour of an entity, in `rule`: a **rule** is what is
done at a moment a plugin catches, a **plan** is what an entity does over time, an **effect** is a
change that holds, a **command** is what an entity has done, and a **fact** is what a plugin tells
an entity. Written on 2026-09-30 when yielding and avoiding outgrew the reflexes, and unified the
same day: behaviors, effects and trees had grown three ways of saying the same thing. The package
was `conduct`, then `act` (rules were "triggers", plans "trees"); on 2026-10-01 it took the names
it has: two constructors, each taking a function that writes the steps. The same day effects got
their own markers, `Dispel`, `Then` and `Chance`, so that a game is written as states and the
rules connecting them (below, "A game: states as effects").

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
rule.Plan("patrol", func(a *rule.Actor) rule.Step {
	return a.Steps(
		a.Order(navigation.MoveTo{Cell: east}).Until[navigation.Arrived](),
		a.Wait(10 * time.Second),
		a.Order(navigation.MoveTo{Cell: west}).Until[navigation.Arrived](),
	)
})
```

- `rule.On(name, filter, func(m *rule.Moment[P]) rule.Step)` is a `plugin.Rule` for the `Hook` of
  the plugin that catches `P`.
- `rule.Plan(name, func(a *rule.Actor) rule.Step)` is the component a kind gives its entities:
  `units.Define("unit", …, patrol)`. Its name is what a save knows it by.
- A function writing part of a rule or a plan takes the Moment or the Actor as its own:
  `func whenBlocked(a *rule.Actor) rule.Step`. Ready-made rules come whole, in a plugin's `hooks`
  package: `collision.Hook(chooks.CountContacts(&stats))`.

## Filters

The second argument of `rule.On` says whom the rule fires for, read before its steps run:

- `rule.All` — every entity the plugin shows it, every pair for a moment of two;
- `rule.Self(tag)` — an entity carrying the tag, an effect's marker among them:
  `rule.Self(frozen.Mark())`; a cell carries the game's tags of places (`cell.Tag`, of the family
  `cell.Family`, given in the board's `Layout`) — `rule.Self(trapdoor)` on a `cell.Now`;
- `rule.Between(a, b)` — a pair whose entity carries `a` and whose other carries `b` (`tag.Any`
  for either side), for a moment that is `rule.Met`: a collision's `Meeting`, a `Sighting`, a
  navigation `Touch`;
- `rule.Having[T]()` — an entity carrying the component `T`: `rule.Having[witch]()`.

The plugin checks the filter on the tag bits of each entity, so a rule that does not concern one
costs nothing for it.

## The five words

- **Rule** — a moment a plugin catches: a `unit.Standing` or a `cell.Now`, a
  `vision.Sighting`, a `collision.Meeting` (pairs) or `Struck`, a `world.Moving` or `Drawing`, a
  `clock.Moment`, a `navigation.Touch`. A Moment's steps are instant — `OneOf`, `Steps`, `If` on
  the moment, `Not`, `Apply`, `Keep`, `Dispel`, `Chance`, `Unless`, `Under`, `During`, `Order`,
  `ForOther`, `Here`, `Around` — so a step that lasts (`Wait`, `Until`, `Ask`) is not to be had in a rule: a
  Moment has no such method, and one made by an Actor is refused as the rule is made. A rule
  remembers nothing of its own, and runs no Go code of its own: there is no step for it. What
  the steps cannot say is a moment, a step or a knob the plugin still lacks.
- **Plan** — what a kind's entities do over time. It remembers its place (`rule.Mind`: the steps
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
  (`control.Issued.ByEntity`). The world keeps a stage's one carrier (`control.Carrier`,
  `world.Plugin.Commands`): the players give it theirs, the entities theirs; the engine carries
  every `plugin.CommandHandler` a stage uses, and a plugin's `plugin.Tick` hands the carrier to its
  rules. Nothing is dropped: a command waits for its handler's pass — given after it, for the next
  frame's. A command that is `rule.Aimed` is told the subject of the fact it stands under, or of
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
  `clock.Clock.In` reads.
- **Where one stands.** On a moment that is `rule.Placed` — a `unit.Standing`, a cell's
  `cell.Now` — `m.Here(step)` runs the step on the cells under the entity (for a cell, on
  itself) and `m.Around(rings, step)` on those and the rings of neighbours round them, each cell
  once, on square and hex boards alike: an effect applied to the ground.
- **Now and then.** `m.Chance(p, step)` runs the step with likelihood `p`, drawn afresh at every
  step of the game from the world's seed (`world.Config.Seed`), the game time and the entity — no
  state kept, so a load and a replay draw alike.
- **One after another.** `effect.Then(next)` casts `next` when an effect's time is up — burning
  leaves smouldering — but not when it is dispelled: put out, nothing smoulders.

## A player's actions

A player changes the game the same way a rule does: by putting an effect on something. Two
commands carry it, given from a binding like any other:

- `selection.Apply{Effect}` puts it on the player's own selected units — an ability, a sprint, a
  spell; another player's units and those nobody owns are never touched;
- `world.Apply{Effect}` puts it on the world itself — its own entity, the clock's — a state of the
  whole game: a lever pulled, an alarm, night called. A rule or a plan may `Order` it too.

Rules and plans read the world's states with `During(e, step)`, as they read an entity's with
`Under`. Example — the trapdoor demo, all of it the game's own: many levers, each with its
trapdoors, every lever a state of the game, every group of trapdoors a tag of places, and a rule
a pair:

```go
open := fx.Define("open", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind = pit })})

wire := func(name string, key control.Key) {
	pulled := fx.Define("lever "+name, effect.Spec{effect.Lasts(2 * time.Second)})
	trapdoor := kinds.DefineTag[cell.Family]("trapdoor " + name) // given to cells in the Layout
	brd.Hook(rule.On("trapdoors "+name, rule.Self(trapdoor), func(m *rule.Moment[cell.Now]) rule.Step {
		return m.During(pulled, m.Keep(open))
	}))
	player.Bind(control.Command(control.KeyPress{Key: key}, "Pull the "+name+" lever",
		func(control.Context) (world.Apply, bool) { return world.Apply{Effect: pulled}, true }))
}
wire("west", control.Key1)
wire("east", control.Key2)

// whoever stands where nothing holds it falls in
brd.Hook(rule.On("fall in", rule.All, func(m *rule.Moment[unit.Standing]) rule.Step {
	return m.If(unit.Standing.Fallen, m.Order(world.Despawn{}))
}))
```

`open` is one for every trapdoor — open means the same for each; the levers are apart, a state
each, and the trapdoors apart by their tags. A trapdoor of two levers carries both tags. A game
has at most 63 effects and 64 tags in a family: tens of levers, not hundreds — more would want a
lever as an entity and what listens to it as data, not yet there.

A lever pulled where it stands is a pressure plate (the pressure plate demo): a cell tagged as the
plate, and whoever stands on it presses it — the game's state, on the world — every step it
stands there. The unit's `Standing` tells the tags of the place under it, and a rule may give the
world's command too:

```go
rule.On("plate "+name, rule.All, func(m *rule.Moment[unit.Standing]) rule.Step {
	return m.If(func(st unit.Standing) bool { return st.Places.Has(plate) }, m.Order(world.Apply{Effect: pressed}))
})
```

`pressed` lasts a second, cast afresh every step someone stands on the plate: the trapdoors stay
open while someone stands there and a second after.

## Knobs

A plugin gives a game **knobs**: components it only reads, which an effect's `Alter` turns and
puts back when it ends — a cell's `cell.Ground`, `steering.Steering` (how fast an entity goes,
speeds up, brakes; `Halted` holds it), `vision.Sight` (how far, which way; `Ahead` looks the way
it moves), `collision.Physics`, `world.Appearance`. What a plugin's system writes lives beside
the knob in a component of its own — `steering.Course` beside `Steering`, `vision.Sighted` beside
`Sight` — given where it is missing: an `Alter` ending puts back the original, and would put back
stale state with it. A new knob goes into the component a plugin reads, never into its state.

## Hooks of a plugin's own

A plugin's own reactions inside another's pass — the board's terrain speed in the world's
`Moving`, navigation's bumps in collision's `Struck` — and the ready-made hooks of a `hooks`
package (`chooks.CountContacts`, `bhooks.LogFalls`, `vhooks.Chase`) are library code, written
straight on `plugin/host` (`host.Each`, `host.Every`, `host.Pair`) and hooked the same way. A game
writes rules.

## Dispel and order

Many rules may touch one effect, from several plugins. What happens is fixed:

- A rule that `Keep`s an effect someone `Dispel`led has it back the step after: its cause goes on,
  so the effect does. A dispeller that is to win casts a shield the keeper checks —
  `m.Steps(m.Apply(doused), m.Dispel(burning))` against `m.Unless(doused, m.Keep(burning))`.
- A plan's `Keep` gives way when someone else takes its effect off: the branch fails and the plan
  goes on to what it does next.
- A cast after a `Dispel` in the same step takes the slot back.
- Within a step, rules of one moment run in the order they were hooked; moments, in the order of
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
4. Keep the defaults a plugin hooks itself unexported; a game adds its own with `Hook`.
5. Test the scenario as ticks: who is where, which facts, which commands, until it ends.

Example — navigation's crowd, the rules of units among others, as in StarCraft II. Navigation
perceives — the moment `Touch`, two units touching, with what each is doing and whether the one
standing has room — and carries out the commands, keeping its own rules inside them (a unit is
never stepped into water, off a cliff or into a wall); the rules decide:

```go
func makeWay() plugin.Rule {
	return rule.On("navigation.make way", rule.All, func(m *rule.Moment[Touch]) rule.Step {
		return m.If(Touch.PushedByAlly, m.Order(StepAside{}))
	})
}

func joinTheGroup() plugin.Rule {
	return rule.On("navigation.join the group", rule.All, func(m *rule.Moment[Touch]) rule.Step {
		return m.If(Touch.ReachedTheGroup, m.Order(Stop{}))
	})
}

func goRound() plugin.Rule {
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
unit touched. A game adds its own rules with `navigation.Plugin.Hook`, narrowed by tags —
`rule.On("guards hold", rule.Self(guard), …)` — or gives its own set in place of the crowd's with
`WithCrowd`.

## A game: states as effects

A game built on the plugins is written in three parts, and none of them is Go code inside a rule:

1. **States are effects.** Burning, frozen, alarmed, doused: each defined once, for a while or
   for good, with its own marker on while it runs. One effect may lead to the next (`Then`).
2. **Rules connect.** A rule hooked on a plugin turns its moment — a touch, a sighting, where one
   stands — into effects: `Apply`, `Keep`, `Dispel`, narrowed by a marker (`rule.Self`,
   `rule.Between`), guarded by another (`Unless`), now and then (`Chance`).
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

// fire spreads to whom a burning one touches, now and then, unless they are wet
coll.Hook(rule.On("fire spreads", rule.Between(burning.Mark(), tag.Any),
	func(m *rule.Moment[collision.Meeting]) rule.Step {
		return m.ForOther(m.Unless(doused, m.Chance(0.3, m.Apply(burning))))
	}))

brd.Hook(
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

One effect serves units and cells: an `Alter` of a component the entity does not carry is passed
over. `Around` takes in the places stood on too — a cell itself — so the spreading rule keeps off
what burns already, or a cell would keep itself burning for ever.

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

Not yet there: which effects a player may apply (any, today, on its own units or the world — a
game over a network will want a list); an effect taking a tag off while it runs (a `Revoke` beside `Grant`); filters
joined (`rule.Self(x)` and `Having[T]` at once); `Here` and `Around` in plans; a step shared by
the moments of several plugins; more than `MaxEffects` (8) effects on one entity and 63 defined.
