// Package rule is how a game says what its entities do at the moments the plugins catch: a [Rule]
// made with [On], its steps casting effects and giving commands, fired for whom its filter lets
// through and, obeyed through a [Role], for those playing it; the engine hands it to the plugin
// that catches its moment. What an entity does over time is a plan (package rule/plan); a change that
// holds is an effect (package rule/effect). The package is gram's core, beside entity and clock;
// the systems a plugin runs the rules of its moments with are package plugin's.
//
//	mortal := rule.Role("mortal").Obeys(rule.Then[unit.Standing]("fall in", rule.All,
//		rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))
//	s.scout = units.Define("scout", land, profile, rule.Plays(mortal))
//
// # Rules
//
// [On] makes a rule of a moment a plugin catches in its own pass — a unit.Standing or a cell.Now on
// the board, a vision.Sighting, a collision.Meeting or Struck, a navigation.Touch, a world.Moving
// or Leaving, a clock.Moment, a climate.Weathering. The moment's type says which plugin hosts the
// rule. Its [Filter], the second argument, says whom it fires for: [All], [Self] one carrying a
// tag — an effect's marker among them — [Between] a pair whose sides carry the tags given and [Other] one
// whose other plays a role, for a moment that is [plugin.Met], [Having] one carrying a component. A Rule's String is its name, its
// moment and what narrowed it — "fall in" of unit.Standing, for the role mortal — and an error
// about the rule names it so.
//
// # Steps
//
// A [Moment]'s steps — OneOf, Steps, If on the moment, Not, Apply, Keep, Dispel, Chance, Unless,
// Under, During, Playing, Order, Trigger, ForOther, Here, Around — are each done within the
// plugin's pass; a step that lasts, a plan's, is refused as the rule is made. A rule is written in
// these steps alone: what they cannot say is a moment, a step or a knob the plugin still lacks. A
// rule keeps no memory of its own: an effect's presence is its memory — "at most once a while" is
// Unless an effect that lasts that while. A moment is [plugin.About] one entity, whom the steps act
// for — a clock.Moment the clock's own, where an effect applied is a phase; one that is
// [plugin.Placed] stands on places of their own, a board's cells, which its host tells in the Tick
// (Tick.Around) and Here and Around turn a step on. Under asks the entity's effects, During the
// world's — a state of the whole game a command put on it (Cast on entity.World). Chance draws from the world's seed,
// the step's game time and the entity, keeping nothing: a load and a replay draw alike.
//
// Apply casts an effect on the entity, lasting as its Spec says; Keep holds one for as long as the
// rule keeps firing it; Dispel takes one off. A rule keeping an effect someone dispelled has it
// back the step after; a dispeller that is to win casts a shield the keeper checks with Unless.
// In a step, rules of one moment run in the order their roles came to be played, moments in the order of their
// plugins' passes, and every cast and Dispel lands together in the effects' pass. Order gives a
// command for the entity — the same a player gives, queued for the plugin that handles its type
// (navigation.MoveTo, world.Despawn) — and does well at once; one that is [plugin.Aimed] is told
// the [plugin.Subject] of the moment, whom the entity touched or struck. The world carries the
// commands (world.Plugin.Carry).
//
// # Who gets a rule
//
// Nobody hands a rule to a plugin. A rule is obeyed by a role, a role is played, and once a
// Stage's Init returns the engine gives every rule of every role played to the plugin in use that
// catches its moment, so a game need not know which plugin catches what. A rule no plugin in use
// catches is an error wrapping [plugin.ErrUnhosted], naming the rule and its role.
//
// # Roles
//
// [Role] is the role named name: a behaviour entities play — mortal, hasty, a trapdoor — not a
// group; whose a unit is, its squad, whether it is selected are tags of families of their own, for
// Self and Between. [Part.Obeys] adds the rules those playing it obey. [Plays] is the component of an entity playing
// roles, for a kind's Spec: every role in one, so a kind names Plays once. A cell plays the roles
// of its kind (board.Plugin.Plays), and a plugin those it is given (its Plays, world.Self's): the rules of a moment of the world as a
// whole — a clock.Moment, a climate.Weathering — fire while the plugin whose moment it is plays
// their role, its own entity carrying the role's tag. [While] runs a step while a plugin's entity
// is under an effect, as [During] does for the world's. [Then] is On without
// the body, its steps the package's own functions ([If], [OneOf], [Apply], [Around]…), its
// conditions predicates of the moment ([Not] turns one round). A program names 64 roles at most, one name one tag, which every world saves by the name.
// A role's String is "the role mortal".
//
//	hasty := rule.Role("hasty")
//	s.scout = units.Define("scout", land, profile, rule.Plays(mortal, hasty))
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
// the place turned to, so a unit pulls the lever beside it and nothing else there:
// Around(1, Playing(lever, Trigger())). A Part's Tag and Rules, [Roles] (the family) and
// [RoleNames] are for the plugins that give roles.
//
// # Commands
//
// What somebody asks for — a player's key, a script, an AI, a rule's Order — is a command, and a
// command about an effect is a [Casting], written as a sentence: [Cast] puts the effect on,
// [Lift] takes it off, [Toggle] switches it; On says whom it is for, For how long a Cast lasts in
// place of the effect's Spec, By who sets it off.
//
//	openWest := rule.Cast(open).On(entity.Group("west trapdoors")).By(entity.Named("west lever"))
//	flipGate := rule.Toggle(ajar).On(entity.Group("gate"))
//	alarm    := rule.Cast(alarmed).On(entity.World)
//	hasten   := rule.Cast(haste).On(s.selection.Selected(hasty))
//	freeze   := rule.Cast(frozen).On(s.selection.Pointed()).For(3 * time.Second)
//
// Whom is a [Target]: entity.Named, the entities bearing those names, one each; entity.Group, all
// those in the groups; entity.World, the world's own entity — the state of the whole game, which
// rules read with During; or a plugin's own (a [Router]), as the selection's Selected and Pointed.
// A cell is called by its cell.Entry's Name and Group, a unit by its kind.Entry's Named and
// InGroup. The plugin the target belongs to carries the command out — the world for the entity
// package's — so a key (control.Give), players.Issue and Order all give it the same way. A Toggle
// takes the effect off them all where any is under it; commands of one effect for one entity in
// one step are netted.
//
// [Trigger] is the step by which an entity sets commands off: every Casting whose By names it or
// its group is given, so a role says only that a plate stood on triggers, and the command says
// what that opens. The game hands its commands to the Initializer's Commands: those with a By
// are kept for Trigger, and every name they say is checked as the game starts — a name nobody
// bears, or one two bear, stops it there.
//
//	plate := rule.Role("plate").Obeys(rule.Then[cell.Now]("press", rule.All, rule.If(cell.Now.Stood, rule.Trigger())))
//	s.board.Plays("plate", plate)
//	s.player.Bind(control.Give(control.KeyPress{Key: control.Key1}, "Pull the west lever", openWest))
//	return ctx.Commands(openWest, flipGate)
//	// Spawn: cell.Entry{Kind: "boards", Cell: c, Group: "west trapdoors"}
//	//        cell.Entry{Kind: "lever", Cell: l, Name: "west lever"}
//
// # Hosts
//
// A plugin runs the rules of its moments inside a pass it makes anyway, so one walk over its
// entities runs them all and a rule costs no query of its own: [plugin.Rules] over entities,
// [plugin.PairRules] over pairs, [plugin.StepRules] once a step, each handing its rules a
// [plugin.Tick] made by the world's [plugin.TickSource] (world.Plugin.Tick). The steps themselves
// are run by gram's internal engine, which package rule and package rule/plan are the faces of.
//
// A rule holds no Go code of its own but the conditions of If: what it does is said in steps,
// commands and effects. How an entity is drawn, decided every frame, is no part of the game and
// is said in Go, by render.Rule.
package rule
