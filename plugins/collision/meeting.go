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

// Who is the entity that met the other: whose moment it is, for a rule.
func (m Meeting) Who() uid.UID64 { return m.Self }

// Whom tells each the other one.
func (m Meeting) Whom(each func(uid.UID64)) { each(m.Other) }
