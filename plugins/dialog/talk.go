package dialog

import "github.com/kjkrol/uid"

// Script is the component saying where an entity's conversations begin.
type Script struct{ Start uint64 }

// Talk is the conversation an entity holds now as its speaker: the node it stands at and the
// listener; Node 0 for none.
type Talk struct {
	Node uint64
	With uid.UID64
}

func (t Talk) talking() bool { return t.Node != 0 }
