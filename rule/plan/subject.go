package plan

import (
	"time"

	"github.com/kjkrol/gram/rule/act"
)

// Entity is the entity the plan is about, with the steps that last besides a rule's.
type Entity struct{ act.Subject }

var (
	// Self is the entity the plan is about.
	Self = Entity{act.Self}
	// Other is the subject of the fact the loop stands on; a step of it fails while there is
	// none.
	Other = act.Other
)

// Around is a rule's; a plan has no places.
var Around = act.Around

// Wait is the entity doing nothing for d, then done well.
func (Entity) Wait(d time.Duration) Part { return part{act.NewWait(d)} }

// Idle is the entity doing nothing for ever: a loop that only waits for its Until, or the
// last part of a OneOf.
func (Entity) Idle() Part { return part{act.NewIdle()} }
