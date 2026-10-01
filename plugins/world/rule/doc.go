// Package rule is how entities behave, in one vocabulary of steps: a rule is what is done at a
// moment a plugin catches, a plan is what an entity does over time, an effect is a change that
// holds, a command is what an entity has done, and a fact is what a plugin tells an entity. The
// world makes and runs the plans (plugins/world), in every step of its simulation, after its
// decision systems: they stand in the tactical pause, go with the tempo, and are saved with the
// game. The effects live in the subpackage effect.
//
// A rule and a plan are written by a function: [On] hands it the [Moment] a rule is for, [Plan]
// the [Actor] a plan is for, and their methods make the steps it returns.
//
//	rule.On("drown", rule.All, func(m *rule.Moment[unit.Standing]) rule.Step {
//		return m.If(unit.Standing.Fallen, m.Order(world.Despawn{}))
//	})
//
//	rule.Plan("patrol", func(a *rule.Actor) rule.Step {
//		return a.Steps(
//			a.Order(navigation.MoveTo{Cell: east}).Until[navigation.Arrived](),
//			a.Order(navigation.MoveTo{Cell: west}).Until[navigation.Arrived](),
//		)
//	})
//
// # Rules
//
// [On] makes a rule of a moment a plugin catches in its own pass — a unit.Standing or a
// cell.Now, a vision.Sighting, a collision.Meeting, a world.Moving, a clock.Moment —
// for the plugin's Hook: board.Plugin.Hook, vision's, collision's, world's, navigation's. Its
// [Filter], the second argument, says whom it fires for: [All], [Self] one carrying a tag — an
// effect's marker among them — [Between] a pair whose sides carry the tags given, for a moment
// that is [Met], [Having] one carrying a component. A Moment's steps — OneOf, Steps, If on the
// moment, Not, Apply, Keep, Dispel, Chance, Unless, Under, During, Order, ForOther, Here, Around
// — are each done within the plugin's pass; a step that lasts, made by an Actor, is refused as
// the rule is made. A rule is written in these steps alone: what they cannot say is a moment, a step or a
// knob the plugin still lacks. A rule keeps no memory of its own: an effect's presence is its
// memory — "at most once a while" is Unless an effect that lasts that while. A moment is [About]
// one entity, whom the steps act for — a clock.Moment the clock's own, where an effect applied is
// a phase; one that is [Placed] stands on places of their own, a board's cells, which its host
// tells in the Tick (plugin.Tick.Around) and Here and Around turn a step on. Under asks the entity's effects, During the world's — a state of the whole
// game a player put on it (world.Apply), a lever pulled. Chance draws from the world's seed, the step's game time and the entity,
// keeping nothing: a load and a replay draw alike.
//
// # Plans
//
// [Plan] is the component a kind gives its entities: units.Define(..., rule.Plan("patrol", …)).
// Its name is what a save knows it by; one name is one plan, written again it must be alike. A
// plan is plain Go: functions taking the Actor and returning steps, each branch named and short.
// An Actor's When opens a branch that runs while the actor carries a fact, its On one that runs
// from the tick a fact comes until its steps are done. OneOf runs its steps in order every tick and
// stands as the first that does not fail — an earlier branch, once it can, takes over and the
// later one is stopped; Steps runs its steps one after another, remembering where it is; If runs a
// step while a fact holds as a function of it says; Until waits for a fact to come afresh; Wait
// waits; Timeout, Cooldown and Not decorate; Idle runs for ever. A plan holds at most [MaxSteps]
// steps; its state is the entity's [Mind] — the plan, the steps running, each one's place and start
// on the world's clock.
//
// # Effects and commands
//
// Apply casts an effect (effect.Effect) on the entity, lasting as its Spec says; Keep holds one
// for as long as its branch runs — in a rule, as long as the rule keeps firing it; Dispel takes one
// off; Unless and Under run a step as the entity is under one or not. A rule keeping an effect
// someone dispelled has it back the step after, its cause going on; a dispeller that is to win
// casts a shield the keeper checks with Unless. A plan's Keep gives way — its branch fails — when
// someone else takes its effect off. Within a step, rules of one moment run in the order they were
// hooked, moments in the order of their plugins' passes, and every cast and Dispel lands together
// in the effects' pass. Order gives a command for the entity — the same
// command a player gives, queued for the plugin that handles its type (navigation.MoveTo,
// world.Despawn) — and does well at once: fire and forget. In a plan it hands back a [Command]:
// its Until waits for what comes of it, a fact — navigation.Arrived — and its Stay keeps the
// branch, so that a reactive branch gives it once, not every tick; put the reaction to what came of
// it earlier in the OneOf. A command that is [Aimed] is told, as it is given, the [Subject] of the
// fact it stands under or of the rule's moment: whom the entity was blocked by, who asked, whom it
// touched. The world carries the commands (world.Plugin.Carry): the engine hands it every
// plugin.CommandHandler a stage uses.
//
// # Facts
//
// A fact is a component a plugin writes for entities with a Mind alone — navigation.Blocked,
// navigation.Arrived — and the plan only reads, so the rest pay nothing. Every fact is a component
// type, and goke registers 128 at most in all.
//
// # Conversation
//
// An Actor's Ask asks the subject of the fact it stands under for something — a type naming the
// ask, a game's own — and waits for the answer: yes runs its agreed step, no or no answer in time
// its refused one. The ask reaches the other entity a tick later as the fact [Asked], which its own
// plan answers with Agree or Refuse, or passes on with Relay to the subject of the fact it stands
// under — someone beside — telling the asker to wait ([Relayed]). The answer comes back as the fact
// [Replied]. Guards hold for every conversation: an ask reaches only an entity with a Mind; an ask
// or an answer nobody takes up is dropped after [AskLife]; a [Chain] lists who an ask passed
// through, at most [MaxChain], and a Relay to anyone on it, to the asker or to itself fails — so
// no ask goes round, and every conversation ends.
package rule
