package vision

import (
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// Sighting is one observer and everything in its view carrying the rule's second tag,
// nearest first, possibly none.
type Sighting struct {
	Self  uid.UID64
	Base  *world.Base
	Sight *Sight
	Seen  []Seen
}

// Who is the observer: whose moment it is, for a rule.
func (s Sighting) Who() uid.UID64 { return s.Self }

// Subject is the nearest one seen, whom an Aimed command given on the moment is about; false
// while none is in view.
func (s Sighting) Subject() (uid.UID64, bool) {
	if len(s.Seen) == 0 {
		return 0, false
	}
	return s.Seen[0].ID, true
}

// Whom tells each one the observer sees.
func (s Sighting) Whom(each func(uid.UID64)) {
	for _, seen := range s.Seen {
		each(seen.ID)
	}
}

// Seen is one entity in an observer's view: which, where and how it moves, how far off — and
// which tags it carries, of the families the plugin's rules name (plugin.Carries).
type Seen struct {
	ID   uid.UID64
	Base *world.Base
	Dist float32
	rule.Marks
}
