package rule

import (
	"fmt"

	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/internal/steps"
)

// Plan is what an entity does over time: root run in every step of the simulation, given to a
// kind in its Spec. A name defined twice in one world panics.
func Plan(name string, root Step) Planned {
	if name == "" {
		panic("rule: a plan needs a name")
	}
	return Planned{
		Template: comp.Const(steps.Mind{Plan: steps.PlanID(name)}),
		p:        &planned{name: name, root: root.node(), defined: map[*steps.Plans]bool{}},
	}
}

// Planned is what Plan makes: the plan's mind as a kind's component.
type Planned struct {
	comp.Template[steps.Mind]
	p *planned
}

type planned struct {
	name    string
	root    steps.Step
	defined map[*steps.Plans]bool
}

// Name is the plan's.
func (p Planned) Name() string { return p.p.name }

// Define defines the plan on plans, a world's, once.
func (p Planned) Define(plans *steps.Plans) {
	if p.p.defined[plans] {
		return
	}
	if id := plans.Define(p.p.name, p.p.root); id != steps.PlanID(p.p.name) {
		panic(fmt.Sprintf("rule: the plan %q got another id than its name's", p.p.name))
	}
	p.p.defined[plans] = true
}
