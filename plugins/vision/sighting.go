package vision

import (
	"github.com/kjkrol/gram/rule"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
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

// Who is the observer.
func (s Sighting) Who() uid.UID64 { return s.Self }

// Subject is the nearest one seen, whom an Aimed command is about; false while none is in view.
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

// Nobody reports that none the rule asks about is in view: a condition for a rule's When.
func (s Sighting) Nobody() bool { return len(s.Seen) == 0 }

// Seeing is the condition that the observer sees one playing role, the Sighting then of those
// alone: a rule's When.
func Seeing(role *rule.Part) rule.Cond { return rule.Paired(role) }

// cos of the widest angle at which one still counts as heading at the other
const onCourse = 0.5

// Closing reports that the observer and the nearest one it sees are on a collision course: a
// condition for a rule's When, false with none in view.
func (s Sighting) Closing() bool {
	if len(s.Seen) == 0 {
		return false
	}
	towards := s.Seen[0].Base.Pos.Center().Sub(s.Base.Pos.Center())
	d := math.Hypot(towards.X, towards.Y)
	if d == 0 {
		return false
	}
	towards = geom.NewVec(towards.X/d, towards.Y/d)
	mine, theirs := s.Base.Vel.Dir, s.Seen[0].Base.Vel.Dir
	return mine.X*towards.X+mine.Y*towards.Y > onCourse || theirs.X*towards.X+theirs.Y*towards.Y < -onCourse
}

// Seen is one entity in an observer's view and how far off it is.
type Seen struct {
	ID   uid.UID64
	Base *world.Base
	Dist float32
}
