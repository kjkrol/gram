package act

import (
	"fmt"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/rule/effect"
)

type whoKind uint8

const (
	ofSelf whoKind = iota
	ofOther
	ofAround
)

// Subject is whom a step is done by or to: Self, Other or Around(n). Its methods are the steps.
type Subject struct {
	kind  whoKind
	rings int
}

var (
	// Self is the entity the branch is about.
	Self = Subject{kind: ofSelf}
	// Other is the other of the pair the moment met, each of them, or the subject of the
	// moment or the fact; a step of it fails while there is none.
	Other = Subject{kind: ofOther}
)

// Around is each place within rings of where the entity stands, in a rule of a moment that is
// plugin.Placed.
func Around(rings int) Subject { return Subject{kind: ofAround, rings: rings} }

// String says whom.
func (s Subject) String() string {
	switch s.kind {
	case ofSelf:
		return "Self"
	case ofOther:
		return "Other"
	}
	return fmt.Sprintf("Around(%d)", s.rings)
}

// Do gives the subject the command cmd and does well at once; a plugin.Aimed command is told
// the Subject of the moment or the fact, and fails while there is none. A command carrying
// rule.From is sent to the other as a message, a plan's alone. A command that names nobody
// panics here.
func (s Subject) Do[C control.Command](cmd C) Step {
	if r, ok := any(cmd).(control.Routed); ok {
		r.Routed()
	}
	return clause{who: s, step: leaf{s: steps.NewOrder(cmd), sign: fmt.Sprintf("Do(%T)", cmd)}}
}

// Put puts the effect on the subject.
func (s Subject) Put(e effect.Effect) Step {
	return clause{who: s,
		step: leaf{s: steps.NewApply(e), sign: fmt.Sprintf("Put(effect %d)", e.Mark())}}
}

// Take takes the effect off the subject.
func (s Subject) Take(e effect.Effect) Step {
	return clause{who: s,
		step: leaf{s: steps.NewDispel(e), sign: fmt.Sprintf("Take(effect %d)", e.Mark())}}
}

// Hold holds the effect on the subject as long as the rule keeps firing, or in a plan as long
// as its loop runs; it fails when someone else takes the effect off.
func (s Subject) Hold(e effect.Effect) Step {
	return clause{who: s,
		step: leaf{s: steps.NewKeep(e), sign: fmt.Sprintf("Hold(effect %d)", e.Mark())}}
}

type clause struct {
	who  Subject
	step leaf
}

func (c clause) Lasting() Lasting { return c }

func (c clause) node(name string, b *build) steps.Step {
	if c.step.lasting && c.who.kind != ofSelf {
		panic(fmt.Sprintf("act: %q: %v.%s: the entity of the plan waits, Self alone", name, c.who,
			c.step.sign))
	}
	switch c.who.kind {
	case ofSelf:
		if b.total == 0 { // nothing turns the steps to anyone else
			return c.step.s
		}
		return steps.NewSame(c.step.s)
	case ofOther:
		return steps.NewForOther(c.step.s)
	}
	return b.place(c, c.step.s)
}

func (c clause) places() placed {
	if c.who.kind == ofAround {
		return placed{rings: c.who.rings, count: 1}
	}
	return placed{}
}
