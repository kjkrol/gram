// Package act is what a rule does: the Then of a Branch, every step said of whom it is done
// by or to, and the Branch itself, which given to a role is a rule.
//
//	act.Branch[unit.Standing]{
//		Name: "fall in",
//		When: cond.Self.If(unit.Fallen()),
//		Then: act.Self.Do(world.Despawn{}),
//	}
//
// # Steps
//
// [Self], [Other] and [Around] are the subjects; their methods are the steps: [Subject.Put],
// [Subject.Take] or [Subject.Hold] an effect, [Subject.Do] a command. [Steps] runs its parts
// in order, [OneOf] the first that does not fail; both take steps and branches.
//
// # Branches
//
// A [Branch] at the top of a rule says its Name; every Branch says its When and its Then. It
// fires at its moment and is done within the pass. A rule turning to places (cond.Around,
// [Around]) runs once a place: the smallest part of it holding every clause of a place.
//
// # For package plan
//
// A plan is made of what lasts over ticks, in package plan, out of the same steps: every
// [Step] is a [Lasting], and [NewWait], [NewIdle], [NewLoop], [LastingSteps], [LastingOneOf]
// and [NewPlan] are the parts package plan builds on.
package act
