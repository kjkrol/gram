package effect

import (
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

// States is the family of an entity's effect markers: states switched by a bit, never by a
// component put on or taken off. The world gives it to every unit (comp.Marks), the board to every
// cell; an entity without it gets it the first time it is needed.
type States struct{}

// Idle is on for the one step after an entity's last effect ended, so anything watching for it
// sees it whatever the order of the plugins' passes.
const Idle tag.Tag[States] = 0

// IdleName is the name the world defines Idle under, as the saves know it.
const IdleName = "effect.idle"

// Idling is what a rule hosted by effects gets, once, for an entity whose Idle is on.
type Idling struct{ ID uid.UID64 }

// Who is the entity idle: whose moment it is, for a rule.
func (i Idling) Who() uid.UID64 { return i.ID }
