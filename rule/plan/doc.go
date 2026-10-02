// Package plan is what an entity does over time: a plan, written for an [Actor] and given to a
// kind as a component ([New]), run by the world in every step of its simulation — standing in the
// tactical pause, going with the tempo, saved with the game. Its steps are those of package rule,
// and the ones that last.
//
//	plan.New("patrol", func(a *plan.Actor) rule.Step {
//		return a.Steps(
//			a.Order(navigation.MoveTo{Cell: east}).Until[navigation.Arrived](),
//			a.Order(navigation.MoveTo{Cell: west}).Until[navigation.Arrived](),
//		)
//	})
//
// # Plans
//
// [New] is the component a kind gives its entities: units.Define(..., plan.New("patrol", …)). Its
// name is what a save knows it by; one name is one plan, written again it must be alike. A plan is
// plain Go: functions taking the Actor and returning steps, each branch named and short. An
// Actor's When opens a branch that runs while the actor carries a fact, its On one that runs from
// the tick a fact comes until its steps are done. OneOf runs its steps in order every tick and
// stands as the first that does not fail — an earlier branch, once it can, takes over and the
// later one is stopped; Steps runs its steps one after another, remembering where it is; If runs a
// step while a fact holds as a function of it says; Until waits for a fact to come afresh; Wait
// waits; Timeout, Cooldown and Not decorate; Idle runs for ever. The state of a plan is the
// entity's [Mind]: the plan, the steps running, each one's place and start on the world's clock.
// The world makes and runs the plans ([NewPlans]).
//
// # Effects and commands
//
// Apply, Keep, Dispel, Unless, Under and During are a rule's, over time: a plan's Keep holds its
// effect for as long as its branch runs and gives way — its branch fails — when someone else
// takes it off. Order gives a command for the actor and hands back a [Command]: its Until waits
// for what comes of it, a fact — navigation.Arrived — and its Stay keeps the branch, so that a
// reactive branch gives it once, not every tick. A command that is rule.Aimed is told the subject
// of the fact it stands under: whom the actor was blocked by, who asked.
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
// its refused one. The ask reaches the other entity a tick later as the fact [Asked], which its
// own plan answers with Agree or Refuse, or passes on with Relay to the subject of the fact it
// stands under — someone beside — telling the asker to wait ([Relayed]). The answer comes back as
// the fact [Replied]. An ask reaches only an entity with a Mind; an ask or an answer nobody takes
// up is dropped after [AskLife]; a [Chain] lists who an ask passed through, at most [MaxChain],
// and a Relay to anyone on it, to the asker or to itself fails — so every conversation ends.
package plan
