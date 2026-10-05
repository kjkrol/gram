package plan

import (
	"time"

	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// Actor is the one a plan is written for: its methods make the steps it takes over time, and its
// When and On open a branch on a fact a plugin tells it. Plan hands one to the plan's body; a
// function writing part of a plan takes it as its own.
type Actor struct{}

// When is a branch, named name, that runs the steps body writes while the actor carries the fact
// F — a component a plugin puts on it — and fails without it, stopping them when F goes.
func (a *Actor) When[F any](name string, body func(a *Actor) rule.Step) rule.Step {
	return steps.NewWhen[F](name, body(a))
}

// On is a branch, named name, that runs the steps body writes from the tick the actor carries the
// fact F until they are done, whether F stays or not: for a fact that lasts a tick, a thing that
// happened.
func (a *Actor) On[F any](name string, body func(a *Actor) rule.Step) rule.Step {
	return steps.NewOn[F](name, body(a))
}

// OneOf runs its steps in order every tick and stands as the first that does not fail — so a step
// earlier in the list, once it can, takes over from a later one, which is stopped; it fails when
// all fail.
func (a *Actor) OneOf(parts ...rule.Step) rule.Step { return steps.NewFirst("", parts...) }

// Steps runs its steps one after another, each once the one before has done well; it fails with
// the first that fails and does well when the last does.
func (a *Actor) Steps(parts ...rule.Step) rule.Step { return steps.NewThen("", parts...) }

// If runs step while the actor carries the fact F and holds says F holds, and fails otherwise:
// If(Blocked.WaitedLong, …).
func (a *Actor) If[F any](holds func(F) bool, step rule.Step) rule.Step {
	return steps.NewIf(holds, step)
}

// Not does well where step fails, and fails where it does well.
func (a *Actor) Not(step rule.Step) rule.Step { return steps.NewInvert(step) }

// Until runs until the actor is given the fact F — holding as holds says, with one — afresh:
// carrying it at a tick after one it did not, so a fact left from before does not count; then it
// does well.
func (a *Actor) Until[F any](holds ...func(F) bool) rule.Step { return steps.NewUntil(holds...) }

// Wait runs for d, then does well.
func (a *Actor) Wait(d time.Duration) rule.Step { return steps.NewWait(d) }

// Timeout runs step for at most d, failing and stopping it when d is up.
func (a *Actor) Timeout(d time.Duration, step rule.Step) rule.Step { return steps.NewTimeout(d, step) }

// Cooldown runs step, and once it is done, fails for d before it runs it again.
func (a *Actor) Cooldown(d time.Duration, step rule.Step) rule.Step {
	return steps.NewCooldown(d, step)
}

// Idle runs for ever and does nothing: a plan's last resort, the actor as it is.
func (a *Actor) Idle() rule.Step { return steps.NewIdle() }

// Apply casts the effect on the actor, lasting as its Spec says — refreshed when on already — and
// does well.
func (a *Actor) Apply(e effect.Effect) rule.Step { return steps.NewApply(e) }

// Keep holds the effect on the actor for as long as it runs — until its branch gives way — and
// takes it off then; it never ends by itself, but fails when someone else takes the effect off.
func (a *Actor) Keep(e effect.Effect) rule.Step { return steps.NewKeep(e) }

// Dispel takes the effect off the actor with the effects' next pass, and does well; a branch that
// Keeps it gives way.
func (a *Actor) Dispel(e effect.Effect) rule.Step { return steps.NewDispel(e) }

// Chance runs step with likelihood p, drawn afresh at every step of the game from the world's seed,
// the clock's time and the actor, and fails otherwise: the same after a load and in a replay.
func (a *Actor) Chance(p float64, step rule.Step) rule.Step { return steps.NewChance(p, step) }

// Unless runs step while the actor is not under the effect, and fails while it is: an effect's
// presence is the actor's memory.
func (a *Actor) Unless(e effect.Effect, step rule.Step) rule.Step { return steps.NewUnless(e, step) }

// Under runs step while the actor is under the effect, and fails while it is not.
func (a *Actor) Under(e effect.Effect, step rule.Step) rule.Step { return steps.NewUnder(e, step) }

// During runs step while the world is under the effect — a state of the whole game, a lever
// pulled, an alarm (rule.Cast on entity.World) — and fails while it is not.
func (a *Actor) During(e effect.Effect, step rule.Step) rule.Step { return steps.NewDuring(e, step) }

// Playing runs step while the actor plays role, and fails while it does not.
func (a *Actor) Playing(role *rule.Part, step rule.Step) rule.Step {
	return steps.NewPlaying(uint8(role.Tag()), step)
}

// Order gives the command cmd for the actor — queued for the plugin that handles its type, the
// same command a player gives — each time it runs, and does well at once: fire and forget. What
// comes of it the branch waits for with the Command's Until, or it stays with Stay, so that a
// reactive branch gives it once. A command that is Aimed is told the subject of the fact it stands
// under first. A command no plugin used by the game handles panics.
func (a *Actor) Order[C any](cmd C) Command { return Command{steps.NewOrder(cmd)} }

// Command is a command an actor orders, as Order makes it: a step of its own, or followed by what
// the branch does after it.
type Command struct{ rule.Step }

// Until is the command, then waiting until the actor is given the fact F afresh — holding as
// holds says, with one: Order(MoveTo{…}).Until[Arrived]().
func (c Command) Until[F any](holds ...func(F) bool) rule.Step {
	return steps.NewThen("", c.Step, steps.NewUntil(holds...))
}

// Stay is the command, then staying for as long as the branch runs: given once as it begins, not
// every tick.
func (c Command) Stay() rule.Step { return steps.NewThen("", c.Step, steps.NewIdle()) }

// Ask asks the subject of the fact it stands under for W and waits up to wait for the answer: a
// yes runs agreed, a no or no answer in time runs refused — fails without one; word that the ask
// was passed on has it wait, up to the life of every ask on the chain, for the last word.
// With no subject, or one with no mind, it runs refused at once.
func (a *Actor) Ask[W any](name string, wait time.Duration, agreed rule.Step, refused ...rule.Step) rule.Step {
	return steps.NewAsk[W](name, wait, agreed, refused...)
}

// OnAsked is a branch, named name, that runs the steps body writes from the tick the actor is
// asked for W — the asker the subject under it, for Agree, Refuse or Relay — until they are done.
func (a *Actor) OnAsked[W any](name string, body func(a *Actor) rule.Step) rule.Step {
	return steps.NewOn[steps.Asked[W]](name, body(a))
}

// Agree answers the actor's ask for W yes, and does well; it fails with no ask.
func (a *Actor) Agree[W any]() rule.Step { return steps.NewAgree[W]() }

// Refuse answers the actor's ask for W no, and does well; it fails with no ask.
func (a *Actor) Refuse[W any]() rule.Step { return steps.NewRefuse[W]() }

// Relay passes the actor's ask for W on to the subject of the fact it stands under — someone
// beside who stands in the way — and tells the asker to wait; it runs, the ask kept, until its
// branch gives way and the actor answers itself, and it fails when the one asked on refuses — or
// at once with no ask, no subject, one already on the ask's chain, the asker or itself, or the
// chain full. Word from further on — passed on again, or a yes — is passed back to the asker as
// word to wait again.
func (a *Actor) Relay[W any]() rule.Step { return steps.NewRelay[W]() }
