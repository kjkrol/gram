// Package rule is how entities behave, one vocabulary in five packages: a rule's branch says
// when what is done (rule/act), a plan's loop what an entity does over time (rule/plan), every
// clause of either saying whom it is about (rule/cond); states are effects (rule/effect); and
// this package ties them to a game — the roles whose players obey rules, the commands about
// effects, the messages between plans.
//
//	roles.Define(MortalRole, act.Branch[unit.Standing]{
//		Name: "fall in",
//		When: cond.Self.If(unit.Fallen()),
//		Then: act.Self.Do(world.Despawn{}),
//	})
//
// # Roles
//
// A [Part] is a role: the rules those playing it obey ([Part.Obeys]), narrowed to its players.
// A kind plays roles through [Plays], one component for all; the world's register makes the
// parts ([NewPart]) and tags them in cond.Roles.
//
// # Commands
//
// A [Command] is what somebody wants done about an effect, written as a sentence: [Put],
// [Take] or [Toggle] an effect, On whom, By whom set off, For how long. A key gives one, a
// rule gives one with act.Self.Do, and an entity gives every Command whose By names it with
// [Trigger].
//
// # Messages
//
// A command embedding [From], given with plan.Other.Do, lands on that entity as a fact in
// place of going to a queue, the engine filling in who sent it; the other answers with a loop
// of the command's type. A message nobody takes up is dropped after [AskLife]; one
// passed on [MaxChain] times is not sent.
package rule
