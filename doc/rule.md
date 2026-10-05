# Rule: how entities behave

One vocabulary for every behaviour of an entity, in `rule`: a **rule** is what is
done at a moment a plugin catches, a **plan** is what an entity does over time, an **effect** is a
change that holds, a **command** is what an entity has done, and a **fact** is what a plugin tells
an entity. Written on 2026-09-30 when yielding and avoiding outgrew the reflexes, and unified the
same day: behaviors, effects and trees had grown three ways of saying the same thing. The package
was `conduct`, then `act` (rules were "triggers", plans "trees"); on 2026-10-01 it took the names
it has: two constructors, each taking a function that writes the steps. The same day effects got
their own markers, `Dispel`, `Then` and `Chance`, so that a game is written as states and the
rules connecting them (below, "A game: states as effects"). On 2026-10-02 entities came to obey rules by
the roles they play; on 2026-10-05 what somebody asks for became one command written as a
sentence, for entities found by name or group, and hooking went: nobody hands a rule to a plugin,
the engine gives the rules of the roles played to the plugins catching their moments (below, "Who
gets a rule", "Roles", "Commands").

## Two constructors

A rule is its steps, made by the package's functions; a plan is written by a function handed the
**Actor** it is for, whose methods make its steps. All are of one type, `rule.Step`. Go 1.27's
methods with type parameters make `a.Order(cmd)`, `a.When[Blocked](…)` and `.Until[Arrived]()`
possible.

```go
// a rule: at a moment, for whom, what to do
rule.Then[unit.Standing]("in the ice", rule.All, rule.OneOf(
		rule.If(caughtInIce, rule.Keep(frozen)),
		rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{})),
	))

// a plan: what a unit does over time
plan.New("patrol", func(a *plan.Actor) rule.Step {
	return a.Steps(
		a.Order(navigation.MoveTo{Cell: east}).Until[navigation.Arrived](),
		a.Wait(10 * time.Second),
		a.Order(navigation.MoveTo{Cell: west}).Until[navigation.Arrived](),
	)
})
```

- `rule.Then[P](name, filter, step)` is a `rule.Rule` for the plugin that catches `P`, obeyed by a
  role (below, "Who gets a rule"): its steps are the package's own functions (`rule.If`, `OneOf`, `Steps`, `Apply`,
  `Keep`, `Dispel`, `Chance`, `Unless`, `Under`, `During`, `Order`, `ForOther`, `Here`, `Around`,
  `Playing`, `Trigger`), its conditions predicates of the moment — a method as a value
  (`unit.Standing.Fallen`), a plugin's own (`unit.On(ice)`), `rule.Not(pred)`; an `If` inside an
  `If` is both, `OneOf` either. A step the moment cannot run (an `Around` where `P` is not
  Placed, an `If` over another moment) panics by the rule's name as the rule is made:

  ```go
  rule.Then[unit.Standing]("fallen in", rule.All,
      rule.If(unit.Standing.Fallen, rule.OneOf(
          rule.If(unit.On(ice), rule.Keep(frozen)),
          rule.Order(world.Despawn{}),
      )))
  ```
- `plan.New(name, func(a *plan.Actor) rule.Step)` is the component a kind gives its entities:
  `units.Define("unit", …, patrol)`. Its name is what a save knows it by.
- A function writing part of a plan takes the Actor as its own (a rule's steps are plain functions):
  `func whenBlocked(a *plan.Actor) rule.Step`. Ready-made rules come whole, in a plugin's `rules`
  package: `rule.Role("body").Obeys(crules.ShowHits(hit))`.

## Filters

The second argument of `rule.Then` says whom the rule fires for, read before its steps run:

- `rule.All` — every entity the plugin shows it, every pair for a moment of two;
- `rule.Self(tag)` — an entity carrying the tag, an effect's marker among them:
  `rule.Self(frozen.Mark())`;
- `rule.Between(a, b)` — a pair whose entity carries `a` and whose other carries `b` (`tag.Any`
  for either side), for a moment that is `plugin.Met`: a collision's `Meeting`, a `Sighting`, a
  navigation `Touch`;
- `rule.Having[T]()` — an entity carrying the component `T`: `rule.Having[witch]()`.

The plugin checks the filter on the tag bits of each entity, so a rule that does not concern one
costs nothing for it. A role narrows a rule further, on top of its filter (below, "Roles").

## Who gets a rule

Nobody hands a rule to a plugin: there is no hooking. A rule is obeyed by a role, and a role is
played — by a kind of unit, by a kind of cell, by a plugin:

```go
mortal := rule.Role("mortal").Obeys(fallIn)     // a rule of unit.Standing: the board's
plate := rule.Role("plate").Obeys(press)        // a rule of cell.Now: the board's
nightly := rule.Role("nightly").Obeys(atDusk)   // a rule of clock.Moment: the world's

s.scout = units.Define("scout", land, profile, rule.Plays(mortal)) // a kind of unit plays it
s.board.Plays("plate", plate)                                      // a kind of cell
s.world.Plays(nightly)                                             // the world itself
```

Once the Stage's `Init` returns, the engine gives every rule of every role somebody plays to the
plugin in use that catches its moment, trying the plugins in the order they were used
(`plugin.Installer.Hosts` is how a plugin says what it catches). A rule no plugin in use catches
fails `Init` with an error wrapping `plugin.ErrUnhosted`, naming the rule by its `String` — its
name, its moment and its role: `"fall in" of unit.Standing, for the role mortal`. A rule of a
moment of the world as a whole (`clock.Moment`, `climate.Weathering`) fires while the plugin
whose moment it is plays the rule's role.

## The five words

- **Rule** — a moment a plugin catches: a `unit.Standing` or a `cell.Now`, a `vision.Sighting`, a
  `collision.Meeting` (pairs) or `Struck`, a `world.Moving` or `Leaving`, a `clock.Moment`, a
  `climate.Weathering`, a `navigation.Touch`. A rule's steps are instant — `OneOf`, `Steps`, `If`
  on the moment, `Not`, `Apply`, `Keep`, `Dispel`, `Chance`, `Unless`, `Under`, `During`,
  `Playing`, `Order`, `Trigger`, `ForOther`, `Here`, `Around` — so a step that lasts (`Wait`, `Until`, `Ask`)
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
  (`crules.Hit`, cast by `ShowHits`, drawn by `HitOverlay` over `rule.Self(hit.Mark())`).
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
- **A rule's memory is an effect.** `rule.Unless(e, rule.Steps(rule.Apply(e), …))` runs its steps at most
  once for as long as `e` lasts.
- **The clock's moment is the clock's.** A `clock.Moment` is of the clock's own entity: an effect
  a clock rule applies lands there — `rule.If(clock.At(dusk), rule.Apply(night))`, a phase that
  `clock.Clock.In` reads. Run once a step, walking no entities, it takes no filter and obeys no role.
- **Where one stands.** On a moment that is `plugin.Placed` — a `unit.Standing`, a cell's
  `cell.Now` — `rule.Here(step)` runs the step on the cells under the entity (for a cell, on
  itself) and `rule.Around(rings, step)` on those and the rings of neighbours round them, each cell
  once, on square and hex boards alike: an effect applied to the ground.
- **Now and then.** `rule.Chance(p, step)` runs the step with likelihood `p`, drawn afresh at every
  step of the game from the world's seed (`world.Config.Seed`), the game time and the entity — no
  state kept, so a load and a replay draw alike.
- **One after another.** `effect.Then(next)` casts `next` when an effect's time is up — burning
  leaves smouldering — but not when it is dispelled: put out, nothing smoulders.

## Commands

Whatever somebody asks for — a player's key, a script, an AI, a rule's `Order` — is a command, and
a command about an effect is one sentence (`rule.Casting`):

```go
openWest := rule.Cast(open).On(entity.Group("west trapdoors")).By(entity.Named("west lever"))
flipGate := rule.Toggle(ajar).On(entity.Group("gate"))
alarm    := rule.Cast(alarmed).On(entity.World).For(30 * time.Second)
hasten   := rule.Cast(haste).On(s.selection.Selected(hasty))
freeze   := rule.Cast(frozen).On(s.selection.Pointed()).For(3 * time.Second)
```

- **The verb**: `rule.Cast` puts the effect on, `rule.Lift` takes it off, `rule.Toggle` takes it
  off them all where any is under it and puts it on them all otherwise — a switch.
- **Whom** (`On`): `entity.Named(names...)`, the entities bearing those names, one each;
  `entity.Group(names...)`, all those in the groups; `entity.World`, the world's own entity — a
  state of the whole game; or a plugin's own: the selection's `Selected(roles...)`, the units the
  player who gives the command has selected (those playing a role named, when any is), and
  `Pointed()`, the entity drawn under the cursor as the key is pressed.
- **How long** (`For`): in place of what the effect's `Spec` says.
- **Who sets it off** (`By`): the entities whose rule's `rule.Trigger()` gives it — a lever, a
  plate. A role then says only *that* a plate stood on triggers; the command says what it opens.

An entity is called by what makes it: a cell by its `cell.Entry{Name, Group}`, a unit by its
`kind.Entry` (`Named`, `InGroup`); a name is one entity's, a group many's, and both are saved. The
game hands its commands to `ctx.Commands(cmds...)`: those with a `By` are kept for `Trigger`, and
every name they say is checked as the game starts — a name nobody bears, or one two bear, stops it
there. A command is given the same way whoever gives it:

```go
s.player.Bind(control.Give(control.KeyPress{Key: control.Key1}, "Pull the west lever", openWest)) // a key
s.players.Issue(ai, openWest)                                                                      // an AI, a script
rule.Order(openWest)                                                                               // a rule, a plan
rule.If(cell.Now.Stood, rule.Trigger())                                                            // its source
```

The plugin its target belongs to carries it out — the world for those named, grouped and the
world itself, the selection for its own — a step later, with the step's effects; commands of one
effect for one entity in one step are netted, so a switch flipped twice stays as it was.
Other plugins' commands are theirs: `navigation.MoveTo`, `world.Despawn`, `bullet.Shoot{Ammo}`
(a shot from the player's selected units, or from the entity that orders it, aimed at its moment's
subject).

Rules and plans read the world's states with `During(e, step)`, as they read an entity's with
`Under`:

```go
alarmed := fx.Define("alarm", effect.Spec{effect.Lasts(30 * time.Second)})
alert := fx.Define("alert", effect.Spec{effect.Alter(func(st *steering.Steering) { st.MaxSpeed *= 1.5 })})
s.player.Bind(control.Give(control.KeyPress{Key: control.KeyN}, "Sound the alarm", rule.Cast(alarmed).On(entity.World)))
guard := rule.Role("guard").Obeys(rule.Then[unit.Standing]("hurry", rule.All, rule.During(alarmed, rule.Keep(alert))))
```

The guards hurry while the alarm sounds and slow down once it is over: `Keep` holds `alert` only
while the rule fires it.

The wire demo (`examples/wire-demo/demo.go`) is a whole program of it: a lever, a plate and a
switch, each a command for a group of cells; the trapdoor and pressure plate demos are the same
in less.

## Roles

A **role** is a behaviour several kinds share, said once: the rules those playing it obey.
`rule.Role(name)` makes it; `Obeys(rules...)` adds rules, each narrowed to the role's players. A
kind plays its roles through `rule.Plays(roles...)`, a cell through its kind
(`board.Plugin.Plays(kind, roles...)`), a plugin through its own `Plays`, and a command may be for
those playing it alone
(`selection.Selected(role)`):

```go
fx := s.world.Effects()
haste := fx.Define("haste", effect.Spec{effect.Lasts(3 * time.Second),
	effect.Alter(func(st *steering.Steering) { st.MaxSpeed *= 2 })})

mortal := rule.Role("mortal").Obeys(rule.Then[unit.Standing]("fall in", rule.All, rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))
hasty := rule.Role("hasty")

s.scout = units.Define("scout", land, profile, rule.Plays(mortal, hasty))
s.porter = units.Define("porter", land, laden, rule.Plays(mortal))
s.player.Bind(control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts",
	rule.Cast(haste).On(s.selection.Selected(hasty))))

// Spawn: a unit is told whose it is and that it may be selected as it is made
mine := []any{players.Give{To: s.player.ID}, selection.Allow{}}
s.world.Seed(s.scout.Entry(row).Told(mine...), s.porter.Entry(row).Told(mine...))
```

What a plugin keeps of an entity is changed by that plugin's command, not written into its kind:
`players.Give{To}` hands a unit to a player, `selection.Allow{}` and `Forbid{}` say whether it may
be selected, and `kind.Entry.Told(cmds...)` has the entity give them itself as it is made — so one
kind serves several players, and a unit changes hands mid-game by the same command. They are
carried out in the first step, a unit nobody's until then.

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

`rule.Playing(role, step)` runs the step while the entity plays the role and fails while it does not.
Inside `Here` or `Around` it asks the place the step turned to: the wire demo's handy scouts pull
the lever beside them with `rule.Under(pull, rule.Around(1, rule.Playing(lever, rule.Trigger())))`,
and nothing else there. A plan's Actor has it too.

A program names 64 roles at most, one name one tag, which every world saves by the name. Make them
in `Init`: the world names them in its kinds as the Stage's ECS is set up. A kind names
`rule.Plays` once, every role it plays in it; a kind giving a component type twice panics.

### Groups and behaviours

**Roles mean behaviour, names and groups mean which ones.** One rule of the role plate serves
every plate: which doors a plate opens is the command naming it. A hundred levers are a hundred
commands, a hundred names and the same rules — never a role or an effect each. A game defines no
tags: tags are how the plugins keep their own states and groups — whose a unit is, whether it is
selected — changed by their commands (`players.Give`, `selection.Allow`); `rule.Self` and
`rule.Between` filter by an effect's marker, and `rule.Other(role)` lets through the pairs whose
other plays a role — `predator.Obeys(rule.Then[vision.Sighting]("chase", rule.Other(prey), …))`.

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
its own work in a pass it makes anyway, never a rule of another plugin's moment (2026-10-02):

- the ground's pace: the board's pass over the units writes each one's `steering.Pace` — the cost
  and the slope of the cell under it — and the world's velocity pass multiplies the speed by it,
  from the next step;
- navigation's bumps: its pass reads the `collision.Collider` contacts of every unit under orders;
- contacts counted and logged, sightings and falls logged: `collision.Plugin.WithStats`,
  `WithLog`, `vision.Plugin.WithLog`, `board.Plugin.WithLog`;
- the weather on the board: the atmosphere's own `Weathering.System`, once a second of game time.

Steering is commands an entity gives itself: `steering.Away{}` and `steering.Toward{}`, aimed at
the moment's subject — a `Sighting`'s nearest seen; an aimed command fails while the moment
names nobody — and `steering.Turn{Angle}`. `vrules.Flee`, `Chase` and `Search` are written so:

```go
rule.Then[vision.Sighting]("vision.chase", rule.Between(tags.Predator, tags.Prey), rule.Order(steering.Toward{}))
```

A switch of a behaviour for the whole game is an effect on the world (`rule.Cast`, `Lift` or
`Toggle` on `entity.World`), the rule running `During` it: `vrules.Flee(threat, fleeing)`.

How an entity is drawn is the one place rules are Go: `render.Over`, `As`, `Swap`, `With` and
`Show`, given to `world.Plugin.Draw` (and `vision.Plugin.Draw`, which views are drawn), run every
frame. They read a component and decide nothing in the game.

**A state of the ground.** A cell stays the kind it is; an effect on it turns the kind's knobs
(`g.Kind.Allows`, `Cost`) and is drawn as a cover, `board.Plugin.Covering(e)` — a texture a state,
laid along a line of its own. A rule asks for the state under a unit, not for a kind:
`rule.If(unit.Over(iced), rule.Keep(slip))`. Which cells take which state is a role they play:
`rule.Around(1, rule.OneOf(rule.Playing(lake, rule.Apply(iced)), rule.Apply(frost)))`.

**The look of a state.** A state is an effect: what it does to the knobs, and its marker. How an
entity looks under it is the kind's, settled at drawing: `frozen.Look(witch.SpriteID())` is the
atlas slot the witch is drawn from while frozen — issued the first time it is asked for, drawn
into the atlas in the scene's `Layers`, swapped in by the world's renderer by itself
(`world.Plugin.WithRenderer`); a scene finds its effects by name, `world.Effects().Named("frozen")`,
at building alone. That is a `render.Swap` under the effect's marker, which a game may give
itself for a state that is no effect: `render.Swap(twins, frozen.Mark().In)`
draws each kind as its own sprite under the marker — the table a sprite a kind, a twin a way
faced after `Facing` — and leaves a kind with no twin as it is; `render.Over(crust, e.Mark().In)`
lays one look over every kind; `Show` hides. Two states compose in the order the rules are given.
This is what Unreal's Gameplay Ability System calls a cue: the effect grants a tag, each class
reacts to it cosmetically; the effect never touches the look. An `Alter(Appearance)` remains a
knob for one look for every kind (`examples/effect-demo` froze everyone pale before; now each kind
freezes in its own look).

## Dispel and order

Many rules may touch one effect, from several plugins. What happens is fixed:

- A rule that `Keep`s an effect someone `Dispel`led has it back the step after: its cause goes on,
  so the effect does. A dispeller that is to win casts a shield the keeper checks —
  `rule.Steps(rule.Apply(doused), rule.Dispel(burning))` against `rule.Unless(doused, rule.Keep(burning))`.
- A plan's `Keep` gives way when someone else takes its effect off: the branch fails and the plan
  goes on to what it does next.
- A cast after a `Dispel` in the same step takes the slot back.
- In a step, rules of one moment run in the order their roles came to be played; moments, in the order of
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
4. Keep the defaults a plugin gives its own host unexported; a game adds its own as roles its
   kinds play.
5. Test the scenario as ticks: who is where, which facts, which commands, until it ends.

Example — navigation's crowd, the rules of units among others, as in StarCraft II. Navigation
perceives — the moment `Touch`, two units touching, with what each is doing and whether the one
standing has room — and carries out the commands, keeping its own rules inside them (a unit is
never stepped into water, off a cliff or into a wall); the rules decide:

```go
func makeWay() rule.Rule {
	return rule.Then[Touch]("navigation.make way", rule.All, rule.If(Touch.PushedByAlly, rule.Order(StepAside{})))
}

func joinTheGroup() rule.Rule {
	return rule.Then[Touch]("navigation.join the group", rule.All, rule.If(Touch.ReachedTheGroup, rule.Order(Stop{})))
}

func goRound() rule.Rule {
	return rule.Then[Touch]("navigation.go round", rule.All, rule.OneOf(
			rule.If(Touch.GoalTaken, rule.Order(Settle{})),
			rule.If(Touch.WaitsFirst, rule.Order(Hold{})),
			rule.If(Touch.NoWayRound, rule.Order(StepAside{})),
			rule.If(Touch.ClearsTheWay, rule.Order(Pass{})),
			rule.If(Touch.Blocks, rule.Order(Detour{})),
		))
}
```

`StepAside`, `Pass`, `Detour` and `Settle` are `Aimed`: each is told the moment's subject, the
unit touched. A game adds its own rules as a role's, for the units playing it —
`rule.Role("guard").Obeys(rule.Then[Touch]("hold the line", rule.All, …))` — or gives its own set in place
of the crowd's with `WithCrowd`.

## A game: states as effects

A game built on the plugins is written in three parts, and none of them is Go code inside a rule:

1. **States are effects.** Burning, frozen, alarmed, doused, a lever pulled: each defined once,
   for a while or for good, with its own marker on while it runs, on a unit, a cell or the world.
   One effect may lead to the next (`Then`).
2. **Rules connect.** A rule turns a plugin's moment — a touch, a sighting, where one
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

// whatever can burn plays one role: the units' kinds and the kinds of cell alike
flammable := rule.Role("flammable").Obeys(
	// fire spreads to whom a burning one touches, now and then, unless they are wet
	rule.Then[collision.Meeting]("fire spreads", rule.Between(burning.Mark(), tag.Any),
		rule.ForOther(rule.Unless(doused, rule.Chance(0.3, rule.Apply(burning))))),
	// water puts a burning one out — nothing smoulders then — and the wet do not catch fire for a
	// while; one on dry ground sets it alight now and then
	rule.Then[unit.Standing]("where it burns", rule.Self(burning.Mark()), rule.OneOf(
			rule.If(inWater, rule.Steps(rule.Apply(doused), rule.Dispel(burning))),
			rule.Here(rule.Chance(0.1, rule.Apply(burning))),
		)),
	// burning ground sets the cells round it alight now and then — not one burning or burnt out
	rule.Then[cell.Now]("fire spreads over the ground", rule.Self(burning.Mark()), rule.Around(1, rule.Unless(burning, rule.Unless(smouldering, rule.Chance(0.05, rule.Apply(burning)))))),
)
s.board.Plays("grass", flammable)
s.board.Plays("forest", flammable)
s.walker = units.Define("walker", land, profile, rule.Plays(flammable))

func inWater(s unit.Standing) bool { return s.Kind.Admits(cell.Water) }
```

The engine hands the first rule to collision and the other two to the board: the game names no
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

Not yet there: which effects a player may apply (any, today, on whatever a command names
— a game over a network will want a list); an effect taking a tag off while it runs (a
`Revoke` beside `Grant`); filters joined (`rule.Self(x)` and `Having[T]` at once); `Here` and
`Around` in plans; a step shared by the moments of several plugins; more than `MaxEffects` (8)
effects on one entity and 63 defined; an entity in more than one group.
