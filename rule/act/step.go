package act

import (
	"reflect"

	"github.com/kjkrol/gram/internal/steps"
)

// Step is a part of a rule's Then: a step said of somebody (Self.Do, Other.Put, Around(n).Put),
// a group of them (Steps, OneOf) or a Branch. Every Step is a Lasting too: a plan takes it.
type Step interface {
	node(name string, b *build) steps.Step
	places() placed
	// Lasting is the step as a plan takes it (plan.Part).
	Lasting() Lasting
}

// Lasting is a step as a plan takes it: every Step, and what package plan makes of its own —
// steps that last over ticks, loops, groups of them.
type Lasting interface {
	node(name string, b *build) steps.Step
	places() placed
}

// a build is one rule's or plan's: whose places the rule turns to, how many clauses say so,
// and the moment of a rule (nil in a plan), which its words must be of
type build struct {
	rings, total int
	moment       reflect.Type
}

// place wraps n, the node of s, to run once a place where s holds every clause of a place and
// none of its parts does alone
func (b *build) place(s Lasting, n steps.Step, parts ...Lasting) steps.Step {
	if b.total == 0 || s.places().count != b.total {
		return n
	}
	for _, p := range parts {
		if p.places().count == b.total {
			return n
		}
	}
	return steps.NewAround(b.rings, n)
}

type placed struct {
	rings, count int
	mixed        bool
}

func (p placed) with(o placed) placed {
	if p.count == 0 {
		return o
	}
	if o.count == 0 {
		return p
	}
	return placed{rings: p.rings, count: p.count + o.count,
		mixed: p.mixed || o.mixed || p.rings != o.rings}
}

type leaf struct {
	s       steps.Step
	lasting bool
	sign    string
}

type group struct {
	first bool
	parts []Lasting
}

func (g group) Lasting() Lasting { return g }

func (g group) node(name string, b *build) steps.Step {
	nodes := make([]steps.Step, len(g.parts))
	for i, p := range g.parts {
		nodes[i] = p.node(name, b)
	}
	var n steps.Step
	if g.first {
		n = steps.NewFirst("", nodes...)
	} else {
		n = steps.NewThen("", nodes...)
	}
	return b.place(g, n, g.parts...)
}

func (g group) places() placed {
	var p placed
	for _, part := range g.parts {
		p = p.with(part.places())
	}
	return p
}

func lasting(parts []Step) []Lasting {
	out := make([]Lasting, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out
}

// OneOf does as the first of its steps that does not fail, and fails when all fail.
func OneOf(parts ...Step) Step { return group{first: true, parts: lasting(parts)} }

// Steps runs its steps one after another while each does well, and fails with the first that
// fails.
func Steps(parts ...Step) Step { return group{parts: lasting(parts)} }
