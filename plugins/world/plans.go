package world

import (
	"fmt"

	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/plan"
)

// Plans are the plans of a Stage, by name: Define says a plan — what its entities do over time —
// and Named is the component a kind gives the entities that follow it. Reached through
// Plugin.Plans; each Stage's world has its own. A save knows a plan by its name.
type Plans struct {
	w      *Plugin
	byName map[string]comp.Comp
}

// Define says the plan called name, its steps written by body for an Actor. Call it where the
// Stage defines its rules; a name defined twice panics.
func (p *Plans) Define(name string, body func(a *plan.Actor) rule.Step) {
	p.w.must(fmt.Sprintf("plan %q defined", name), section.Rules)
	if p.byName == nil {
		p.byName = map[string]comp.Comp{}
	}
	p.byName[name] = comp.Const(steps.Mind{Plan: p.w.module.plans.Define(name, body(&plan.Actor{}))})
}

// Named is the plan defined as name, as the component of a kind whose entities follow it; an
// unknown name panics.
func (p *Plans) Named(name string) comp.Comp {
	c, ok := p.byName[name]
	if !ok {
		panic(fmt.Sprintf("world: no plan is defined as %q", name))
	}
	return c
}
