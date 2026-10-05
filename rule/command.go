package rule

import (
	"fmt"
	"time"

	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/rule/effect"
)

// Casting is a command about an effect and whom it is for, written as a sentence: Cast(open).
// On(entity.Group("trapdoors")).By(entity.Named("lever")).For(pulse). A key, a script, a rule's
// Order or an entity's Trigger gives it; the plugin its target belongs to carries it out — the
// world for those named, grouped or the world itself.
type Casting struct {
	Verb   Verb
	Effect effect.Effect
	Whom   Target        // whom it is for: On
	Source entity.Whom   // who sets it off with Trigger: By; nobody for none
	Lasts  time.Duration // how long a Cast lasts: For; zero as the effect's Spec says
}

// Verb is what a Casting does with its effect.
type Verb uint8

const (
	Casts   Verb = iota // puts it on
	Lifts               // takes it off
	Toggles             // takes it off where any target is under it, else puts it on
)

// Target is whom a Casting is for: entity.Named, entity.Group, entity.World, or a plugin's own —
// the selected units, the one pointed at — which is a Router besides.
type Target interface{ Target() }

// Router is a Target of a plugin's own: Route is the command that plugin carries out for c.
type Router interface {
	Target
	Route(c Casting) any
}

// Cast is the command to put e on whoever On names.
func Cast(e effect.Effect) Casting { return Casting{Verb: Casts, Effect: e} }

// Lift is the command to take e off whoever On names.
func Lift(e effect.Effect) Casting { return Casting{Verb: Lifts, Effect: e} }

// Toggle is the command to take e off whoever On names where any of them is under it, and to put
// it on them all otherwise: a switch.
func Toggle(e effect.Effect) Casting { return Casting{Verb: Toggles, Effect: e} }

// On says whom the command is for.
func (c Casting) On(whom Target) Casting { c.Whom = whom; return c }

// By says who sets the command off with a rule's Trigger: a lever, a plate. The game hands such
// commands to its Initializer's Commands.
func (c Casting) By(source entity.Whom) Casting { c.Source = source; return c }

// For says how long a Cast lasts, in place of what the effect's Spec says.
func (c Casting) For(d time.Duration) Casting { c.Lasts = d; return c }

// Routed is the command as the plugin of its target takes it: the Casting itself for the world's
// targets. It panics for a command that names no target.
func (c Casting) Routed() any {
	switch t := c.Whom.(type) {
	case nil:
		panic(fmt.Sprintf("rule: %v names nobody: say whom with On", c))
	case Router:
		return t.Route(c)
	}
	return c
}

// String says what the command does.
func (c Casting) String() string {
	verb := [...]string{"Cast", "Lift", "Toggle"}[c.Verb]
	if c.Whom == nil {
		return fmt.Sprintf("%s(effect %d)", verb, c.Effect.Mark())
	}
	return fmt.Sprintf("%s(effect %d).On(%v)", verb, c.Effect.Mark(), c.Whom)
}

// Triggered is the command an entity gives itself with Trigger: the world gives every Casting
// whose Source names it.
type Triggered struct{}

// Trigger gives the commands the entity sets off — those whose By names it or its group — and
// does well at once: a plate pressed, a lever pulled.
func Trigger() Step { return steps.NewOrder(Triggered{}) }
