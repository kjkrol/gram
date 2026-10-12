package dialog

import "github.com/kjkrol/uid"

// Script is where an entity's conversations begin: a kind's component (Plugin.Script), the same
// for all its entities with comp.Const or each one's own with comp.Load.
type Script struct{ Start uint64 }

// Talk is the conversation an entity holds now as its speaker: the node it stands at and the
// listener; Node 0 for none. The plugin gives it where missing.
type Talk struct {
	Node uint64
	With uid.UID64
}

// talking reports whether the entity holds a conversation.
func (t Talk) talking() bool { return t.Node != 0 }
