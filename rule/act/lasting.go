package act

import (
	"fmt"
	"reflect"
	"time"

	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/rule/cond"
)

// NewWait is the entity doing nothing for d, then done well; for package plan.
func NewWait(d time.Duration) Lasting {
	return clause{who: Self, step: leaf{s: steps.NewWait(d), lasting: true,
		sign: fmt.Sprintf("Wait(%v)", d)}}
}

// NewIdle is the entity doing nothing for ever; for package plan.
func NewIdle() Lasting {
	return clause{who: Self, step: leaf{s: steps.NewIdle(), lasting: true, sign: "Idle()"}}
}

// LastingSteps is Steps of what a plan takes; for package plan.
func LastingSteps(parts ...Lasting) Lasting { return group{parts: parts} }

// LastingOneOf is OneOf of what a plan takes; for package plan.
func LastingOneOf(parts ...Lasting) Lasting { return group{first: true, parts: parts} }

// NewLoop is a loop of a plan on the fact P (any for none): then run from the start as when
// comes to hold, ending well once until holds, failing after timeout, not beginning again
// within cooldown of its last run; for package plan.
func NewLoop[P any](name string, when cond.Cond, then Lasting, until cond.Cond,
	timeout, cooldown time.Duration) Lasting {
	return loop[P]{name: name, when: when, then: then, until: until, timeout: timeout,
		cooldown: cooldown}
}

type loop[P any] struct {
	name              string
	when, until       cond.Cond
	then              Lasting
	timeout, cooldown time.Duration
}

func (l loop[P]) node(outer string, b *build) steps.Step {
	name := l.name
	if name == "" {
		name = outer
	}
	if l.when == nil {
		panic(fmt.Sprintf("act: %q needs a When: cond.Self.If(cond.Always) for every time", name))
	}
	if l.then == nil {
		panic(fmt.Sprintf("act: %q needs a Then", name))
	}
	low := l.when.(steps.Lowered).Lower(steps.Inside, name)
	if low.Places > 0 || l.then.places().count > 0 {
		panic(fmt.Sprintf("act: %q: Around is a rule's: a plan has no places", name))
	}
	var until steps.Guard
	if l.until != nil {
		until = l.until.(steps.Lowered).Lower(steps.AsUntil, name).Guard
	}
	n := steps.NewBranch[P](name, low.Guard, l.then.node(name, b), until)
	if l.timeout > 0 {
		n = steps.NewTimeout(l.timeout, n)
	}
	if l.cooldown > 0 {
		n = steps.NewCooldown(l.cooldown, n)
	}
	return n
}

func (loop[P]) places() placed { return placed{} }

func (l loop[P]) topNode() steps.Step {
	if l.name == "" {
		panic(fmt.Sprintf("act: a loop of %v at the top of a plan needs a Name",
			reflect.TypeFor[P]()))
	}
	return l.node(l.name, &build{})
}

// topper is a part at the top of a plan, checked as one
type topper interface{ topNode() steps.Step }

func topNode(s Lasting) steps.Step {
	if t, ok := s.(topper); ok {
		return t.topNode()
	}
	if g, ok := s.(group); ok {
		nodes := make([]steps.Step, len(g.parts))
		for i, p := range g.parts {
			nodes[i] = topNode(p)
		}
		if g.first {
			return steps.NewFirst("", nodes...)
		}
		return steps.NewThen("", nodes...)
	}
	return s.node("", &build{})
}
