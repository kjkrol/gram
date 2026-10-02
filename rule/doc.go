// Package rule is how a game says what its entities do at the moments the plugins catch: a [Rule]
// made with [On], hooked on the plugin that catches the moment, its steps casting effects and
// giving commands. What an entity does over time is a plan (package rule/plan); a change that
// holds is an effect (package rule/effect). The package is gram's core, beside entity and clock;
// the systems a plugin runs the rules hooked on it with are package plugin's.
//
//	rule.On("drown", rule.All, func(m *rule.Moment[unit.Standing]) rule.Step {
//		return m.If(unit.Standing.Fallen, m.Order(world.Despawn{}))
//	})
//
// # Rules
//
// [On] makes a rule of a moment a plugin catches in its own pass — a unit.Standing or a cell.Now, a
// vision.Sighting, a collision.Meeting, a world.Moving, a clock.Moment — for the plugin's Hook:
// board.Plugin.Hook, vision's, collision's, world's, navigation's. The moment's type says which
// plugin hosts the rule; another refuses it with [plugin.ErrUnhosted], and a rule hooked after its
// host was built is refused with [plugin.ErrHostBuilt]. Its [Filter], the second argument, says
// whom it fires for: [All], [Self] one carrying a tag — an effect's marker among them — [Between] a
// pair whose sides carry the tags given, for a moment that is [plugin.Met], [Having] one carrying a
// component.
//
// A [Moment]'s steps — OneOf, Steps, If on the moment, Not, Apply, Keep, Dispel, Chance, Unless,
// Under, During, Order, ForOther, Here, Around — are each done within the plugin's pass; a step
// that lasts, a plan's, is refused as the rule is made. A rule is written in these steps alone:
// what they cannot say is a moment, a step or a knob the plugin still lacks. A rule keeps no memory
// of its own: an effect's presence is its memory — "at most once a while" is Unless an effect that
// lasts that while. A moment is [plugin.About] one entity, whom the steps act for — a clock.Moment
// the clock's own, where an effect applied is a phase; one that is [plugin.Placed] stands on places
// of their own, a board's cells, which its host tells in the Tick (Tick.Around) and Here and Around
// turn a step on. Under asks the entity's effects, During the world's — a state of the whole game a
// player put on it (world.Apply), a lever pulled. Chance draws from the world's seed, the step's
// game time and the entity, keeping nothing: a load and a replay draw alike.
//
// Apply casts an effect on the entity, lasting as its Spec says; Keep holds one for as long as the
// rule keeps firing it; Dispel takes one off. A rule keeping an effect someone dispelled has it
// back the step after; a dispeller that is to win casts a shield the keeper checks with Unless.
// Within a step, rules of one moment run in the order they were hooked, moments in the order of
// their plugins' passes, and every cast and Dispel lands together in the effects' pass. Order gives
// a command for the entity — the same a player gives, queued for the plugin that handles its type
// (navigation.MoveTo, world.Despawn) — and does well at once; one that is [plugin.Aimed] is told
// the [plugin.Subject] of the moment, whom the entity touched or struck. The world carries the
// commands (world.Plugin.Carry).
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
