package rule

import (
	"fmt"
	"time"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/rule/effect"
)

// Command is a command about an effect and whom it is for, written as a sentence:
// Put(open).On(entity.Group("trapdoors")).By(entity.Named("lever")).For(pulse). Without an On
// it is a step of a rule or a plan, acting on whom the branch does.
type Command struct {
	Verb   Verb
	Effect effect.Effect
	Whom   Target
	Source entity.Whom   // nobody for none
	Lasts  time.Duration // zero: as the effect's Spec says
	Name   string        // empty for one not in the Stage's register
}

// Defined reports whether the command was defined in a Stage's register.
func (c Command) Defined() bool { return c.Name != "" }

// Verb is what a Command does with its effect.
type Verb uint8

const (
	Puts Verb = iota
	Takes
	Toggles // takes it off where any target is under it, else puts it on
)

// Target is whom a Command is for: an entity.Whom, or a plugin's own, which is a Router.
type Target interface{ Target() }

// Router is a Target of a plugin's own: Route is the command that plugin carries out for c.
type Router interface {
	Target
	Route(c Command) control.Command
}

// Put puts e on whoever On names, or as a step on whom the branch acts.
func Put(e effect.Effect) Command { return Command{Verb: Puts, Effect: e} }

// Take takes e off whoever On names, or as a step off whom the branch acts.
func Take(e effect.Effect) Command { return Command{Verb: Takes, Effect: e} }

// Toggle takes e off whoever On names where any of them is under it, and puts it on them all
// otherwise; a command alone, never a step.
func Toggle(e effect.Effect) Command { return Command{Verb: Toggles, Effect: e} }

// On says whom the command is for.
func (c Command) On(whom Target) Command { c.Whom = whom; return c }

// By says who sets the command off: an entity bearing the name or the group, as it gives
// itself Triggered.
func (c Command) By(source entity.Whom) Command { c.Source = source; return c }

// For says how long a Put lasts, in place of what the effect's Spec says.
func (c Command) For(d time.Duration) Command { c.Lasts = d; return c }

// Routed is the command as the plugin of its target takes it. It panics for a command that
// names no target.
func (c Command) Routed() control.Command {
	switch t := c.Whom.(type) {
	case nil:
		panic(fmt.Sprintf("rule: %v names nobody: say whom with On", c))
	case Router:
		return t.Route(c)
	}
	return c
}

func (c Command) node() steps.Step {
	if c.Whom != nil {
		return steps.NewOrder(c)
	}
	switch c.Verb {
	case Puts:
		if c.Lasts > 0 {
			return steps.NewApplyFor(c.Effect, c.Lasts)
		}
		return steps.NewApply(c.Effect)
	case Takes:
		return steps.NewDispel(c.Effect)
	}
	panic(fmt.Sprintf("rule: %v is a command, not a step: say whom with On", c))
}

// String says what the command does.
func (c Command) String() string {
	verb := [...]string{"Put", "Take", "Toggle"}[c.Verb]
	if c.Whom == nil {
		return fmt.Sprintf("%s(effect %d)", verb, c.Effect.Mark())
	}
	return fmt.Sprintf("%s(effect %d).On(%v)", verb, c.Effect.Mark(), c.Whom)
}

// Triggered is the command by which an entity sets off every Command whose By names it: Apply
// it in a rule.
type Triggered struct{}

func (Command) Command()   {}
func (Triggered) Command() {}

var (
	_ control.Command = Command{}
	_ control.Command = Triggered{}
)
