package act

import "time"

// MaxNodes is how many nodes one tree holds at most: a Mind keeps a slot for each.
const MaxNodes = 128

// Mind is the state of an entity's tree: which tree, the nodes running since the last tick, and
// what each keeps — a sequence its step, a While whether it holds, a wait when it began. A kind
// gives it with Tree; the trees' system runs it.
type Mind struct {
	Tree uint64 // the tree's name, hashed
	// The rest is the trees' own, exported for the saves: the nodes that ran and were not done
	// last tick, a bit each; what each keeps; when each began, on the world's clock.
	Running Nodes
	Slot    [MaxNodes]uint8
	Since   [MaxNodes]time.Duration
}

// Nodes is a set of a tree's nodes, a bit each.
type Nodes [MaxNodes / 64]uint64

func (n *Nodes) has(at int) bool { return n[at>>6]&(1<<(at&63)) != 0 }
func (n *Nodes) add(at int)      { n[at>>6] |= 1 << (at & 63) }
