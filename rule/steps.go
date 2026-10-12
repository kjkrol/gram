package rule

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/rule/effect"
)

// Step is a part of a rule or a plan: a Branch, a Command without an On, or what the package's
// functions make. The steps that last over ticks are a plan's alone; a rule is refused one as
// it is made.
type Step interface{ node() steps.Step }

type step struct{ s steps.Step }

func (s step) node() steps.Step { return s.s }

// topper is a step at the top of a plan, checked as one
type topper interface{ topNode() steps.Step }

func topNode(s Step) steps.Step {
	if t, ok := s.(topper); ok {
		return t.topNode()
	}
	return s.node()
}

type group struct {
	first bool
	parts []Step
}

func (g group) node() steps.Step { return g.made(func(s Step) steps.Step { return s.node() }) }

func (g group) topNode() steps.Step { return g.made(topNode) }

func (g group) made(each func(Step) steps.Step) steps.Step {
	nodes := make([]steps.Step, len(g.parts))
	for i, p := range g.parts {
		nodes[i] = each(p)
	}
	if g.first {
		return steps.NewFirst("", nodes...)
	}
	return steps.NewThen("", nodes...)
}

// OneOf does as the first of its steps that does not fail, and fails when all fail. In a plan an
// earlier step, once it can, takes over from a later one.
func OneOf(parts ...Step) Step { return group{first: true, parts: parts} }

// Steps runs its steps one after another while each does well, and fails with the first that
// fails.
func Steps(parts ...Step) Step { return group{parts: parts} }

// Hold holds the effect on the entity as long as the rule keeps firing, or in a plan as long as
// its branch runs; it fails when someone else takes the effect off.
func Hold(e effect.Effect) Step { return step{steps.NewKeep(e)} }

// Apply gives the entity the command cmd and does well at once; a plugin.Aimed command is told
// the Subject of the moment or the fact, and fails while there is none. A command carrying From
// is sent to that Subject instead, a plan's alone.
func Apply[C control.Command](cmd C) Step { return step{steps.NewOrder(cmd)} }
