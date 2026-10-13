# Rule: how entities behave

One vocabulary for every behaviour of an entity, in `rule` and its packages: a **rule** is what
is done at a moment a plugin catches, a **plan** is what an entity does over time, an **effect**
is a change that holds, a **command** is what an entity has done, and a **fact** is what a plugin
tells an entity. A branch (`rule/act`) says when what is done, a loop (`rule/plan`) when what is
done over time and when it is over, every clause of either (a condition, `rule/cond`, or a step)
saying whom it is about; `rule` ties them to the game: the roles whose players obey rules, the
commands about effects, the messages between plans.

## Two bricks: a Branch and a Loop

A rule is a sentence — **who, when, what** — and an `act.Branch` is all but the who: `When` the
condition, `Then` the steps, each clause of either said of a subject (`cond.Self.If(x)`,
`act.Other.Do(cmd)`). Who is the role the branch is given to, so the rules of a role read as a
paragraph under it. The type `P` a Branch is of is the moment it fires on; `any` for a branch
inside another, asking only of state. A plan is made of `plan.Loop`s, a Branch with an end: its
`P` is the fact it waits for, or `any`; `Until` says when it is done, `Timeout` when it has
failed, `Cooldown` how soon it may begin again.

```go
// rules: the players of mortal fall in; the predator chases what it sees, and turns to look
// round when it sees nothing
roles.Define(MortalRole, act.Branch[unit.Standing]{
	Name: "fall in",
	When: cond.Self.If(unit.Fallen()),
	Then: act.Self.Do(world.Despawn{}),
})
roles.Define(PredatorRole,
	act.Branch[vision.Sighting]{
		Name: "chase",
		When: cond.Other.If(prey),
		Then: act.Self.Do(steering.Toward{}),
	},
	act.Branch[vision.Sighting]{
		Name: "search",
		When: cond.Self.If(vision.SeesNobody()),
		Then: act.Branch[any]{
			When: cond.Self.If(cond.Not(turned)),
			Then: act.Steps(
				act.Self.Put(turned),
				act.OneOf(
					act.Branch[any]{
						When: cond.Self.If(cond.Chance(0.5)),
						Then: act.Self.Do(left),
					},
					act.Self.Do(right),
				),
			),
		},
	},
	act.Branch[collision.Meeting]{
		Name: "caught",
		When: cond.Other.If(prey),
		Then: act.Other.Do(world.Despawn{}),
	},
)

// a plan: what a guard does over time, loops choosing
guard := plan.New(GuardPlan, plan.OneOf(
	plan.Loop[any]{
		Name:    "to the gate",
		When:    cond.On(s.world).If(alarm),
		Then:    plan.Self.Do(navigation.MoveTo{To: entity.Named(GateName)}),
		Until:   cond.Self.If(navigation.Arrived{}),
		Timeout: 20 * time.Second,
	},
	plan.Loop[navigation.Blocked]{
		Name: "blocked",
		When: cond.Self.If(navigation.Stuck()),
		Then: plan.Self.Do(navigation.StepAside{}),
	},
	plan.Loop[navigation.Arrived]{
		Name: "rest",
		When: cond.Self.If(cond.Always),
		Then: plan.Self.Wait(2 * time.Second),
	},
	plan.Loop[any]{
		Name:  "patrol",
		When:  cond.Self.If(cond.Always),
		Then:  plan.Self.Do(navigation.Onward{}),
		Until: cond.Self.If(navigation.Arrived{}),
	},
	plan.Loop[any]{Name: "idle", When: cond.Self.If(cond.Always), Then: plan.Self.Idle()},
))
units.Define(GuardKind, land, profile, guard,
	comp.Load(func(g guardRow) navigation.Round { return navigation.NewRound(g.Round[:]...) }))
```

The grammar:

```
rule    = act.Branch[Moment]                   // given to a role: roles.Define(role, rules...)
plan    = plan.New(name, root)                 // given to a kind, in its Spec
root    = plan.OneOf(part...) | plan.Steps(part...) | loop
branch  = act.Branch[P]{Name, When, Then}      // Name at the top; When and Then always
loop    = plan.Loop[P]{Name, When, Then, Until, Timeout, Cooldown}
P       = Moment (a branch) | Fact (a loop) | any
When    = cond.Cond                            // a loop's Until the same: done once it holds
Cond    = clause | Cond.And(Cond) | Cond.Or(Cond) | cond.Not(Cond)
clause  = subject.If(what)
subject = cond.Self | cond.Other | cond.Around(n) | cond.On(plugin)
what    = effect | role | word | fact{} | cond.Chance(p) | cond.Always | cond.Not(what)
          // fact{}: in Until alone, of Self alone
word    = unit.Fallen() | unit.StandingOn(e) | vision.SeesNobody() | clock.Every(p, o) | ...
Then    = step | act.Steps(Then...) | act.OneOf(Then...) | branch    // a branch's
part    = Then | plan.Self.Wait(d) | plan.Self.Idle()                // a loop's
        | plan.Steps(part...) | plan.OneOf(part...) | loop
step    = actor.Put(e) | actor.Take(e) | actor.Hold(e) | actor.Do(cmd)
actor   = act.Self | act.Other | act.Around(n)   // in a plan: plan.Self, plan.Other
```

- A Branch given to a role (`world.Roles.Define`) is a rule of `P` for the plugin that catches
  `P`, obeyed by the role (below, "Who gets a rule"); its `Name` is for the errors about it. A
  Branch inside another's `Then` asks more of the same `P`; one of `any` asks only of state. Every
  Branch says its `When` (`cond.Self.If(cond.Always)` for every time) and its `Then`; one missing
  panics by the branch's name. So does what the rule cannot run, as it is built, in `Init`: a
  `Not` of a state outside a clause, a word of the wrong side, a fact in a `When`, `Around` where
  `P` is not Placed, a Branch of another moment inside, a message, a Branch at the top of a rule
  without a `Name`. A plan's parts — `Wait`, `Idle`, a Loop — do not compile in a rule's `Then`.
- `plan.New(name, root)` is a plan, a value a kind carries in its Spec as it carries
  `rule.Plays`; the world defines it as the kind is registered, under the name a save knows it
  by, and runs it in every step of its simulation. `OneOf` is run every tick, from the top, the
  first loop whose When holds; an earlier one, once it can, takes over from a later one, which
  is stopped. `Steps` runs its parts one after another and remembers where it is. A plan done
  begins again the next tick. Every loop at the top of a plan — a part of its root — says its
  `Name`; one inside a `Then` goes by the outer's. A rule's steps and groups stand in a plan too.
- **A fact is a thing that happened.** A Loop of a fact `P` — a component a plugin writes for
  the entities with a plan: `navigation.Arrived`, `Blocked`, a message — begins once as the fact
  comes to hold its When (it was absent, or its word did not hold; one carried already counts
  once), runs its Then to the end though the fact goes, and does not begin again until the fact
  comes afresh. A Loop of `any` is on a state: it runs while its When holds and is stopped when
  it stops. So the guard rests once on arriving, and runs to the gate while the alarm is on and
  back to the patrol when it is off.
- **The end is in the head.** `Then` is steps alone, run once from the start; a loop with an
  `Until` then waits for it. `Until` is checked every tick from the loop's start, before the
  steps: once it holds the loop is done, its steps stopped — before any, when it holds at the
  start. A fact's value in it (`cond.Self.If(navigation.Arrived{})`) is the fact come afresh
  since the loop began, one carried from before not counting; a word beside it holds while the
  fact is carried and the word holds. `Timeout` fails the loop once it has run that long — in a
  `OneOf` the next part takes over — and `Cooldown` keeps it from beginning again within that of
  its last run. `Wait(d)` and `Idle()` are steps of `plan.Self` alone; an `Until` of `Other` is
  refused. A rule runs in one pass and has none of them.
- **Defining registers and hands nothing back.** Effects (`world.Effects`), roles
  (`world.Roles`), commands (`world.Commands`) and kinds (`kind.Define`, `units.Define`) are each
  defined under a name in the Stage's world, and `Named(name)` is the thing wherever it is built
  on; a plan is carried, not looked up. A game keeps its names as constants (a `names.go`), each
  with a suffix saying what it names — `BloodMoonEf`, `BleedCmd`, `MortalRole`, `GuardPlan`,
  `ScoutKind`, `GrassCell` — so a name mistyped does not compile, and a Stage's struct holds its
  plugins alone. Each Stage has registers of its own: nothing a game defines is the program's.
- A function writing part of a rule returns an `act.Step`, part of a plan a `plan.Part`, a whole
  rule an `act.Rule`, a whole plan an `act.Planned`. A plugin ships no ready-made reactions: it
  gives moments and their words, the game writes the rule.

## Conditions

A `When` is a `cond.Cond`: clauses joined with `And` and `Or`, `cond.Not` of a whole condition
turning it round. A clause is a subject in a state, `subject.If(what)`:

- **whom**: `cond.Self`, the entity the branch is about; `cond.Other`, the other of the pair the
  moment met, or the subject of the moment or the fact — the nearest seen, the one touched, who
  sent a message — which does not hold while there is none; `cond.Around(n)`, each place within
  `n` rings of where the entity stands, in a rule of a Placed moment (below, "Where one
  stands"); `cond.On(plugin)`, a plugin's own entity: `cond.On(s.world).If(alarm)` a state of
  the whole game, `cond.On(s.atmosphere).If(bloodMoon)` one of the sky;
- **what**: an effect, the one under it; a role, the one playing it; a word of the moment or the
  fact, a plugin's own — `unit.Fallen()`, `unit.StandingOn(iced)`, `unit.OnKind(k)`,
  `vision.SeesNobody()`, `vision.Closing()`, `navigation.Stuck()`, `bullet.Within(d)`,
  `bullet.Lying()`, `gravity.HarderThan(v)`, `sky.FullMoon()`, `clock.At(dusk)`,
  `clock.Every(period, offset)` — or a game's own fact's, `cond.NewWord(name, func(F) bool)`;
  `cond.Chance(p)`, drawn for whom it is asked, afresh every step from the world's seed, the game
  time and that entity, so a load and a replay draw alike; `cond.Always`; `cond.Not` of any of
  them; in a loop's `Until` alone, a fact's value (`navigation.Arrived{}`), the fact come afresh
  — in a `When` a fact is the loop's type, `plan.Loop[F]`, and is refused as a state.

A word is of the entity or of the other (`vision.Closing()` and `bullet.Within(d)` are of the
other), and a clause of the wrong side is refused as the branch is built. A role of `Self` or of
`Other` at the top of a rule — in its `And` chain — the plugin reads off its own query, on the
tag bits, so a rule that does not concern an entity costs nothing for it; anywhere else a role
is asked of the entity as the branch runs. A role narrows a rule further, on top of its When
(below, "Roles").

## Who gets a rule

Nobody hands a rule to a plugin: there is no hooking. A rule is obeyed by a role, and a role is
played — by a kind of entity, by a kind of cell, by a plugin:

```go
roles := s.world.Roles()
roles.Define(MortalRole, fallIn)   // a rule of unit.Standing: the board's
roles.Define(PlateRole, press)     // a rule of collision.Meeting: collision's
roles.Define(NightlyRole, atDusk)  // a rule of clock.Moment: the world's

// a kind of unit plays it
units.Define(ScoutKind, land, profile, rule.Plays(roles.Named(MortalRole)))
// a kind standing on the board
plates.Define(PlateKind, comp.Const(collision.Collider{}), rule.Plays(roles.Named(PlateRole)))
// a kind of cell, as it is defined
kinds.Define("lava", cell.Kind{Cost: 9}, roles.Named(ScorchingRole))
// one cell of the Layout alone
layout.Cells = append(layout.Cells, water.Entry(c).Plays(lakeRole))
// the world itself
s.world.Plays(roles.Named(NightlyRole))
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
// the plugin is whom the command is for, and a rule reads its state
cmds.Define(BleedCmd, rule.Put(fx.Named(BloodMoonEf)).On(s.atmosphere))
act.Branch[world.Moving]{
	Name: "frenzy",
	When: cond.On(s.atmosphere).If(fx.Named(BloodMoonEf)),
	Then: act.Self.Hold(fx.Named(FrenziedEf)),
}
```

## The five words

- **Rule** — a moment a plugin catches: a `unit.Standing` or a `cell.Now`, a `vision.Sighting`, a
  `collision.Meeting` (pairs) or `Struck`, a `world.Moving` or `Leaving`, a `clock.Moment`, a
  `climate.Weathering`, a `navigation.Touch`. A rule's steps are instant — `OneOf`, `Steps`, a
  Branch asking more, `Put`, `Take`, `Hold`, `Do` — so what lasts (`Wait`, `Idle`, a Loop) does
  not compile in one, and a message is refused as the rule is made. A rule remembers nothing of
  its own, and runs no Go code of its own: there is no step for it. What the steps cannot say is
  a moment, a word, a step or a knob the plugin still lacks.
- **Plan** — what a kind's entities do over time (`plan.New`). It remembers its place (its
  mind, the component the plan is: the loops running, each one's place and start on the world's
  clock), waits (`Until`, `Wait`), talks to other entities (messages), and is saved with the
  game. `OneOf` is a reactive choice, `Steps` a sequence with memory; a Loop of a fact runs once
  as the fact comes, to the end.
- **Effect** — `effect.Effect`, defined once from a `Spec` (`Lasts`, `Stacking`, `Then`, `Grant`,
  `Alter`; `Described` a sentence for a UI, `Description()`), with its own marker, on while it
  runs (`Mark()`). `Put` puts it on; `Hold` holds it as long as its branch runs, or as long as a
  rule keeps firing it; `Take` takes it off. **An effect's presence is state**: a branch
  `When: cond.Self.If(cond.Not(turned))` that puts `turned` on is "at most once a while", a
  memory for rules that keep none, and its marker is what rules of other plugins ask for
  (`cond.Self.If(e)`, `cond.Other.If(e)`). The collision demo's hit is one (put on at every
  `collision.Struck`, drawn by `render.Over(overlay, hit.Mark().In)`).
- **Command** — `act.Self.Do(cmd)` gives the command of whom the step is said of, the same one a
  player gives (`navigation.MoveTo`, `world.Despawn`), queued for the plugin that handles its
  type, and goes on at once: **fire and forget**. The handler carries it out for the entity alone
  (`control.Issued.ByEntity`); `world.Spawn{Entry}` is the one that makes another entity, of the
  Entry fixed as the rule is written (a nest laying an egg where it stands). The world keeps a
  stage's one carrier (`control.Carrier`, `world.Plugin.Commands`): the players give it theirs,
  the entities theirs; the engine carries every `plugin.CommandHandler` a stage uses, and a
  plugin's `plugin.Tick` hands the carrier to its rules. Nothing is dropped: a command waits for
  its handler's pass — given after it, for the next frame's. A command that is `plugin.Aimed` is
  told the subject of the fact it stands under, or of the rule's moment: whom the entity touched,
  who asked. A command embedding `rule.From`, given with `plan.Other.Do(…)`, is a message
  (below).
- **Fact** — a component a plugin writes for entities with a `Mind` alone (navigation's
  `Blocked`, `Arrived`), so entities without a plan pay nothing. Facts are how a plan learns what
  came of its commands; a fact comes, and a loop of it runs once, or one waiting for it is done.

## Idioms

- **Give, then wait for what comes of it.** `plan.Loop[any]{…, Then:
  plan.Self.Do(navigation.MoveTo{…}), Until: cond.Self.If(navigation.Arrived{})}`: a fact in an
  `Until` is one come *afresh* — a fact left from before does not count — and a word beside it,
  `.And(cond.Self.If(word))`, waits for it to come to hold the word.
- **A patrol is two loops.** The unit carries its own `navigation.Round` of cells (from its row,
  `comp.Load`), and `plan.Steps` of a loop giving `navigation.Onward{}` `Until` `Arrived` comes
  and one standing a while (`plan.Self.Wait(pause)`) goes round, the plan done beginning again
  (the guard demo's wanderers). A step aside on the way carries the goal over (navigation queues
  it after), so `Arrived` still comes.
- **No answer is a timeout, not a fact.** `plan.Loop[any]{When: cond.Self.If(cond.Always),
  Then: plan.Self.Idle(), Until: cond.Self.If(reply{}), Timeout: d}` as the first part of a
  `plan.OneOf`, the part after it what to do then.
- **Not too often.** `Cooldown: time.Second` on a loop giving `jump{}` has it leap no sooner
  than a second after the last.
- **A rule's memory is an effect.** `When: cond.Self.If(cond.Not(e))` with `act.Self.Put(e)`
  among the steps runs a thing and not again for as long as `e` lasts.
- **The clock's moment is the clock's.** A `clock.Moment` is of the clock's own entity: an effect
  a clock rule puts on lands there — `act.Branch[clock.Moment]{Name: "dusk", When:
  cond.Self.If(clock.At(dusk)), Then: act.Self.Put(night)}`, a phase that `clock.Clock.In`
  reads. Run once a step, walking no entities, it fires while the clock's plugin, the world,
  plays its role.
- **Where one stands.** On a moment that is `plugin.Placed` — a `unit.Standing`, a cell's
  `cell.Now` — `cond.Around(n).If(x)` and `act.Around(n).Put(e)` turn to each place within `n`
  rings of the cells under the entity (for a cell, of itself): the cell and the entities standing
  on it, each once, on square and hex boards alike; `0` the cells under it alone. The smallest
  part of the rule holding every clause of a place runs once a place, one ring a rule: an effect
  put on the ground round a witch, a lever pulled beside a scout. The cell under a unit is a word
  of its `Standing`: `unit.StandingOn(e)`, `unit.OnKind(k)`. The units there are no part of it.
- **Now and then.** `cond.Self.If(cond.Chance(p))` holds with likelihood `p`; a coin is
  `act.OneOf(act.Branch[any]{When: cond.Self.If(cond.Chance(0.5)), Then: a}, b)`.
- **One after another.** `effect.Then(next)` casts `next` when an effect's time is up — burning
  leaves smouldering — but not when it is taken off: put out, nothing smoulders.

## Commands

Whatever somebody asks for — a player's key, a script, an AI, a branch's `Do` — is a
command, and a command about an effect is one sentence (`rule.Command`):

```go
openWest := rule.Put(open).On(entity.Group("west trapdoors")).By(entity.Named("west lever"))
flipGate := rule.Toggle(ajar).On(entity.Group("gate"))
alarm    := rule.Put(alarmed).On(entity.World).For(30 * time.Second)
hasten   := rule.Put(haste).On(s.selection.Selected(hasty))
freeze   := rule.Put(frozen).On(s.selection.Pointed()).For(3 * time.Second)
```

- **The verb**: `rule.Put` puts the effect on, `rule.Take` takes it off, `rule.Toggle` takes it
  off them all where any is under it and puts it on them all otherwise — a switch. In a branch
  `act.Self.Put(e)` and `act.Other.Take(e)` are steps, acting on whom they are said of; a
  command with an `On` is given as any command, `act.Self.Do(rule.Put(alarmed).On(entity.World))`.
- **Whom** (`On`): `entity.Named(names...)`, the entities bearing those names, one each;
  `entity.Group(names...)`, all those in the groups; `entity.World`, the world's own entity — a
  state of the whole game; or a plugin's own: the selection's `Selected(roles...)`, the units the
  player who gives the command has selected (those playing a role named, when any is), and
  `Pointed()`, the entity drawn under the cursor as the key is pressed.
- **How long** (`For`): in place of what the effect's `Spec` says.
- **Who sets it off** (`By`): the entities that give themselves `rule.Trigger{}` — a lever, a
  plate. A role then says only *that* a plate stood on triggers; the command says what it opens.

An entity is called by what makes it: a cell by its `cell.Entry{Name, Group}`, a unit by its
`kind.Entry` (`Named`, `InGroup`); a name is one entity's, a group many's, and both are saved. The
game defines its commands by name in `s.world.Commands()` (`Define(name, cmd)`, in the Commands
section; `Named(name)` is the command for a key): those with a `By` are kept for `Trigger`, and
every name they say is checked as the game starts — a name nobody bears, or one two bear, stops it
there. A command is given the same way whoever gives it:

```go
openWest := s.world.Commands().Named(OpenWestCmd)
s.player.Bind(control.Give(control.KeyPress{Key: control.Key1}, "Pull the west lever",
	openWest))                       // a key
s.players.Issue(ai, openWest)        // an AI, a script
act.Self.Do(openWest)                // a rule; plan.Self.Do(openWest) a plan
act.Self.Do(rule.Trigger{})          // its source, pressed
```

The plugin its target belongs to carries it out — the world for those named, grouped and the
world itself, the selection for its own — a step later, with the step's effects; commands of one
effect for one entity in one step are netted, so a switch flipped twice stays as it was.
Other plugins' commands are theirs: `navigation.MoveTo` (a cell, or an entity by name:
`MoveTo{To: entity.Named(GateName)}`), `world.Despawn`, `bullet.Shoot{Ammo}` (a shot from the
player's selected units, or from the entity that gives it, aimed at its moment's subject).

Rules and plans read the world's states with `cond.On(s.world).If(e)`, as they read an entity's
with `cond.Self.If(e)`:

```go
fx.Define(AlarmEf, effect.Spec{effect.Lasts(30 * time.Second)})
fx.Define(AlertEf, effect.Spec{effect.Alter(func(st *steering.Steering) { st.MaxSpeed *= 1.5 })})
roles.Define(GuardRole, act.Branch[unit.Standing]{
	Name: "hurry",
	When: cond.On(s.world).If(fx.Named(AlarmEf)),
	Then: act.Self.Hold(fx.Named(AlertEf)),
})
cmds.Define(SoundAlarmCmd, rule.Put(fx.Named(AlarmEf)).On(entity.World))
s.player.Bind(control.Give(control.KeyPress{Key: control.KeyN}, "Sound the alarm",
	cmds.Named(SoundAlarmCmd)))
```

The guards hurry while the alarm sounds and slow down once it is over: `Hold` holds `alert` only
while the rule fires it.

The wire demo (`examples/rules/wire/demo.go`) is a whole program of it: a lever, a plate and a
switch, each a command for a group of trapdoors or a gate, entities standing on the board; the
trapdoor and pressure plate demos are the same in less.

## Messages

A plan talks to another entity with the bricks it has. A command embedding `rule.From`, given
with `plan.Other.Do(…)`, lands on that entity as a fact in place of going to a queue, the engine
filling in who sent it; one without a plan takes none, and the step fails. A Loop on the
command's type is how the other answers — its `Other` is the sender, and `plan.Other.Do(…)`
again the reply, a command of its own — or passes it on, `Other` under a loop of another fact
being that fact's subject. A message nobody takes up is dropped after `AskLife`; one passed on
`MaxChain` times is not sent, so a chain ends. Delivery a tick later means no recursion within a
tick.

```go
type pass struct{ rule.From }
type reply struct {
	rule.From
	Yes bool
}

func (r reply) yes() bool { return r.Yes }

// the asker: let me pass; walks on let through, flees turned away or unanswered
plan.Loop[navigation.Blocked]{
	Name: "ask to pass",
	When: cond.Self.If(navigation.Stuck()),
	Then: plan.OneOf(
		plan.Loop[any]{
			When:    cond.Self.If(cond.Always),
			Then:    plan.Other.Do(pass{}),
			Until:   cond.Self.If(reply{}),
			Timeout: time.Second,
		},
		plan.Self.Do(flee{}),
	),
},
plan.Loop[reply]{
	Name: "let through",
	When: cond.Self.If(cond.NewWord("yes", reply.yes)),
	Then: plan.Self.Do(walk{}),
},
plan.Loop[reply]{
	Name: "turned away",
	When: cond.Self.If(cond.Not(cond.NewWord("yes", reply.yes))),
	Then: plan.Self.Do(flee{}),
},

// the one asked: lets pass where it has room, jumping aside; asks its ally on where it has one
plan.Loop[pass]{
	Name: "asked to let pass",
	When: cond.Self.If(cond.Always),
	Then: plan.OneOf(
		plan.Loop[room]{
			When: cond.Self.If(cond.NewWord("free", room.free)),
			Then: plan.Steps(
				plan.Other.Do(reply{Yes: true}),
				plan.Self.Do(jump{}),
			),
		},
		plan.Loop[room]{
			When: cond.Self.If(cond.NewWord("by an ally", room.byAlly)),
			Then: plan.Other.Do(pass{}),
		},
		plan.Other.Do(reply{}),
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

roles.Define(MortalRole, act.Branch[unit.Standing]{
	Name: "fall in",
	When: cond.Self.If(unit.Fallen()),
	Then: act.Self.Do(world.Despawn{}),
})
roles.Define(HastyRole)

cmds.Define(HastenCmd, rule.Put(fx.Named(HasteEf)).On(s.selection.Selected(roles.Named(HastyRole))))

units.Define(ScoutKind, land, profile, rule.Plays(roles.Named(MortalRole), roles.Named(HastyRole)))
units.Define(PorterKind, land, laden, rule.Plays(roles.Named(MortalRole)))
s.player.Bind(control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts",
	cmds.Named(HastenCmd)))

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

`Obeys` narrows a rule on top of its own When: a rule of `cond.Self.If(hungry)` obeyed by mortal
fires for a mortal entity while it is hungry, one of `cond.Self.If(hasty)` for a mortal that is
hasty. On a moment that is `plugin.Met` it fires for the pairs whose own entity plays the role.
A collision `Meeting` rule with no `cond.Other.If(role)`, or whose other's role is its own, still
runs at most once a pair; a `Touch` belongs to each of the two units, a `Sighting` to each
observer — and its `cond.Other.If(role)` narrows what the observer sees, not whether the rule
fires. An entity playing two roles that obey the same rule obeys it once for each. A rule of a
moment of the world as a whole — a `clock.Moment`, a `climate.Weathering`, run once a step
walking no entities — asks nothing of the entity and fires while the plugin whose moment it is
plays its role.

A role in a clause holds while the one the clause is about plays it; under
`cond.Around(n).If(role)` that is the place turned to: the wire demo's handy scouts pull the
lever beside them with `When: cond.Self.If(pull).And(cond.Around(1).If(lever)), Then:
act.Around(1).Do(rule.Trigger{})`, and nothing else there.

A program names 64 roles at most, one name one tag, which every world saves by the name. Make them
in `Init`: the world names them in its kinds as the Stage's ECS is set up. A kind names
`rule.Plays` once, every role it plays in it; a kind giving a component type twice panics.

### Groups and behaviours

**Roles mean behaviour, names and groups mean which ones.** One rule of the role plate serves
every plate: which doors a plate opens is the command naming it. A hundred levers are a hundred
commands, a hundred names and the same rules — never a role or an effect each. A game defines no
tags: tags are how the plugins keep their own states and groups — whose a unit is, whether it is
selected — changed by their commands (`players.Give`, `selection.Allow`); `cond.Self.If(e)` asks
for an effect, and `cond.Other.If(role)` lets through the pairs whose other plays a role:
`roles.Define(PredatorRole, act.Branch[vision.Sighting]{Name: "chase", When:
cond.Other.If(prey), …})`.

## Knobs

A plugin gives a game **knobs**: components it only reads, which an effect's `Alter` turns and
puts back when it ends — a cell's `cell.Ground`, `steering.Steering` (how fast an entity goes,
speeds up, brakes; `Halted` holds it), `vision.Sight` (how far, which way; `Ahead` looks the way
it moves), `collision.Physics`, `world.Appearance`. What a plugin's system writes lives beside
the knob in a component of its own — `steering.Course` beside `Steering`, `vision.Sighted` beside
`Sight` — given where it is missing: an `Alter` ending puts back the original, and would put back
stale state with it. A new knob goes into the component a plugin reads, never into its state.

## No Go code in a rule

A rule holds no Go code of its own but its words. What a plugin does of its own is its own work
in a pass it makes anyway, never a rule of another plugin's moment:

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
act.Branch[vision.Sighting]{
	Name: "chase",
	When: cond.Other.If(prey),
	Then: act.Self.Do(steering.Toward{}),
}
act.Branch[vision.Sighting]{
	Name: "give way",
	When: cond.Other.If(vision.Closing()),
	Then: act.Self.Do(steering.Away{}),
}
```

A switch of a behaviour for the whole game is an effect on the world (`rule.Put`, `Take` or
`Toggle` on `entity.World`), the rule running `cond.On(s.world).If(e)` it (the vision demo's
fleeing).

How an entity is drawn is the one place rules are Go: `render.Over`, `As`, `Swap`, `With` and
`Show`, given on the world's atlas in a scene's Layers (and `vision.Plugin.WithViews`, which views
are drawn), run every frame. They read a component and decide nothing in the game.

**A state of the ground.** A cell stays the kind it is; an effect on it turns the kind's knobs
(`g.Kind.Allows`, `Cost`) and is drawn as a cover, `board.Plugin.Covering(e)` — a texture a state,
laid along a line of its own. A rule asks for the state under a unit, not for a kind:
`When: cond.Self.If(unit.StandingOn(iced)), Then: act.Self.Hold(slip)`. Which cells take which
state is a role they play: `act.OneOf(act.Branch[any]{When: cond.Around(1).If(lake), Then:
act.Around(1).Put(iced)}, act.Around(1).Put(frost))`.

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
knob for one look for every kind.

## Take and order

Many rules may touch one effect, from several plugins. What happens is fixed:

- A rule that `Hold`s an effect someone took off has it back the step after: its cause goes on,
  so the effect does. A taker that is to win puts on a shield the holder keeps clear of —
  `act.Steps(act.Self.Put(doused), act.Self.Take(burning))` against
  `act.Branch[any]{When: cond.Self.If(cond.Not(doused)), Then: act.Self.Hold(burning)}`.
- A plan's `Hold` gives way when someone else takes its effect off: the loop fails and the plan
  goes on to what it does next.
- A `Put` after a `Take` in the same step takes the slot back.
- In a step, rules of one moment run in the order their roles came to be played; moments, in the
  order of their plugins' passes — that is the systems' order, not something a rule picks. Every
  `Put` and `Take` of a step lands together in the effects' pass at its end.

## Writing a new behaviour

1. A moment a plugin already catches, instant: a rule on it; its memory, an effect; what it
   changes, a knob the effect alters — a component the plugin only reads, added where missing.
2. Something that lasts: name the facts the plugin can perceive and write them for units with a
   `Mind` only; name the commands and handle them in the plugin (a `CommandHandler`'s queue),
   carrying them out for the entity that gives them, the engine's own rules inside; tell what came
   of them as facts.
3. Write the plan from short named loops (`plan.New`) and give it to the kinds. A plan holds
   at most 128 steps.
4. Keep the defaults a plugin gives its own host unexported; a game adds its own as roles its
   kinds play.
5. Test the scenario as ticks: who is where, which facts, which commands, until it ends.

Example — navigation's crowd, the rules of units among others, as in StarCraft II. Navigation
perceives — the moment `Touch`, two units touching, with what each is doing and whether the one
standing has room, in words of its own — and carries out the commands, keeping its own rules
inside them (a unit is never stepped into water, off a cliff or into a wall); the rules decide:

```go
func makeWay() act.Rule {
	return act.Branch[Touch]{
		Name: "navigation.make way",
		When: cond.Self.If(PushedByAlly()),
		Then: act.Self.Do(StepAside{}),
	}
}

func joinTheGroup() act.Rule {
	return act.Branch[Touch]{
		Name: "navigation.join the group",
		When: cond.Self.If(ReachedTheGroup()),
		Then: act.Self.Do(Stop{}),
	}
}

func goRound() act.Rule {
	return act.Branch[Touch]{
		Name: "navigation.go round",
		When: cond.Self.If(cond.Always),
		Then: act.OneOf(
			act.Branch[Touch]{When: cond.Self.If(GoalTaken()), Then: act.Self.Do(Settle{})},
			act.Branch[Touch]{When: cond.Self.If(WaitsFirst()), Then: act.Self.Do(Hold{})},
			act.Branch[Touch]{When: cond.Self.If(NoWayRound()), Then: act.Self.Do(StepAside{})},
			act.Branch[Touch]{When: cond.Self.If(ClearsTheWay()), Then: act.Self.Do(Pass{})},
			act.Branch[Touch]{When: cond.Self.If(Blocks()), Then: act.Self.Do(Detour{})},
		),
	}
}
```

`StepAside`, `Pass`, `Detour` and `Settle` are `Aimed`: each is told the moment's subject, the
unit touched. A game adds its own rules as a role's, for the units playing it —
`roles.Define(GuardRole, act.Branch[navigation.Touch]{Name: "hold the line", …})` — or gives its
own set in place of the crowd's with `WithCrowd`. What a unit does over time is its plan: the
guard demo's guards patrol their own round, step aside when blocked long, rest where they arrive
and run to the gate, named on the map, while the alarm is on (above, "Two bricks").

## A game: states as effects

A game built on the plugins is written in three parts, and none of them is Go code inside a rule:

1. **States are effects.** Burning, frozen, alarmed, doused, a lever pulled: each defined once,
   for a while or for good, with its own marker on while it runs, on a unit, a cell or the world.
   One effect may lead to the next (`Then`).
2. **Rules connect.** A rule turns a plugin's moment — a touch, a sighting, where one
   stands — into effects: `Put`, `Hold`, `Take`, under a state (`cond.Self.If(e)`) or obeyed by
   a role, of the entity, the other or the places round (`Self`, `Other`, `Around`), now and
   then (`cond.Chance`).
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
inWater := cond.NewWord("in water",
	func(s unit.Standing) bool { return s.Kind.Admits(cell.Water) })

// whatever can burn plays one role: the units' kinds and the kinds of cell alike
roles.Define(FlammableRole,
	// fire spreads to whom a burning one touches, now and then, unless they are wet
	act.Branch[collision.Meeting]{
		Name: "fire spreads",
		When: cond.Self.If(burning).
			And(cond.Other.If(cond.Not(doused))).
			And(cond.Other.If(cond.Chance(0.3))),
		Then: act.Other.Put(burning),
	},
	// water puts a burning one out — nothing smoulders then — and the wet do not catch fire for a
	// while; one on dry ground sets it alight now and then
	act.Branch[unit.Standing]{
		Name: "where it burns",
		When: cond.Self.If(burning),
		Then: act.OneOf(
			act.Branch[unit.Standing]{
				When: cond.Self.If(inWater),
				Then: act.Steps(act.Self.Put(doused), act.Self.Take(burning)),
			},
			act.Branch[any]{
				When: cond.Self.If(cond.Chance(0.1)),
				Then: act.Around(0).Put(burning),
			},
		),
	},
	// burning ground sets the cells round it alight now and then — not one burning or burnt out
	act.Branch[cell.Now]{
		Name: "fire spreads over the ground",
		When: cond.Self.If(burning).
			And(cond.Around(1).If(cond.Not(burning))).
			And(cond.Around(1).If(cond.Not(smouldering))).
			And(cond.Around(1).If(cond.Chance(0.05))),
		Then: act.Around(1).Put(burning),
	},
)
kinds.Define("grass", cell.Kind{Cost: 2, Allows: cell.Land}, roles.Named(FlammableRole))
kinds.Define("forest", cell.Kind{Cost: 3, Allows: cell.Land}, roles.Named(FlammableRole))
units.Define(WalkerKind, land, profile, rule.Plays(roles.Named(FlammableRole)))
```

The engine hands the first rule to collision and the other two to the board: the game names no
plugin. One effect serves units and cells: an `Alter` of a component the entity does not carry is
passed over. `Around` takes in the places stood on too — a cell itself — so the spreading rule
keeps off what burns already, or a cell would keep itself burning for ever.

Another plugin, added later, adds its own rules for fire — vision's units flee from what they see
burning, a rule of a `Sighting` with `When: cond.Other.If(burning)` — without touching these: the
effect is the common word.

Where a rule cannot say what the game needs, a moment, a word, a step or a knob is missing; it is
added to the plugin, not written as code in a rule. Fire needed three: acting on the places round
(`Around`), a board moment of a cell (`cell.Now`), and the clock's moment as its own entity's, so
that a clock rule puts a phase on.

## Later

The same words serve formations and escorts ("follow me"), handing over a load, combat ("cover
me", "fall back"), and an AI player's orders to its units; messages and commands are deterministic,
fit to replay and to send over a network.

Not yet there: which effects a player may apply (any, today, on whatever a command names
— a game over a network will want a list); an effect taking a tag off while it runs (a
`Revoke` beside `Grant`); `Around` in plans; a step shared by the moments of several
plugins; more than `MaxEffects` (8) effects on one entity and 63 defined; an entity in more than
one group; a dictionary of idioms (`Once`, `Either`, the patrol) written out of the bricks.
