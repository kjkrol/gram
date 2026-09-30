package collision

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

// Meeting is one confirmed contact as seen from Self: who it met, the impulse exchanged
// (zero when only detected), and the way Self left Other.
type Meeting struct {
	Self, Other uid.UID64
	Impact      float64
	Normal      geom.Vec
}

// Who is the entity that met the other: whose moment it is, for a trigger.
func (m Meeting) Who() uid.UID64 { return m.Self }

// Whom tells each the other one.
func (m Meeting) Whom(each func(uid.UID64)) { each(m.Other) }

// Struck is what a trigger hosted here is told about an entity:
// which it is, and what it struck the tick before.
type Struck struct {
	ID       uid.UID64
	Contacts []Contact
}

// Who is the entity that struck: whose moment it is, for a trigger.
func (s Struck) Who() uid.UID64 { return s.ID }

// Hit reports whether the entity struck anything the tick before.
func (s Struck) Hit() bool { return len(s.Contacts) > 0 }
