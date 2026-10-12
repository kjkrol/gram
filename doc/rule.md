# Rule: how entities behave

One vocabulary for every behaviour of an entity, in `rule`: a **rule** is what is done at a
moment a plugin catches, a **plan** is what an entity does over time, an **effect** is a change
that holds, a **command** is what an entity has done, and a **fact** is what a plugin tells an
entity. Written on 2026-09-30 when yielding and avoiding outgrew the reflexes; since 2026-10-02
entities obey rules by the roles they play; since 2026-10-05 what somebody asks for is one
command written as a sentence and nobody hands a rule to a plugin. On 2026-10-12 the notation
became one brick, a `rule.Branch`, and the same day its grammar was settled: a branch says when,
on whom and what; a fact is a thing that happened; time is in a plan alone; a message is a
command sent to another.

## One brick: a Branch

A rule is a sentence — **who, when, on whom, what** — and a `rule.Branch` is all but the who:
`When` the conditions, `On` whom the steps act on, `Do` the steps. Who is the role the branch is
given to, so the rules of a role read as a paragraph under it. The type `P` a Branch is of is the
moment it fires on, in a rule, or the fact it waits for, in a plan; `any` for a branch on nothing
but its When.

```go
// rules: the players of mortal fall in; the predator chases what it sees, and turns to look
// round when it sees nothing
roles.Define(MortalRole, rule.Branch[unit.Standing]{
	Name: "fall in",
	When: unit.Standing.Fallen,
	On:   rule.Self,
	Do:   rule.Apply(world.Despawn{}),
})
roles.Define(PredatorRole,
	rule.Branch[vision.Sighting]{
		Name: "chase",
		When: vision.Seeing(prey),
		On:   rule.Self,
		Do:   rule.Apply(steering.Toward{}),
	},
	rule.Branch[vision.Sighting]{
		Name: "search",
		When: rule.All(vision.Seeing(prey), vision.Sighting.Nobody),
		On:   rule.Self,
		Do: rule.Branch[any]{
			When: rule.Not(rule.Under(turned)),
			Do: rule.Steps(
				rule.Put(turned),
				rule.OneOf(
					rule.Branch[any]{When: rule.Chance(0.5), Do: rule.Apply(left)},
					rule.Apply(right),
				),
			),
		},
	},
	rule.Branch[collision.Meeting]{
		Name: "caught",
		When: collision.Met(prey),
		On:   rule.Other,
		Do:   rule.Apply(world.Despawn{}),
	},
)

// a plan: what a guard does over time, branches choosing
guard := rule.Plan(GuardPlan, rule.OneOf(
	rule.Branch[any]{
		Name: "to the gate",
		When: rule.Under(alarm).On(entity.World),
		On:   rule.Self,
		Do: rule.Steps(
			rule.Apply(navigation.MoveTo{To: entity.Named(GateName)}),
			rule.Until[navigation.Arrived](),
		),
	}.Timeout(20 * time.Second),
	rule.Branch[navigation.Blocked]{
		Name: "blocked",
		When: navigation.Blocked.WaitedLong,
		On:   rule.Self,
		Do:   rule.Apply(navigation.StepAside{}),
	},
	rule.Branch[navigation.Arrived]{
		Name: "rest",
		On:   rule.Self,
		Do:   rule.Wait(2 * time.Second),
	},
	rule.Branch[any]{
		Name: "patrol",
		On:   rule.Self,
		Do:   rule.Steps(rule.Apply(navigation.Onward{}), rule.Until[navigation.Arrived]()),
	},
	rule.Branch[any]{Name: "idle", On: rule.Self, Do: rule.Idle()},
))
units.Define(GuardKind, land, profile, guard,
	comp.Load(func(g guardRow) navigation.Round { return navigation.NewRound(g.Round[:]...) }))
```

The grammar:

```
rule    = Branch[Moment]                        // given to a role: roles.Define(role, rules...)
plan    = Plan(name, root)                      // given to a kind, in its Spec
root    = OneOf(branch...) | Steps(branch...) | branch
branch  = Branch[P]{Name, When, On, Do}         // inside a Do: Name needless, On the outer one's
P       = Moment | Fact | any
When    = pred(P) | cond | All(When...) | Not(pred or cond) | nil
cond    = Under(e) | Under(e).On(whom) | Playing(role) | Chance(p) | a moment's word
On      = Self | Other | Here | Around(n)       // Here, Around: a rule on a Placed moment
Do      = step | OneOf(step...) | Steps(step...) | branch
step    = Put(e) | Take(e) | Hold(e) | Apply(cmd)
        | Until[F](pred...) | Wait(d) | Idle()  // a plan's alone
frame   = branch.Timeout(d) | branch.Every(d)   // a plan's alone
```

- A Branch given to a role (`world.Roles.Define`) is a rule of `P` for the plugin that catches
  `P`, obeyed by the role (below, "Who gets a rule"); its `Name` is for the errors about it. A
  Branch inside another's `Do` asks more of the same `P`; one of `any` asks only of state; it acts
  on what the outer one does unless its own `On` says otherwise, and may say so at any depth
  (`On: rule.Self` inside `On: rule.Other`: back to the entity itself). A step the moment cannot
  run — `Here` where `P` is not Placed, a Branch of another moment inside, a step that lasts, a
  message — panics by the rule's name as the rule is built, in `Init`; so does a Branch at the
  top of a rule or a plan without a `Name` or an `On`.
- `rule.Plan(name, root)` is a plan, a value a kind carries in its Spec as it carries
  `rule.Plays`; the world defines it as the kind is registered, under the name a save knows it
  by, and runs it in every step of its simulation. `OneOf` is run every tick, from the top, the
  first branch whose When holds; an earlier one, once it can, takes over from a later one, which
  is stopped. `Steps` runs its parts one after another and remembers where it is. A plan done
  begins again the next tick: a patrol of one branch is a loop.
- **A fact is a thing that happened.** In a plan a Branch of a fact `P` — a component a plugin
  writes for the entities with a plan: `navigation.Arrived`, `Blocked`, a message — begins once
  as the fact comes to hold its When (it was absent, or the predicate did not hold; one carried
  already counts once), runs its Do to the end though the fact goes, and does not begin again
  until the fact comes afresh. A Branch of `any` is on a state: it runs while its When holds and
  is stopped when it stops. So the guard rests once on arriving, and runs to the gate while the
  alarm is on and back to the patrol when it is off.
- **Time is a plan's.** `Until[F]` waits for a fact coming afresh (or coming to hold a
  predicate), `Wait` for a time, `Idle` for ever; `Timeout(d)` has a branch fail once it has run
  for `d`, `Every(d)` keeps it from beginning again within `d` of its last run. A rule runs in
  one pass and is refused them as it is built.
- **Defining registers and hands nothing back.** Effects (`world.Effects`), roles
  (`world.Roles`), commands (`world.Commands`) and kinds (`kind.Define`, `units.Define`) are each
  defined under a name in the Stage's world, and `Named(name)` is the thing wherever it is built
  on; a plan is carried, not looked up. A game keeps its names as constants (a `names.go`), each
  with a suffix saying what it names — `BloodMoonEf`, `BleedCmd`, `MortalRole`, `GuardPlan`,
  `ScoutKind`, `GrassCell` — so a name mistyped does not compile, and a Stage's struct holds its
  plugins alone. Each Stage has registers of its own: nothing a game defines is the program's.
- A function writing part of a rule or a plan returns a `rule.Step`. A plugin ships no
  ready-made reactions: it gives moments and their conditions, the game writes the rule.

## Conditions

A Branch's `When` is one condition or several (`rule.All(…)`), each a sentence about the entity
the branch is for, nil for always:

- a predicate of the moment or the fact — a method as a value (`unit.Standing.Fallen`,
  `vision.Sighting.Nobody`, `navigation.Blocked.WaitedLong`), a plugin's own (`unit.On(ice)`,
  `unit.Over(iced)`, `bullet.Within(d)`, `gravity.HarderThan(v)`, `clock.At(dusk)`);
- the entity's state: `rule.Under(e)`; the world's: `rule.Under(e).On(entity.World)` (a state of
  the whole game, `Put` on `entity.World`); a plugin's: `rule.Under(e).On(s.atmosphere)`;
- `rule.Playing(role)`: the entity, or under `Here`/`Around` the place turned to, plays the role;
- `rule.Chance(p)`: with likelihood `p`, drawn afresh every step from the world's seed, the game
  time and the entity, so a load and a replay draw alike;
- the other of a pair, on a moment that is `plugin.Met`, in the moment's word:
  `vision.Seeing(role)`, `collision.Met(role)`, `bullet.Reaching(role)`, `navigation.Touching(role)`
  (each `rule.Paired(role)`, the word plugins make theirs of);
- `rule.Not(c)` of any of them.

`Playing` and `Paired` at the top of a rule the plugin reads off its own query, on the tag bits,
so a rule that does not concern an entity costs nothing for it. A role narrows a rule further,
on top of its When (below, "Roles").

## Who gets a rule

Nobody hands a rule to a plugin: there is no hooking. A rule is obeyed by a role, and a role is
played — by a kind of entity, by a kind of cell, by a plugin:

```go
roles := s.world.Roles()
roles.Define(MortalRole, fallIn)   // a rule of unit.Standing: the board's
roles.Define(PlateRole, press)     // a rule of collision.Meeting: collision's
roles.Define(NightlyRole, atDusk)  // a rule of clock.Moment: the world's

units.Define(ScoutKind, land, profile, rule.Plays(roles.Named(MortalRole))) // a kind of unit plays it
plates.Define(PlateKind, comp.Const(collision.Collider{}),
	rule.Plays(roles.Named(PlateRole)))                                 // a kind standing on the board
kinds.Define("lava", cell.Kind{Cost: 9}, roles.Named(ScorchingRole))     // a kind of cell, as it is defined
layout.Cells = append(layout.Cells, water.Entry(c).Plays(lakeRole))     // one cell of the Layout alone
s.world.Plays(roles.Named(NightlyRole))                                 // the world itself
```

Once the Stage's `Init` returns, the engine gives every rule of every role somebody plays to the
plugin in use that catches its moment, trying the plugins in the order they were used
(`plugin.Installer.Hosts` is how a plugin says what it catches). A rule no plugin in use catches
fails `Init` with an error wrapping `plugin.ErrUnhosted`, naming the rule by its `String` — its
name, its moment and its role: `"fall in" of unit.Standing, for the role mortal`. A rule of a
moment of the world as a whole (`clock.Moment`, `climate.Weathering`) fires while the plugin
whose moment it is plays the rule's role.

A plugin is an entity too (`world.Self`): one of its own, called by the plugin's name, carrying
its knobs, its roles and the effects it is under — as many at once as a game defines. So a state
of the sky is an effect on the atmosphere, as a state of the whole game is one on the world:

```go
fx.Define(BloodMoonEf, effect.Spec{effect.Lasts(night), effect.Alter(func(m *sky.Moon) { … })})
cmds.Define(BleedCmd, rule.Put(fx.Named(BloodMoonEf)).On(s.atmosphere)) // the plugin is whom the command is for
rule.Branch[world.Moving]{ // … and a rule reads its state
	Name: "frenzy",
	When: rule.Under(fx.Named(BloodMoonEf)).On(s.atmosphere),
	On:   rule.Self,
	Do:   rule.Hold(fx.Named(FrenziedEf)),
}
```

## The five words

- **Rule** — a moment a plugin catches: a `unit.Standing` or a `cell.Now`, a `vision.Sighting`, a
  `collision.Meeting` (pairs) or `Struck`, a `world.Moving` or `Leaving`, a `clock.Moment`, a
  `climate.Weathering`, a `navigation.Touch`. A rule's steps are instant — `OneOf`, `Steps`, a
  Branch asking more, `Put`, `Take`, `Hold`, `Apply` — so a step that lasts (`Wait`, `Until`,
  `Idle`) or a message is not to be had in a rule: it is refused as the rule is made. A rule
  remembers nothing of its own, and runs no Go code of its own: there is no step for it. What
  the steps cannot say is a moment, a step or a knob the plugin still lacks.
- **Plan** — what a kind's entities do over time (`rule.Plan`). It remembers its place (its
  mind, the component the plan is: the steps running, each one's place and start on the world's
  clock), waits (`Wait`, `Until`), talks to other entities (messages), and is saved with the
  game. `OneOf` is a reactive choice, `Steps` a sequence with memory; a Branch of a fact runs
  once as the fact comes, to the end.
- **Effect** — `effect.Effect`, defined once from a `Spec` (`Lasts`, `Stacking`, `Then`, `Grant`,
  `Alter`; `Described` a sentence for a UI, `Description()`), with its own marker, on while it
  runs (`Mark()`). `Put` puts it on; `Hold` holds it as long as its branch runs, or as long as a
  rule keeps firing it; `Take` takes it off. **An effect's presence is state**: a branch
  `When: rule.Not(rule.Under(turned))` that puts `turned` on is "at most once a while", a memory
  for rules that keep none, and its marker is what rules of other plugins ask for (`rule.Under`).
  The collision demo's hit is one (put on at every `collision.Struck`, drawn by
  `render.Over(overlay, hit.Mark().In)`).
- **Command** — `Apply(cmd)` gives the command of whom the branch acts on, the same one a player
  gives (`navigation.MoveTo`, `world.Despawn`), queued for the plugin that handles its type, and
  goes on at once: **fire and forget**. The handler carries it out for the entity alone
  (`control.Issued.ByEntity`); `world.Spawn{Entry}` is the one that makes another entity, of the
  Entry fixed as the rule is written (a nest laying an egg where it stands). The world keeps a
  stage's one carrier (`control.Carrier`, `world.Plugin.Commands`): the players give it theirs,
  the entities theirs; the engine carries every `plugin.CommandHandler` a stage uses, and a
  plugin's `plugin.Tick` hands the carrier to its rules. Nothing is dropped: a command waits for
  its handler's pass — given after it, for the next frame's. A command that is `plugin.Aimed` is
  told the subject of the fact it stands under, or of the rule's moment: whom the entity touched,
  who asked. A command embedding `rule.From`, given on `Other`, is a message (below).
- **Fact** — a component a plugin writes for entities with a `Mind` alone (navigation's
  `Blocked`, `Arrived`), so entities without a plan pay nothing. Facts are how a plan learns what
  came of its commands; a fact comes, and a branch on it runs once.

## Idioms

- **Give, then wait for what comes of it.** `rule.Steps(rule.Apply(navigation.MoveTo{…}),
  rule.Until[navigation.Arrived]())`: `Until[F]()` waits for a fact to come *afresh* — a fact
  left from before does not count — and `Until[F](pred)` for one to come to hold it.
- **A patrol is a plan.** The unit carries its own `navigation.Round` of cells (from its row,
  `comp.Load`), and one branch gives `navigation.Onward{}`, waits for `Arrived` and stands a
  while; the plan done begins again, so the round goes round. A step aside on the way carries
  the goal over (navigation queues it after), so `Arrived` still comes.
- **No answer is a frame, not a fact.** `rule.Branch[any]{Do: rule.Until[reply]()}.Timeout(d)`
  inside a `OneOf`, the branch after it what to do then.
- **A rule's memory is an effect.** `When: rule.Not(rule.Under(e))` with `rule.Put(e)` among the
  steps runs a thing and not again for as long as `e` lasts.
- **The clock's moment is the clock's.** A `clock.Moment` is of the clock's own entity: an effect
  a clock rule puts on lands there — `rule.Branch[clock.Moment]{Name: "dusk", When:
  clock.At(dusk), On: rule.Self, Do: rule.Put(night)}`, a phase that `clock.Clock.In` reads. Run
  once a step, walking no entities, it asks nothing of the entity and fires while the clock's
  plugin, the world, plays its role.
- **Where one stands.** On a moment that is `plugin.Placed` — a `unit.Standing`, a cell's
  `cell.Now` — `On: rule.Here` acts on the cells under the entity (for a cell, on itself) and the
  entities standing on them, `On: rule.Around(rings)` on those and the rings of neighbours round
  them, each cell once, on square and hex boards alike: an effect put on the ground, a lever
  pulled. The units there are no part of it.
- **Now and then.** `When: rule.Chance(p)` holds with likelihood `p`; a coin is `OneOf(Branch[any]{
  When: Chance(0.5), Do: a}, b)`.
- **One after another.** `effect.Then(next)` casts `next` when an effect's time is up — burning
  leaves smouldering — but not when it is taken off: put out, nothing smoulders.

## Commands

Whatever somebody asks for — a player's key, a script, an AI, a branch's `Apply` — is a command,
and a command about an effect is one sentence (`rule.Command`):

```go
openWest := rule.Put(open).On(entity.Group("west trapdoors")).By(entity.Named("west lever"))
flipGate := rule.Toggle(ajar).On(entity.Group("gate"))
alarm    := rule.Put(alarmed).On(entity.World).For(30 * time.Second)
hasten   := rule.Put(haste).On(s.selection.Selected(hasty))
freeze   := rule.Put(frozen).On(s.selection.Pointed()).For(3 * time.Second)
```

- **The verb**: `rule.Put` puts the effect on, `rule.Take` takes it off, `rule.Toggle` takes it
  off them all where any is under it and puts it on them all otherwise — a switch. Without an
  `On`, `Put` and `Take` are steps, acting on whom the branch does; `Toggle` is a command alone.
- **Whom** (`On`): `entity.Named(names...)`, the entities bearing those names, one each;
  `entity.Group(names...)`, all those in the groups; `entity.World`, the world's own entity — a
  state of the whole game; or a plugin's own: the selection's `Selected(roles...)`, the units the
  player who gives the command has selected (those playing a role named, when any is), and
  `Pointed()`, the entity drawn under the cursor as the key is pressed.
- **How long** (`For`): in place of what the effect's `Spec` says.
- **Who sets it off** (`By`): the entities that give themselves `rule.Triggered{}` — a lever, a
  plate. A role then says only *that* a plate stood on triggers; the command says what it opens.

An entity is called by what makes it: a cell by its `cell.Entry{Name, Group}`, a unit by its
`kind.Entry` (`Named`, `InGroup`); a name is one entity's, a group many's, and both are saved. The
game defines its commands by name in `s.world.Commands()` (`Define(name, cmd)`, in the Commands
section; `Named(name)` is the command for a key): those with a `By` are kept for `Triggered`, and
every name they say is checked as the game starts — a name nobody bears, or one two bear, stops it
there. A command is given the same way whoever gives it:

```go
openWest := s.world.Commands().Named(OpenWestCmd)
s.player.Bind(control.Give(control.KeyPress{Key: control.Key1}, "Pull the west lever", openWest)) // a key
s.players.Issue(ai, openWest)                                                                      // an AI, a script
rule.Apply(openWest)                        // a rule, a plan
rule.Apply(rule.Triggered{})                // its source, pressed
```

The plugin its target belongs to carries it out — the world for those named, grouped and the
world itself, the selection for its own — a step later, with the step's effects; commands of one
effect for one entity in one step are netted, so a switch flipped twice stays as it was.
Other plugins' commands are theirs: `navigation.MoveTo` (a cell, or an entity by name:
`MoveTo{To: entity.Named(GateName)}`), `world.Despawn`, `bullet.Shoot{Ammo}` (a shot from the
player's selected units, or from the entity that gives it, aimed at its moment's subject).

Rules and plans read the world's states with `Under(e).On(entity.World)`, as they read an
entity's with `Under(e)`:

```go
fx.Define(AlarmEf, effect.Spec{effect.Lasts(30 * time.Second)})
fx.Define(AlertEf, effect.Spec{effect.Alter(func(st *steering.Steering) { st.MaxSpeed *= 1.5 })})
roles.Define(GuardRole, rule.Branch[unit.Standing]{
	Name: "hurry",
	When: rule.Under(fx.Named(AlarmEf)).On(entity.World),
	On:   rule.Self,
	Do:   rule.Hold(fx.Named(AlertEf)),
})
cmds.Define(SoundAlarmCmd, rule.Put(fx.Named(AlarmEf)).On(entity.World))
s.player.Bind(control.Give(control.KeyPress{Key: control.KeyN}, "Sound the alarm", cmds.Named(SoundAlarmCmd)))
```

The guards hurry while the alarm sounds and slow down once it is over: `Hold` holds `alert` only
while the rule fires it.

The wire demo (`examples/rules/wire/demo.go`) is a whole program of it: a lever, a plate and a
switch, each a command for a group of trapdoors or a gate, entities standing on the board; the
trapdoor and pressure plate demos are the same in less.

## Messages

A plan talks to another entity with the bricks it has. A command embedding `rule.From`, given
with `Apply` on a branch whose `On` is `Other`, lands on that entity as a fact in place of going
to a queue, the engine filling in who sent it; one without a plan takes none, and the step
fails. A Branch on the command's type is how the other answers — `Apply` on `Other` again, the
reply a command of its own — or passes it on, `Apply` on the subject of another fact. A message
nobody takes up is dropped after `AskLife`; one passed on `MaxChain` times is not sent, so a
chain ends. Delivery a tick later means no recursion within a tick.

```go
type pass struct{ rule.From }
type reply struct {
	rule.From
	Yes bool
}

func (r reply) yes() bool { return r.Yes }

// the asker: let me pass; walks on let through, flees turned away or unanswered
rule.Branch[navigation.Blocked]{
	Name: "ask to pass",
	When: navigation.Blocked.WaitedLong,
	On:   rule.Other,
	Do: rule.Steps(
		rule.Apply(pass{}),
		rule.OneOf(
			rule.Branch[any]{Do: rule.Until[reply]()}.Timeout(time.Second),
			rule.Branch[any]{On: rule.Self, Do: rule.Apply(flee{})},
		),
	),
},
rule.Branch[reply]{Name: "let through", When: reply.yes, On: rule.Self, Do: rule.Apply(walk{})},
rule.Branch[reply]{
	Name: "turned away",
	When: rule.Not(reply.yes),
	On:   rule.Self,
	Do:   rule.Apply(flee{}),
},

// the one asked: lets pass where it has room, jumping aside; asks its ally on where it has one
rule.Branch[pass]{
	Name: "asked to let pass",
	On:   rule.Other,
	Do: rule.OneOf(
		rule.Branch[room]{
			When: room.free,
			Do: rule.Steps(
				rule.Apply(reply{Yes: true}),
				rule.Branch[any]{On: rule.Self, Do: rule.Apply(jump{})},
			),
		},
		rule.Branch[room]{When: room.byAlly, On: rule.Other, Do: rule.Apply(pass{})},
		rule.Apply(reply{}),
	),
},
```

The game's types are the words: `pass`, `reply`, `room` (a fact the game writes, whose subject is
the ally beside). Nothing in `rule` names a conversation.

## Roles

A **role** is a behaviour several kinds share, said once: the rules those playing it obey.
`s.world.Roles().Define(name, rules...)` says it, each rule narrowed to the role's players, and
`Roles().Named(name)` is the role; each Stage's world has its own roles, 64 at most. A
kind plays its roles through `rule.Plays(roles...)`, a cell through its kind
(`cell.Kinds.Define(name, kind, roles...)`) or one cell of the Layout alone (`cell.Entry.Plays`), a
plugin through its own `Plays`, and a command may be for those playing it alone
(`selection.Selected(role)`):

```go
fx, roles, cmds := s.world.Effects(), s.world.Roles(), s.world.Commands()
fx.Define(HasteEf, effect.Spec{effect.Lasts(3 * time.Second),
	effect.Alter(func(st *steering.Steering) { st.MaxSpeed *= 2 })})

roles.Define(MortalRole, rule.Branch[unit.Standing]{
	Name: "fall in",
	When: unit.Standing.Fallen,
	On:   rule.Self,
	Do:   rule.Apply(world.Despawn{}),
})
roles.Define(HastyRole)

cmds.Define(HastenCmd, rule.Put(fx.Named(HasteEf)).On(s.selection.Selected(roles.Named(HastyRole))))

units.Define(ScoutKind, land, profile, rule.Plays(roles.Named(MortalRole), roles.Named(HastyRole)))
units.Define(PorterKind, land, laden, rule.Plays(roles.Named(MortalRole)))
s.player.Bind(control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts", cmds.Named(HastenCmd)))

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

`Obeys` narrows a rule on top of its own When: a rule of `rule.Under(hungry)` obeyed by mortal
fires for a mortal entity while it is hungry, one of `rule.Playing(hasty)` for a mortal that is
hasty. On a moment that is `plugin.Met` it fires for the pairs whose own entity plays the role.
A collision `Meeting` rule with no `Met`, or whose other's role is its own, still runs at most
once a pair; a `Touch` belongs to each of the two units, a `Sighting` to each observer — and its
`Seeing` narrows what the observer sees, not whether the rule fires. An entity playing two roles
that obey the same rule obeys it once for each. A rule of a moment of the world as a whole — a
`clock.Moment`, a `climate.Weathering`, run once a step walking no entities — asks nothing of
the entity and fires while the plugin whose moment it is plays its role.

`rule.Playing(role)` holds while the entity plays the role. Under `On: rule.Here` or
`rule.Around(n)` it asks the place turned to: the wire demo's handy scouts pull the lever beside
them with `When: rule.Under(pull), On: rule.Around(1), Do: rule.Branch[any]{When:
rule.Playing(lever), Do: rule.Apply(rule.Triggered{})}`, and nothing else there.

A program names 64 roles at most, one name one tag, which every world saves by the name. Make them
in `Init`: the world names them in its kinds as the Stage's ECS is set up. A kind names
`rule.Plays` once, every role it plays in it; a kind giving a component type twice panics.

### Groups and behaviours

**Roles mean behaviour, names and groups mean which ones.** One rule of the role plate serves
every plate: which doors a plate opens is the command naming it. A hundred levers are a hundred
commands, a hundred names and the same rules — never a role or an effect each. A game defines no
tags: tags are how the plugins keep their own states and groups — whose a unit is, whether it is
selected — changed by their commands (`players.Give`, `selection.Allow`); `rule.Under` asks for an
effect, and a moment's word for the other of a pair — `vision.Seeing(prey)`,
`collision.Met(prey)` — lets through the pairs whose other plays a role:
`roles.Define(PredatorRole, rule.Branch[vision.Sighting]{Name: "chase", When:
vision.Seeing(prey), …})`.

## Knobs

A plugin gives a game **knobs**: components it only reads, which an effect's `Alter` turns and
puts back when it ends — a cell's `cell.Ground`, `steering.Steering` (how fast an entity goes,
speeds up, brakes; `Halted` holds it), `vision.Sight` (how far, which way; `Ahead` looks the way
it moves), `collision.Physics`, `world.Appearance`. What a plugin's system writes lives beside
the knob in a component of its own — `steering.Course` beside `Steering`, `vision.Sighted` beside
`Sight` — given where it is missing: an `Alter` ending puts back the original, and would put back
stale state with it. A new knob goes into the component a plugin reads, never into its state.

## No Go code in a rule

A rule holds no Go code of its own but its predicates. What a plugin does of its own is its own
work in a pass it makes anyway, never a rule of another plugin's moment (2026-10-02):

- the ground's pace: the board's pass over the units writes each one's `steering.Pace` — the cost
  and the slope of the cell under it — and the world's velocity pass multiplies the speed by it,
  from the next step;
- navigation's bumps: its pass reads the `collision.Collider` contacts of every unit under orders;
- contacts counted and logged, sightings and falls logged: `collision.Plugin.WithStats`,
  `WithLog`, `vision.Plugin.WithLog`, `board.Plugin.WithLog`;
- the weather on the board: the atmosphere's own `Weathering.System`, once a second of game time.

Steering is commands an entity gives itself: `steering.Away{}` and `steering.Toward{}`, aimed at
the moment's subject — a `Sighting`'s nearest seen; an aimed command fails while the moment
names nobody — and `steering.Turn{Angle}`. The vision demo's hunter and prey are written so:

```go
rule.Branch[vision.Sighting]{
	Name: "chase",
	When: vision.Seeing(prey),
	On:   rule.Self,
	Do:   rule.Apply(steering.Toward{}),
}
rule.Branch[vision.Sighting]{
	Name: "give way",
	When: vision.Sighting.Closing,
	On:   rule.Self,
	Do:   rule.Apply(steering.Away{}),
}
```

A switch of a behaviour for the whole game is an effect on the world (`rule.Put`, `Take` or
`Toggle` on `entity.World`), the rule running `Under(e).On(entity.World)` it (the vision demo's
fleeing).

How an entity is drawn is the one place rules are Go: `render.Over`, `As`, `Swap`, `With` and
`Show`, given on the world's atlas in a scene's Layers (and `vision.Plugin.WithViews`, which views
are drawn), run every frame. They read a component and decide nothing in the game.

**A state of the ground.** A cell stays the kind it is; an effect on it turns the kind's knobs
(`g.Kind.Allows`, `Cost`) and is drawn as a cover, `board.Plugin.Covering(e)` — a texture a state,
laid along a line of its own. A rule asks for the state under a unit, not for a kind:
`When: unit.Over(iced), On: rule.Self, Do: rule.Hold(slip)`. Which cells take which state is a
role they play: `On: rule.Around(1), Do: rule.OneOf(rule.Branch[any]{When: rule.Playing(lake),
Do: rule.Put(iced)}, rule.Put(frost))`.

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
knob for one look for every kind (`examples/rules/effect` froze everyone pale before; now each kind
freezes in its own look).

## Take and order

Many rules may touch one effect, from several plugins. What happens is fixed:

- A rule that `Hold`s an effect someone took off has it back the step after: its cause goes on,
  so the effect does. A taker that is to win puts on a shield the holder keeps clear of —
  `rule.Steps(rule.Put(doused), rule.Take(burning))` against `rule.Branch[any]{When:
  rule.Not(rule.Under(doused)), Do: rule.Hold(burning)}`.
- A plan's `Hold` gives way when someone else takes its effect off: the branch fails and the plan
  goes on to what it does next.
- A `Put` after a `Take` in the same step takes the slot back.
- In a step, rules of one moment run in the order their roles came to be played; moments, in the order of
  their plugins' passes — that is the systems' order, not something a rule picks. Every `Put`
  and `Take` of a step lands together in the effects' pass at its end.

## Writing a new behaviour

1. A moment a plugin already catches, instant: a rule on it; its memory, an effect; what it
   changes, a knob the effect alters — a component the plugin only reads, added where missing.
2. Something that lasts: name the facts the plugin can perceive and write them for units with a
   `Mind` only; name the commands and handle them in the plugin (a `CommandHandler`'s queue),
   carrying them out for the entity that gives them, the engine's own rules inside; tell what came
   of them as facts.
3. Write the plan from short named branches (`rule.Plan`) and give it to the kinds. A plan holds
   at most 128 steps.
4. Keep the defaults a plugin gives its own host unexported; a game adds its own as roles its
   kinds play.
5. Test the scenario as ticks: who is where, which facts, which commands, until it ends.

Example — navigation's crowd, the rules of units among others, as in StarCraft II. Navigation
perceives — the moment `Touch`, two units touching, with what each is doing and whether the one
standing has room — and carries out the commands, keeping its own rules inside them (a unit is
never stepped into water, off a cliff or into a wall); the rules decide:

```go
func makeWay() rule.Rule {
	return rule.Branch[Touch]{
		Name: "navigation.make way",
		When: Touch.PushedByAlly,
		On:   rule.Self,
		Do:   rule.Apply(StepAside{}),
	}
}

func joinTheGroup() rule.Rule {
	return rule.Branch[Touch]{
		Name: "navigation.join the group",
		When: Touch.ReachedTheGroup,
		On:   rule.Self,
		Do:   rule.Apply(Stop{}),
	}
}

func goRound() rule.Rule {
	return rule.Branch[Touch]{
		Name: "navigation.go round",
		On:   rule.Self,
		Do: rule.OneOf(
			rule.Branch[Touch]{When: Touch.GoalTaken, Do: rule.Apply(Settle{})},
			rule.Branch[Touch]{When: Touch.WaitsFirst, Do: rule.Apply(Hold{})},
			rule.Branch[Touch]{When: Touch.NoWayRound, Do: rule.Apply(StepAside{})},
			rule.Branch[Touch]{When: Touch.ClearsTheWay, Do: rule.Apply(Pass{})},
			rule.Branch[Touch]{When: Touch.Blocks, Do: rule.Apply(Detour{})},
		),
	}
}
```

`StepAside`, `Pass`, `Detour` and `Settle` are `Aimed`: each is told the moment's subject, the
unit touched. A game adds its own rules as a role's, for the units playing it —
`roles.Define(GuardRole, rule.Branch[Touch]{Name: "hold the line", …})` — or gives its own set in
place of the crowd's with `WithCrowd`. What a unit does over time is its plan: the guard demo's
guards patrol their own round, step aside when blocked long, rest where they arrive and run to the
gate, named on the map, while the alarm is on (above, "One brick").

## A game: states as effects

A game built on the plugins is written in three parts, and none of them is Go code inside a rule:

1. **States are effects.** Burning, frozen, alarmed, doused, a lever pulled: each defined once,
   for a while or for good, with its own marker on while it runs, on a unit, a cell or the world.
   One effect may lead to the next (`Then`).
2. **Rules connect.** A rule turns a plugin's moment — a touch, a sighting, where one
   stands — into effects: `Put`, `Hold`, `Take`, under a state (`Under`) or obeyed by a role,
   on the entity, the other or the places round (`On`), now and then (`Chance`).
3. **Plugins give knobs.** What an effect changes is a component a plugin reads — `Steering`,
   `Sight`, a cell's `Ground`, `collision.Physics`, `Appearance` — turned by the effect's `Alter`
   and put back when it ends. A plugin's own effects stay private; a game reaches it through its
   knobs and moments.

Example — fire, across collision, the board and the world, on units and on the ground alike:

```go
fx, roles := s.world.Effects(), s.world.Roles()
fx.Define(SmoulderingEf, effect.Spec{effect.Lasts(20 * time.Second)})
fx.Define(BurningEf, effect.Spec{
	effect.Lasts(8 * time.Second),
	effect.Then(fx.Named(SmoulderingEf)),                          // burnt out, it smoulders
	effect.Alter(func(a *world.Appearance) { a.SpriteID = flames }), // a unit drawn burning,
	effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 1.5 }),  // running about;
	effect.Alter(func(g *cell.Ground) { g.Kind = embers }),         // a cell's ground aflame
})
fx.Define(DousedEf, effect.Spec{effect.Lasts(10 * time.Second)})
burning, smouldering, doused := fx.Named(BurningEf), fx.Named(SmoulderingEf), fx.Named(DousedEf)

// whatever can burn plays one role: the units' kinds and the kinds of cell alike
roles.Define(FlammableRole,
	// fire spreads to whom a burning one touches, now and then, unless they are wet
	rule.Branch[collision.Meeting]{
		Name: "fire spreads",
		When: rule.Under(burning),
		On:   rule.Other,
		Do: rule.Branch[any]{
			When: rule.All(rule.Not(rule.Under(doused)), rule.Chance(0.3)),
			Do:   rule.Put(burning),
		},
	},
	// water puts a burning one out — nothing smoulders then — and the wet do not catch fire for a
	// while; one on dry ground sets it alight now and then
	rule.Branch[unit.Standing]{
		Name: "where it burns",
		When: rule.Under(burning),
		On:   rule.Self,
		Do: rule.OneOf(
			rule.Branch[unit.Standing]{
				When: inWater,
				Do:   rule.Steps(rule.Put(doused), rule.Take(burning)),
			},
			rule.Branch[any]{When: rule.Chance(0.1), On: rule.Here, Do: rule.Put(burning)},
		),
	},
	// burning ground sets the cells round it alight now and then — not one burning or burnt out
	rule.Branch[cell.Now]{
		Name: "fire spreads over the ground",
		When: rule.Under(burning),
		On:   rule.Around(1),
		Do: rule.Branch[any]{
			When: rule.All(
				rule.Not(rule.Under(burning)),
				rule.Not(rule.Under(smouldering)),
				rule.Chance(0.05),
			),
			Do:   rule.Put(burning),
		},
	},
)
kinds.Define("grass", cell.Kind{Cost: 2, Allows: cell.Land}, roles.Named(FlammableRole))
kinds.Define("forest", cell.Kind{Cost: 3, Allows: cell.Land}, roles.Named(FlammableRole))
units.Define(WalkerKind, land, profile, rule.Plays(roles.Named(FlammableRole)))

func inWater(s unit.Standing) bool { return s.Kind.Admits(cell.Water) }
```

The engine hands the first rule to collision and the other two to the board: the game names no
plugin. One effect serves units and cells: an `Alter` of a component the entity does not carry is
passed over. `Around` takes in the places stood on too — a cell itself — so the spreading rule
keeps off what burns already, or a cell would keep itself burning for ever.

Another plugin, added later, adds its own rules for fire — vision's units flee from what they see
burning, a rule of a `Sighting` with `When: rule.Under(burning)` on the other's side — without
touching these: the effect is the common word.

Where a rule cannot say what the game needs, a moment, a step or a knob is missing; it is added
to the plugin, not written as code in a rule. Fire showed three, added on 2026-10-01: acting on
the places round (`Here`, `Around`), a board moment of a cell (`cell.Now`), and the clock's
moment as its own entity's, so that a clock rule puts a phase on.

## Later

The same words serve formations and escorts ("follow me"), handing over a load, combat ("cover
me", "fall back"), and an AI player's orders to its units; messages and commands are deterministic,
fit to replay and to send over a network.

Not yet there: which effects a player may apply (any, today, on whatever a command names
— a game over a network will want a list); an effect taking a tag off while it runs (a
`Revoke` beside `Grant`); `Here` and `Around` in plans; a step shared by the moments of several
plugins; more than `MaxEffects` (8) effects on one entity and 63 defined; an entity in more than
one group; a dictionary of idioms (`Once`, `Either`, the patrol) written out of the bricks.
