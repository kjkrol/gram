package plugin

import (
	"errors"

	"github.com/kjkrol/uid"
)

// About is a moment of one entity: whose it is. A rule-driven system's moment — a unit.Standing,
// a vision.Sighting, a collision.Struck, a clock.Moment (the clock's own entity) — is one; on a
// moment of no entity a rule's steps acting on one fail.
type About interface{ Who() uid.UID64 }

// Met is a moment of one entity with others — whom it saw, whom it struck: PairRules matches it
// in pairs, and a rule's ForOther turns a step on the others.
type Met interface {
	About
	Whom(each func(uid.UID64))
}

// Placed is a moment of an entity standing somewhere, on places that are entities of their own —
// a board's cells — whose system tells, in the Tick, which places lie round it (Tick.Around): a
// rule's Here and Around turn a step on them. The moment is data alone.
type Placed interface {
	About
	Placed()
}

// Subject is a moment or a fact about another entity — whom it was struck by, who asked, whom it
// touched: an Aimed command given on it is about that entity; false for nobody just now, and an
// Aimed command given then fails.
type Subject interface{ Subject() (uid.UID64, bool) }

// Aimed is a command told whom it is about — the subject of the moment or the fact it is given
// on — as it is issued: whose way to step off, whose goal to swap with.
type Aimed interface{ Aim(who uid.UID64) }

// ErrUnhosted is what a plugin's Hook reports for a rule of a moment it does not catch.
var ErrUnhosted = errors.New("plugin: rule cannot be hosted here")

// ErrHostBuilt is what hooking a rule reports once the system running it is built.
var ErrHostBuilt = errors.New("plugin: rule hooked after its system was built")
