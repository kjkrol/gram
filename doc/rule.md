# Rule: how entities behave

One vocabulary for every behaviour of an entity, in `plugins/world/rule`: a **rule** is what is
done at a moment a plugin catches, a **plan** is what an entity does over time, an **effect** is a
change that holds, a **command** is what an entity has done, and a **fact** is what a plugin tells
an entity. Written on 2026-09-30 when yielding and avoiding outgrew the reflexes, and unified the
same day: behaviors, effects and trees had grown three ways of saying the same thing. The package
was `conduct`, then `act` (rules were "triggers", plans "trees"); on 2026-10-01 it took the names
it has: two constructors, each taking a function that writes the steps.

## Two constructors

A rule and a plan are each written by a function. The constructor hands it the one the steps are
for — the **Moment** a rule fires at, the **Actor** a plan is for — and their methods make the
steps it returns, all of one type, `rule.Step`. Go 1.27's methods with type parameters make
`a.Order(cmd)`, `a.When[Blocked](…)` and `.Until[Arrived]()` possible.

```go
// a rule: at a moment, for whom, what to do
rule.On("in the ice", rule.All, func(m *rule.Moment[board.Standing]) rule.Step {
	return m.OneOf(
		m.If(caughtInIce, m.Keep(frozen)),
		m.If(board.Standing.Fallen, m.Order(world.Despawn{})),
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
- `rule.Self(tag)` — an entity carrying the tag;
- `rule.Between(a, b)` — a pair whose entity carries `a` and whose other carries `b` (`tag.Any`
  for either side), for a moment that is `rule.Met`: a collision's `Meeting`, a `Sighting`, a
  navigation `Touch`;
- `rule.Having[T]()` — an entity carrying the component `T`, which `m.CallOn` hands its function.

The plugin checks the filter on the tag bits of each entity, so a rule that does not concern one
costs nothing for it.

## The five words

- **Rule** — a moment a plugin catches: a `board.Standing`, a `vision.Sighting`, a
  `collision.Meeting` (pairs) or `Struck`, a `world.Moving`, an `effect.Idling`, a `clock.Moment`,
  a `navigation.Touch`. A Moment's steps are instant — `OneOf`, `Steps`, `If` on the moment, `Not`,
  `Apply`, `Keep`, `Unless`, `Under`, `Order`, `ForOther`, `Call`, `CallOn` — so a step that lasts
  (`Wait`, `Until`, `Ask`) is not to be had in a rule: a Moment has no such method, and one made by
  an Actor is refused as the rule is made. A rule remembers nothing of its own.
- **Plan** — what a kind's entities do over time. It remembers its place (`rule.Mind`: the steps
  running, each one's place and start on the world's clock), waits (`Wait`, `Until`), talks to
  other entities (`Ask`), and is saved with the game. `OneOf` is a reactive choice, `Steps` a
  sequence with memory; the Actor's `When[F]` and `On[F]` open a branch on a fact, `If` reads one.
- **Effect** — `effect.Effect`, defined once from a `Spec` (`Grant`, `Alter`, `Lasts`,
  `Stacking`). `Apply` casts it; `Keep` holds it as long as its branch runs, or as long as a rule
  keeps firing it. **An effect's presence is state**: `Unless(alarmed, …)` is "at most once a
  while", a memory for rules that keep none. Collision's hit is one (`hooks.Hit`, cast by
  `ShowHits`, drawn by `HitOverlay`).
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
- **A moment of no entity.** A `clock.Moment` belongs to nobody: `Apply`, `Keep`, `Order` and the
  rest fail on it, so a clock's rule never acts on entity 0.

## Conversation and guards

`Ask` → `Asked` on the other unit a tick later → `Agree`, `Refuse` or `Relay` → `Replied` back;
delivery a tick later means no recursion within a tick. Every conversation ends: an ask reaches
only a unit with a mind; unanswered asks and answers are dropped after `AskLife`; a relay chain
holds at most `MaxChain` links and never returns to anyone on it; navigation's stall counter still
gives up an order that makes no headway, whatever the rules and the plans do.

## Writing a new behaviour

1. A moment a plugin already catches, instant: a rule on it; its memory, an effect.
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

## Later

The same words serve formations and escorts ("follow me"), handing over a load, combat ("cover
me", "fall back"), and an AI player's orders to its units; asks and commands are deterministic
messages, fit to replay and to send over a network.
