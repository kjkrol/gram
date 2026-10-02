package navigation

import (
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/uid"
)

// States is the family of a unit's navigation markers: states switched by a bit, never by a
// component put on or taken off. The plugin gives it to every unit the world's roster makes
// (comp.Marks); a unit without it gets it the first time it is needed.
type States struct{}

// Entered is on for the step a unit's At changed: unit.At says which cell it entered.
const Entered tag.Tag[States] = 0

// enteredName is the name Entered is defined under, as the saves know it.
const enteredName = "navigation.entered"

// enter has the unit at row i of states have Entered on for this step; a unit whose chunk has no
// family is noted in lacking, to get it once the chunk's own changes are queued.
func (s *navigationSystem) enter(states []tag.Tags[States], i int, id uid.UID64) {
	if states != nil {
		states[i] = states[i].With(Entered)
	} else {
		s.lacking = append(s.lacking, id)
	}
	s.entered++
}
