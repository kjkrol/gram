// Package rule is how a game says what its entities do at the moments the plugins catch: a [Rule]
// made with [On], hooked on the plugin that catches the moment, its steps casting effects and
// giving commands. What an entity does over time is a plan (package rule/plan); a change that
// holds is an effect (package rule/effect). The package is gram's core, beside entity and clock,
// and it is also how a plugin runs the rules hooked on it: its hosts.
//
//	rule.On("drown", rule.All, func(m *rule.Moment[unit.Standing]) rule.Step {
//		return m.If(unit.Standing.Fallen, m.Order(world.Despawn{}))
//	})
//
// # Rules
//
// [On] makes a rule of a moment a plugin catches in its own pass — a unit.Standing or a
// cell.Now, a vision.Sighting, a collision.Meeting, a world.Moving, a clock.Moment — for the
// plugin's Hook: board.Plugin.Hook, vision's, collision's, world's, navigation's. The moment's
// type says which plugin hosts the rule; another refuses it with [ErrUnhosted], and a rule hooked
// after its host was built is refused with [ErrHostBuilt]. Its [Filter], the second argument, says
// whom it fires for: [All], [Self] one carrying a tag — an effect's marker among them — [Between]
// a pair whose sides carry the tags given, for a moment that is [Met], [Having] one carrying a
// component.
//
// A [Moment]'s steps — OneOf, Steps, If on the moment, Not, Apply, Keep, Dispel, Chance, Unless,
// Under, During, Order, ForOther, Here, Around — are each done within the plugin's pass; a step that
// lasts, a plan's, is refused as the rule is made. A rule is written in these steps alone: what
// they cannot say is a moment, a step or a knob the plugin still lacks. A rule keeps no memory of
// its own: an effect's presence is its memory — "at most once a while" is Unless an effect that
// lasts that while. A moment is [About] one entity, whom the steps act for — a clock.Moment the
// clock's own, where an effect applied is a phase; one that is [Placed] stands on places of their
// own, a board's cells, which its host tells in the Tick (Tick.Around) and Here and Around turn a
// step on. Under asks the entity's effects, During the world's — a state of the whole game a
// player put on it (world.Apply), a lever pulled. Chance draws from the world's seed, the step's
// game time and the entity, keeping nothing: a load and a replay draw alike.
//
// Apply casts an effect on the entity, lasting as its Spec says; Keep holds one for as long as the
// rule keeps firing it; Dispel takes one off. A rule keeping an effect someone dispelled has it
// back the step after; a dispeller that is to win casts a shield the keeper checks with Unless.
// Within a step, rules of one moment run in the order they were hooked, moments in the order of
// their plugins' passes, and every cast and Dispel lands together in the effects' pass. Order
// gives a command for the entity — the same a player gives, queued for the plugin that handles its
// type (navigation.MoveTo, world.Despawn) — and does well at once; one that is [Aimed] is told the
// [Subject] of the moment, whom the entity touched or struck. The world carries the commands
// (world.Plugin.Carry).
//
// # Hosts
//
// A plugin runs the rules hooked on it inside a pass it makes anyway, so one walk over its
// entities runs them all and a rule costs no query of its own. [EachHost] runs rules over
// entities: Bind adds what the rules read to the plugin's query, then Run over each chunk walked
// with a function describing its i-th entity — RunWhere over those of the chunk a moment is about
// (collision's Struck, for whoever struck something); [Own] shares a column the plugin reads
// itself. [PairHost] runs the rules of a pair: Bind its families to the plugin's queries, read
// what an entity carries (InChunk, At) as [Marks] and Dispatch, DispatchEitherWay or
// DispatchGrouped per pair or per observer. [ListHost] runs the rules of a moment of the world as
// a whole — the clock's — once a pass. One host's rules may name at most [MaxFamilies] tag
// families. A host hands its rules a [Tick], made by the world's [TickSource]
// (world.Plugin.Tick): the command buffer, the carrier, the game time, the world's seed and own
// entity.
//
// A rule holds no Go code of its own but the conditions of If: what it does is said in steps,
// commands and effects. How an entity is drawn, decided every frame, is no part of the game and
// is said in Go, by render.Rule.
package rule
