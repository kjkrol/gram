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
// Cast(open).On(entity.Group("trapdoors")).By(entity.Named("lever")).For(pulse).
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
	Casts Verb = iota
	Lifts
	Toggles // takes it off where any target is under it, else puts it on
)

// Target is whom a Command is for: an entity.Whom, or a plugin's own, which is a Router.
type Target interface{ Target() }

// Router is a Target of a plugin's own: Route is the command that plugin carries out for c.
type Router interface {
	Target
	Route(c Command) control.Command
}

// Cast is the command to put e on whoever On names.
func Cast(e effect.Effect) Command { return Command{Verb: Casts, Effect: e} }

// Lift is the command to take e off whoever On names.
func Lift(e effect.Effect) Command { return Command{Verb: Lifts, Effect: e} }

// Toggle is the command to take e off whoever On names where any of them is under it, and to put
// it on them all otherwise.
func Toggle(e effect.Effect) Command { return Command{Verb: Toggles, Effect: e} }

// On says whom the command is for.
func (c Command) On(whom Target) Command { c.Whom = whom; return c }

// By says who sets the command off with Trigger.
func (c Command) By(source entity.Whom) Command { c.Source = source; return c }

// For says how long a Cast lasts, in place of what the effect's Spec says.
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

// String says what the command does.
func (c Command) String() string {
	verb := [...]string{"Cast", "Lift", "Toggle"}[c.Verb]
	if c.Whom == nil {
		return fmt.Sprintf("%s(effect %d)", verb, c.Effect.Mark())
	}
	return fmt.Sprintf("%s(effect %d).On(%v)", verb, c.Effect.Mark(), c.Whom)
}

// Triggered is the command an entity gives itself with Trigger.
type Triggered struct{}

func (Command) Command()   {}
func (Triggered) Command() {}

var (
	_ control.Command = Command{}
	_ control.Command = Triggered{}
)

// Trigger gives the commands whose By names the entity or its group, and does well at once.
func Trigger() Step { return step{steps.NewOrder(Triggered{})} }
