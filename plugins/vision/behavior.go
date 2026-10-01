package vision

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// Sighting is one observer and everything in its view carrying the rule's second tag,
// nearest first, possibly none. Helm is the zero one for an observer that cannot be steered.
type Sighting struct {
	Self  uid.UID64
	Base  *world.Base
	Sight *Sight
	Helm  steering.Helm
	Seen  []Seen
}

// Who is the observer: whose moment it is, for a rule.
func (s Sighting) Who() uid.UID64 { return s.Self }

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
	plugin.Marks
}
