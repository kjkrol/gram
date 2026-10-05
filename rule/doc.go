// Package rule is how a game says what its entities do at the moments the plugins catch: a [Rule]
// made with [On], its steps casting effects and giving commands, fired for whom its filter lets
// through or, obeyed through a [Role], for those playing it, and hooked through the Stage's
// Initializer (game.Initializer.Hook) on the plugin that catches its moment. What an entity does over time is a plan (package rule/plan); a change that
// holds is an effect (package rule/effect). The package is gram's core, beside entity and clock;
// the systems a plugin runs the rules hooked on it with are package plugin's.
//
//	mortal := rule.Role("mortal").Obeys(rule.On("fall in", rule.All, func(m *rule.Moment[unit.Standing]) rule.Step {
//		return m.If(unit.Standing.Fallen, m.Order(world.Despawn{}))
//	}))
//	return ctx.Hook(mortal)
//
// # Rules
//
// [On] makes a rule of a moment a plugin catches in its own pass — a unit.Standing or a cell.Now on
// the board, a vision.Sighting, a collision.Meeting or Struck, a navigation.Touch, a world.Moving
// or Leaving, a clock.Moment, a climate.Weathering. The moment's type says which plugin hosts the
// rule. Its [Filter], the second argument, says whom it fires for: [All], [Self] one carrying a
// tag — an effect's marker among them — [Between] a pair whose sides carry the tags given, for a
// moment that is [plugin.Met], [Having] one carrying a component. A Rule's String is its name, its
// moment and what narrowed it — "fall in" of unit.Standing, for the role mortal — and an error
// about the rule names it so.
//
// # Steps
//
// A [Moment]'s steps — OneOf, Steps, If on the moment, Not, Apply, Keep, Dispel, Chance, Unless,
// Under, During, OnWire, WhileWire, Playing, Order, ForOther, Here, Around — are each done within the
// plugin's pass; a step that lasts, a plan's, is refused as the rule is made. A rule is written in
// these steps alone: what they cannot say is a moment, a step or a knob the plugin still lacks. A
// rule keeps no memory of its own: an effect's presence is its memory — "at most once a while" is
// Unless an effect that lasts that while. A moment is [plugin.About] one entity, whom the steps act
// for — a clock.Moment the clock's own, where an effect applied is a phase; one that is
// [plugin.Placed] stands on places of their own, a board's cells, which its host tells in the Tick
// (Tick.Around) and Here and Around turn a step on. Under asks the entity's effects, During the
// world's — a state of the whole game a player put on it (world.Apply) — and WhileWire those of
// the entity's wire, on which OnWire runs a step (see Wires). Chance draws from the world's seed,
// the step's game time and the entity, keeping nothing: a load and a replay draw alike.
//
// Apply casts an effect on the entity, lasting as its Spec says; Keep holds one for as long as the
// rule keeps firing it; Dispel takes one off. A rule keeping an effect someone dispelled has it
// back the step after; a dispeller that is to win casts a shield the keeper checks with Unless.
// In a step, rules of one moment run in the order they were hooked, moments in the order of their
// plugins' passes, and every cast and Dispel lands together in the effects' pass. Order gives a
// command for the entity — the same a player gives, queued for the plugin that handles its type
// (navigation.MoveTo, world.Despawn) — and does well at once; one that is [plugin.Aimed] is told
// the [plugin.Subject] of the moment, whom the entity touched or struck. The world carries the
// commands (world.Plugin.Carry).
//
// # Hooking
//
// A Stage hooks its rules and roles with its Initializer's Hook once its plugins are Used, before
// Init returns: each rule, and every rule of a role, goes to the plugin in use that hosts its
// moment, so the Stage need not know which plugin hosts what. A rule no plugin in use hosts is an
// error wrapping [plugin.ErrUnhosted]. A plugin's own Hook takes the rules of its moments too,
// before or after Use, until the Stage's ecs.Setup builds its systems; later is
// [plugin.ErrHostBuilt].
//
// # Roles
//
// [Role] is the role named name: a behaviour entities play — mortal, hasty, a trapdoor — not a
// group; whose a unit is, its squad, whether it is selected are tags of families of their own, for
// Self and Between. [Part.Obeys] adds the rules those playing it obey; [Part.Can] adds an ability,
// an effect the player's trigger puts on its selected units playing the role, listed under a label
// (selection.Plugin.Abilities makes the bindings). [Plays] is the component of an entity playing
// roles, for a kind's Spec: every role in one, so a kind names Plays once. A cell plays the roles
// of its cell.Entry.Roles. A Part is a Rule for the Initializer's Hook, which hooks every rule it
// obeys; the engine hooks a role some kind plays itself, once Init returns. [Then] is On without
// the body, its steps the package's own functions ([If], [OneOf], [Apply], [Around]…), its
// conditions predicates of the moment ([Not] turns one round). A program names 64 roles at most, one name one tag, which every world saves by the name.
// A role's String is "the role mortal".
//
//	hasty := rule.Role("hasty").Can(haste, control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts")
//	selectable, mine := comp.Tagged(s.selection.Tags().Selectable), comp.Tagged(s.player.Owner())
//	s.scout = units.Define("scout", land, profile, selectable, mine, rule.Plays(mortal, hasty))
//	s.player.Bind(s.selection.Abilities(hasty)...)
//	return ctx.Hook(mortal, hasty)
//
// Obeys narrows each rule on top of its own filter: a rule of Self(hungry.Mark()) obeyed by mortal
// fires for a mortal entity under hungry; on a moment that is Met, for the pairs whose own entity
// plays the role; a collision.Meeting rule with one tag on both sides, All among them, still runs
// at most once a pair (a navigation.Touch is each unit's own, a vision.Sighting each observer's).
// An entity playing two roles that obey the same rule obeys it once for each. A rule of a moment
// of the world as a whole, a clock.Moment or a climate.Weathering, is run once a step, walking no
// entities: filtered or narrowed, its host refuses it with plugin.ErrUnhosted — say it with
// During, or in a rule over entities.
//
// Playing(role, step) runs a step while the entity plays the role; inside Here or Around it asks
// the place turned to, so a unit pulls the lever beside it and leaves alone the trapdoors on the
// same wire: m.Around(1, m.Playing(lever, m.OnWire(m.Apply(on)))). A Part's Tag, Rules and
// Abilities, [Roles] (the family) and [RoleNames] are for the plugins that give roles and bind
// them.
//
// # Wires
//
// A [Wire] is a connection by name — a lever and its trapdoors, a plate and its gate — defined with
// world.Plugin.Wire: its own entity, made at Setup and found again by the name after a load. Its
// state is an effect on that entity, so one effect serves any number of wires: roles are the
// behaviour, wires the groups. An entity carrying [Wired], the wire's name hashed, is wired to
// one — a cell given cell.Entry.Wired, a kind's entities comp.Const(west.Wired()) — and its rules
// follow it: OnWire(step) runs the step on the wire in place of the entity, so OnWire(Apply(on))
// puts the wire on, and WhileWire(e, step) runs the step while the wire is under e. Both fail for
// an entity wired to none; a plan's Actor has both. A player drives a wire with a binding:
// [Wire.Key] puts an effect on it each time its trigger fires, lasting as its Spec says;
// [Wire.Switch] puts it on or takes it off, an effect without Lasts holding until switched off,
// saved with the game; the world carries them out. A wire's String is "the wire west".
//
//	on := fx.Define("on", effect.Spec{effect.Lasts(2 * time.Second)})
//	open := fx.Define("open", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind = pit })})
//	trapdoor := rule.Role("trapdoor").Obeys(rule.On("open while on", rule.All, func(m *rule.Moment[cell.Now]) rule.Step {
//		return m.WhileWire(on, m.Keep(open))
//	}))
//	plate := rule.Role("plate").Obeys(rule.On("press", rule.All, func(m *rule.Moment[cell.Now]) rule.Step {
//		return m.If(cell.Now.Stood, m.OnWire(m.Apply(on)))
//	}))
//	west := s.world.Wire("west")
//	s.player.Bind(west.Key(on, control.KeyPress{Key: control.Key1}, "Pull the west lever"))
//	return ctx.Hook(trapdoor, plate)
//	// Spawn: cell.Entry{Kind: "boards", Cell: c, Roles: []*rule.Part{trapdoor}, Wired: west}
//
// # Hosts
//
// A plugin runs the rules hooked on it inside a pass it makes anyway, so one walk over its
// entities runs them all and a rule costs no query of its own: [plugin.Rules] over entities,
// [plugin.PairRules] over pairs, [plugin.StepRules] once a step, each handing its rules a
// [plugin.Tick] made by the world's [plugin.TickSource] (world.Plugin.Tick). The steps themselves
// are run by gram's internal engine, which package rule and package rule/plan are the faces of.
//
// A rule holds no Go code of its own but the conditions of If: what it does is said in steps,
// commands and effects. How an entity is drawn, decided every frame, is no part of the game and
// is said in Go, by render.Rule.
package rule
