package plan

import "github.com/kjkrol/gram/rule/act"

// Part is a part of a plan: a rule's step (act.Step), a step that lasts (Self.Wait,
// Self.Idle), a Loop, or Steps and OneOf of them.
type Part interface {
	// Lasting is the part as the plan is built.
	Lasting() act.Lasting
}

type part struct{ l act.Lasting }

func (p part) Lasting() act.Lasting { return p.l }

func lasting(parts []Part) []act.Lasting {
	out := make([]act.Lasting, len(parts))
	for i, p := range parts {
		out[i] = p.Lasting()
	}
	return out
}

// Steps runs its parts one after another while each does well, remembering where it is,
// and fails with the first that fails.
func Steps(parts ...Part) Part { return part{act.LastingSteps(lasting(parts)...)} }

// OneOf does as the first of its parts that does not fail: every tick from the top, an earlier
// part taking over from a later one once it can.
func OneOf(parts ...Part) Part { return part{act.LastingOneOf(lasting(parts)...)} }

// New is the plan named name: root run in every step of the simulation, given to a kind in
// its Spec; a loop at its top says its Name. A name defined twice in one world panics.
func New(name string, root Part) act.Planned { return act.NewPlan(name, root.Lasting()) }
