package vision

import (
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/fact"
	"github.com/kjkrol/ceremony/host"
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

// SeesNobody is the word of an observer with nobody in view.
func SeesNobody() cond.Word[Sighting] { return cond.NewWord("sees nobody", Sighting.nobody) }

func (s Sighting) nobody() bool { return len(s.Seen) == 0 }

// Closing is the word of the nearest one seen being on a collision course with the observer,
// of the other.
func Closing() cond.Word[Sighting] { return cond.NewWord("closing", Sighting.closing).OfOther() }

// cos of the widest angle at which one still counts as heading at the other
const onCourse = 0.5

func (s Sighting) closing() bool {
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
	return mine.X*towards.X+mine.Y*towards.Y > onCourse ||
		theirs.X*towards.X+theirs.Y*towards.Y < -onCourse
}

// Seen is one entity in an observer's view and how far off it is.
type Seen struct {
	ID   uid.UID64
	Base *world.Base
	Dist float32
}

var (
	_ host.Met     = Sighting{}
	_ fact.Subject = Sighting{}
)
