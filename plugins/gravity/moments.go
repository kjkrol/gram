package gravity

import "github.com/kjkrol/uid"

// Landing is what a rule hosted here is told once as a body comes down on what bears it: Speed is
// how fast it fell, in world units a second — a rule's If says how hard is a fall.
type Landing struct {
	ID    uid.UID64
	Speed float64
}

// Who is the body landing: whose moment it is, for a rule.
func (l Landing) Who() uid.UID64 { return l.ID }
