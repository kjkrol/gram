package rule

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/gram/internal/steps"
)

// Branch is what is done When: at the moment P a plugin catches, as a rule a role obeys, or in a
// plan while the entity carries the fact P (any for nothing but its When). When is a predicate
// of P, a Cond, All of several or nil for always; Through, in a plan, goes on once begun until
// Do is done though P goes.
type Branch[P any] struct {
	Name    string
	When    any
	Through bool
	Do      Step
}

var _ Rule = Branch[struct{}]{}

func (Branch[P]) rule() {}

// String is the branch's name and its moment: "fall in" of unit.Standing.
func (b Branch[P]) String() string { return fmt.Sprintf("%q of %v", b.Name, reflect.TypeFor[P]()) }

func (b Branch[P]) narrowed(n narrowing) Rule { return b.build().narrowed(n) }

func (b Branch[P]) node() steps.Step {
	a := asked[P]{name: b.Name}
	a.take(b.When)
	return steps.NewBranch(b.Name, a.preds, a.guards, b.Through, b.do())
}

func (b Branch[P]) do() steps.Step {
	if b.Do == nil {
		return steps.NewThen("")
	}
	return b.Do.node()
}

func (b Branch[P]) build() Rule {
	if b.Name == "" {
		panic(fmt.Sprintf("rule: a rule of %v needs a name", reflect.TypeFor[P]()))
	}
	if b.Through {
		panic(fmt.Sprintf("rule: %q goes on past its moment, which a rule cannot", b.Name))
	}
	a := asked[P]{name: b.Name, top: true}
	a.take(b.When)
	root := steps.NewBranch(b.Name, a.preds, a.guards, false, b.do())
	return construct[P](b.Name, a.sides, root)
}

// Hosted is the branch built into the rule its moment's plugin runs.
func (b Branch[P]) Hosted() any { return b.build() }

// Build is r as its host takes it: a Branch built, any other rule as it is. It panics where
// the branch cannot be built.
func Build(r Rule) Rule {
	if b, ok := r.(interface{ build() Rule }); ok {
		return b.build()
	}
	return r
}
