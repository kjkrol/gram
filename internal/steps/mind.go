package steps

import "time"

// MaxSteps is how many steps one plan holds at most.
const MaxSteps = 128

// Mind is the state of an entity's plan: which plan, the steps running since the last tick and
// what each keeps. goke needs exported, fixed-size fields.
type Mind struct {
	Plan    uint64 // the name hashed
	Running StepSet
	Slot    [MaxSteps]uint8
	Since   [MaxSteps]time.Duration // on the world's clock
}

// StepSet is a set of a plan's steps, a bit each.
type StepSet [MaxSteps / 64]uint64

func (n *StepSet) has(at int) bool { return n[at>>6]&(1<<(at&63)) != 0 }
func (n *StepSet) add(at int)      { n[at>>6] |= 1 << (at & 63) }
