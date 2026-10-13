package plugin

import (
	"errors"

	"github.com/kjkrol/uid"
)

// About is a moment of one entity, whom a rule's steps act for.
type About interface{ Who() uid.UID64 }

// Met is a moment of one entity with others, which PairRules matches in pairs.
type Met interface {
	About
	Whom(each func(uid.UID64))
}

// Placed is a moment of an entity standing on places that are entities of their own, which
// Tick.Around tells and act.Around turns a step on.
type Placed interface {
	About
	Placed()
}

// Subject is a moment or a fact about another entity, whom an Aimed command given on it is
// about; false for nobody just now, when such a command fails.
type Subject interface{ Subject() (uid.UID64, bool) }

// Aimed is a command told the Subject of the moment or the fact it is given on as it is issued.
type Aimed interface{ Aim(who uid.UID64) }

// Hosted is a rule that builds itself for its host, as an act.Branch does.
type Hosted interface{ Hosted() any }

func hosted(r any) any {
	if h, ok := r.(Hosted); ok {
		return h.Hosted()
	}
	return r
}

// ErrUnhosted is what a Host reports for a rule of a moment it does not catch.
var ErrUnhosted = errors.New("plugin: rule cannot be hosted here")

// ErrHostBuilt is what a Host reports for a rule handed over once the system running it is built.
var ErrHostBuilt = errors.New("plugin: rule handed over after its system was built")
