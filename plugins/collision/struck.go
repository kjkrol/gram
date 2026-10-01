package collision

import "github.com/kjkrol/uid"

// Struck is what a rule hosted here is told about an entity that struck something the tick
// before: which it is, and what it struck. An entity that struck nothing is not told.
type Struck struct {
	ID       uid.UID64
	Contacts []Contact
}

// Who is the entity that struck: whose moment it is, for a rule.
func (s Struck) Who() uid.UID64 { return s.ID }
