package bullet

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

// Landing is what a rule hosted here is told once, as a shot's flight ends: where, and why — it
// Struck an entity (Other, collision's contact), it hit a Wall of the ground (Cell), it came down
// (Grounded: a thrown one, or its Range flown), it stopped at a closed edge, or it Left by an open
// one (world.Outside follows). Its Subject is the entity struck.
type Landing struct {
	ID       uid.UID64
	At       geom.Vec
	Struck   bool
	Other    uid.UID64
	Wall     bool
	Cell     uint64
	Grounded bool
	Left     bool
}

// Who is the shot landing: whose moment it is, for a rule.
func (l Landing) Who() uid.UID64 { return l.ID }

// Subject is the entity struck, whom an Aimed command given on the moment is about and a rule's
// ForOther acts on; false for a flight ending on nobody.
func (l Landing) Subject() (uid.UID64, bool) { return l.Other, l.Struck }

// Resting is what a rule hosted here is told every step a landed shot lies where it ended.
type Resting struct {
	ID uid.UID64
	At geom.Vec
}

// Who is the shot resting: whose moment it is, for a rule.
func (r Resting) Who() uid.UID64 { return r.ID }

// Blast is what a rule hosted here is told for each entity within the Radius of a Burst: the
// shot bursting (Self), the one reached (Other) and how far off its centre is.
type Blast struct {
	Self, Other uid.UID64
	Distance    float64
}

// Who is the shot bursting: whose moment it is, for a rule.
func (b Blast) Who() uid.UID64 { return b.Self }

// Whom tells each the one reached.
func (b Blast) Whom(each func(uid.UID64)) { each(b.Other) }
