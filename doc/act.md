# Act: how entities behave

One vocabulary for every behaviour of an entity, in `plugins/world/act`: a **trigger** is a moment
a plugin catches and what is done then, a **tree** is how a kind deliberates over time, an
**effect** is a change that holds, a **command** is what an entity has done, and a **fact** is what
a plugin tells an entity. First used by navigation's `Courteous`, written on 2026-09-30 when
yielding and avoiding outgrew the reflexes, and unified the same day: behaviors, effects and trees
had grown three ways of saying the same thing. The package was called `conduct` until it moved to
builders with methods.

## Builders

Nodes are made by the methods of a builder, not by package functions: a `Branch` for a tree, a
`Reaction` for a trigger. The constructor names and opens it, the methods make the nodes, and `Do`
closes it round its body. Only the root carries a name; inner `First` and `Then` need none. Go
1.27's methods with type parameters make `c.Issue(cmd)`, `c.Ask[MakeWay](…)` and
`c.Until[Arrived]()` possible.

- `act.When[F](name)`, `act.On[F](name)`, `act.Named(name)` — a tree's branch: while the entity
  carries the fact `F`, from the tick it comes until the body is done, or just so (a root).
- `act.Trigger[P](name)` — a trigger of the moment `P`, narrowed with `.Self(tag)`, `.Other(tag)`,
  `.Having[T]()`; `.Do(body)` is the `plugin.Trigger` for the host's `Hook`, `.Runs(fn)` and
  `.RunsOn(fn)` one of a function alone.
- A function handing back part of a body may use a builder of its own; the zero `act.Branch` or
  `act.Reaction[P]` will do (`collision/trigger`'s `CountContacts` does).

## The five words

- **Trigger** — a moment the host catches: a `board.Standing`, a `vision.Sighting`, a
  `collision.Meeting` (pairs) or `Struck`, a `world.Moving`, an `effect.Idling`, a `clock.Moment`.
  A `Reaction` makes `Instant` nodes alone — `First`, `Then`, `If` on the moment, `Unless`,
  `IfUnder`, `Apply`, `While`, `Issue`, `ToOther`, `Run`, `RunOn` — so a node that lasts (`Wait`,
  `Until`, `Ask`) is not to be had in a trigger: the compiler refuses it. A trigger remembers
  nothing of its own.
- **Tree** — `act.Tree(root)`, the component a kind gives its entities. It lasts over ticks,
  remembers its place (`act.Mind`: the running nodes, each one's step and start on the world's
  clock), waits (`Wait`, `Until`), talks to other entities (`Ask`), and is saved with the game.
  `First` is a reactive selector, `Then` a sequence with memory; `When`, `On` and `If` read facts.
- **Effect** — `effect.Effect`, defined once from a `Spec` (`Grant`, `Alter`, `Lasts`,
  `Stacking`). `Apply` casts it; `While` holds it as long as its branch runs, or as long as a
  trigger keeps firing it. **An effect's presence is state**: `Unless(alarmed, …)` is "at most
  once a while", a memory for triggers that keep none. Collision's hit is one
  (`trigger.Hit`, cast by `ShowHits`, drawn by `HitOverlay`).
- **Command** — `Issue(cmd)` gives the entity's command, the same one a player gives
  (`navigation.MoveTo`, `world.Despawn`), queued for the plugin that handles its type, and goes
  on at once: **fire and forget**. The handler carries it out for the entity alone
  (`control.Issued.ByEntity`). The world keeps a stage's one carrier (`control.Carrier`,
  `world.Plugin.Commands`): the players give it theirs, the entities theirs; the engine carries
  every `plugin.CommandHandler` a stage uses, and a host's `plugin.Tick` hands the carrier to its
  triggers. Nothing is dropped: a command waits for its handler's pass — given after it, for the
  next frame's.
- **Fact** — a component a plugin writes for entities with a `Mind` alone (navigation's
  `Blocked`, `Room`, `Arrived`), so entities without a tree pay nothing. Facts are how a tree
  learns what came of its commands.

```go
// definitions: a stateful change, one place
frozen := fx.Define("frozen", effect.Spec{effect.Grant(frozenTag), effect.Alter(func(p *collision.Physics) { p.Mass = math.Inf(1) })})

// triggers: moments the hosts catch, pairs too; instant nodes only
t := act.Trigger[board.Standing]("in the ice")
board.Hook(t.Do(t.First(
	t.If(caughtInIce, t.While(frozen)),
	t.If(board.Standing.Fallen, t.Issue(world.Despawn{})),
)))
world.Hook(act.Trigger[world.Moving]("frozen fast").Self(frozenTag).Runs(stop))

// trees: a kind's deliberation over time; Issue gives a command and goes on
c := act.Named("patrol")
patrol := c.Do(c.Then(
	c.Issue(navigation.MoveTo{Cell: a}).Until[navigation.Arrived](),
	c.Wait(10*time.Second),
	c.Issue(navigation.MoveTo{Cell: b}).Until[navigation.Arrived](),
))
```

## Idioms

- **Issue, then wait for what comes of it.** A tree's `Issue` hands back a `Command`: its
  `Until[F]()` waits for a fact to come *afresh* — a fact left from before does not count — and
  `Until(pred)` for one to come to hold it. `c.Until(pred)` alone waits without a command.
- **In a reactive branch, give the command once.** A branch of `First` that issues and succeeds
  is run again at the next tick, and would issue again: end it with `.Until(…)` or `.Stay()` so the
  branch stays while it holds, and put the reaction to the outcome *earlier* in the `First` —
  `If(Blocked.NoWayRound, stepAside)` before `goRound` — so it takes over when the fact changes.
- **A branch is a value.** `hold := c.Issue(Hold{}).Until(Blocked.WaitedLong)` may stand in
  several places; each is laid out on its own.
- **A command's outcome is a fact.** Fire and forget has no success or failure; navigation says
  what came of a `Detour` with `Blocked.Cornered` and of a `Hold` with `Blocked.WaitedOut`.
- **A trigger's memory is an effect.** `t.Unless(e, t.Then(t.Apply(e), …))` runs its body at most
  once for as long as `e` lasts.
- **A moment of no entity.** A `clock.Moment` belongs to nobody: `Apply`, `While`, `Issue` and the
  rest fail on it, so a clock's trigger never acts on entity 0.

## Conversation and guards

`Ask` → `Asked` on the other unit a tick later → `Agree`, `Refuse` or `Relay` → `Replied` back;
delivery a tick later means no recursion within a tick. Every conversation ends: an ask reaches
only a unit with a mind; unanswered asks and answers are dropped after `AskLife`; a relay chain
holds at most `MaxChain` links and never returns to anyone on it; a swap of goals must strictly
shorten both ways, so swaps cannot go round in circles; navigation's stall counter still gives up
an order that makes no headway, whatever the tree does. Of two units on the move, the one with the
lower id waits and the other goes round, the other way round when the same two meet again, so
strangers never reroute into each other for ever.

## Writing a new behaviour

1. A moment a plugin already catches, instant: a trigger on it; its memory, an effect.
2. Something that lasts: name the facts the plugin can perceive and write them for units with a
   `Mind` only; name the commands and handle them in the plugin (a `CommandHandler`'s queue),
   carrying them out for the entity that gives them; tell what came of them as facts.
3. Compose the tree from short named branches and give it to the kinds with `act.Tree`. A tree
   holds at most `MaxNodes` (128) nodes.
4. Test the scenario as ticks: who is where, which facts, which commands, until it ends.

Example — navigation's `WhenBlocked`:

```go
func WhenBlocked() act.Node {
	c := act.When[Blocked]("when blocked")
	hold := c.Issue(Hold{}).Until(Blocked.WaitedLong)
	goRound := c.Issue(Detour{}).Until(Blocked.NoWayRound)
	return c.Do(c.First(
		c.If(Blocked.WhileGivingWay, c.First(c.If(Blocked.WaitedLong, goRound), hold)),
		c.If(Blocked.NoWayRound, c.Issue(StepAside{Return: true}).Stay()),
		c.If(Blocked.WaitedLong, goRound),
		c.If(Blocked.SwapShortens, c.Ask[SwapGoals]("I'll stand on yours", time.Second, c.Issue(SwapGoals{}).Stay())),
		c.If(Blocked.IdleAllyOnMyGoal, c.Ask[FreeGoal]("you cover my goal", time.Second, hold, c.Issue(Settle{}).Stay())),
		c.If(Blocked.IdleAlly, c.Ask[MakeWay]("you close my way", time.Second, hold, goRound)),
		c.If(Blocked.StrangerOnMyGoal, c.Issue(Settle{}).Stay()),
		c.If(Blocked.GivesWayFirst, hold),
		goRound,
	))
}
```

## Later

The same words serve formations and escorts ("follow me"), handing over a load, combat ("cover
me", "fall back"), and an AI player's orders to its units; asks and commands are deterministic
messages, fit to replay and to send over a network.
