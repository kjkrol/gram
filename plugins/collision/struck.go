package collision

import "github.com/kjkrol/uid"

// Struck is an entity that struck something the tick before, and what it struck. An entity that
// struck nothing is not told.
type Struck struct {
	ID       uid.UID64
	Contacts []Contact
}

// Who is the entity that struck.
func (s Struck) Who() uid.UID64 { return s.ID }
