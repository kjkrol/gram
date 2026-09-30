package act

import (
	"time"

	"github.com/kjkrol/gram/plugins/world/act/effect"
)

// Branch builds a branch of a tree: its constructor — When, On or Named — says what it stands
// under and names it, its methods make the nodes under it, and Do closes it round its body. The
// root's name is what a save knows the tree by. A function handing back part of a branch may use
// a Branch of its own; its zero value will do.
type Branch struct {
	name string
	wrap func(name string, body Node) Node
}

// When is a branch that runs its body while the entity carries the fact F — a component a plugin
// puts on it — and fails without it, stopping the body when F goes.
func When[F any](name string) *Branch {
	return &Branch{name: name, wrap: func(name string, body Node) Node { return newWhen[F](name, body) }}
}

// On is a branch that runs its body from the tick the entity carries the fact F until the body is
// done, whether F stays or not: for a fact that lasts a tick, a thing that happened.
func On[F any](name string) *Branch {
	return &Branch{name: name, wrap: func(name string, body Node) Node { return newOn[F](name, body) }}
}

// Named is a branch that runs its body, under name: a tree's root.
func Named(name string) *Branch { return &Branch{name: name} }

// Do is the branch round body.
func (c *Branch) Do(body Node) Node {
	if c.wrap != nil {
		return c.wrap(c.name, body)
	}
	if c.name == "" {
		return body
	}
	if n, ok := bare(body).(composite); ok && n.name == "" && (n.sign == "first" || n.sign == "then") {
		n.name = c.name
		return n
	}
	return newThen(c.name, body)
}

// First runs its nodes in order every tick and stands as the first that does not fail — so a
// node earlier in the list, once it can, takes over from a later one, which is stopped; it fails
// when all fail.
func (c *Branch) First(nodes ...Node) Node { return newFirst("", nodes...) }

// Then runs its nodes one after another, each once the one before has done well; it fails with
// the first that fails and does well when the last does.
func (c *Branch) Then(nodes ...Node) Node { return newThen("", nodes...) }

// If runs node while the entity carries the fact F and holds says F holds, and fails otherwise:
// If(Blocked.ByStranger, …).
func (c *Branch) If[F any](holds func(F) bool, node Node) Node { return newIf(holds, node) }

// Until runs until the entity is given the fact F — holding as holds says, with one — afresh:
// carrying it at a tick after one it did not, so a fact left from before does not count; then it
// does well.
func (c *Branch) Until[F any](holds ...func(F) bool) Node { return newUntil(holds...) }

// Wait runs for d, then does well.
func (c *Branch) Wait(d time.Duration) Node { return newWait(d) }

// Timeout runs node for at most d, failing and stopping it when d is up.
func (c *Branch) Timeout(d time.Duration, node Node) Node { return newTimeout(d, node) }

// Cooldown runs node, and once it is done, fails for d before it runs it again.
func (c *Branch) Cooldown(d time.Duration, node Node) Node { return newCooldown(d, node) }

// Invert does well where node fails, and fails where it does well.
func (c *Branch) Invert(node Node) Node { return newInvert(node) }

// Idle runs for ever and does nothing: a tree's last resort, the entity as it is.
func (c *Branch) Idle() Node { return newIdle() }

// Apply casts the effect on the entity, lasting as its Spec says — refreshed when on already —
// and does well.
func (c *Branch) Apply(e effect.Effect) Node { return newApply(e) }

// While holds the effect on the entity for as long as it runs — until its branch gives way — and
// takes it off then; it never ends by itself.
func (c *Branch) While(e effect.Effect) Node { return newWhile(e) }

// Unless runs node while the entity is not under the effect, and fails while it is: an effect's
// presence is the entity's memory.
func (c *Branch) Unless(e effect.Effect, node Node) Node { return newUnless(e, node) }

// IfUnder runs node while the entity is under the effect, and fails while it is not.
func (c *Branch) IfUnder(e effect.Effect, node Node) Node { return newIfUnder(e, node) }

// Issue gives the command cmd for the entity — queued for the plugin that handles its type, the
// same command a player gives — each time it runs, and does well at once: fire and forget. What
// comes of it the branch waits for with the Command's Until, or it stays with Stay, so that a
// reactive branch gives it once. A command that is Aimed is told the subject of the fact it stands
// under first. A command no plugin used by the game handles panics.
func (c *Branch) Issue[C any](cmd C) Command { return Command{newIssue(cmd)} }

// Command is a command a branch issues, as Issue makes it: a node of its own, or followed by what
// the branch does after it.
type Command struct{ Node }

// Until is the command, then waiting until the entity is given the fact F afresh — holding as
// holds says, with one: Issue(MoveTo{…}).Until[Arrived]().
func (c Command) Until[F any](holds ...func(F) bool) Node {
	return newThen("", c.Node, newUntil(holds...))
}

// Stay is the command, then staying for as long as the branch runs: given once as it begins, not
// every tick.
func (c Command) Stay() Node { return newThen("", c.Node, newIdle()) }

// Ask asks the subject of the fact it stands under for W and waits up to wait for the answer: a
// yes runs agreed, a no or no answer in time runs refused — fails without one; word that the ask
// was passed on has it wait up to AskLife, the life of every ask on the chain, for the last word.
// With no subject, or one with no mind, it runs refused at once.
func (c *Branch) Ask[W any](name string, wait time.Duration, agreed Node, refused ...Node) Node {
	return newAsk[W](name, wait, agreed, refused...)
}

// Agree answers the entity's ask for W yes, and does well; it fails with no ask.
func (c *Branch) Agree[W any]() Node { return newAgree[W]() }

// Refuse answers the entity's ask for W no, and does well; it fails with no ask.
func (c *Branch) Refuse[W any]() Node { return newRefuse[W]() }

// Relay passes the entity's ask for W on to the subject of the fact it stands under — someone
// beside who stands in the way — and tells the asker to wait; it runs, the ask kept, until its
// branch gives way and the entity answers itself, and it fails when the one asked on refuses — or
// at once with no ask, no subject, one already on the ask's chain, the asker or itself, or the
// chain full. Word from further on — passed on again, or a yes — is passed back to the asker as
// word to wait again.
func (c *Branch) Relay[W any]() Node { return newRelay[W]() }
