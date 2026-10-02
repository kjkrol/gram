package plan

import (
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/rule"
)

// New is the plan named name — what a save knows it by — whose steps body writes for an Actor: a
// component a kind gives its entities, which behave by it from then on. One name is one plan:
// written again, it must be alike.
func New(name string, body func(a *Actor) rule.Step) comp.Comp {
	return comp.Const(steps.Mind{Plan: steps.Register(name, body(&Actor{}))})
}
