package bullet

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/fact"
	"github.com/kjkrol/ceremony/host"
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

// Within is the word of the one reached being within d of the shot, of the other.
func Within(d float64) cond.Word[Blast] {
	return cond.NewWord("within", func(b Blast) bool { return b.Distance < d }).OfOther()
}

var (
	_ host.About   = Resting{}
	_ host.Met     = Blast{}
	_ fact.Subject = Blast{}
	_ host.About   = Landing{}
	_ fact.Subject = Landing{}
)
