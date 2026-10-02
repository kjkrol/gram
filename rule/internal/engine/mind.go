package engine

import "time"

// MaxSteps is how many steps one plan holds at most: a Mind keeps a slot for each.
const MaxSteps = 128

// Mind is the state of an entity's plan: which plan, the steps running since the last tick, and
// what each keeps — a sequence its place, a Keep whether it holds, a wait when it began. A kind
// gives it with Plan; the plans' system runs it.
type Mind struct {
	Plan uint64 // the plan's name, hashed
	// The rest is the plans' own, exported for the saves: the steps that ran and were not done
	// last tick, a bit each; what each keeps; when each began, on the world's clock.
	Running StepSet
	Slot    [MaxSteps]uint8
	Since   [MaxSteps]time.Duration
}

// StepSet is a set of a plan's steps, a bit each.
type StepSet [MaxSteps / 64]uint64

func (n *StepSet) has(at int) bool { return n[at>>6]&(1<<(at&63)) != 0 }
func (n *StepSet) add(at int)      { n[at>>6] |= 1 << (at & 63) }
