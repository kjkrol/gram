# Behaviour: how a game uses ceremony over gram's plugins

How entities behave — rules, plans, effects, roles, commands, facts — is the library
[ceremony](https://github.com/kjkrol/ceremony), which gram builds on with goke as the store. The
language is ceremony's own: its [README](https://github.com/kjkrol/ceremony) and
[doc/ceremony.md](https://github.com/kjkrol/ceremony/blob/main/doc/ceremony.md) say what a
branch, a loop, a condition, a step, an effect, a role, a command and a message are, how an
effect runs, what a command about an effect says (`Put`, `Take`, `Toggle`, `On`, `For`, `By`,
`Trigger`), what a knob is and why a rule holds no Go code. None of that is repeated here. This
page is the game's side: what gram's plugins give to write rules over — the moments, the words,
the facts, the knobs — where a game defines things, how a player gives a command, and how a game
and a plugin are written in these words.

## The plugins' moments and words

A plugin **perceives**: a moment its pass catches, handed to the rules hosted there, with words
of its own to ask of it. A rule of a moment goes to the plugin that catches it; the game names no
plugin.

| moment | caught by | of | subject | words |
|---|---|---|---|---|
| `unit.Standing` | the board, every unit a step | one; `host.Placed`, `host.Headed` | — | `unit.Fallen()`, `unit.StandingOn(e)`, `unit.OnKind(k)` |
| `cell.Now` | the board, every cell a step | one; `host.Placed` | — | — |
| `collision.Meeting` | collision, a confirmed contact, a pair resting against each other too | a pair; `host.Met` | — | `collision.Facing(within)` (turned to the other within the angle) |
| `collision.Struck` | collision, one that struck something | one | — | — |
| `vision.Sighting` | vision, every observer a step | a pair grouped by observer; `host.Met` | the nearest seen | `vision.SeesNobody()`, `vision.Closing()` (of the other) |
| `navigation.Touch` | navigation, two units touching | a pair; `host.Met` | the other | `PushedByAlly()`, `ReachedTheGroup()`, `InTheWay()`, `ClearsTheWay()`, `Blocks()`, `GoalTaken()`, `StrangerOnMyGoal()`, `WaitsFirst()`, `NoWayRound()`, `WaitedLong()` |
| `world.Moving` | the world, every entity before it moves | one | — | `world.Still()` (neither walking nor strafing), `world.Braking()` (slowing down) |
| `world.Leaving` | the world, every tick past an open edge | one | — | — |
| `clock.Moment` | the world, once a step | the clock's own entity | — | `clock.At(t)`, `clock.Every(period, offset)` |
| `climate.Weathering` | the atmosphere, once a step | the atmosphere's own entity | — | — |
| `sky.Moonrise` | the sky, as the moon comes over the horizon | the atmosphere's own entity | — | `sky.FullMoon()` |
| `bullet.Landing` | bullet, a shot landing | one | what it struck, if anything | — |
| `bullet.Resting` | bullet, a landed shot every step | one | — | — |
| `bullet.Blast` | bullet, a burst reaching an entity | a pair; `host.Met` | the shot | `bullet.Within(d)` (of the other) |
| `gravity.Landing` | gravity, once as a body lands | one | — | `gravity.HarderThan(v)` |

Every plugin's `doc.go` ends with a `# Vocabulary` section listing its moments, words, facts,
knobs and commands; it is the authoritative list.

A moment is one by its `Who()` (`host.About`), a pair by `Whom` (`host.Met`), one standing on
places by `Placed()` (`host.Placed`), confirmed beside its type (`var _ host.About = Standing{}`).
A word is a `cond.Word` made with `cond.NewWord(name, func(P) bool)`, `.OfOther()` for one of the
other; a game writes its own the same way over a plugin's moment or over a fact.

- **Where one stands.** On a `host.Placed` moment — a `unit.Standing`, a cell's `cell.Now` —
  `cond.Around(n).If(x)` and `act.Around(n).Put(e)` turn to each place within `n` rings of the
  cells under the entity (for a cell, of itself): the cell and the entities standing on it, each
  once, on square and hex boards alike; `0` the cells under it alone. An effect put on the ground
  round a witch, a lake's cells iced. The cell under a unit is a word of its `Standing`:
  `unit.StandingOn(e)`, `unit.OnKind(k)`. The units there are no part of it. A `unit.Standing`
  is `host.Headed` too, carrying the way the unit faces (`Heading`, its `Vel.Dir`):
  `cond.Ahead(n).If(x)` and `act.Ahead(n).Put(e)` turn to the places on the cells one to `n`
  steps in front of it, the grid's direction nearest its heading, stopping at the edge — a
  lever in front of a scout, a wall three cells on; one kind of places a rule, and a cell faces
  nowhere (a `cell.Now` refuses `Ahead`). A thing used by touch — a lever — is a
  `collision.Meeting` under `collision.Facing(angle)`, which holds without a board.
- **The clock's moment is the clock's.** A `clock.Moment` is of the clock's own entity: an effect
  a clock rule puts on lands there — `act.Branch[clock.Moment]{Name: "dusk", When:
  cond.Self.If(clock.At(dusk)), Then: act.Self.Put(night)}`, a phase that `clock.Clock.In` reads
  (`calendar.Daily`, `Yearly`, `Seasonal` give `clock.Every` its period and offset). Run once a
  step, walking no entities, it fires while the clock's plugin, the world, plays its role; a
  `climate.Weathering` the same while the atmosphere plays it.
- **A moment's subject.** A command that is `host.Aimed` — `steering.Away{}`, `steering.Toward{}`,
  `gravity.Away{}`, `bullet.Shoot{}`, `dialog.Begin{}`, `dialog.Feel{}`, navigation's
  `StepAside{}`, `Detour{}`, `Pass{}` and `Settle{}` — is told the subject of the rule's moment:
  a `Sighting`'s nearest seen, the one touched, the shot that burst, what a shot struck. It fails
  while the moment names nobody.

## Facts, knobs and commands: what is marked

What a game and a plugin write are of three kinds, each marked by a method never called and
confirmed beside the type; what is not marked is the engine's (the binding's `Active`, the
roles' tags, a plan's mind, a label) or a value in flight (a moment, a word).

| thing | what it is | mark | gram's own |
|---|---|---|---|
| fact | a component told to an entity with a plan, carried until it goes; a message is one by embedding `fact.From`; its words are of it (`bullet.Lying()` of a `Flight`) | `Fact()`, `var _ fact.Fact = Arrived{}` | `navigation.Blocked` (its subject whom by), `navigation.Arrived`, `bullet.Flight` |
| knob | a component a plugin only reads, which an effect's `Alter` turns and puts back | `Knob()`, `var _ effect.Knob = Steering{}` | `steering.Steering`, `cell.Ground`, `cell.Floor`, `cell.Wall`, `cell.Rail`, `collision.Physics`, `bullet.Body`, `gravity.Pull`, `sky.Moon`, `vision.Sight`, `render.Appearance` |
| command | a request carried by its type to the plugin that handles it; the handler says who may give it — an entity's own (`world.Despawn`, `steering.Away`, `gravity.Leap`, `players.Give`), a player's (the looks, the cameras', the tools') or both (`world.Spawn`, `navigation.MoveTo`, `bullet.Shoot`, the driving's) | `Command()`, `var _ command.Command = MoveTo{}` | every plugin's `commands.go` (the topography's views in `camera_commands.go`, its shaping in `shaping.go`) |

- **Facts** are how a plan learns what came of its commands: navigation tells `Blocked` (from a
  unit's touches while on the move, held a while after the last) and `Arrived` (its order over,
  until the next) to units with a mind alone, so entities without a plan pay nothing; `Blocked`'s
  words are `navigation.Stuck()`, `Cornered()`, `StrangerOnGoal()`, `IdleAlly()`,
  `IdleAllyOnMyGoal()`.
- **Knobs** in gram: what a plugin's system writes lives beside the knob in a component of its
  own — `steering.Course` beside `Steering`, `vision.Sighted` beside `Sight`, a cell's heights
  apart from its `Ground` — given where it is missing. A plugin's own effects stay private.
- **Commands** are carried out by the plugin that defines the type, in its own pass, for the
  entity alone (`command.Issued.ByEntity`), the engine's own rules inside (a unit is never
  stepped into water, off a cliff or into a wall); `world.Spawn{Entry}` is the one that makes
  another entity, of the Entry fixed as the rule is written. The world keeps a Stage's one
  carrier (`command.Carrier`, `world.Plugin.Carrier`): the players give it theirs, the entities
  theirs; the engine carries every `plugin.CommandHandler` a Stage uses, and the `host.Tick` a
  plugin hands its rules carries the carrier. Nothing is dropped: a command waits for its
  handler's pass. `navigation.MoveTo` takes a cell, or an entity by name (`MoveTo{To:
  ceremony.Named(GateName)}`); `bullet.Shoot{Ammo}` fires from the player's selected units, or
  from the entity that gives it, aimed at its moment's subject.

## Where a game defines things

**Defining registers and hands nothing back.** Effects (`world.Effects()`), roles
(`world.Roles()`), commands (`world.Commands()`) and kinds (`kind.Define`, `units.Define`,
`cell.Kinds.Define`) are each defined under a name in the Stage's world, in the section of the
Stage that is theirs, and `Named(name)` is the thing wherever it is built on; a plan is carried by
its kind, not looked up. A game keeps its names as constants (a `names.go`), each with a suffix
saying what it names — `BloodMoonEf`, `BleedCmd`, `MortalRole`, `GuardPlan`, `ScoutKind`,
`GrassCell` — so a name mistyped does not compile, and a Stage's struct holds its plugins alone.
Each Stage has registers of its own: nothing a game defines is the program's.

```go
fx, roles, cmds := s.world.Effects(), s.world.Roles(), s.world.Commands()

// Effects
fx.Define(HasteEf, effect.Spec{effect.Lasts(3 * time.Second),
	effect.Alter(func(st *steering.Steering) { st.MaxSpeed *= 2 })})

// Rules: a role and what its players obey
roles.Define(MortalRole, act.Branch[unit.Standing]{
	Name: "fall in",
	When: cond.Self.If(unit.Fallen()),
	Then: act.Self.Do(world.Despawn{}),
})
roles.Define(HastyRole)

// Commands
cmds.Define(HastenCmd,
	ceremony.Put(fx.Named(HasteEf)).On(s.selection.Selected(roles.Named(HastyRole))))

// Kinds: who plays what
units.Define(ScoutKind, land, profile,
	ceremony.Plays(roles.Named(MortalRole), roles.Named(HastyRole)))
units.Define(PorterKind, land, laden, ceremony.Plays(roles.Named(MortalRole)))
units.Define(GuardKind, land, profile, guard,
	comp.Load(func(g guardRow) navigation.Round { return navigation.NewRound(g.Round[:]...) }))
```

A role is played — by a kind of unit (`ceremony.Plays` in its Spec, once, every role it plays in
it), by a kind standing on the board (`board.NewEntities`), by a kind of cell as it is defined
(`cell.Kinds.Define(name, kind, roles...)`), by one cell of the Layout alone
(`cell.Entry.Plays`), by a plugin (`s.world.Plays(role)`, `s.atmosphere.Plays(role)`):

```go
plates.Define(PlateKind, comp.Const(collision.Collider{}), ceremony.Plays(roles.Named(PlateRole)))
kinds.Define("lava", cell.Kind{Cost: 9}, roles.Named(ScorchingRole))
layout.Cells = append(layout.Cells, water.Entry(c).Plays(lakeRole))
s.world.Plays(roles.Named(NightlyRole))
```

Once the Stage's `Init` returns, the engine gives every rule of every role somebody plays to the
plugin in use that catches its moment, trying the plugins in the order they were used
(`plugin.Installer.Hosts` is how a plugin says what it catches; `ceremony.Deliver` underneath). A
rule no plugin in use catches fails `Init`, naming the rule. A Stage names 64 roles at most, one
name one tag, saved by name.

**A plugin is an entity too** (`world.Self`): one of its own, called by the plugin's name,
carrying its knobs, its roles and the effects it is under — as many at once as a game defines. So
a state of the sky is an effect on the atmosphere, as a state of the whole game is one on the
world:

```go
fx.Define(BloodMoonEf, effect.Spec{effect.Lasts(night), effect.Alter(func(m *sky.Moon) { … })})
cmds.Define(BleedCmd, ceremony.Put(fx.Named(BloodMoonEf)).On(s.atmosphere))
act.Branch[world.Moving]{
	Name: "frenzy",
	When: cond.On(s.atmosphere).If(fx.Named(BloodMoonEf)),
	Then: act.Self.Hold(fx.Named(FrenziedEf)),
}
```

What a plugin keeps of an entity is changed by that plugin's command, not written into its kind:
`players.Give{To}` hands a unit to a player, `selection.Allow{}` and `Forbid{}` say whether it may
be selected, and `kind.Entry.Told(cmds...)` has the entity give them itself as it is made — so one
kind serves several players, and a unit changes hands mid-game by the same command:

```go
mine := []command.Command{players.Give{To: s.player.ID}, selection.Allow{}}
s.world.Seed(s.scout.Entry(row).Told(mine...), s.porter.Entry(row).Told(mine...))
```

## Commands from keys and ui

Whatever somebody asks for — a player's key, a ui button, a script, an AI, a branch's `Do` — is
a command, given the same way whoever gives it. In a game the targets of a command about an
effect are the entities called by name or group (an entity is called by what makes it: a cell by
its `cell.Entry{Name, Group}`, a unit by its `kind.Entry` or the map file's `name` and `group`),
the world itself (`ceremony.World`), or the selection's own: `Selected(roles...)`, the units the
player who gives the command has selected (those playing a role named, when any is), and
`Pointed()`, the entity drawn under the cursor as the key is pressed.

```go
openWest := ceremony.Put(open).On(ceremony.Group("west trapdoors")).By(ceremony.Named("west lever"))
flipGate := ceremony.Toggle(ajar).On(ceremony.Group("gate"))
alarm    := ceremony.Put(alarmed).On(ceremony.World).For(30 * time.Second)
hasten   := ceremony.Put(haste).On(s.selection.Selected(hasty))
freeze   := ceremony.Put(frozen).On(s.selection.Pointed()).For(3 * time.Second)
```

The game defines them by name in `s.world.Commands()` (`Define(name, cmd)`, in the Commands
section; `Named(name)` is the command for a key): those with a `By` are kept for
`ceremony.Trigger`, and every name they say is checked as the game starts — a name nobody bears,
or one two bear, stops it there. A key must take its command from the register (`control.Give`
panics for one made on the spot):

```go
openWest := s.world.Commands().Named(OpenWestCmd)
s.player.Bind(control.Give(control.KeyPress{Key: control.Key1}, "Pull the west lever",
	openWest))                          // a key
s.players.Issue(ai, openWest)           // an AI, a script
ui.Button("Open", openWest)             // a ui button, through the scene's Issue
act.Self.Do(openWest)                   // a rule; plan.Self.Do(openWest) a plan
act.Self.Do(ceremony.Trigger{})         // its source, pressed
```

The plugin its target belongs to carries it out — the world for those named, grouped and the
world itself, the selection for its own — a step later, with the step's effects; commands of one
effect for one entity in one step are netted. A ui element pinned to an entity gives its commands
for that entity (`ceremony.Take(greeting).On(ui.It)`); a plugin's own command that is a
`ui.About` (`dialog.Choose`) gets the entity the same way. The wire demo
(`examples/rules/wire/demo.go`) is a whole program of it: a lever, a plate and a switch, each a
command for a group of trapdoors or a gate, entities standing on the board; the trapdoor and
pressure plate demos are the same in less.

## What a plugin does of its own

A rule holds no Go code (ceremony's "No Go code in a rule"); what a plugin does of its own is its
own work in a pass it makes anyway, never a rule of another plugin's moment:

- the ground's pace: the board's pass over the units writes each one's `steering.Pace` — the cost
  and the slope of the cell under it — and the world's velocity pass multiplies the speed by it,
  from the next step;
- navigation's bumps: its pass reads the `collision.Collider` contacts of every unit under orders;
- contacts counted and logged, sightings and falls logged: `collision.Plugin.WithStats`,
  `WithLog`, `vision.Plugin.WithLog`, `board.Plugin.WithLog`;
- the weather on the board: the atmosphere's own `Weathering.System`, once a second of game time.

Steering is commands an entity gives itself: `steering.Away{}` and `steering.Toward{}`, aimed at
the moment's subject, and `steering.Turn{Angle}`. The vision demo's hunter and prey are written so:

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
entity looks under it is the kind's, settled at drawing, on the world's atlas in the scene's
`Layers`: `worldAtlas.Add(witch, size, witchLook).Under(frozen, frozenWitchLook)` is the slot the
witch is drawn from while frozen — a twin of its sprite the atlas issues, swapped in by the
world's renderer by itself (`world.Plugin.SetAtlas` as the atlas closes, a `render.Swap` under
the effect's
marker); a scene finds its effects by name, `world.Effects().Named("frozen")`, at building alone.
A game may give the same for a state that is no effect: `render.Swap(twins, mark.In)` draws each
kind as its own sprite under a marker and leaves a kind with no twin as it is;
`render.Over(crust, e.Mark().In)` lays one look over every kind; `Show` hides. Two states compose
in the order the rules are given. This is what Unreal's Gameplay Ability System calls a cue: the
effect grants a tag, each class reacts to it cosmetically; the effect never touches the look. An
`Alter(render.Appearance)` remains a knob for one look for every kind.

## Writing a new behaviour in a plugin

1. A moment a plugin already catches, instant: a rule on it; its memory, an effect; what it
   changes, a knob the effect alters — a component the plugin only reads, added where missing,
   marked `effect.Knob`.
2. Something that lasts: name the facts the plugin can perceive, mark them `fact.Fact` and write
   them for units with a mind only; name the commands (`command.Command`) and handle them in the
   plugin (a `CommandHandler`'s queue), carrying them out for the entity that gives them, the
   engine's own rules inside; tell what came of them as facts.
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

`StepAside`, `Pass`, `Detour` and `Settle` are `host.Aimed`: each is told the moment's subject,
the unit touched. A game adds its own rules as a role's, for the units playing it —
`roles.Define(GuardRole, act.Branch[navigation.Touch]{Name: "hold the line", …})` — or gives its
own set in place of the crowd's with `WithCrowd`. What a unit does over time is its plan: the
guard demo's guards patrol their own round, step aside when blocked long, rest where they arrive
and run to the gate, named on the map, while the alarm is on (ceremony's README has the plan).

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
	effect.Then(fx.Named(SmoulderingEf)),                           // burnt out, it smoulders
	effect.Alter(func(a *render.Appearance) { a.SpriteID = flames }), // a unit drawn burning,
	effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 1.5 }),  // running about;
	effect.Alter(func(g *cell.Ground) { g.Kind = embers }),          // a cell's ground aflame
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
units.Define(WalkerKind, land, profile, ceremony.Plays(roles.Named(FlammableRole)))
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
