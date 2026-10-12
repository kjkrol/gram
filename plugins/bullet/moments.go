package bullet

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// Landing is the moment a shot's flight ends: where, and why — Struck an entity (Other), hit a
// Wall of the ground (Cell), Grounded (come down, or its Range flown) or Left by an open edge.
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

// Who is the shot landing.
func (l Landing) Who() uid.UID64 { return l.ID }

// Subject is the entity struck; false for a flight ending on nobody.
func (l Landing) Subject() (uid.UID64, bool) { return l.Other, l.Struck }

// Resting is the moment told every step a landed shot lies where it ended.
type Resting struct {
	ID uid.UID64
	At geom.Vec
}

// Who is the shot resting.
func (r Resting) Who() uid.UID64 { return r.ID }

// Blast is the moment told for each entity within the Radius of a Burst: the shot bursting
// (Self), the one reached (Other) and how far off its centre is.
type Blast struct {
	Self, Other uid.UID64
	Distance    float64
}

// Who is the shot bursting.
func (b Blast) Who() uid.UID64 { return b.Self }

// Whom tells each the one reached.
func (b Blast) Whom(each func(uid.UID64)) { each(b.Other) }

// Subject is the shot bursting, whom an Aimed command given on the moment is about.
func (b Blast) Subject() (uid.UID64, bool) { return b.Self, true }

// Reaching is the condition that the one reached plays role, for a rule's When on a Blast.
func Reaching(role *rule.Part) rule.Cond { return rule.Other(role.Tag()) }

// Within is the condition that the one reached is within d of the shot, for a rule's When on a
// Blast.
func Within(d float64) func(Blast) bool { return func(b Blast) bool { return b.Distance < d } }
